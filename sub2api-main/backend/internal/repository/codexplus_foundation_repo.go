package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type codexPlusDeviceRepository struct {
	db *sql.DB
}

type codexPlusManagedProviderKeyRepository struct {
	db *sql.DB
}

type codexPlusEventRepository struct {
	db *sql.DB
}

func NewCodexPlusDeviceRepository(db *sql.DB) service.CodexPlusDeviceRepository {
	return &codexPlusDeviceRepository{db: db}
}

func NewCodexPlusManagedProviderKeyRepository(db *sql.DB) service.CodexPlusManagedProviderKeyRepository {
	return &codexPlusManagedProviderKeyRepository{db: db}
}

func NewCodexPlusEventRepository(db *sql.DB) service.CodexPlusEventRepository {
	return &codexPlusEventRepository{db: db}
}

func (r *codexPlusDeviceRepository) Upsert(ctx context.Context, input service.CodexPlusDeviceUpsert) (*service.CodexPlusDevice, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil codexplus device repository")
	}
	if input.Status == "" {
		input.Status = "active"
	}
	metadata, err := marshalCodexPlusJSON(input.Metadata)
	if err != nil {
		return nil, err
	}
	query := `
		INSERT INTO codexplus_devices (
			user_id, device_id, device_name, platform, app_version, fingerprint_hash,
			status, last_seen_at, metadata, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,NOW())
		ON CONFLICT (user_id, device_id) WHERE deleted_at IS NULL DO UPDATE SET
			device_name = EXCLUDED.device_name,
			platform = EXCLUDED.platform,
			app_version = EXCLUDED.app_version,
			fingerprint_hash = EXCLUDED.fingerprint_hash,
			status = CASE
				WHEN codexplus_devices.status IN ('revoked', 'blocked') AND EXCLUDED.status = 'active'
					THEN codexplus_devices.status
				ELSE EXCLUDED.status
			END,
			last_seen_at = EXCLUDED.last_seen_at,
			metadata = EXCLUDED.metadata,
			updated_at = NOW()
		RETURNING id, user_id, device_id, device_name, platform, app_version,
			fingerprint_hash, status, last_seen_at, revoked_at, metadata, created_at, updated_at
	`
	device := &service.CodexPlusDevice{}
	if err := scanCodexPlusDevice(ctx, r.db, query, []any{
		input.UserID,
		input.DeviceID,
		nullStringFromPtr(input.DeviceName),
		nullStringFromPtr(input.Platform),
		nullStringFromPtr(input.AppVersion),
		nullStringFromPtr(input.FingerprintHash),
		input.Status,
		nullTimeFromPtr(input.LastSeenAt),
		metadata,
	}, device); err != nil {
		return nil, err
	}
	return device, nil
}

func (r *codexPlusDeviceRepository) GetByUserAndDevice(ctx context.Context, userID int64, deviceID string) (*service.CodexPlusDevice, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil codexplus device repository")
	}
	query := `
		SELECT id, user_id, device_id, device_name, platform, app_version,
			fingerprint_hash, status, last_seen_at, revoked_at, metadata, created_at, updated_at
		FROM codexplus_devices
		WHERE user_id = $1 AND device_id = $2 AND deleted_at IS NULL
	`
	device := &service.CodexPlusDevice{}
	if err := scanCodexPlusDevice(ctx, r.db, query, []any{userID, deviceID}, device); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return device, nil
}

