package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

type rpcClient struct {
	dec *json.Decoder
	enc *json.Encoder
}

type summary struct {
	Command        string         `json:"command"`
	CodexVersion   string         `json:"codexVersion,omitempty"`
	Initialize     string         `json:"initialize"`
	AccountRead    string         `json:"accountRead,omitempty"`
	RateLimitsRead string         `json:"rateLimitsRead,omitempty"`
	UsageRead      string         `json:"usageRead,omitempty"`
	TurnStart      string         `json:"turnStart,omitempty"`
	TurnUsage      map[string]any `json:"turnUsage,omitempty"`
}

func main() {
	var dockerContainer string
	var command string
	var tokenEnv string
	var probeAccount bool
	var model string
	var turnPrompt string
	var timeout time.Duration
	flag.StringVar(&dockerContainer, "docker", "codexppp-backend", "Docker container that has the Codex CLI installed. Use an empty value to run a local command.")
	flag.StringVar(&command, "command", "codex", "Local Codex command when -docker is empty.")
	flag.StringVar(&tokenEnv, "token-env", "CODEXPPP_VERIFY_CODEX_ACCESS_TOKEN", "Environment variable containing a real Codex access token for authenticated checks.")
	flag.BoolVar(&probeAccount, "probe-account", false, "Run account/read even when no Codex access token is configured.")
	flag.StringVar(&model, "model", "", "Optional model for an authenticated turn smoke test.")
	flag.StringVar(&turnPrompt, "turn-prompt", "", "Optional prompt for a real authenticated thread/start and turn/start smoke test.")
	flag.DurationVar(&timeout, "timeout", 90*time.Second, "Overall verification timeout.")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	accessToken := strings.TrimSpace(os.Getenv(tokenEnv))
	out := summary{}
	if err := verify(ctx, dockerContainer, command, accessToken, probeAccount, model, turnPrompt, &out); err != nil {
		_ = json.NewEncoder(os.Stdout).Encode(out)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

func verify(ctx context.Context, dockerContainer, command, accessToken string, probeAccount bool, model, turnPrompt string, out *summary) error {
	version, _ := codexVersion(ctx, dockerContainer, command)
	out.CodexVersion = version

	cmd := appServerCommand(ctx, dockerContainer, command, accessToken)
	out.Command = publicCommand(dockerContainer, command, accessToken != "")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("codex_app_server_start_failed")
	}
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()
	go drain(stderr)

	rpc := rpcClient{dec: json.NewDecoder(stdout), enc: json.NewEncoder(stdin)}
	if err := rpc.send(map[string]any{"id": 1, "method": "initialize", "params": map[string]any{"clientInfo": map[string]any{"name": "Codex+++ verifier", "version": "0.1.0"}, "capabilities": map[string]any{"experimentalApi": true}}}); err != nil {
		return err
	}
	if _, err := rpc.readResult(1); err != nil {
		out.Initialize = "failed"
		return err
	}
	out.Initialize = "ok"
	if err := rpc.send(map[string]any{"method": "initialized"}); err != nil {
		return err
	}
	if accessToken == "" && !probeAccount {
		out.AccountRead = "skipped_no_token"
		return nil
	}

	accountResult, err := callResult(&rpc, 2, "account/read", map[string]any{"refreshToken": false})
	if err != nil {
		if accessToken == "" {
			out.AccountRead = "auth_error_expected"
			return nil
		}
		out.AccountRead = "failed"
		return err
	}
	out.AccountRead = "ok"
	_ = accountResult
	if accessToken == "" {
		return nil
	}

	if _, err := callResult(&rpc, 3, "account/rateLimits/read", nil); err != nil {
		out.RateLimitsRead = "failed"
		return err
	}
	out.RateLimitsRead = "ok"
	if _, err := callResult(&rpc, 4, "account/usage/read", nil); err != nil {
		out.UsageRead = "failed"
		return err
	}
	out.UsageRead = "ok"
	if strings.TrimSpace(turnPrompt) == "" {
		return nil
	}
	usage, err := runTurn(&rpc, model, turnPrompt)
	if err != nil {
		out.TurnStart = "failed"
		return err
	}
	out.TurnStart = "ok"
	out.TurnUsage = usage
	return nil
}

func appServerCommand(ctx context.Context, dockerContainer, command, accessToken string) *exec.Cmd {
	if strings.TrimSpace(dockerContainer) != "" {
		args := []string{"exec", "-i"}
		if accessToken != "" {
			args = append(args, "-e", "CODEX_ACCESS_TOKEN="+accessToken)
		}
		args = append(args, dockerContainer, "codex", "app-server", "--listen", "stdio://")
		return exec.CommandContext(ctx, "docker", args...)
	}
	cmd := exec.CommandContext(ctx, command, "app-server", "--listen", "stdio://")
	if accessToken != "" {
		cmd.Env = append(os.Environ(), "CODEX_ACCESS_TOKEN="+accessToken)
	}
	return cmd
}

func publicCommand(dockerContainer, command string, withToken bool) string {
	tokenPart := ""
	if withToken {
		tokenPart = " -e CODEX_ACCESS_TOKEN=<redacted>"
	}
	if strings.TrimSpace(dockerContainer) != "" {
		return "docker exec -i" + tokenPart + " " + dockerContainer + " codex app-server --listen stdio://"
	}
	return command + " app-server --listen stdio://"
}

func codexVersion(ctx context.Context, dockerContainer, command string) (string, error) {
	var cmd *exec.Cmd
	if strings.TrimSpace(dockerContainer) != "" {
		cmd = exec.CommandContext(ctx, "docker", "exec", dockerContainer, "codex", "--version")
	} else {
		cmd = exec.CommandContext(ctx, command, "--version")
	}
	b, err := cmd.Output()
	return strings.TrimSpace(string(b)), err
}

func callResult(rpc *rpcClient, id int, method string, params any) (map[string]any, error) {
	msg := map[string]any{"id": id, "method": method}
	if params != nil {
		msg["params"] = params
	}
	if err := rpc.send(msg); err != nil {
		return nil, err
	}
	return rpc.readResult(id)
}

func runTurn(rpc *rpcClient, model, prompt string) (map[string]any, error) {
	threadParams := map[string]any{}
	if strings.TrimSpace(model) != "" {
		threadParams["model"] = strings.TrimSpace(model)
	}
	thread, err := callResult(rpc, 5, "thread/start", threadParams)
	if err != nil {
		return nil, err
	}
	threadID := firstString(thread, "threadId", "id")
	if nested, ok := thread["thread"].(map[string]any); ok && threadID == "" {
		threadID = firstString(nested, "id")
	}
	if threadID == "" {
		return nil, errors.New("codex_app_server_thread_missing")
	}
	turnParams := map[string]any{
		"threadId": threadID,
		"input":    []map[string]string{{"type": "text", "text": strings.TrimSpace(prompt)}},
	}
	if strings.TrimSpace(model) != "" {
		turnParams["model"] = strings.TrimSpace(model)
	}
	if _, err := callResult(rpc, 6, "turn/start", turnParams); err != nil {
		return nil, err
	}
	var usage map[string]any
	for {
		msg, err := rpc.readMessage()
		if err != nil {
			return nil, err
		}
		if u := findUsage(msg); u != nil {
			usage = u
		}
		rpc.respondToServerRequest(msg)
		if stringField(msg, "method") == "turn/completed" {
			if errObj, ok := messageError(msg); ok {
				return nil, fmt.Errorf("codex_app_server_turn_failed: %s", errObj)
			}
			break
		}
	}
	if usage == nil {
		return nil, errors.New("codex_app_server_usage_missing")
	}
	return usage, nil
}

func (rpc *rpcClient) send(message map[string]any) error {
	if err := rpc.enc.Encode(message); err != nil {
		return fmt.Errorf("codex_app_server_write_failed")
	}
	return nil
}

func (rpc *rpcClient) readResult(id int) (map[string]any, error) {
	for {
		msg, err := rpc.readMessage()
		if err != nil {
			return nil, err
		}
		msgID, ok := numberAsInt(msg["id"])
		if !ok || msgID != id || stringField(msg, "method") != "" {
			rpc.respondToServerRequest(msg)
			continue
		}
		if errObj, ok := msg["error"]; ok && errObj != nil {
			return nil, fmt.Errorf("codex_app_server_error: %s", sanitizedError(errObj))
		}
		result, _ := msg["result"].(map[string]any)
		if result == nil {
			result = map[string]any{}
		}
		return result, nil
	}
}

func (rpc *rpcClient) readMessage() (map[string]any, error) {
	var msg map[string]any
	if err := rpc.dec.Decode(&msg); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("codex_app_server_closed")
		}
		return nil, fmt.Errorf("codex_app_server_read_failed")
	}
	return msg, nil
}

