package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	CodexPlusAuditEventDesktopLoginStarted   = "desktop_login_started"
	CodexPlusAuditEventDesktopLoginCompleted = "desktop_login_completed"
	CodexPlusAuditEventDesktopLoginPolled    = "desktop_login_polled"
	CodexPlusAuditEventDeviceRegistered      = "device_registered"
	CodexPlusAuditEventBootstrapRequested    = "bootstrap_requested"
	CodexPlusAuditEventUsageRequested        = "usage_requested"
	CodexPlusAuditEventRedeemAttempted       = "redeem_attempted"
	CodexPlusAuditEventManagedKeyCreated     = "managed_key_created"
	CodexPlusAuditEventManagedKeyRotated     = "managed_key_rotated"
	CodexPlusAuditEventGatewayRejected       = codexPlusAuditGatewayPolicyEventRejected
	CodexPlusAuditEventAdminDeviceRevoked    = "admin_device_revoked"
	CodexPlusAuditEventAdminUserBlocked      = "admin_user_blocked"
	CodexPlusAuditEventConfigPublished       = "config_published"
	CodexPlusAuditEventConfigRolledBack      = "config_rolled_back"
	CodexPlusAuditEventEntitlementAdjusted   = "entitlement_adjusted"
	CodexPlusAuditEventPaymentCompensated    = "payment_compensated"
	CodexPlusAuditEventUsageBackfilled       = "usage_backfilled"
	CodexPlusAuditEventRiskSignal            = "risk_signal"

	CodexPlusAuditRiskTagLogin                 = "login"
	CodexPlusAuditRiskTagBootstrap             = "bootstrap"
	CodexPlusAuditRiskTagDeviceActivity        = "device_activity"
	CodexPlusAuditRiskTagManagedKeyLifecycle   = "managed_key_lifecycle"
	CodexPlusAuditRiskTagGatewayRejected       = "gateway_policy_rejected"
	CodexPlusAuditRiskTagModelRejected         = "model_rejected"
	CodexPlusAuditRiskTagBalanceInsufficient   = "balance_insufficient"
	CodexPlusAuditRiskTagRateLimited           = "rate_limited"
	CodexPlusAuditRiskTagDeviceRevoked         = "device_revoked"
	CodexPlusAuditRiskTagAdminEnforcement      = "admin_enforcement"
	CodexPlusAuditRiskTagConfigChange          = "config_change"
	CodexPlusAuditRiskTagEntitlementMutation   = "entitlement_mutation"
	CodexPlusAuditRiskTagPaymentReconciliation = "payment_reconciliation"
	CodexPlusAuditRiskTagUsageReconciliation   = "usage_reconciliation"
	CodexPlusAuditRiskTagGatewayUnhealthy      = "gateway_unhealthy"
	CodexPlusAuditRiskTagAbnormalDeviceSwitch  = "abnormal_device_switch"
	CodexPlusAuditRiskTagAccountSharingSignal  = "account_sharing_signal"
	CodexPlusAuditRiskTagHighFrequencyFailure  = "high_frequency_failure"
	CodexPlusAuditRiskTagModelProbe            = "model_probe"

	CodexPlusAuditRetentionDaysDefault = 365
)

const (
	codexPlusAuditGatewayPolicyEventRejected = "gateway_policy_rejected"

	codexPlusAuditGatewayErrorModelNotAllowed     = "GATEWAY_POLICY_MODEL_NOT_ALLOWED"
	codexPlusAuditGatewayErrorBalanceInsufficient = "GATEWAY_POLICY_BALANCE_INSUFFICIENT"
	codexPlusAuditGatewayErrorRateLimited         = "GATEWAY_POLICY_RATE_LIMITED"
	codexPlusAuditGatewayErrorDeviceRevoked       = "GATEWAY_POLICY_DEVICE_REVOKED"
	codexPlusAuditGatewayErrorDeviceBlocked       = "GATEWAY_POLICY_DEVICE_BLOCKED"
	codexPlusAuditGatewayErrorConfigUnavailable   = "GATEWAY_POLICY_CONFIG_UNAVAILABLE"
)

