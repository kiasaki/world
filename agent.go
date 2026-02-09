package main

// env vars
// export OPENROUTER_API_KEY="sk-or-v1-..."
// export KAGENT_MODEL="qwen/qwen3-coder-next"
// export KAGENT_MODEL="moonshotai/kimi-k2.5"
// export KAGENT_MODEL="openai/gpt-5.2-codex"
// export KAGENT_MODEL="anthropic/claude-opus-4.6"

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

func main() {
	history := []Message{}

	workDir, err := os.Getwd()
	if err != nil {
		workDir = "."
	}
	projectContext := ""
	if bs, err := os.ReadFile("AGENTS.md"); err == nil {
		projectContext = "The following project context files have been loaded:\n\n## AGENTS.md\n\n" + string(bs) + "\n"
	}
	systemPrompt := fmt.Sprintf("You are an expert coding assistant. You help users with coding tasks by reading files, executing commands, editing code, and writing new files.\n\nAvailable tools:\n- read: Read file contents\n- bash: Execute bash commands\n- edit: Make surgical edits to files\n- write: Create or overwrite files\n- web_search: search for websites\n- web_fetch: fetch a web page\n\nGuidelines:\n\n- Use bash for file operations like ls, rg, find\n- Use read to examine files before editing. You must use this tool instead of cat or sed\n- Use edit for precise changes (old text must match exactly)\n- Use write only for new files or complete rewrites\n- When summarizing your actions, output plain text directly - do NOT use cat or bash to display what you did\n- Be concise in your responses\n- Show file paths clearly when working with files\n\n%s\nCurrent date and time: %s\nCurrent working directory: %s", projectContext, time.Now().Format("2006-01-02 15:04"), workDir)
	history = append(history, Message{Role: "system", Content: systemPrompt})

	exitAfterFirstTurn := false
	for _, a := range os.Args[1:] {
		if a == "-p" {
			exitAfterFirstTurn = true
		} else {
			history = append(history, Message{Role: "user", Content: a})
		}
	}
	if exitAfterFirstTurn {
		history = complete(history, workDir, noopOutput)
		fmt.Println(history[len(history)-1].Content)
		return
	}
	if len(history) > 1 {
		history = complete(history, workDir, stdOutput)
	}

	run(history, workDir)
}

func run(history []Message, workDir string) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("ERROR: ", err)
			run(history, workDir)
		}
	}()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\n> ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		history = append(history, Message{Role: "user", Content: input})
		history = complete(history, workDir, stdOutput)
	}
}

func stdOutput(f string, v ...any) {
	fmt.Printf(f, v...)
}

func noopOutput(f string, v ...any) {
}

func complete(history []Message, workDir string, output func(string, ...any)) []Message {
	for {
		content, toolCalls, err := callAPI(history, output)
		if os.Getenv("DEBUG") == "1" {
			fmt.Printf("%#v %#v\n", content, toolCalls)
		}
		if err != nil {
			output("Error: %v", err)
			break
		}
		if len(toolCalls) == 0 {
			if content != "" {
				history = append(history, Message{Role: "assistant", Content: content})
			}
			break
		}

		assistantMsg := Message{Role: "assistant", ToolCalls: toolCalls}
		if content != "" {
			assistantMsg.Content = content
		}
		history = append(history, assistantMsg)

		for _, tc := range toolCalls {
			args := map[string]interface{}{}
			json.Unmarshal([]byte(tc.Function.Arguments), &args)
			if tc.Function.Name == "read" {
				output("READ %v\n", args["path"])
			} else if tc.Function.Name == "edit" {
				output("EDIT %v (%d)\n", args["path"], len(args["new_text"].(string)))
			} else if tc.Function.Name == "write" {
				output("WRITE %v (%d)\n", args["path"], len(args["content"].(string)))
			} else if tc.Function.Name == "bash" {
				output("BASH %v\n", args["command"])
			} else if tc.Function.Name == "web_search" {
				output("WEB SEARCH %v\n", args["query"])
			} else if tc.Function.Name == "web_fetch" {
				output("WEB FETCH %v\n", args["url"])
			} else {
				output("%s: %s\n", strings.ToUpper(tc.Function.Name), tc.Function.Arguments)
			}
			result := executeTool(tc.Function.Name, tc.Function.Arguments, workDir)
			history = append(history, Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    result,
			})
		}
	}

	return history
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type Tool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Parameters  map[string]interface{} `json:"parameters"`
	} `json:"function"`
}