func (rpc *rpcClient) respondToServerRequest(msg map[string]any) {
	if _, hasID := msg["id"]; !hasID || stringField(msg, "method") == "" {
		return
	}
	_ = rpc.send(map[string]any{"id": msg["id"], "result": map[string]any{"decision": "deny", "reason": "non-interactive verification"}})
}

func findUsage(value any) map[string]any {
	switch v := value.(type) {
	case map[string]any:
		if usage := tokenUsageBreakdown(v); usage != nil {
			return usage
		}
		if hasUsageShape(v) {
			return v
		}
		if tokenUsage, ok := v["tokenUsage"].(map[string]any); ok {
			if usage := tokenUsageBreakdown(tokenUsage); usage != nil {
				return usage
			}
		}
		for _, nested := range v {
			if usage := findUsage(nested); usage != nil {
				return usage
			}
		}
	case []any:
		for _, nested := range v {
			if usage := findUsage(nested); usage != nil {
				return usage
			}
		}
	}
	return nil
}

func tokenUsageBreakdown(m map[string]any) map[string]any {
	for _, key := range []string{"last", "total"} {
		if nested, ok := m[key].(map[string]any); ok && hasUsageShape(nested) {
			return nested
		}
	}
	return nil
}

func hasUsageShape(m map[string]any) bool {
	for _, key := range []string{"total_tokens", "totalTokens", "tokens", "input_tokens", "inputTokens", "output_tokens", "outputTokens"} {
		if _, ok := m[key]; ok {
			return true
		}
	}
	return false
}