type CodexPlusAuditRiskEventCreateInput struct {
	UserID          int64
	DeviceID        string
	EventType       string
	Severity        string
	RequestID       string
	ConfigVersion   string
	SnapshotVersion string
	ModelID         string
	ErrorCode       string
	ProviderKeyID   string
	APIKeyID        string
	UsageEventID    string
	Summary         string
	RiskTags        []string
	Metadata        map[string]any
	CreatedAt       time.Time
}

type CodexPlusAuditRiskQuery struct {
	UserID         int64
	DeviceID       string
	RequestID      string
	EventTypes     []string
	RiskTags       []string
	Limit          int
	IncludePayload bool
}

type CodexPlusAuditRiskEvent struct {
	ID            int64          `json:"id"`
	EventType     string         `json:"event_type"`
	Severity      string         `json:"severity"`
	UserID        *int64         `json:"user_id,omitempty"`
	DeviceID      *string        `json:"device_id,omitempty"`
	RequestID     *string        `json:"request_id,omitempty"`
	ConfigVersion *string        `json:"config_version,omitempty"`
	ModelID       string         `json:"model_id,omitempty"`
	ErrorCode     string         `json:"error_code,omitempty"`
	ProviderKeyID string         `json:"provider_key_id,omitempty"`
	UsageEventID  string         `json:"usage_event_id,omitempty"`
	RiskTags      []string       `json:"risk_tags"`
	Summary       string         `json:"summary"`
	CreatedAt     string         `json:"created_at"`
	Payload       map[string]any `json:"payload,omitempty"`
}

type CodexPlusAuditRiskGatewayRejectSummary struct {
	RequestID     string   `json:"request_id,omitempty"`
	DeviceID      string   `json:"device_id,omitempty"`
	ModelID       string   `json:"model_id,omitempty"`
	ErrorCode     string   `json:"error_code,omitempty"`
	ConfigVersion string   `json:"config_version,omitempty"`
	RiskTags      []string `json:"risk_tags,omitempty"`
	CreatedAt     string   `json:"created_at"`
	Summary       string   `json:"summary,omitempty"`
}

type CodexPlusAuditRiskUserSummary struct {
	EventCount           int                                      `json:"event_count"`
	RiskEventCount       int                                      `json:"risk_event_count"`
	GatewayRejectCount   int                                      `json:"gateway_reject_count"`
	WarningCount         int                                      `json:"warning_count"`
	ErrorCount           int                                      `json:"error_count"`
	RecentRiskTags       []string                                 `json:"recent_risk_tags"`
	RecentRequestIDs     []string                                 `json:"recent_request_ids"`
	RecentGatewayRejects []CodexPlusAuditRiskGatewayRejectSummary `json:"recent_gateway_rejects"`
	LastEventAt          string                                   `json:"last_event_at,omitempty"`
}

type codexPlusAuditRiskEventStore interface {
	Append(ctx context.Context, input CodexPlusEventCreate) (*CodexPlusEvent, error)
	ListByUser(ctx context.Context, userID int64, limit int) ([]CodexPlusEvent, error)
}

type codexPlusAuditRiskListByRequestID interface {
	ListByRequestID(ctx context.Context, requestID string, limit int) ([]CodexPlusEvent, error)
}

type codexPlusAuditRiskListByDevice interface {
	ListByDevice(ctx context.Context, userID int64, deviceID string, limit int) ([]CodexPlusEvent, error)
}

func BuildCodexPlusAuditRiskEventCreate(input CodexPlusAuditRiskEventCreateInput) CodexPlusEventCreate {
	payload := BuildCodexPlusAuditRiskPayload(input)
	userID := codexPlusAuditInt64PtrIfPositive(input.UserID)
	deviceID := codexPlusAuditStringPtrIfNotBlank(input.DeviceID)
	return CodexPlusEventCreate{
		UserID:        userID,
		DeviceID:      deviceID,
		EventType:     codexPlusAuditEventType(input.EventType),
		Severity:      codexPlusAuditSeverity(input.Severity),
		RequestID:     codexPlusAuditStringPtrIfNotBlank(input.RequestID),
		ConfigVersion: codexPlusAuditStringPtrIfNotBlank(input.ConfigVersion),
		Payload:       payload,
	}
}