func (r *codexPlusDeviceRepository) ListByUser(ctx context.Context, userID int64, limit int) ([]service.CodexPlusDevice, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil codexplus device repository")
	}
	limit = normalizeCodexPlusListLimit(limit)
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, device_id, device_name, platform, app_version,
			fingerprint_hash, status, last_seen_at, revoked_at, metadata, created_at, updated_at
		FROM codexplus_devices
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY COALESCE(last_seen_at, updated_at, created_at) DESC, id DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]service.CodexPlusDevice, 0, limit)
	for rows.Next() {
		var item service.CodexPlusDevice
		if err := scanCodexPlusDeviceRow(rows, &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *codexPlusDeviceRepository) ListByUserAndStatuses(ctx context.Context, userID int64, statuses []string, limit int) ([]service.CodexPlusDevice, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil codexplus device repository")
	}
	limit = normalizeCodexPlusListLimit(limit)
	args := []any{userID}
	seen := map[string]struct{}{}
	placeholders := make([]string, 0, len(statuses))
	for _, status := range statuses {
		status = strings.ToLower(strings.TrimSpace(status))
		if status == "" {
			continue
		}
		if _, ok := seen[status]; ok {
			continue
		}
		seen[status] = struct{}{}
		args = append(args, status)
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
	}
	if len(placeholders) == 0 {
		return r.ListByUser(ctx, userID, limit)
	}
	args = append(args, limit)
	query := fmt.Sprintf(`
		SELECT id, user_id, device_id, device_name, platform, app_version,
			fingerprint_hash, status, last_seen_at, revoked_at, metadata, created_at, updated_at
		FROM codexplus_devices
		WHERE user_id = $1 AND deleted_at IS NULL AND status IN (%s)
		ORDER BY COALESCE(last_seen_at, updated_at, created_at) DESC, id DESC
		LIMIT $%d
	`, strings.Join(placeholders, ","), len(args))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]service.CodexPlusDevice, 0, limit)
	for rows.Next() {
		var item service.CodexPlusDevice
		if err := scanCodexPlusDeviceRow(rows, &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *codexPlusDeviceRepository) SetStatus(ctx context.Context, userID int64, deviceID, status string, revokedAt *time.Time) (*service.CodexPlusDevice, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil codexplus device repository")
	}
	status = strings.ToLower(strings.TrimSpace(status))
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, infraerrors.BadRequest("CODEXPLUS_DEVICE_ID_REQUIRED", "device id is required")
	}
	if status == "" {
		status = "active"
	}
	query := `
		UPDATE codexplus_devices
		SET status = $3, revoked_at = $4, updated_at = NOW()
		WHERE user_id = $1 AND device_id = $2 AND deleted_at IS NULL
		RETURNING id, user_id, device_id, device_name, platform, app_version,
			fingerprint_hash, status, last_seen_at, revoked_at, metadata, created_at, updated_at
	`
	device := &service.CodexPlusDevice{}
	if err := scanCodexPlusDevice(ctx, r.db, query, []any{userID, deviceID, status, nullTimeFromPtr(revokedAt)}, device); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, infraerrors.NotFound("CODEXPLUS_DEVICE_NOT_FOUND", "device not found")
		}
		return nil, err
	}
	return device, nil
}

func (r *codexPlusDeviceRepository) UpdateStatus(ctx context.Context, userID int64, deviceID, status string, revokedAt *time.Time) error {
	_, err := r.SetStatus(ctx, userID, deviceID, status, revokedAt)
	return err
}

