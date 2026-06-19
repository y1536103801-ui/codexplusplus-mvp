package service

import (
	"context"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func TestCodexPlusAdminDeviceManagementListFiltersAndFormatsDeviceDetails(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	platform := "windows"
	appVersion := "0.2.0"
	repo := &codexPlusDeviceManagementRepo{
		devices: []CodexPlusDevice{{
			ID:         1,
			UserID:     10,
			DeviceID:   "dev-1",
			Platform:   &platform,
			AppVersion: &appVersion,
			Status:     ClientDeviceStatusRevoked,
			LastSeenAt: &now,
			RevokedAt:  &now,
			Metadata:   map[string]any{"codex_version": "0.2.0"},
			CreatedAt:  now,
			UpdatedAt:  now,
		}},
	}
	svc := NewCodexPlusAdminService(nil, nil, nil, nil, nil, repo, nil)

	devices, err := svc.ListUserDevices(context.Background(), 10, CodexPlusDeviceListFilter{
		Statuses: []string{"revoked,blocked", "revoked"},
		Limit:    700,
	})
	if err != nil {
		t.Fatalf("ListUserDevices: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("devices len = %d, want 1", len(devices))
	}
	if repo.listLimit != 500 {
		t.Fatalf("repo list limit = %d, want capped 500", repo.listLimit)
	}
	if len(repo.listStatuses) != 2 || repo.listStatuses[0] != ClientDeviceStatusRevoked || repo.listStatuses[1] != ClientDeviceStatusBlocked {
		t.Fatalf("statuses = %+v, want revoked and blocked", repo.listStatuses)
	}
	device := devices[0]
	if device.Platform == nil || *device.Platform != platform || device.AppVersion == nil || *device.AppVersion != appVersion {
		t.Fatalf("device = %+v, want platform and app version", device)
	}
	if device.CodexVersion == nil || *device.CodexVersion != "0.2.0" {
		t.Fatalf("codex version = %v, want 0.2.0", device.CodexVersion)
	}
	if device.LastSeenAt == nil || device.RevokedAt == nil {
		t.Fatalf("device times = %+v, want last_seen_at and revoked_at", device)
	}
}

func TestCodexPlusAdminDeviceManagementRejectsInvalidStatusFilter(t *testing.T) {
	svc := NewCodexPlusAdminService(nil, nil, nil, nil, nil, &codexPlusDeviceManagementRepo{}, nil)
	_, err := svc.ListUserDevices(context.Background(), 10, CodexPlusDeviceListFilter{Statuses: []string{"active,disabled"}})
	if !infraerrors.IsBadRequest(err) {
		t.Fatalf("ListUserDevices error = %v, want bad request", err)
	}
}

func TestCodexPlusAdminDeviceManagementRevokeAndRestore(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	repo := &codexPlusDeviceManagementRepo{
		devices: []CodexPlusDevice{{
			ID:        1,
			UserID:    10,
			DeviceID:  "dev-1",
			Status:    ClientDeviceStatusActive,
			CreatedAt: now,
			UpdatedAt: now,
		}},
	}
	events := &codexPlusDeviceManagementEvents{}
	svc := NewCodexPlusAdminService(nil, nil, nil, nil, nil, repo, events)
	svc.now = func() time.Time { return now }

	revoked, err := svc.RevokeUserDevice(context.Background(), CodexPlusDeviceAdminActionInput{
		UserID:   10,
		DeviceID: " dev-1 ",
		Actor:    "admin:7",
		Reason:   "lost laptop",
	})
	if err != nil {
		t.Fatalf("RevokeUserDevice: %v", err)
	}
	if revoked.Status != ClientDeviceStatusRevoked || revoked.RevokedAt == nil {
		t.Fatalf("revoked = %+v, want revoked with revoked_at", revoked)
	}
	if repo.lastStatus != ClientDeviceStatusRevoked || repo.lastRevokedAt == nil {
		t.Fatalf("repo status = %q revoked_at=%v, want revoked timestamp", repo.lastStatus, repo.lastRevokedAt)
	}
	if len(events.events) != 1 || events.events[0].EventType != codexPlusAdminDeviceEventRevoked {
		t.Fatalf("events = %+v, want revoke event", events.events)
	}
	if events.events[0].Payload["actor"] != "admin:7" || events.events[0].Payload["reason"] != "lost laptop" {
		t.Fatalf("event payload = %+v, want actor and reason", events.events[0].Payload)
	}

	restored, err := svc.RestoreUserDevice(context.Background(), CodexPlusDeviceAdminActionInput{
		UserID:   10,
		DeviceID: "dev-1",
		Actor:    "admin:7",
	})
	if err != nil {
		t.Fatalf("RestoreUserDevice: %v", err)
	}
	if restored.Status != ClientDeviceStatusActive || restored.RevokedAt != nil {
		t.Fatalf("restored = %+v, want active without revoked_at", restored)
	}
	if repo.lastStatus != ClientDeviceStatusActive || repo.lastRevokedAt != nil {
		t.Fatalf("repo status = %q revoked_at=%v, want active nil revoked_at", repo.lastStatus, repo.lastRevokedAt)
	}
	if len(events.events) != 2 || events.events[1].EventType != codexPlusAdminDeviceEventRestored {
		t.Fatalf("events = %+v, want restore event", events.events)
	}
}

func TestCodexPlusClientBootstrapRestoredDeviceIsAvailable(t *testing.T) {
	now := time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC)
	keys := &codexPlusClientTestKeys{now: now, nextID: 100}
	svc := newCodexPlusClientTestService(now, &User{ID: 1, Balance: 1520, Status: StatusActive}, keys)
	svc.SetDeviceStore(&codexPlusClientTestDevices{
		record: &CodexPlusDevice{UserID: 1, DeviceID: "device-1234", Status: ClientDeviceStatusActive},
	})

	snapshot, err := svc.Bootstrap(context.Background(), CodexPlusBootstrapInput{UserID: 1, DeviceID: "device-1234"})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if snapshot.Service.Status != ClientServiceStatusAvailable {
		t.Fatalf("status = %q, want available", snapshot.Service.Status)
	}
	if snapshot.Provider.APIKey == nil {
		t.Fatal("available restored device did not include provider api_key")
	}
	if keys.createCount != 1 {
		t.Fatalf("create count = %d, want 1", keys.createCount)
	}
}

type codexPlusDeviceManagementRepo struct {
	devices       []CodexPlusDevice
	listStatuses  []string
	listLimit     int
	lastStatus    string
	lastRevokedAt *time.Time
}

func (r *codexPlusDeviceManagementRepo) Upsert(_ context.Context, input CodexPlusDeviceUpsert) (*CodexPlusDevice, error) {
	device := CodexPlusDevice{
		ID:         int64(len(r.devices) + 1),
		UserID:     input.UserID,
		DeviceID:   input.DeviceID,
		Status:     codexPlusAdminDeviceStatus(input.Status),
		LastSeenAt: input.LastSeenAt,
		Metadata:   input.Metadata,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	r.devices = append(r.devices, device)
	return &device, nil
}

func (r *codexPlusDeviceManagementRepo) GetByUserAndDevice(_ context.Context, userID int64, deviceID string) (*CodexPlusDevice, error) {
	for i := range r.devices {
		if r.devices[i].UserID == userID && r.devices[i].DeviceID == deviceID {
			device := r.devices[i]
			return &device, nil
		}
	}
	return nil, infraerrors.NotFound("CODEXPLUS_DEVICE_NOT_FOUND", "device not found")
}

func (r *codexPlusDeviceManagementRepo) ListByUser(_ context.Context, userID int64, limit int) ([]CodexPlusDevice, error) {
	r.listLimit = limit
	out := make([]CodexPlusDevice, 0, len(r.devices))
	for _, device := range r.devices {
		if device.UserID == userID {
			out = append(out, device)
		}
	}
	return out, nil
}

func (r *codexPlusDeviceManagementRepo) ListByUserAndStatuses(_ context.Context, userID int64, statuses []string, limit int) ([]CodexPlusDevice, error) {
	r.listStatuses = append([]string(nil), statuses...)
	r.listLimit = limit
	allowed := map[string]struct{}{}
	for _, status := range statuses {
		allowed[status] = struct{}{}
	}
	out := make([]CodexPlusDevice, 0, len(r.devices))
	for _, device := range r.devices {
		if device.UserID != userID {
			continue
		}
		if _, ok := allowed[codexPlusAdminDeviceStatus(device.Status)]; ok {
			out = append(out, device)
		}
	}
	return out, nil
}

func (r *codexPlusDeviceManagementRepo) SetStatus(_ context.Context, userID int64, deviceID, status string, revokedAt *time.Time) (*CodexPlusDevice, error) {
	r.lastStatus = status
	r.lastRevokedAt = revokedAt
	for i := range r.devices {
		if r.devices[i].UserID == userID && r.devices[i].DeviceID == deviceID {
			r.devices[i].Status = status
			r.devices[i].RevokedAt = revokedAt
			r.devices[i].UpdatedAt = time.Now().UTC()
			device := r.devices[i]
			return &device, nil
		}
	}
	return nil, infraerrors.NotFound("CODEXPLUS_DEVICE_NOT_FOUND", "device not found")
}

func (r *codexPlusDeviceManagementRepo) UpdateStatus(ctx context.Context, userID int64, deviceID, status string, revokedAt *time.Time) error {
	_, err := r.SetStatus(ctx, userID, deviceID, status, revokedAt)
	return err
}

type codexPlusDeviceManagementEvents struct {
	events []CodexPlusEventCreate
}

func (r *codexPlusDeviceManagementEvents) Append(_ context.Context, input CodexPlusEventCreate) (*CodexPlusEvent, error) {
	r.events = append(r.events, input)
	return &CodexPlusEvent{EventType: input.EventType, Payload: input.Payload}, nil
}

func (r *codexPlusDeviceManagementEvents) ListByUser(_ context.Context, _ int64, _ int) ([]CodexPlusEvent, error) {
	return nil, nil
}
