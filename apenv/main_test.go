package main

import (
	"bytes"
	"os"
	"reflect"
	"testing"
)

func TestParseEnvText(t *testing.T) {
	vars, err := parseEnvText("# comment\nFOO=bar\n  # another comment\nBAR=baz=qux\n\nBAZ=\n")
	if err != nil {
		t.Fatal(err)
	}

	want := []envVar{
		{Key: "FOO", Value: "bar"},
		{Key: "BAR", Value: "baz=qux"},
		{Key: "BAZ", Value: ""},
	}

	if !reflect.DeepEqual(vars, want) {
		t.Fatalf("got %#v want %#v", vars, want)
	}
}

func TestParseEnvTextRejectsInvalidLine(t *testing.T) {
	if _, err := parseEnvText("NOPE\n"); err == nil {
		t.Fatal("expected error")
	}
}

func TestMergeEnv(t *testing.T) {
	got := mergeEnv([]string{"A=1", "B=2"}, []envVar{{Key: "B", Value: "3"}, {Key: "C", Value: "4"}})
	want := []string{"A=1", "B=3", "C=4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestShellQuote(t *testing.T) {
	got := shellQuote("a 'quoted' value")
	want := `'a '"'"'quoted'"'"' value'`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestListProjects(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir, err := projectDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/one.env.enc", []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/two.env.enc", []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/ignore.txt", []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := listProjects(&out); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	if got != "one\ntwo\n" {
		t.Fatalf("got %q", got)
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key, _, err := generateKey()
	if err != nil {
		t.Fatal(err)
	}

	ciphertext, err := encrypt([]byte("FOO=bar\n"), key)
	if err != nil {
		t.Fatal(err)
	}

	plaintext, err := decrypt(ciphertext, key)
	if err != nil {
		t.Fatal(err)
	}

	if string(plaintext) != "FOO=bar\n" {
		t.Fatalf("got %q", plaintext)
	}
}