func (r *codexPlusManagedProviderKeyRepository) Upsert(ctx context.Context, input service.CodexPlusManagedProviderKeyUpsert) (*service.CodexPlusManagedProviderKey, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil codexplus managed provider key repository")
	}
	if input.ManagedProviderID == "" {
		input.ManagedProviderID = service.CodexPlusManagedProviderID
	}
	if input.DisplayName == "" {
		input.DisplayName = "Codex++ Cloud"
	}
	if input.Status == "" {
		input.Status = "active"
	}
	query := `
		INSERT INTO codexplus_managed_provider_keys (
			user_id, api_key_id, managed_provider_id, display_name, key_prefix, status, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,NOW())
		ON CONFLICT (user_id, managed_provider_id) WHERE deleted_at IS NULL DO UPDATE SET
			api_key_id = EXCLUDED.api_key_id,
			display_name = EXCLUDED.display_name,
			key_prefix = EXCLUDED.key_prefix,
			status = EXCLUDED.status,
			updated_at = NOW()
		RETURNING id, user_id, api_key_id, managed_provider_id, display_name,
			key_prefix, status, last_used_at, created_at, updated_at
	`
	out := &service.CodexPlusManagedProviderKey{}
	if err := scanCodexPlusManagedProviderKey(ctx, r.db, query, []any{
		input.UserID,
		input.APIKeyID,
		input.ManagedProviderID,
		input.DisplayName,
		nullStringFromPtr(input.KeyPrefix),
		input.Status,
	}, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *codexPlusManagedProviderKeyRepository) GetByUserAndProvider(ctx context.Context, userID int64, managedProviderID string) (*service.CodexPlusManagedProviderKey, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil codexplus managed provider key repository")
	}
	if managedProviderID == "" {
		managedProviderID = service.CodexPlusManagedProviderID
	}
	query := `
		SELECT id, user_id, api_key_id, managed_provider_id, display_name,
			key_prefix, status, last_used_at, created_at, updated_at
		FROM codexplus_managed_provider_keys
		WHERE user_id = $1 AND managed_provider_id = $2 AND deleted_at IS NULL
	`
	out := &service.CodexPlusManagedProviderKey{}
	if err := scanCodexPlusManagedProviderKey(ctx, r.db, query, []any{userID, managedProviderID}, out); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return out, nil
}

func (r *codexPlusManagedProviderKeyRepository) GetByAPIKeyID(ctx context.Context, apiKeyID int64) (*service.CodexPlusManagedProviderKey, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil codexplus managed provider key repository")
	}
	query := `
		SELECT id, user_id, api_key_id, managed_provider_id, display_name,
			key_prefix, status, last_used_at, created_at, updated_at
		FROM codexplus_managed_provider_keys
		WHERE api_key_id = $1 AND deleted_at IS NULL
	`
	out := &service.CodexPlusManagedProviderKey{}
	if err := scanCodexPlusManagedProviderKey(ctx, r.db, query, []any{apiKeyID}, out); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return out, nil
}

func (r *codexPlusEventRepository) Append(ctx context.Context, input service.CodexPlusEventCreate) (*service.CodexPlusEvent, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil codexplus event repository")
	}
	if input.Severity == "" {
		input.Severity = "info"
	}
	payload, err := marshalCodexPlusJSON(input.Payload)
	if err != nil {
		return nil, err
	}
	query := `
		INSERT INTO codexplus_events (
			user_id, device_id, event_type, severity, request_id, config_version, payload
		) VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb)
		RETURNING id, user_id, device_id, event_type, severity, request_id,
			config_version, payload, created_at, updated_at
	`
	event := &service.CodexPlusEvent{}
	if err := scanCodexPlusEvent(ctx, r.db, query, []any{
		nullInt64FromPtr(input.UserID),
		nullStringFromPtr(input.DeviceID),
		input.EventType,
		input.Severity,
		nullStringFromPtr(input.RequestID),
		nullStringFromPtr(input.ConfigVersion),
		payload,
	}, event); err != nil {
		return nil, err
	}
	return event, nil
}