func BuildCodexPlusAuditRiskPayload(input CodexPlusAuditRiskEventCreateInput) map[string]any {
	createdAt := input.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	eventType := codexPlusAuditEventType(input.EventType)
	errorCode := strings.TrimSpace(input.ErrorCode)
	riskTags := NormalizeCodexPlusAuditRiskTags(append(defaultCodexPlusAuditRiskTags(eventType, errorCode), input.RiskTags...))
	payload := map[string]any{
		"event_id":          codexPlusAuditRiskEventID(eventType, input.RequestID, createdAt),
		"event_type":        eventType,
		"event_category":    CodexPlusAuditRiskEventCategory(eventType),
		"severity":          codexPlusAuditSeverity(input.Severity),
		"user_id":           codexPlusAuditNullableInt64String(input.UserID),
		"device_id":         codexPlusAuditNullableString(input.DeviceID),
		"request_id":        codexPlusAuditNullableString(input.RequestID),
		"config_version":    codexPlusAuditNullableString(input.ConfigVersion),
		"snapshot_version":  codexPlusAuditNullableString(input.SnapshotVersion),
		"model_id":          codexPlusAuditNullableString(input.ModelID),
		"error_code":        codexPlusAuditNullableString(errorCode),
		"provider_key_id":   codexPlusAuditNullableString(input.ProviderKeyID),
		"api_key_id":        codexPlusAuditNullableString(input.APIKeyID),
		"usage_event_id":    codexPlusAuditNullableString(input.UsageEventID),
		"risk_tags":         riskTags,
		"summary":           codexPlusAuditNullableString(input.Summary),
		"created_at":        createdAt.UTC().Format(time.RFC3339),
		"retention_days":    CodexPlusAuditRetentionDaysDefault,
		"redaction_applied": true,
		"metadata":          RedactCodexPlusAuditRiskMetadata(input.Metadata),
	}
	return NormalizeCodexPlusAuditRiskPayload(eventType, payload)
}

func RecordCodexPlusAuditRiskEvent(ctx context.Context, store codexPlusAuditRiskEventStore, input CodexPlusAuditRiskEventCreateInput) (*CodexPlusEvent, error) {
	if store == nil {
		return nil, fmt.Errorf("codexplus audit risk event store is required")
	}
	return store.Append(ctx, BuildCodexPlusAuditRiskEventCreate(input))
}

func NormalizeCodexPlusAuditRiskPayload(eventType string, payload map[string]any) map[string]any {
	if payload == nil {
		payload = map[string]any{}
	}
	eventType = codexPlusAuditEventType(codexPlusAuditPayloadString(payload, "event_type", eventType))
	payload["event_type"] = eventType
	payload["event_category"] = CodexPlusAuditRiskEventCategory(eventType)
	payload["severity"] = codexPlusAuditSeverity(codexPlusAuditPayloadString(payload, "severity", "info"))
	payload["retention_days"] = CodexPlusAuditRetentionDaysDefault
	payload["redaction_applied"] = true
	payload["risk_tags"] = NormalizeCodexPlusAuditRiskTags(append(defaultCodexPlusAuditRiskTags(eventType, codexPlusAuditPayloadString(payload, "error_code", "")), codexPlusAuditPayloadStringSlice(payload, "risk_tags")...))
	if _, ok := payload["created_at"]; !ok {
		payload["created_at"] = time.Now().UTC().Format(time.RFC3339)
	}
	if _, ok := payload["event_id"]; !ok {
		payload["event_id"] = codexPlusAuditRiskEventID(eventType, codexPlusAuditPayloadString(payload, "request_id", ""), time.Now())
	}
	if metadata, ok := payload["metadata"].(map[string]any); ok {
		payload["metadata"] = RedactCodexPlusAuditRiskMetadata(metadata)
	} else {
		payload["metadata"] = map[string]any{}
	}
	for key, value := range payload {
		if key == "metadata" || key == "risk_tags" {
			continue
		}
		payload[key] = redactCodexPlusAuditRiskValue(key, value)
	}
	return payload
}

