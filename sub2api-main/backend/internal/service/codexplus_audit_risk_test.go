package service

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestCodexPlusAuditRiskPayloadRedactsSensitiveFields(t *testing.T) {
	payload := BuildCodexPlusAuditRiskPayload(CodexPlusAuditRiskEventCreateInput{
		UserID:        42,
		DeviceID:      "device-a",
		EventType:     CodexPlusAuditEventBootstrapRequested,
		RequestID:     "req-redact",
		ConfigVersion: "cfg-v1",
		APIKeyID:      "api-key-id-1",
		Summary:       `opened C:\Users\1\Documents\secret-project\main.go`,
		Metadata: map[string]any{
			"authorization":         "Bearer secret",
			"api_key":               "sk-full-secret",
			"prompt":                "do not keep this prompt",
			"file_content":          "package main",
			"workspace_path":        `C:\Users\1\Documents\secret-project`,
			"callback_url":          "https://example.test/callback?token=secret",
			"redacted_key_summary":  "sk-live-123456...abcd",
			"plain_diagnostic_code": "GATEWAY_POLICY_MODEL_NOT_ALLOWED",
			"nested": map[string]any{
				"source_code": "fmt.Println(secret)",
				"device_hint": "windows",
			},
		},
		CreatedAt: time.Date(2026, 6, 17, 10, 11, 12, 0, time.UTC),
	})

	metadata := payload["metadata"].(map[string]any)
	for _, forbidden := range []string{"authorization", "api_key", "prompt", "file_content"} {
		if _, ok := metadata[forbidden]; ok {
			t.Fatalf("metadata contains forbidden field %q: %#v", forbidden, metadata)
		}
	}
	if got := metadata["workspace_path"]; got != "[redacted_local_path]" {
		t.Fatalf("workspace_path = %#v, want redacted local path", got)
	}
	if got := payload["summary"]; got != "[redacted_local_path]" {
		t.Fatalf("summary = %#v, want redacted local path", got)
	}
	if got := metadata["callback_url"]; got != "[redacted_url]" {
		t.Fatalf("callback_url = %#v, want redacted url", got)
	}
	if got := metadata["redacted_key_summary"]; got != "[redacted]" {
		t.Fatalf("redacted_key_summary = %#v, want redacted secret-like value", got)
	}
	nested := metadata["nested"].(map[string]any)
	if _, ok := nested["source_code"]; ok {
		t.Fatalf("nested source_code was retained: %#v", nested)
	}
	if got := nested["device_hint"]; got != "windows" {
		t.Fatalf("nested device_hint = %#v, want windows", got)
	}
	if got := payload["api_key_id"]; got != "api-key-id-1" {
		t.Fatalf("api_key_id = %#v, want stable id retained", got)
	}
	if got := payload["retention_days"]; got != CodexPlusAuditRetentionDaysDefault {
		t.Fatalf("retention_days = %#v, want %d", got, CodexPlusAuditRetentionDaysDefault)
	}
}

