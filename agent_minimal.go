package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	history := []Message{}
	scanner := bufio.NewScanner(os.Stdin)

	workDir, _ := os.Getwd()
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
		history = append(history, Message{Role: "user", Content: input})

		for {
			content, toolCalls, err := complete(history)
			if err != nil {
				fmt.Println("Error:", err)
				break
			}
			fmt.Println(content)
			if len(toolCalls) == 0 {
				history = append(history, Message{Role: "assistant", Content: content})
				break
			}
			history = append(history, Message{Role: "assistant", Content: content, ToolCalls: toolCalls})

			for _, tc := range toolCalls {
				fmt.Printf("\n[Calling %s: %s]\n", tc.Function.Name, tc.Function.Arguments)
				result := executeTool(tc.Function.Name, tc.Function.Arguments)
				fmt.Printf("[Result: %s]\n\n", result[:int(math.Min(float64(len(result)), 200))])
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
	Content    string     `json:"content,omitempty"`
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

var tools = []Tool{
	makeTool("read", "Read contents of a file", "path:File path"),
	makeTool("edit", "Edit a file by replacing exact text with new text", "path:File path", "old_text:Exact text to find and replace", "new_text:Replacement text"),
	makeTool("write", "Write contents to a file", "path:File path", "content:Content to write"),
	makeTool("bash", "Execute a shell command", "command:Shell command to execute"),
}

func makeTool(name, desc string, params ...string) Tool {
	t := Tool{Type: "function"}
	t.Function.Name = name
	t.Function.Description = desc
	required := []string{}
	properties := map[string]interface{}{}
	for _, p := range params {
		parts := strings.SplitN(p, ":", 2)
		properties[parts[0]] = map[string]interface{}{"type": "string", "description": parts[1]}
		required = append(required, parts[0])
	}
	t.Function.Parameters = map[string]interface{}{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}
	return t
}

func executeTool(name, argsStr string) string {
	var args map[string]string
	json.Unmarshal([]byte(argsStr), &args)
	handleErr := func(err error, msg string) string {
		if err != nil {
			return "Error: " + err.Error()
		}
		return msg
	}
	switch name {
	case "read":
		data, err := os.ReadFile(args["path"])
		return handleErr(err, string(data))
	case "write":
		err := os.WriteFile(args["path"], []byte(args["content"]), 0644)
		return handleErr(err, "File written successfully")
	case "edit":
		data, err := os.ReadFile(args["path"])
		if err != nil {
			return "Error: " + err.Error()
		}
		content := string(data)
		count := strings.Count(content, args["old_text"])
		if count == 0 {
			return "Error: old_text not found in file"
		} else if count > 1 {
			return fmt.Sprintf("Error: old_text found %d times, must be unique", count)
		}
		newContent := strings.Replace(content, args["old_text"], args["new_text"], 1)
		err = os.WriteFile(args["path"], []byte(newContent), 0644)
		return handleErr(err, "File edited successfully")
	case "bash":
		cmd := exec.Command("bash", "-c", args["command"])
		out, err := cmd.CombinedOutput()
		if err != nil {
			return string(out) + "\nError: " + err.Error()
		}
		return string(out)
	}
	return "Unknown tool"
}

func complete(messages []Message) (string, []ToolCall, error) {
	reqBody := map[string]interface{}{"model": "grok-code-fast-1", "messages": messages, "tools": tools}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "https://api.x.ai/v1/chat/completions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+os.Getenv("XAI_API_KEY"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}
	r := struct {
		Choices []struct {
			Message struct {
				Content   string     `json:"content"`
				ToolCalls []ToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}{}
	err = json.Unmarshal(body, &r)
	if err != nil {
		panic(err)
	}
	return r.Choices[0].Message.Content, r.Choices[0].Message.ToolCalls, nil
}