func QueryCodexPlusAuditRiskEvents(ctx context.Context, store codexPlusAuditRiskEventStore, query CodexPlusAuditRiskQuery) ([]CodexPlusAuditRiskEvent, error) {
	if store == nil {
		return []CodexPlusAuditRiskEvent{}, nil
	}
	limit := codexPlusAuditNormalizeLimit(query.Limit)
	sourceLimit := limit
	if strings.TrimSpace(query.DeviceID) != "" || strings.TrimSpace(query.RequestID) != "" || len(query.RiskTags) > 0 || len(query.EventTypes) > 0 {
		sourceLimit = codexPlusAuditNormalizeLimit(500)
	}
	events, err := loadCodexPlusAuditRiskEvents(ctx, store, query, sourceLimit)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].CreatedAt.Equal(events[j].CreatedAt) {
			return events[i].ID > events[j].ID
		}
		return events[i].CreatedAt.After(events[j].CreatedAt)
	})
	out := make([]CodexPlusAuditRiskEvent, 0, codexPlusAuditMinInt(limit, len(events)))
	for _, event := range events {
		summary := codexPlusAuditRiskEventFromService(event, query.IncludePayload)
		if !codexPlusAuditRiskEventMatches(summary, query) {
			continue
		}
		out = append(out, summary)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func CodexPlusAuditRiskSummaryForUser(ctx context.Context, store codexPlusAuditRiskEventStore, userID int64, limit int) (*CodexPlusAuditRiskUserSummary, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("codexplus audit risk user_id is required")
	}
	events, err := QueryCodexPlusAuditRiskEvents(ctx, store, CodexPlusAuditRiskQuery{
		UserID:         userID,
		Limit:          limit,
		IncludePayload: false,
	})
	if err != nil {
		return nil, err
	}
	return BuildCodexPlusAuditRiskUserSummary(events), nil
}

func BuildCodexPlusAuditRiskUserSummary(events []CodexPlusAuditRiskEvent) *CodexPlusAuditRiskUserSummary {
	summary := &CodexPlusAuditRiskUserSummary{
		RecentRiskTags:       []string{},
		RecentRequestIDs:     []string{},
		RecentGatewayRejects: []CodexPlusAuditRiskGatewayRejectSummary{},
	}
	tagSeen := map[string]struct{}{}
	reqSeen := map[string]struct{}{}
	for _, event := range events {
		summary.EventCount++
		if event.CreatedAt != "" && summary.LastEventAt == "" {
			summary.LastEventAt = event.CreatedAt
		}
		switch strings.ToLower(strings.TrimSpace(event.Severity)) {
		case "warning":
			summary.WarningCount++
		case "error", "critical":
			summary.ErrorCount++
		}
		if len(event.RiskTags) > 0 {
			summary.RiskEventCount++
		}
		for _, tag := range event.RiskTags {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			if _, ok := tagSeen[tag]; ok {
				continue
			}
			tagSeen[tag] = struct{}{}
			summary.RecentRiskTags = append(summary.RecentRiskTags, tag)
			if len(summary.RecentRiskTags) >= 20 {
				break
			}
		}
		if event.RequestID != nil && strings.TrimSpace(*event.RequestID) != "" {
			req := strings.TrimSpace(*event.RequestID)
			if _, ok := reqSeen[req]; !ok {
				reqSeen[req] = struct{}{}
				summary.RecentRequestIDs = append(summary.RecentRequestIDs, req)
			}
		}
		if event.EventType == CodexPlusAuditEventGatewayRejected {
			summary.GatewayRejectCount++
			if len(summary.RecentGatewayRejects) < 10 {
				deviceID := ""
				if event.DeviceID != nil {
					deviceID = *event.DeviceID
				}
				requestID := ""
				if event.RequestID != nil {
					requestID = *event.RequestID
				}
				configVersion := ""
				if event.ConfigVersion != nil {
					configVersion = *event.ConfigVersion
				}
				summary.RecentGatewayRejects = append(summary.RecentGatewayRejects, CodexPlusAuditRiskGatewayRejectSummary{
					RequestID:     requestID,
					DeviceID:      deviceID,
					ModelID:       event.ModelID,
					ErrorCode:     event.ErrorCode,
					ConfigVersion: configVersion,
					RiskTags:      append([]string(nil), event.RiskTags...),
					CreatedAt:     event.CreatedAt,
					Summary:       event.Summary,
				})
			}
		}
	}
	return summary
}