func TestCodexPlusAuditRiskQueryByUserDeviceAndRequestID(t *testing.T) {
	ctx := context.Background()
	store := &codexPlusAuditRiskMemoryStore{}
	_, _ = RecordCodexPlusAuditRiskEvent(ctx, store, CodexPlusAuditRiskEventCreateInput{
		UserID:    7,
		DeviceID:  "device-a",
		EventType: CodexPlusAuditEventBootstrapRequested,
		RequestID: "req-a",
		Summary:   "bootstrap",
		CreatedAt: time.Date(2026, 6, 17, 9, 0, 0, 0, time.UTC),
	})
	_, _ = RecordCodexPlusAuditRiskEvent(ctx, store, CodexPlusAuditRiskEventCreateInput{
		UserID:    7,
		DeviceID:  "device-b",
		EventType: CodexPlusAuditEventDeviceRegistered,
		RequestID: "req-b",
		Summary:   "device",
		CreatedAt: time.Date(2026, 6, 17, 9, 1, 0, 0, time.UTC),
	})
	_, _ = RecordCodexPlusAuditRiskEvent(ctx, store, CodexPlusAuditRiskEventCreateInput{
		UserID:    8,
		DeviceID:  "device-a",
		EventType: CodexPlusAuditEventBootstrapRequested,
		RequestID: "req-c",
		Summary:   "other user",
		CreatedAt: time.Date(2026, 6, 17, 9, 2, 0, 0, time.UTC),
	})

	byDevice, err := QueryCodexPlusAuditRiskEvents(ctx, store, CodexPlusAuditRiskQuery{
		UserID:   7,
		DeviceID: "device-a",
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("query by device: %v", err)
	}
	if len(byDevice) != 1 || byDevice[0].RequestID == nil || *byDevice[0].RequestID != "req-a" {
		t.Fatalf("byDevice = %#v, want req-a only", byDevice)
	}

	byRequest, err := QueryCodexPlusAuditRiskEvents(ctx, store, CodexPlusAuditRiskQuery{
		RequestID: "req-b",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("query by request id: %v", err)
	}
	if len(byRequest) != 1 || byRequest[0].UserID == nil || *byRequest[0].UserID != 7 {
		t.Fatalf("byRequest = %#v, want user 7 req-b only", byRequest)
	}

	byUserAndTag, err := QueryCodexPlusAuditRiskEvents(ctx, store, CodexPlusAuditRiskQuery{
		UserID:   7,
		RiskTags: []string{CodexPlusAuditRiskTagBootstrap},
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("query by risk tag: %v", err)
	}
	if len(byUserAndTag) != 1 || byUserAndTag[0].EventType != CodexPlusAuditEventBootstrapRequested {
		t.Fatalf("byUserAndTag = %#v, want only bootstrap event", byUserAndTag)
	}
}

func TestCodexPlusAuditRiskGatewayRejectIsLocatable(t *testing.T) {
	ctx := context.Background()
	store := &codexPlusAuditRiskMemoryStore{}
	_, _ = RecordCodexPlusAuditRiskEvent(ctx, store, CodexPlusAuditRiskEventCreateInput{
		UserID:        9,
		DeviceID:      "device-denied",
		EventType:     CodexPlusAuditEventGatewayRejected,
		Severity:      "warning",
		RequestID:     "req-denied",
		ConfigVersion: "cfg-v2",
		ModelID:       "codex-pro",
		ErrorCode:     codexPlusAuditGatewayErrorModelNotAllowed,
		ProviderKeyID: "provider-key-3",
		UsageEventID:  "usage-req-denied",
		Summary:       "requested model is outside entitlement",
		Metadata: map[string]any{
			"endpoint":        "/v1/responses",
			"requested_model": "codex-pro",
			"api_key":         "sk-should-not-persist",
			"prompt":          "never persist prompt",
		},
		CreatedAt: time.Date(2026, 6, 17, 9, 3, 0, 0, time.UTC),
	})

	events, err := QueryCodexPlusAuditRiskEvents(ctx, store, CodexPlusAuditRiskQuery{
		UserID:         9,
		RequestID:      "req-denied",
		IncludePayload: true,
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("query gateway reject: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events len = %d, want 1: %#v", len(events), events)
	}
	event := events[0]
	if event.EventType != CodexPlusAuditEventGatewayRejected ||
		event.DeviceID == nil || *event.DeviceID != "device-denied" ||
		event.RequestID == nil || *event.RequestID != "req-denied" ||
		event.ConfigVersion == nil || *event.ConfigVersion != "cfg-v2" ||
		event.ModelID != "codex-pro" ||
		event.ErrorCode != codexPlusAuditGatewayErrorModelNotAllowed ||
		event.ProviderKeyID != "provider-key-3" ||
		event.UsageEventID != "usage-req-denied" {
		t.Fatalf("gateway reject event is not locatable enough: %#v", event)
	}
	if !reflect.DeepEqual(event.RiskTags, []string{CodexPlusAuditRiskTagGatewayRejected, CodexPlusAuditRiskTagModelRejected}) {
		t.Fatalf("risk tags = %#v, want gateway and model rejection", event.RiskTags)
	}
	metadata := event.Payload["metadata"].(map[string]any)
	if _, ok := metadata["api_key"]; ok {
		t.Fatalf("api_key retained in gateway metadata: %#v", metadata)
	}
	if _, ok := metadata["prompt"]; ok {
		t.Fatalf("prompt retained in gateway metadata: %#v", metadata)
	}

	summary := BuildCodexPlusAuditRiskUserSummary(events)
	if summary.GatewayRejectCount != 1 || len(summary.RecentGatewayRejects) != 1 {
		t.Fatalf("summary gateway rejects = %#v, want one reject", summary)
	}
	reject := summary.RecentGatewayRejects[0]
	if reject.RequestID != "req-denied" ||
		reject.DeviceID != "device-denied" ||
		reject.ModelID != "codex-pro" ||
		reject.ErrorCode != codexPlusAuditGatewayErrorModelNotAllowed ||
		reject.ConfigVersion != "cfg-v2" {
		t.Fatalf("reject summary = %#v, want request/device/model/error/config", reject)
	}
}

type codexPlusAuditRiskMemoryStore struct {
	events []CodexPlusEvent
	nextID int64
}

func (s *codexPlusAuditRiskMemoryStore) Append(_ context.Context, input CodexPlusEventCreate) (*CodexPlusEvent, error) {
	s.nextID++
	createdAt := time.Now().UTC().Add(time.Duration(s.nextID) * time.Second)
	if raw, ok := input.Payload["created_at"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
			createdAt = parsed
		}
	}
	event := CodexPlusEvent{
		ID:            s.nextID,
		UserID:        input.UserID,
		DeviceID:      input.DeviceID,
		EventType:     input.EventType,
		Severity:      input.Severity,
		RequestID:     input.RequestID,
		ConfigVersion: input.ConfigVersion,
		Payload:       input.Payload,
		CreatedAt:     createdAt,
		UpdatedAt:     createdAt,
	}
	s.events = append(s.events, event)
	return &event, nil
}

func (s *codexPlusAuditRiskMemoryStore) ListByUser(_ context.Context, userID int64, limit int) ([]CodexPlusEvent, error) {
	out := make([]CodexPlusEvent, 0, len(s.events))
	for _, event := range s.events {
		if event.UserID != nil && *event.UserID == userID {
			out = append(out, event)
		}
	}
	return codexPlusAuditRiskLimitEvents(out, limit), nil
}

func (s *codexPlusAuditRiskMemoryStore) ListByDevice(_ context.Context, userID int64, deviceID string, limit int) ([]CodexPlusEvent, error) {
	out := make([]CodexPlusEvent, 0, len(s.events))
	for _, event := range s.events {
		if event.UserID != nil && *event.UserID == userID && event.DeviceID != nil && *event.DeviceID == deviceID {
			out = append(out, event)
		}
	}
	return codexPlusAuditRiskLimitEvents(out, limit), nil
}

func (s *codexPlusAuditRiskMemoryStore) ListByRequestID(_ context.Context, requestID string, limit int) ([]CodexPlusEvent, error) {
	out := make([]CodexPlusEvent, 0, len(s.events))
	for _, event := range s.events {
		if event.RequestID != nil && *event.RequestID == requestID {
			out = append(out, event)
		}
	}
	return codexPlusAuditRiskLimitEvents(out, limit), nil
}

func codexPlusAuditRiskLimitEvents(events []CodexPlusEvent, limit int) []CodexPlusEvent {
	limit = codexPlusAuditNormalizeLimit(limit)
	if len(events) <= limit {
		return events
	}
	return events[:limit]
}