func (r *codexPlusEventRepository) ListByUser(ctx context.Context, userID int64, limit int) ([]service.CodexPlusEvent, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil codexplus event repository")
	}
	limit = normalizeCodexPlusListLimit(limit)
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, device_id, event_type, severity, request_id,
			config_version, payload, created_at, updated_at
		FROM codexplus_events
		WHERE user_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]service.CodexPlusEvent, 0, limit)
	for rows.Next() {
		var item service.CodexPlusEvent
		if err := scanCodexPlusEventRow(rows, &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func scanCodexPlusDevice(ctx context.Context, q sqlQueryer, query string, args []any, out *service.CodexPlusDevice) error {
	return scanCodexPlusDeviceWith(codexPlusScanner(func(dest ...any) error {
		return scanSingleRow(ctx, q, query, args, dest...)
	}), out)
}

type codexPlusScanner func(dest ...any) error

func (s codexPlusScanner) Scan(dest ...any) error {
	return s(dest...)
}

func scanCodexPlusDeviceRow(row interface{ Scan(dest ...any) error }, out *service.CodexPlusDevice) error {
	return scanCodexPlusDeviceWith(row, out)
}

func scanCodexPlusDeviceWith(row interface{ Scan(dest ...any) error }, out *service.CodexPlusDevice) error {
	var deviceName sql.NullString
	var platform sql.NullString
	var appVersion sql.NullString
	var fingerprintHash sql.NullString
	var lastSeenAt sql.NullTime
	var revokedAt sql.NullTime
	var metadataRaw []byte
	if err := row.Scan(
		&out.ID,
		&out.UserID,
		&out.DeviceID,
		&deviceName,
		&platform,
		&appVersion,
		&fingerprintHash,
		&out.Status,
		&lastSeenAt,
		&revokedAt,
		&metadataRaw,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return err
	}
	out.DeviceName = ptrFromNullString(deviceName)
	out.Platform = ptrFromNullString(platform)
	out.AppVersion = ptrFromNullString(appVersion)
	out.FingerprintHash = ptrFromNullString(fingerprintHash)
	out.LastSeenAt = ptrFromNullTime(lastSeenAt)
	out.RevokedAt = ptrFromNullTime(revokedAt)
	out.Metadata = unmarshalCodexPlusJSON(metadataRaw)
	return nil
}

func scanCodexPlusManagedProviderKey(ctx context.Context, q sqlQueryer, query string, args []any, out *service.CodexPlusManagedProviderKey) error {
	var keyPrefix sql.NullString
	var lastUsedAt sql.NullTime
	if err := scanSingleRow(ctx, q, query, args,
		&out.ID,
		&out.UserID,
		&out.APIKeyID,
		&out.ManagedProviderID,
		&out.DisplayName,
		&keyPrefix,
		&out.Status,
		&lastUsedAt,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return err
	}
	out.KeyPrefix = ptrFromNullString(keyPrefix)
	out.LastUsedAt = ptrFromNullTime(lastUsedAt)
	return nil
}

func scanCodexPlusEvent(ctx context.Context, q sqlQueryer, query string, args []any, out *service.CodexPlusEvent) error {
	return scanCodexPlusEventWith(codexPlusScanner(func(dest ...any) error {
		return scanSingleRow(ctx, q, query, args, dest...)
	}), out)
}

func scanCodexPlusEventRow(row interface{ Scan(dest ...any) error }, out *service.CodexPlusEvent) error {
	return scanCodexPlusEventWith(row, out)
}

func scanCodexPlusEventWith(row interface{ Scan(dest ...any) error }, out *service.CodexPlusEvent) error {
	var userID sql.NullInt64
	var deviceID sql.NullString
	var requestID sql.NullString
	var configVersion sql.NullString
	var payloadRaw []byte
	if err := row.Scan(
		&out.ID,
		&userID,
		&deviceID,
		&out.EventType,
		&out.Severity,
		&requestID,
		&configVersion,
		&payloadRaw,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return err
	}
	out.UserID = ptrFromNullInt64(userID)
	out.DeviceID = ptrFromNullString(deviceID)
	out.RequestID = ptrFromNullString(requestID)
	out.ConfigVersion = ptrFromNullString(configVersion)
	out.Payload = unmarshalCodexPlusJSON(payloadRaw)
	return nil
}

func normalizeCodexPlusListLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 500 {
		return 500
	}
	return limit
}

func marshalCodexPlusJSON(value map[string]any) (string, error) {
	if value == nil {
		value = map[string]any{}
	}
	b, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("marshal codexplus json: %w", err)
	}
	return string(b), nil
}

func unmarshalCodexPlusJSON(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func nullStringFromPtr(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}

func nullInt64FromPtr(value *int64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *value, Valid: true}
}

func nullTimeFromPtr(value *time.Time) sql.NullTime {
	if value == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *value, Valid: true}
}

func ptrFromNullString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	v := value.String
	return &v
}

func ptrFromNullInt64(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	v := value.Int64
	return &v
}

func ptrFromNullTime(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	v := value.Time
	return &v
}