func CodexPlusAuditRiskEventCategory(eventType string) string {
	switch strings.TrimSpace(eventType) {
	case CodexPlusAuditEventDesktopLoginStarted, CodexPlusAuditEventDesktopLoginCompleted, CodexPlusAuditEventDesktopLoginPolled:
		return "login"
	case CodexPlusAuditEventBootstrapRequested:
		return "bootstrap"
	case CodexPlusAuditEventDeviceRegistered:
		return "device"
	case CodexPlusAuditEventManagedKeyCreated, CodexPlusAuditEventManagedKeyRotated:
		return "managed_key"
	case CodexPlusAuditEventGatewayRejected:
		return "gateway_policy"
	case CodexPlusAuditEventAdminDeviceRevoked, CodexPlusAuditEventAdminUserBlocked:
		return "admin_enforcement"
	case CodexPlusAuditEventConfigPublished, CodexPlusAuditEventConfigRolledBack:
		return "config_change"
	case CodexPlusAuditEventEntitlementAdjusted, CodexPlusAuditEventRedeemAttempted:
		return "entitlement"
	case CodexPlusAuditEventPaymentCompensated:
		return "payment_reconciliation"
	case CodexPlusAuditEventUsageRequested, CodexPlusAuditEventUsageBackfilled:
		return "usage"
	case CodexPlusAuditEventRiskSignal:
		return "risk_signal"
	default:
		return "audit"
	}
}

func NormalizeCodexPlusAuditRiskTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := map[string]struct{}{}
	for _, tag := range tags {
		normalized := strings.ToLower(strings.TrimSpace(tag))
		if normalized == "" {
			continue
		}
		normalized = strings.ReplaceAll(normalized, " ", "_")
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func RedactCodexPlusAuditRiskMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(metadata))
	for key, value := range metadata {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" || codexPlusAuditRiskDropsField(trimmed) {
			continue
		}
		out[trimmed] = redactCodexPlusAuditRiskValue(trimmed, value)
	}
	return out
}

func loadCodexPlusAuditRiskEvents(ctx context.Context, store codexPlusAuditRiskEventStore, query CodexPlusAuditRiskQuery, limit int) ([]CodexPlusEvent, error) {
	requestID := strings.TrimSpace(query.RequestID)
	deviceID := strings.TrimSpace(query.DeviceID)
	if query.UserID > 0 {
		if deviceID != "" {
			if byDevice, ok := store.(codexPlusAuditRiskListByDevice); ok {
				return byDevice.ListByDevice(ctx, query.UserID, deviceID, limit)
			}
		}
		return store.ListByUser(ctx, query.UserID, limit)
	}
	if requestID != "" {
		if byRequest, ok := store.(codexPlusAuditRiskListByRequestID); ok {
			return byRequest.ListByRequestID(ctx, requestID, limit)
		}
		return nil, fmt.Errorf("codexplus audit risk request_id query requires repository support or user_id")
	}
	return nil, fmt.Errorf("codexplus audit risk query requires user_id")
}

