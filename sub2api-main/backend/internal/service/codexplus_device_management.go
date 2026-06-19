package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	codexPlusAdminDeviceEventRevoked  = "admin_device_revoked"
	codexPlusAdminDeviceEventRestored = "admin_device_restored"
)

type CodexPlusDeviceListFilter struct {
	Statuses []string
	Limit    int
}

type CodexPlusAdminDevice struct {
	ID           int64   `json:"id"`
	UserID       int64   `json:"user_id"`
	DeviceID     string  `json:"device_id"`
	DeviceName   *string `json:"device_name,omitempty"`
	Platform     *string `json:"platform,omitempty"`
	AppVersion   *string `json:"app_version,omitempty"`
	CodexVersion *string `json:"codex_version,omitempty"`
	Status       string  `json:"status"`
	LastSeenAt   *string `json:"last_seen_at,omitempty"`
	RevokedAt    *string `json:"revoked_at,omitempty"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

type CodexPlusDeviceAdminActionInput struct {
	UserID   int64
	DeviceID string
	Actor    string
	Reason   string
}

func (s *CodexPlusAdminService) ListUserDevices(ctx context.Context, userID int64, filter CodexPlusDeviceListFilter) ([]CodexPlusAdminDevice, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("CODEXPLUS_USER_ID_INVALID", "invalid user id")
	}
	if s == nil || s.deviceRepo == nil {
		return []CodexPlusAdminDevice{}, nil
	}
	statuses, err := normalizeCodexPlusAdminDeviceStatuses(filter.Statuses)
	if err != nil {
		return nil, err
	}
	limit := normalizeCodexPlusAdminDeviceLimit(filter.Limit)
	var devices []CodexPlusDevice
	if len(statuses) == 0 {
		devices, err = s.deviceRepo.ListByUser(ctx, userID, limit)
	} else {
		devices, err = s.deviceRepo.ListByUserAndStatuses(ctx, userID, statuses, limit)
	}
	if err != nil {
		return nil, err
	}
	out := make([]CodexPlusAdminDevice, 0, len(devices))
	for _, device := range devices {
		out = append(out, adminDeviceFromCodexPlusDevice(device))
	}
	return out, nil
}

func (s *CodexPlusAdminService) RevokeUserDevice(ctx context.Context, input CodexPlusDeviceAdminActionInput) (*CodexPlusAdminDevice, error) {
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	if err := validateCodexPlusAdminDeviceAction(input); err != nil {
		return nil, err
	}
	if s == nil || s.deviceRepo == nil {
		return nil, infraerrors.ServiceUnavailable("CODEXPLUS_DEVICE_REPOSITORY_UNAVAILABLE", "device repository is unavailable")
	}
	now := s.codexPlusAdminDeviceNow()
	device, err := s.deviceRepo.SetStatus(ctx, input.UserID, input.DeviceID, ClientDeviceStatusRevoked, &now)
	if err != nil {
		return nil, err
	}
	s.recordCodexPlusAdminDeviceEvent(ctx, codexPlusAdminDeviceEventRevoked, input, device, now)
	out := adminDeviceFromCodexPlusDevice(*device)
	return &out, nil
}

func (s *CodexPlusAdminService) RestoreUserDevice(ctx context.Context, input CodexPlusDeviceAdminActionInput) (*CodexPlusAdminDevice, error) {
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	if err := validateCodexPlusAdminDeviceAction(input); err != nil {
		return nil, err
	}
	if s == nil || s.deviceRepo == nil {
		return nil, infraerrors.ServiceUnavailable("CODEXPLUS_DEVICE_REPOSITORY_UNAVAILABLE", "device repository is unavailable")
	}
	now := s.codexPlusAdminDeviceNow()
	device, err := s.deviceRepo.SetStatus(ctx, input.UserID, input.DeviceID, ClientDeviceStatusActive, nil)
	if err != nil {
		return nil, err
	}
	s.recordCodexPlusAdminDeviceEvent(ctx, codexPlusAdminDeviceEventRestored, input, device, now)
	out := adminDeviceFromCodexPlusDevice(*device)
	return &out, nil
}

func validateCodexPlusAdminDeviceAction(input CodexPlusDeviceAdminActionInput) error {
	if input.UserID <= 0 {
		return infraerrors.BadRequest("CODEXPLUS_USER_ID_INVALID", "invalid user id")
	}
	if strings.TrimSpace(input.DeviceID) == "" {
		return infraerrors.BadRequest("CODEXPLUS_DEVICE_ID_REQUIRED", "device id is required")
	}
	if len(input.DeviceID) > 128 {
		return infraerrors.BadRequest("CODEXPLUS_DEVICE_ID_INVALID", "device id is too long")
	}
	return nil
}

func normalizeCodexPlusAdminDeviceStatuses(statuses []string) ([]string, error) {
	if len(statuses) == 0 {
		return nil, nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(statuses))
	for _, raw := range statuses {
		for _, part := range strings.Split(raw, ",") {
			status := strings.ToLower(strings.TrimSpace(part))
			if status == "" {
				continue
			}
			switch status {
			case ClientDeviceStatusActive, ClientDeviceStatusRevoked, ClientDeviceStatusBlocked:
			default:
				return nil, infraerrors.BadRequest("CODEXPLUS_DEVICE_STATUS_INVALID", "device status is invalid")
			}
			if _, ok := seen[status]; ok {
				continue
			}
			seen[status] = struct{}{}
			out = append(out, status)
		}
	}
	return out, nil
}

func normalizeCodexPlusAdminDeviceLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > 500 {
		return 500
	}
	return limit
}

func (s *CodexPlusAdminService) codexPlusAdminDeviceNow() time.Time {
	if s == nil || s.now == nil {
		return time.Now().UTC()
	}
	return s.now().UTC()
}

func (s *CodexPlusAdminService) recordCodexPlusAdminDeviceEvent(ctx context.Context, eventType string, input CodexPlusDeviceAdminActionInput, device *CodexPlusDevice, now time.Time) {
	if s == nil || s.eventRepo == nil || device == nil {
		return
	}
	payload := map[string]any{
		"summary":    fmt.Sprintf("%s device %s", strings.TrimPrefix(eventType, "admin_device_"), device.DeviceID),
		"actor":      strings.TrimSpace(input.Actor),
		"reason":     strings.TrimSpace(input.Reason),
		"status":     device.Status,
		"device_id":  device.DeviceID,
		"created_at": now.UTC().Format(time.RFC3339),
	}
	_, _ = s.eventRepo.Append(ctx, CodexPlusEventCreate{
		UserID:    &input.UserID,
		DeviceID:  &device.DeviceID,
		EventType: eventType,
		Severity:  "warning",
		Payload:   payload,
	})
}

func adminDeviceFromCodexPlusDevice(device CodexPlusDevice) CodexPlusAdminDevice {
	return CodexPlusAdminDevice{
		ID:           device.ID,
		UserID:       device.UserID,
		DeviceID:     device.DeviceID,
		DeviceName:   device.DeviceName,
		Platform:     device.Platform,
		AppVersion:   device.AppVersion,
		CodexVersion: codexPlusAdminDeviceMetadataString(device.Metadata, "codex_version"),
		Status:       codexPlusAdminDeviceStatus(device.Status),
		LastSeenAt:   codexPlusAdminDeviceTimeString(device.LastSeenAt),
		RevokedAt:    codexPlusAdminDeviceTimeString(device.RevokedAt),
		CreatedAt:    device.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:    device.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func codexPlusAdminDeviceStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		return ClientDeviceStatusActive
	}
	return status
}

func codexPlusAdminDeviceTimeString(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}

func codexPlusAdminDeviceMetadataString(metadata map[string]any, key string) *string {
	if len(metadata) == 0 {
		return nil
	}
	value, ok := metadata[key]
	if !ok || value == nil {
		return nil
	}
	out := strings.TrimSpace(fmt.Sprint(value))
	if out == "" || out == "<nil>" {
		return nil
	}
	return &out
}