func messageError(msg map[string]any) (string, bool) {
	if errObj, ok := msg["error"]; ok && errObj != nil {
		return sanitizedError(errObj), true
	}
	params, _ := msg["params"].(map[string]any)
	if params == nil {
		return "", false
	}
	if errObj, ok := params["error"]; ok && errObj != nil {
		return sanitizedError(errObj), true
	}
	return "", false
}

func sanitizedError(value any) string {
	if m, ok := value.(map[string]any); ok {
		parts := []string{}
		if code := stringField(m, "code"); code != "" {
			parts = append(parts, "code="+code)
		}
		if status, ok := numberAsInt(m["status"]); ok {
			parts = append(parts, fmt.Sprintf("status=%d", status))
		}
		if data, ok := m["data"].(map[string]any); ok {
			if status, ok := numberAsInt(data["httpStatusCode"]); ok {
				parts = append(parts, fmt.Sprintf("httpStatusCode=%d", status))
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, " ")
		}
	}
	return "codex_app_server_error"
}

func stringField(m map[string]any, key string) string {
	if value, ok := m[key].(string); ok {
		return value
	}
	return ""
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringField(m, key); value != "" {
			return value
		}
	}
	return ""
}

func numberAsInt(value any) (int, bool) {
	switch v := value.(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	case json.Number:
		i, err := v.Int64()
		return int(i), err == nil
	default:
		return 0, false
	}
}

func drain(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
	}
}