func codexPlusAuditRiskEventFromService(event CodexPlusEvent, includePayload bool) CodexPlusAuditRiskEvent {
	payload := NormalizeCodexPlusAuditRiskPayload(event.EventType, codexPlusAuditCloneMap(event.Payload))
	riskTags := NormalizeCodexPlusAuditRiskTags(codexPlusAuditPayloadStringSlice(payload, "risk_tags"))
	summary := codexPlusAuditPayloadString(payload, "summary", "")
	if summary == "" {
		if metadata, ok := payload["metadata"].(map[string]any); ok {
			summary = codexPlusAuditPayloadString(metadata, "reason", "")
			if summary == "" {
				summary = codexPlusAuditPayloadString(metadata, "service_status", "")
			}
		}
	}
	if summary == "" {
		summary = codexPlusAuditPayloadString(payload, "error_code", "")
	}
	if summary == "" {
		summary = event.EventType
	}
	createdAt := event.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	out := CodexPlusAuditRiskEvent{
		ID:            event.ID,
		EventType:     codexPlusAuditEventType(event.EventType),
		Severity:      codexPlusAuditSeverity(event.Severity),
		UserID:        event.UserID,
		DeviceID:      event.DeviceID,
		RequestID:     event.RequestID,
		ConfigVersion: event.ConfigVersion,
		ModelID:       codexPlusAuditPayloadString(payload, "model_id", ""),
		ErrorCode:     codexPlusAuditPayloadString(payload, "error_code", ""),
		ProviderKeyID: codexPlusAuditPayloadString(payload, "provider_key_id", ""),
		UsageEventID:  codexPlusAuditPayloadString(payload, "usage_event_id", ""),
		RiskTags:      riskTags,
		Summary:       summary,
		CreatedAt:     createdAt.UTC().Format(time.RFC3339),
	}
	if out.ModelID == "" {
		if metadata, ok := payload["metadata"].(map[string]any); ok {
			out.ModelID = codexPlusAuditPayloadString(metadata, "requested_model", "")
		}
	}
	if includePayload {
		out.Payload = payload
	}
	return out
}

func codexPlusAuditRiskEventMatches(event CodexPlusAuditRiskEvent, query CodexPlusAuditRiskQuery) bool {
	if query.UserID > 0 {
		if event.UserID == nil || *event.UserID != query.UserID {
			return false
		}
	}
	if deviceID := strings.TrimSpace(query.DeviceID); deviceID != "" {
		if event.DeviceID == nil || strings.TrimSpace(*event.DeviceID) != deviceID {
			return false
		}
	}
	if requestID := strings.TrimSpace(query.RequestID); requestID != "" {
		if event.RequestID == nil || strings.TrimSpace(*event.RequestID) != requestID {
			return false
		}
	}
	if len(query.EventTypes) > 0 && !codexPlusAuditContainsString(query.EventTypes, event.EventType) {
		return false
	}
	if len(query.RiskTags) > 0 {
		for _, wanted := range NormalizeCodexPlusAuditRiskTags(query.RiskTags) {
			if !codexPlusAuditContainsString(event.RiskTags, wanted) {
				return false
			}
		}
	}
	return true
}

func defaultCodexPlusAuditRiskTags(eventType, errorCode string) []string {
	tags := []string{}
	switch eventType {
	case CodexPlusAuditEventDesktopLoginStarted, CodexPlusAuditEventDesktopLoginCompleted, CodexPlusAuditEventDesktopLoginPolled:
		tags = append(tags, CodexPlusAuditRiskTagLogin)
	case CodexPlusAuditEventBootstrapRequested:
		tags = append(tags, CodexPlusAuditRiskTagBootstrap)
	case CodexPlusAuditEventDeviceRegistered:
		tags = append(tags, CodexPlusAuditRiskTagDeviceActivity)
	case CodexPlusAuditEventManagedKeyCreated, CodexPlusAuditEventManagedKeyRotated:
		tags = append(tags, CodexPlusAuditRiskTagManagedKeyLifecycle)
	case CodexPlusAuditEventGatewayRejected:
		tags = append(tags, CodexPlusAuditRiskTagGatewayRejected)
	case CodexPlusAuditEventAdminDeviceRevoked, CodexPlusAuditEventAdminUserBlocked:
		tags = append(tags, CodexPlusAuditRiskTagAdminEnforcement)
	case CodexPlusAuditEventConfigPublished, CodexPlusAuditEventConfigRolledBack:
		tags = append(tags, CodexPlusAuditRiskTagConfigChange)
	case CodexPlusAuditEventEntitlementAdjusted, CodexPlusAuditEventRedeemAttempted:
		tags = append(tags, CodexPlusAuditRiskTagEntitlementMutation)
	case CodexPlusAuditEventPaymentCompensated:
		tags = append(tags, CodexPlusAuditRiskTagPaymentReconciliation)
	case CodexPlusAuditEventUsageRequested, CodexPlusAuditEventUsageBackfilled:
		tags = append(tags, CodexPlusAuditRiskTagUsageReconciliation)
	}
	switch strings.TrimSpace(errorCode) {
	case codexPlusAuditGatewayErrorModelNotAllowed:
		tags = append(tags, CodexPlusAuditRiskTagModelRejected)
	case codexPlusAuditGatewayErrorBalanceInsufficient:
		tags = append(tags, CodexPlusAuditRiskTagBalanceInsufficient)
	case codexPlusAuditGatewayErrorRateLimited:
		tags = append(tags, CodexPlusAuditRiskTagRateLimited)
	case codexPlusAuditGatewayErrorDeviceRevoked, codexPlusAuditGatewayErrorDeviceBlocked:
		tags = append(tags, CodexPlusAuditRiskTagDeviceRevoked)
	case codexPlusAuditGatewayErrorConfigUnavailable:
		tags = append(tags, CodexPlusAuditRiskTagGatewayUnhealthy)
	}
	return tags
}

