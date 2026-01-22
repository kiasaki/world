package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	history := []Message{}
	scanner := bufio.NewScanner(os.Stdin)

	workDir, err := os.Getwd()
	if err != nil {
		workDir = "."
	}
	projectContext := ""
	if bs, err := os.ReadFile("AGENTS.md"); err == nil {
		projectContext = "The following project context files have been loaded:\n\n## AGENTS.md\n\n" + string(bs) + "\n"
	}
	systemPrompt := fmt.Sprintf("You are a helpful coding assistant. Use tools when needed to help the user.\n\nAvailable tools:\n- read\n- bash\n- edit\n- write\n\nGuidelines:\n\n- Use read to examine files before editing. You must use this tool instead of cat or sed.\n- Use edit for precise changes (old text must match exactly).\n- Use write only for new files or complete rewrites\n- When summarizing your actions, output plain text directly - do NOT use cat or bash to display what you did.\n- Be concise in your responses.\n- Show file paths clearly when working with files.\n\n%s\nCurrent date and time: %s\nCurrent working directory: %s", projectContext, time.Now().Format("2006-01-02 15:04"), workDir)
	history = append(history, Message{Role: "system", Content: systemPrompt})

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

		for {
			content, toolCalls, err := callAPI(history)
			if err != nil {
				fmt.Println("Error:", err)
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
					fmt.Printf("\nREAD %v\n", args["path"])
				} else if tc.Function.Name == "edit" {
					fmt.Printf("\nEDIT %v (%d)\n", args["path"], len(args["new_text"].(string)))
				} else if tc.Function.Name == "write" {
					fmt.Printf("\nWRITE %v (%d)\n", args["path"], len(args["content"].(string)))
				} else if tc.Function.Name == "bash" {
					fmt.Printf("\nBASH %v\n", args["command"])
				} else {
					fmt.Printf("\n%s: %s\n", strings.ToUpper(tc.Function.Name), tc.Function.Arguments)
				}
				result := executeTool(tc.Function.Name, tc.Function.Arguments, workDir)
				//fmt.Printf("[Result: %s]\n\n", truncate(result, 200))
				history = append(history, Message{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    result,
				})
			}
		}
	}
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
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Tools    []Tool    `json:"tools"`
	Stream   bool      `json:"stream"`
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
	makeTool("read", "Read contents of a file", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path":   map[string]interface{}{"type": "string", "description": "File path"},
			"offset": map[string]interface{}{"type": "integer", "description": "Line number to start reading from (1-indexed, optional)"},
			"limit":  map[string]interface{}{"type": "integer", "description": "Maximum number of lines to read (optional)"},
		},
		"required": []string{"path"},
	}),
	makeTool("edit", "Edit a file by replacing exact text with new text", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path":     map[string]interface{}{"type": "string", "description": "File path"},
			"old_text": map[string]interface{}{"type": "string", "description": "Exact text to find and replace"},
			"new_text": map[string]interface{}{"type": "string", "description": "Replacement text"},
		},
		"required": []string{"path", "old_text", "new_text"},
	}),
	makeTool("write", "Write contents to a file", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path":    map[string]interface{}{"type": "string", "description": "File path"},
			"content": map[string]interface{}{"type": "string", "description": "Content to write"},
		},
		"required": []string{"path", "content"},
	}),
	makeTool("bash", "Execute a shell command", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{"type": "string", "description": "Shell command to execute"},
		},
		"required": []string{"command"},
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
	}
	return "Unknown tool"
}

func callAPI(messages []Message) (string, []ToolCall, error) {
	apiKey := os.Getenv("XAI_API_KEY")
	if apiKey == "" {
		return "", nil, fmt.Errorf("XAI_API_KEY environment variable not set")
	}

	reqBody := Request{Model: "grok-code-fast-1", Messages: messages, Tools: tools, Stream: true}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "https://api.x.ai/v1/chat/completions", bytes.NewBuffer(jsonBody))
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
			fmt.Print(delta.Content)
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
		fmt.Println()
	}
	return content.String(), toolCalls, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
