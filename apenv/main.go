package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

const serviceName = "apenv"

var projectNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
var envKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type envVar struct {
	Key   string
	Value string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	editMode := flag.Bool("e", false, "edit a project's env vars")
	printMode := flag.Bool("p", false, "print a project's decrypted env vars")
	shellMode := flag.Bool("s", false, "print a project's env vars as shell exports")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage:\n  apenv -e <project>\n  apenv -p <project>\n  apenv -s <project>\n  apenv ls\n  apenv <project> <command> [args...]\n")
	}
	flag.Parse()

	modes := 0
	if *editMode {
		modes++
	}
	if *printMode {
		modes++
	}
	if *shellMode {
		modes++
	}
	if modes > 1 {
		flag.Usage()
		return errors.New("-e, -p, and -s cannot be used together")
	}

	if *editMode {
		if flag.NArg() != 1 {
			flag.Usage()
			return errors.New("expected exactly one project name")
		}
		return editProject(flag.Arg(0))
	}

	if *printMode {
		if flag.NArg() != 1 {
			flag.Usage()
			return errors.New("expected exactly one project name")
		}
		return printProject(flag.Arg(0))
	}

	if *shellMode {
		if flag.NArg() != 1 {
			flag.Usage()
			return errors.New("expected exactly one project name")
		}
		return printProjectShell(flag.Arg(0))
	}

	if flag.NArg() == 1 && flag.Arg(0) == "ls" {
		return listProjects(os.Stdout)
	}

	if flag.NArg() < 2 {
		flag.Usage()
		return errors.New("expected a project name and command")
	}

	project := flag.Arg(0)
	command := flag.Args()[1:]
	return execProject(project, command)
}

func editProject(project string) error {
	if err := validateProjectName(project); err != nil {
		return err
	}

	content, err := readProject(project)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		content = ""
	}

	if _, err := parseEnvText(content); err != nil {
		return err
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		return errors.New("EDITOR is not set")
	}

	dir, err := os.MkdirTemp("", "apenv-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, project+".env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return err
	}

	if err := runEditor(editor, path); err != nil {
		return err
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if _, err := parseEnvText(string(updated)); err != nil {
		return err
	}

	return writeProject(project, updated)
}

func execProject(project string, args []string) error {
	if err := validateProjectName(project); err != nil {
		return err
	}

	content, err := readProject(project)
	if err != nil {
		return err
	}

	vars, err := parseEnvText(content)
	if err != nil {
		return err
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = mergeEnv(os.Environ(), vars)
	return cmd.Run()
}

func printProject(project string) error {
	if err := validateProjectName(project); err != nil {
		return err
	}

	content, err := readProject(project)
	if err != nil {
		return err
	}

	if _, err := parseEnvText(content); err != nil {
		return err
	}

	_, err = fmt.Print(content)
	return err
}

func printProjectShell(project string) error {
	if err := validateProjectName(project); err != nil {
		return err
	}

	content, err := readProject(project)
	if err != nil {
		return err
	}

	vars, err := parseEnvText(content)
	if err != nil {
		return err
	}

	for _, variable := range vars {
		if _, err := fmt.Printf("export %s=%s\n", variable.Key, shellQuote(variable.Value)); err != nil {
			return err
		}
	}

	return nil
}

func validateProjectName(project string) error {
	if !projectNamePattern.MatchString(project) {
		return fmt.Errorf("invalid project name %q", project)
	}
	return nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func parseEnvText(content string) ([]envVar, error) {
	if content == "" {
		return nil, nil
	}

	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	vars := make([]envVar, 0, len(lines))

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx <= 0 {
			return nil, fmt.Errorf("invalid env line %d", i+1)
		}
		key := strings.TrimSpace(line[:idx])
		value := line[idx+1:]
		if !envKeyPattern.MatchString(key) {
			return nil, fmt.Errorf("invalid env key %q on line %d", key, i+1)
		}
		if strings.ContainsRune(value, '\x00') {
			return nil, fmt.Errorf("invalid env value on line %d", i+1)
		}
		vars = append(vars, envVar{Key: key, Value: value})
	}

	return vars, nil
}

func mergeEnv(base []string, vars []envVar) []string {
	index := make(map[string]int, len(base))
	merged := append([]string(nil), base...)

	for i, entry := range merged {
		key, _, ok := strings.Cut(entry, "=")
		if ok {
			index[key] = i
		}
	}

	for _, variable := range vars {
		entry := variable.Key + "=" + variable.Value
		if i, ok := index[variable.Key]; ok {
			merged[i] = entry
			continue
		}
		index[variable.Key] = len(merged)
		merged = append(merged, entry)
	}

	return merged
}

func readProject(project string) (string, error) {
	path, err := projectFilePath(project)
	if err != nil {
		return "", err
	}

	ciphertext, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	key, err := loadOrCreateKey()
	if err != nil {
		return "", err
	}

	plaintext, err := decrypt(ciphertext, key)
	if err != nil {
		return "", fmt.Errorf("decrypt %s: %w", project, err)
	}

	return string(plaintext), nil
}

func writeProject(project string, plaintext []byte) error {
	path, err := projectFilePath(project)
	if err != nil {
		return err
	}

	key, err := loadOrCreateKey()
	if err != nil {
		return err
	}

	ciphertext, err := encrypt(plaintext, key)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	return os.WriteFile(path, ciphertext, 0o600)
}

func projectFilePath(project string) (string, error) {
	if err := validateProjectName(project); err != nil {
		return "", err
	}

	dir, err := projectDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, project+".env.enc"), nil
}

func projectDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".config", "apenv", "projects"), nil
}

func listProjects(w io.Writer) error {
	dir, err := projectDir()
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	projects := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".env.enc") {
			continue
		}
		projects = append(projects, strings.TrimSuffix(name, ".env.enc"))
	}

	sort.Strings(projects)
	for _, project := range projects {
		if _, err := fmt.Fprintln(w, project); err != nil {
			return err
		}
	}

	return nil
}

func loadOrCreateKey() ([]byte, error) {
	if runtime.GOOS == "darwin" {
		if _, err := exec.LookPath("security"); err == nil {
			return loadOrCreateMacKey()
		}
	}
	return loadOrCreateFileKey()
}

func loadOrCreateMacKey() ([]byte, error) {
	user := os.Getenv("USER")
	if user == "" {
		user = "default"
	}

	findOutput, findErr := runSecurity("find-generic-password", "-a", user, "-s", serviceName, "-w")
	if findErr == nil {
		return decodeStoredKey(findOutput)
	}
	if !strings.Contains(string(findOutput), "could not be found") {
		return nil, fmt.Errorf("read key from keychain: %v: %s", findErr, strings.TrimSpace(string(findOutput)))
	}

	key, encoded, err := generateKey()
	if err != nil {
		return nil, err
	}

	addOutput, addErr := runSecurity("add-generic-password", "-U", "-a", user, "-s", serviceName, "-w", encoded)
	if addErr != nil {
		return nil, fmt.Errorf("store key in keychain: %v: %s", addErr, strings.TrimSpace(string(addOutput)))
	}

	return key, nil
}

func runSecurity(args ...string) ([]byte, error) {
	output, err := exec.Command("security", args...).CombinedOutput()
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 36 {
		fmt.Fprintln(os.Stderr, "Keychain is locked. Enter your password to unlock.")
		unlock := exec.Command("security", "unlock-keychain")
		unlock.Stdin = os.Stdin
		unlock.Stdout = os.Stdout
		unlock.Stderr = os.Stderr
		if unlockErr := unlock.Run(); unlockErr != nil {
			return output, fmt.Errorf("unlock keychain: %v", unlockErr)
		}
		return exec.Command("security", args...).CombinedOutput()
	}
	return output, err
}

func loadOrCreateFileKey() ([]byte, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(home, ".apenv")
	data, err := os.ReadFile(path)
	if err == nil {
		return decodeStoredKey(data)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	key, encoded, err := generateKey()
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(path, []byte(encoded+"\n"), 0o600); err != nil {
		return nil, err
	}

	return key, nil
}

func runEditor(editor, path string) error {
	cmd := exec.Command("sh", "-c", editor+" \"$1\"", "apenv-editor", path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func generateKey() ([]byte, string, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, "", err
	}
	return key, base64.RawStdEncoding.EncodeToString(key), nil
}

func decodeStoredKey(data []byte) ([]byte, error) {
	encoded := strings.TrimSpace(string(data))
	if encoded == "" {
		return nil, errors.New("stored key is empty")
	}
	key, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("stored key must decode to 32 bytes, got %d", len(key))
	}
	return key, nil
}

func encrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	var out bytes.Buffer
	out.WriteString("apenv1:")
	out.WriteString(base64.RawStdEncoding.EncodeToString(nonce))
	out.WriteByte(':')
	out.WriteString(base64.RawStdEncoding.EncodeToString(ciphertext))
	out.WriteByte('\n')
	return out.Bytes(), nil
}

func decrypt(data, key []byte) ([]byte, error) {
	parts := strings.Split(strings.TrimSpace(string(data)), ":")
	if len(parts) != 3 || parts[0] != "apenv1" {
		return nil, errors.New("invalid ciphertext format")
	}

	nonce, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	ciphertext, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return gcm.Open(nil, nonce, ciphertext, nil)
}