func codexPlusAuditRiskDropsField(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	switch {
	case normalized == "":
		return true
	case strings.Contains(normalized, "authorization"),
		strings.Contains(normalized, "password"),
		strings.Contains(normalized, "credential"),
		strings.Contains(normalized, "secret"),
		strings.Contains(normalized, "jwt"),
		strings.Contains(normalized, "token"),
		strings.Contains(normalized, "prompt"),
		strings.Contains(normalized, "messages"),
		strings.Contains(normalized, "completion"),
		strings.Contains(normalized, "request_body"),
		strings.Contains(normalized, "response_body"),
		strings.Contains(normalized, "request_payload"),
		strings.Contains(normalized, "response_payload"),
		strings.Contains(normalized, "file_content"),
		strings.Contains(normalized, "file_body"),
		strings.Contains(normalized, "code_content"),
		strings.Contains(normalized, "source_code"):
		return true
	case strings.Contains(normalized, "api_key") && !strings.HasSuffix(normalized, "_id"):
		return true
	case strings.Contains(normalized, "apikey"):
		return true
	case strings.Contains(normalized, "key") && !codexPlusAuditRiskAllowedKeyField(normalized):
		return true
	default:
		return false
	}
}

func codexPlusAuditRiskAllowedKeyField(normalized string) bool {
	return normalized == "key_id" ||
		normalized == "api_key_id" ||
		normalized == "provider_key_id" ||
		normalized == "key_prefix" ||
		normalized == "masked_key" ||
		normalized == "redacted_key_summary"
}

func redactCodexPlusAuditRiskValue(key string, value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case string:
		return redactCodexPlusAuditRiskString(key, typed)
	case fmt.Stringer:
		return redactCodexPlusAuditRiskString(key, typed.String())
	case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return typed
	case []string:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, redactCodexPlusAuditRiskString(key, item))
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, redactCodexPlusAuditRiskValue(key, item))
		}
		return out
	case map[string]any:
		return RedactCodexPlusAuditRiskMetadata(typed)
	case map[string]string:
		out := make(map[string]any, len(typed))
		for childKey, childValue := range typed {
			if codexPlusAuditRiskDropsField(childKey) {
				continue
			}
			out[childKey] = redactCodexPlusAuditRiskString(childKey, childValue)
		}
		return out
	default:
		return fmt.Sprintf("%T", typed)
	}
}

func redactCodexPlusAuditRiskString(key, value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	lowerKey := strings.ToLower(strings.TrimSpace(key))
	lowerValue := strings.ToLower(trimmed)
	if strings.HasPrefix(lowerValue, "bearer ") || strings.Contains(lowerValue, "sk-") || strings.Count(trimmed, ".") >= 2 {
		return "[redacted]"
	}
	if looksLikeCodexPlusSecretBearingURL(trimmed) {
		return "[redacted_url]"
	}
	if strings.Contains(lowerKey, "path") || strings.Contains(lowerKey, "directory") || strings.Contains(lowerKey, "workspace") {
		if looksLikeCodexPlusLocalPath(trimmed) || containsCodexPlusLocalPathFragment(trimmed) {
			return "[redacted_local_path]"
		}
	}
	if looksLikeCodexPlusLocalPath(trimmed) || containsCodexPlusLocalPathFragment(trimmed) {
		return "[redacted_local_path]"
	}
	if len(trimmed) > 512 {
		return trimmed[:512] + "...[truncated]"
	}
	return trimmed
}