type Request struct {
	Model     string          `json:"model"`
	Messages  []Message       `json:"messages"`
	Tools     []Tool          `json:"tools"`
	Stream    bool            `json:"stream"`
	Reasoning RequestResoning `json:"reasoning"`
}

type RequestResoning struct {
	Effort string `json:"effort"`
}

type StreamChoice struct {
	Delta struct {
		Content   string     `json:"content"`
		ToolCalls []ToolCall `json:"tool_calls"`
	} `json:"delta"`
	FinishReason *string `json:"finish_reason"`
}

type StreamResponse struct {
	Choices []StreamChoice `json:"choices"`
}

var tools = []Tool{
	makeTool("read", "Read file contents. Use offset/limit for large files.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path":   map[string]interface{}{"type": "string", "description": "Path to the file to read (relative or absolute)"},
			"offset": map[string]interface{}{"type": "integer", "description": "Line number to start reading from (1-indexed)"},
			"limit":  map[string]interface{}{"type": "integer", "description": "Maximum number of lines to read"},
		},
		"required": []string{"path"},
	}),
	makeTool("write", "Write content to a file. Creates the file if it doesn't exist, overwrites if it does. Automatically creates parent directories.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path":    map[string]interface{}{"type": "string", "description": "Path to the file to write (relative or absolute)"},
			"content": map[string]interface{}{"type": "string", "description": "Content to write to the file"},
		},
		"required": []string{"path", "content"},
	}),
	makeTool("edit", "Make surgical edits to files (find exact text and replace)", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path":     map[string]interface{}{"type": "string", "description": "Path to the file to edit (relative or absolute)"},
			"old_text": map[string]interface{}{"type": "string", "description": "Exact text to find and replace (must match exactly)"},
			"new_text": map[string]interface{}{"type": "string", "description": "New text to replace the old text with"},
		},
		"required": []string{"path", "old_text", "new_text"},
	}),
	makeTool("bash", "Execute bash commands (ls, grep, find, etc.)", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{"type": "string", "description": "Execute a bash command in the current working directory. Returns stdout and stderr"},
		},
		"required": []string{"command"},
	}),
	makeTool("web_search", "Perform a web search using Brave Search API", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{"type": "string", "description": "Search query"},
		},
		"required": []string{"query"},
	}),
	makeTool("web_fetch", "Fetch the content of a web page and strip HTML", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{"type": "string", "description": "URL to fetch"},
		},
		"required": []string{"url"},
	}),
}

func makeTool(name, desc string, params map[string]interface{}) Tool {
	t := Tool{Type: "function"}
	t.Function.Name = name
	t.Function.Description = desc
	t.Function.Parameters = params
	return t
}

