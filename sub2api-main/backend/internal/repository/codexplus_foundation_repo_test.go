package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestCodexPlusDeviceUpsertPreservesRevokedStatusOnActiveHeartbeat(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 6, 16, 1, 2, 3, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"id", "user_id", "device_id", "device_name", "platform", "app_version",
		"fingerprint_hash", "status", "last_seen_at", "revoked_at", "metadata", "created_at", "updated_at",
	}).AddRow(int64(1), int64(10), "dev-1", nil, "windows", "1.0.0", nil, "revoked", now, now, []byte(`{}`), now, now)
	mock.ExpectQuery(`(?s)status = CASE\s+WHEN codexplus_devices\.status IN \('revoked', 'blocked'\) AND EXCLUDED\.status = 'active'\s+THEN codexplus_devices\.status\s+ELSE EXCLUDED\.status\s+END`).
		WithArgs(int64(10), "dev-1", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), "active", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(rows)

	repo := NewCodexPlusDeviceRepository(db)
	seenAt := now
	device, err := repo.Upsert(context.Background(), serviceCodexPlusDeviceUpsertForTest(10, "dev-1", "windows", "1.0.0", &seenAt))
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if device.Status != "revoked" {
		t.Fatalf("status = %q, want revoked", device.Status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func serviceCodexPlusDeviceUpsertForTest(userID int64, deviceID, platform, appVersion string, seenAt *time.Time) service.CodexPlusDeviceUpsert {
	return service.CodexPlusDeviceUpsert{
		UserID:     userID,
		DeviceID:   deviceID,
		Platform:   &platform,
		AppVersion: &appVersion,
		Status:     "active",
		LastSeenAt: seenAt,
		Metadata:   map[string]any{},
	}
}

func TestCodexPlusDeviceSetStatusReturnsUpdatedDevice(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 6, 16, 1, 2, 3, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"id", "user_id", "device_id", "device_name", "platform", "app_version",
		"fingerprint_hash", "status", "last_seen_at", "revoked_at", "metadata", "created_at", "updated_at",
	}).AddRow(int64(1), int64(10), "dev-1", "Workstation", "windows", "1.0.0", nil, "revoked", now, now, []byte(`{"codex_version":"0.2.0"}`), now, now)
	mock.ExpectQuery(`(?s)UPDATE codexplus_devices\s+SET status = \$3, revoked_at = \$4, updated_at = NOW\(\)\s+WHERE user_id = \$1 AND device_id = \$2 AND deleted_at IS NULL\s+RETURNING`).
		WithArgs(int64(10), "dev-1", "revoked", sqlmock.AnyArg()).
		WillReturnRows(rows)

	repo := NewCodexPlusDeviceRepository(db)
	device, err := repo.SetStatus(context.Background(), 10, "dev-1", "revoked", &now)
	if err != nil {
		t.Fatalf("SetStatus: %v", err)
	}
	if device.Status != "revoked" || device.RevokedAt == nil || device.Platform == nil || *device.Platform != "windows" {
		t.Fatalf("device = %+v, want revoked windows device with revoked_at", device)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestCodexPlusDeviceSetStatusNotFound(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`(?s)UPDATE codexplus_devices\s+SET status = \$3, revoked_at = \$4, updated_at = NOW\(\)\s+WHERE user_id = \$1 AND device_id = \$2 AND deleted_at IS NULL\s+RETURNING`).
		WithArgs(int64(10), "missing-dev", "active", sqlmock.AnyArg()).
		WillReturnError(sql.ErrNoRows)

	repo := NewCodexPlusDeviceRepository(db)
	_, err = repo.SetStatus(context.Background(), 10, "missing-dev", "active", nil)
	if !infraerrors.IsNotFound(err) {
		t.Fatalf("SetStatus error = %v, want not found", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestCodexPlusDeviceListByUserAndStatuses(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 6, 16, 1, 2, 3, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"id", "user_id", "device_id", "device_name", "platform", "app_version",
		"fingerprint_hash", "status", "last_seen_at", "revoked_at", "metadata", "created_at", "updated_at",
	}).
		AddRow(int64(1), int64(10), "dev-revoked", nil, "darwin", "1.0.0", nil, "revoked", now, now, []byte(`{}`), now, now).
		AddRow(int64(2), int64(10), "dev-blocked", nil, "linux", "1.0.0", nil, "blocked", now, nil, []byte(`{}`), now, now)
	mock.ExpectQuery(`(?s)FROM codexplus_devices\s+WHERE user_id = \$1 AND deleted_at IS NULL AND status IN \(\$2,\$3\).*LIMIT \$4`).
		WithArgs(int64(10), "revoked", "blocked", 2).
		WillReturnRows(rows)

	repo := NewCodexPlusDeviceRepository(db)
	devices, err := repo.ListByUserAndStatuses(context.Background(), 10, []string{"revoked", "blocked", "revoked"}, 2)
	if err != nil {
		t.Fatalf("ListByUserAndStatuses: %v", err)
	}
	if len(devices) != 2 || devices[0].DeviceID != "dev-revoked" || devices[1].Status != "blocked" {
		t.Fatalf("devices = %+v, want revoked and blocked devices", devices)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestCodexPlusEventListByUserMatchesAppendOnlySchema(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 6, 16, 1, 2, 3, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"id", "user_id", "device_id", "event_type", "severity", "request_id",
		"config_version", "payload", "created_at", "updated_at",
	}).AddRow(int64(1), int64(10), "dev-1", "bootstrap", "info", "req-1", "cfg-1", []byte(`{"ok":true}`), now, now)
	mock.ExpectQuery(`(?s)FROM codexplus_events\s+WHERE user_id = \$1\s+ORDER BY created_at DESC, id DESC\s+LIMIT \$2`).
		WithArgs(int64(10), 20).
		WillReturnRows(rows)

	repo := NewCodexPlusEventRepository(db)
	events, err := repo.ListByUser(context.Background(), 10, 20)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(events) != 1 || events[0].EventType != "bootstrap" {
		t.Fatalf("events = %#v, want one bootstrap event", events)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