func looksLikeCodexPlusLocalPath(value string) bool {
	if len(value) >= 3 && ((value[1] == ':' && (value[2] == '\\' || value[2] == '/')) || strings.HasPrefix(value, `\\`)) {
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(lower, "~/") ||
		strings.HasPrefix(lower, "/users/") ||
		strings.HasPrefix(lower, "/home/") ||
		strings.HasPrefix(lower, "/mnt/") ||
		strings.HasPrefix(lower, "/volumes/")
}

func containsCodexPlusLocalPathFragment(value string) bool {
	lower := strings.ToLower(value)
	if strings.Contains(value, `:\Users\`) ||
		strings.Contains(value, `:\users\`) ||
		strings.Contains(value, `:\Documents\`) ||
		strings.Contains(value, `:\documents\`) ||
		strings.Contains(value, `\\Users\`) ||
		strings.Contains(value, `\\users\`) {
		return true
	}
	return strings.Contains(lower, "/users/") ||
		strings.Contains(lower, "/home/") ||
		strings.Contains(lower, "/mnt/") ||
		strings.Contains(lower, "/volumes/")
}

func looksLikeCodexPlusSecretBearingURL(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return false
	}
	return strings.Contains(lower, "authorization=") ||
		strings.Contains(lower, "api_key=") ||
		strings.Contains(lower, "apikey=") ||
		strings.Contains(lower, "credential=") ||
		strings.Contains(lower, "secret=") ||
		strings.Contains(lower, "token=")
}

func codexPlusAuditEventType(value string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return "audit_event"
}

func codexPlusAuditSeverity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug", "info", "notice", "warning", "error", "critical":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "info"
	}
}

func codexPlusAuditNullableString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func codexPlusAuditNullableInt64String(value int64) any {
	if value <= 0 {
		return nil
	}
	return fmt.Sprintf("%d", value)
}

func codexPlusAuditRiskEventID(eventType, requestID string, createdAt time.Time) string {
	prefix := strings.ReplaceAll(strings.TrimSpace(eventType), " ", "_")
	if prefix == "" {
		prefix = "audit_event"
	}
	if requestID = strings.TrimSpace(requestID); requestID != "" {
		return prefix + "-" + requestID
	}
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	return fmt.Sprintf("%s-%d", prefix, createdAt.UTC().UnixNano())
}

func codexPlusAuditPayloadString(payload map[string]any, key string, fallback string) string {
	if len(payload) == 0 {
		return strings.TrimSpace(fallback)
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return strings.TrimSpace(fallback)
	}
	switch typed := value.(type) {
	case string:
		if trimmed := strings.TrimSpace(typed); trimmed != "" {
			return trimmed
		}
	case fmt.Stringer:
		if trimmed := strings.TrimSpace(typed.String()); trimmed != "" {
			return trimmed
		}
	default:
		if trimmed := strings.TrimSpace(fmt.Sprint(typed)); trimmed != "" && trimmed != "<nil>" {
			return trimmed
		}
	}
	return strings.TrimSpace(fallback)
}

func codexPlusAuditPayloadStringSlice(payload map[string]any, key string) []string {
	if len(payload) == 0 {
		return nil
	}
	value := payload[key]
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if s := strings.TrimSpace(fmt.Sprint(item)); s != "" && s != "<nil>" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		return []string{typed}
	default:
		return nil
	}
}

func codexPlusAuditCloneMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func codexPlusAuditContainsString(values []string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

func codexPlusAuditStringPtrIfNotBlank(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func codexPlusAuditInt64PtrIfPositive(value int64) *int64 {
	if value <= 0 {
		return nil
	}
	return &value
}

func codexPlusAuditNormalizeLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 500 {
		return 500
	}
	return limit
}

func codexPlusAuditMinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
