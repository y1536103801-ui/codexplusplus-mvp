package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
)

type limitedResponseCapture struct {
	buffer    bytes.Buffer
	truncated bool
}

func (c *limitedResponseCapture) Write(chunk []byte) {
	if c.truncated {
		return
	}
	remaining := int(maxStoredCodexResponseBodyBytes) - c.buffer.Len()
	if len(chunk) > remaining {
		c.truncated = true
		c.buffer.Reset()
		return
	}
	_, _ = c.buffer.Write(chunk)
}

func (c *limitedResponseCapture) Bytes() []byte {
	if c.truncated {
		return nil
	}
	return append([]byte(nil), c.buffer.Bytes()...)
}

type codexSSEAccumulator struct {
	lineBuffer []byte
	completed  any
	last       any
	usage      gatewayUsage
	hasUsage   bool
	failed     bool
}

func (p *codexSSEAccumulator) Feed(chunk []byte) error {
	p.lineBuffer = append(p.lineBuffer, chunk...)
	if int64(len(p.lineBuffer)) > maxCodexResponseBodyBytes {
		return errors.New("codex_responses_too_large")
	}
	for {
		newline := bytes.IndexByte(p.lineBuffer, '\n')
		if newline < 0 {
			return nil
		}
		line := append([]byte(nil), p.lineBuffer[:newline]...)
		p.lineBuffer = p.lineBuffer[newline+1:]
		if err := p.consumeLine(line); err != nil {
			return err
		}
	}
}

func (p *codexSSEAccumulator) consumeLine(line []byte) error {
	line = bytes.TrimSpace(line)
	if !bytes.HasPrefix(line, []byte("data:")) {
		return nil
	}
	data := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
	if len(data) == 0 || bytes.Equal(data, []byte("[DONE]")) {
		return nil
	}
	payload, err := decodeJSONValue(data)
	if err != nil {
		return errors.New("codex_responses_invalid_body")
	}
	p.last = payload
	if usage, ok := extractGatewayUsage(payload); ok {
		p.usage = usage
		p.hasUsage = true
	}
	if event, ok := payload.(map[string]any); ok {
		switch stringField(event, "type") {
		case "response.completed":
			if response, exists := event["response"]; exists {
				p.completed = response
			}
		case "response.failed":
			p.failed = true
		}
	}
	return nil
}

func (p *codexSSEAccumulator) Finish() (any, gatewayUsage, error) {
	if len(bytes.TrimSpace(p.lineBuffer)) > 0 {
		if err := p.consumeLine(p.lineBuffer); err != nil {
			return nil, gatewayUsage{}, err
		}
	}
	if p.failed {
		return nil, gatewayUsage{}, errors.New("codex_responses_upstream_failed")
	}
	completed := p.completed
	if completed == nil {
		completed = p.last
	}
	if completed == nil {
		return nil, gatewayUsage{}, errors.New("codex_responses_invalid_body")
	}
	if usage, ok := extractGatewayUsage(completed); ok {
		p.usage = usage
		p.hasUsage = true
	}
	if !p.hasUsage {
		return nil, gatewayUsage{}, errors.New("codex_responses_usage_missing")
	}
	return completed, p.usage, nil
}

func runCodexResponsesStreamingRequest(ctx context.Context, credentials codexProbeCredentials, requestBody []byte, requestHeaders http.Header, target *codexStreamTarget) (codexResponsesResult, error) {
	return runCodexResponsesStreamingRequestSource(ctx, credentials, newMemoryGatewayRequestSource(requestBody), requestHeaders, target)
}

func runCodexResponsesStreamingRequestSource(ctx context.Context, credentials codexProbeCredentials, requestBody *gatewayRequestSource, requestHeaders http.Header, target *codexStreamTarget) (codexResponsesResult, error) {
	if target == nil || target.Writer == nil {
		return codexResponsesResult{}, errors.New("codex_responses_downstream_unavailable")
	}
	if strings.TrimSpace(credentials.AccessToken) == "" {
		return codexResponsesResult{}, errors.New("codex_responses_unavailable")
	}
	if strings.TrimSpace(credentials.ChatGPTAccountID) == "" {
		return codexResponsesResult{}, errors.New("codex_responses_missing_chatgpt_account_id")
	}
	endpoint := strings.TrimSpace(os.Getenv("CODEXPPP_CODEX_RESPONSES_URL"))
	if endpoint == "" {
		endpoint = defaultCodexResponsesURL
	}
	bodyReader, err := requestBody.Open()
	if err != nil {
		return codexResponsesResult{}, errors.New("codex_responses_request_invalid")
	}
	defer bodyReader.Close()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bodyReader)
	if err != nil {
		return codexResponsesResult{}, errors.New("codex_responses_request_invalid")
	}
	req.ContentLength = requestBody.size
	copyCodexRequestHeaders(req.Header, requestHeaders)
	req.Header.Set("Authorization", "Bearer "+credentials.AccessToken)
	req.Header.Set("ChatGPT-Account-ID", credentials.ChatGPTAccountID)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return codexResponsesResult{}, errors.New("codex_responses_unavailable")
	}
	defer res.Body.Close()
	result := codexResponsesResult{Status: res.StatusCode, Header: filterCodexResponseHeaders(res.Header), ContentType: res.Header.Get("Content-Type")}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, maxCodexResponseBodyBytes+1))
		if readErr != nil {
			return codexResponsesResult{}, errors.New("codex_responses_read_failed")
		}
		if int64(len(raw)) > maxCodexResponseBodyBytes {
			return codexResponsesResult{}, errors.New("codex_responses_too_large")
		}
		result.Body = raw
		return result, nil
	}
	for name, values := range result.Header {
		for _, value := range values {
			target.Writer.Header().Add(name, value)
		}
	}
	if target.Writer.Header().Get("Content-Type") == "" {
		target.Writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	}
	target.Writer.Header().Set("Cache-Control", "no-cache")
	target.Writer.Header().Set("X-Accel-Buffering", "no")
	target.Writer.WriteHeader(res.StatusCode)
	target.Started = true
	if flusher, ok := target.Writer.(http.Flusher); ok {
		flusher.Flush()
	}

	isSSE := strings.Contains(strings.ToLower(result.ContentType), "text/event-stream")
	var parser codexSSEAccumulator
	var jsonBody bytes.Buffer
	var replay limitedResponseCapture
	buffer := make([]byte, 32*1024)
	var total int64
	for {
		read, readErr := res.Body.Read(buffer)
		if read > 0 {
			chunk := buffer[:read]
			total += int64(read)
			if total > maxCodexResponseBodyBytes {
				return codexResponsesResult{}, errors.New("codex_responses_too_large")
			}
			replay.Write(chunk)
			if isSSE {
				if err := parser.Feed(chunk); err != nil {
					return codexResponsesResult{}, err
				}
			} else {
				_, _ = jsonBody.Write(chunk)
			}
			if _, err := target.Writer.Write(chunk); err != nil {
				return codexResponsesResult{}, errors.New("codex_responses_downstream_failed")
			}
			if flusher, ok := target.Writer.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return codexResponsesResult{}, errors.New("codex_responses_read_failed")
		}
	}
	if isSSE {
		result.Payload, result.Usage, err = parser.Finish()
	} else {
		result.Payload, result.Usage, err = parseCodexResponsesPayload(jsonBody.Bytes(), result.ContentType)
	}
	if err != nil {
		return codexResponsesResult{}, err
	}
	result.Body = replay.Bytes()
	return result, nil
}