func executeTool(name, argsJSON, workDir string) string {
	var args map[string]interface{}
	json.Unmarshal([]byte(argsJSON), &args)

	getString := func(key string) string {
		if v, ok := args[key]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
		return ""
	}

	getInt := func(key string) int {
		if v, ok := args[key]; ok {
			switch n := v.(type) {
			case float64:
				return int(n)
			case int:
				return n
			}
		}
		return 0
	}

	resolvePath := func(p string) string {
		if strings.HasPrefix(p, "/") {
			return p
		}
		return workDir + "/" + p
	}

	switch name {
	case "read":
		path := resolvePath(getString("path"))
		data, err := os.ReadFile(path)
		if err != nil {
			return "Error: " + err.Error()
		}
		lines := strings.Split(string(data), "\n")
		offset := getInt("offset")
		limit := getInt("limit")
		if offset > 0 {
			offset-- // convert to 0-indexed
			if offset >= len(lines) {
				return ""
			}
			lines = lines[offset:]
		}
		if limit > 0 && limit < len(lines) {
			lines = lines[:limit]
		}
		return strings.Join(lines, "\n")

	case "write":
		path := resolvePath(getString("path"))
		err := os.WriteFile(path, []byte(getString("content")), 0644)
		if err != nil {
			return "Error: " + err.Error()
		}
		return "File written successfully"

	case "edit":
		path := resolvePath(getString("path"))
		data, err := os.ReadFile(path)
		if err != nil {
			return "Error: " + err.Error()
		}
		content := string(data)
		oldText := getString("old_text")
		newText := getString("new_text")
		if !strings.Contains(content, oldText) {
			return "Error: old_text not found in file"
		}
		count := strings.Count(content, oldText)
		if count > 1 {
			return fmt.Sprintf("Error: old_text found %d times, must be unique", count)
		}
		newContent := strings.Replace(content, oldText, newText, 1)
		err = os.WriteFile(path, []byte(newContent), 0644)
		if err != nil {
			return "Error: " + err.Error()
		}
		return "File edited successfully"

	case "bash":
		cmd := exec.Command("bash", "-c", getString("command"))
		cmd.Dir = workDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return string(out) + "\nError: " + err.Error()
		}
		return string(out)

	case "web_search":
		braveKey := os.Getenv("BRAVE_API_KEY")
		if braveKey == "" {
			return "Error: BRAVE_API_KEY environment variable not set"
		}
		query := getString("query")
		searchURL := "https://api.search.brave.com/res/v1/web/search?q=" + url.QueryEscape(query)
		req, err := http.NewRequest("GET", searchURL, nil)
		if err != nil {
			return "Error: " + err.Error()
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-Subscription-Token", braveKey)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "Error: " + err.Error()
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Sprintf("Error: %d %s", resp.StatusCode, string(body))
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "Error: " + err.Error()
		}
		var response map[string]interface{}
		if json.Unmarshal(body, &response) != nil {
			return string(body)
		}
		if web, ok := response["web"]; ok {
			if webMap, ok := web.(map[string]interface{}); ok {
				if results, ok := webMap["results"]; ok {
					if resSlice, ok := results.([]interface{}); ok {
						var output strings.Builder
						for _, r := range resSlice {
							if rMap, ok := r.(map[string]interface{}); ok {
								title, _ := rMap["title"].(string)
								url, _ := rMap["url"].(string)
								desc, _ := rMap["description"].(string)
								output.WriteString(fmt.Sprintf("Title: %s\nURL: %s\nDescription: %s\n\n", title, url, desc))
							}
						}
						return output.String()
					}
				}
			}
		}
		return string(body)

	case "web_fetch":
		fetchURL := getString("url")
		req, err := http.NewRequest("GET", fetchURL, nil)
		if err != nil {
			return "Error: " + err.Error()
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "Error: " + err.Error()
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return fmt.Sprintf("Error: %d", resp.StatusCode)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "Error: " + err.Error()
		}
		content := string(body)
		content = regexp.MustCompile(`<style([\s\S]+?)</style>`).ReplaceAllString(content, "")
		content = regexp.MustCompile(`<script([\s\S]+?)</script>`).ReplaceAllString(content, "")
		content = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(content, "")
		content = regexp.MustCompile(`\s+`).ReplaceAllString(content, " ")
		return strings.TrimSpace(content)
	}
	return "Unknown tool"
}

func callAPI(messages []Message, output func(string, ...any)) (string, []ToolCall, error) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return "", nil, fmt.Errorf("OPENROUTER_API_KEY environment variable not set")
	}

	reqBody := Request{Model: or(os.Getenv("KAGENT_MODEL"), "anthropic/claude-opus-4.6"), Messages: messages, Tools: tools, Stream: true, Reasoning: RequestResoning{Effort: "high"}}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	reader := bufio.NewReader(resp.Body)
	var content strings.Builder
	var toolCalls []ToolCall
	toolCallArgs := make(map[int]*strings.Builder)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var sr StreamResponse
		if json.Unmarshal([]byte(data), &sr) != nil || len(sr.Choices) == 0 {
			continue
		}

		delta := sr.Choices[0].Delta
		if delta.Content != "" {
			output("%s", delta.Content)
			content.WriteString(delta.Content)
		}

		for _, tc := range delta.ToolCalls {
			idx := len(toolCalls)
			if tc.ID != "" {
				toolCalls = append(toolCalls, tc)
				toolCallArgs[idx] = &strings.Builder{}
			}
			if idx > 0 && tc.Function.Arguments != "" {
				toolCallArgs[idx-1].WriteString(tc.Function.Arguments)
			}
		}
	}

	for i, tc := range toolCalls {
		if builder, ok := toolCallArgs[i]; ok {
			toolCalls[i].Function.Arguments = tc.Function.Arguments + builder.String()
		}
	}

	if content.Len() > 0 {
		output("\n")
	}
	return content.String(), toolCalls, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
