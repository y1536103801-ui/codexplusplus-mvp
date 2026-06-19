package service

import (
	"context"
	"time"
)

type CodexPlusDevice struct {
	ID              int64
	UserID          int64
	DeviceID        string
	DeviceName      *string
	Platform        *string
	AppVersion      *string
	FingerprintHash *string
	Status          string
	LastSeenAt      *time.Time
	RevokedAt       *time.Time
	Metadata        map[string]any
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CodexPlusDeviceUpsert struct {
	UserID          int64
	DeviceID        string
	DeviceName      *string
	Platform        *string
	AppVersion      *string
	FingerprintHash *string
	Status          string
	LastSeenAt      *time.Time
	Metadata        map[string]any
}

type CodexPlusManagedProviderKey struct {
	ID                int64
	UserID            int64
	APIKeyID          int64
	ManagedProviderID string
	DisplayName       string
	KeyPrefix         *string
	Status            string
	LastUsedAt        *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type CodexPlusManagedProviderKeyUpsert struct {
	UserID            int64
	APIKeyID          int64
	ManagedProviderID string
	DisplayName       string
	KeyPrefix         *string
	Status            string
}

type CodexPlusEvent struct {
	ID            int64
	UserID        *int64
	DeviceID      *string
	EventType     string
	Severity      string
	RequestID     *string
	ConfigVersion *string
	Payload       map[string]any
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type CodexPlusEventCreate struct {
	UserID        *int64
	DeviceID      *string
	EventType     string
	Severity      string
	RequestID     *string
	ConfigVersion *string
	Payload       map[string]any
}

type CodexPlusDeviceRepository interface {
	Upsert(ctx context.Context, input CodexPlusDeviceUpsert) (*CodexPlusDevice, error)
	GetByUserAndDevice(ctx context.Context, userID int64, deviceID string) (*CodexPlusDevice, error)
	ListByUser(ctx context.Context, userID int64, limit int) ([]CodexPlusDevice, error)
	ListByUserAndStatuses(ctx context.Context, userID int64, statuses []string, limit int) ([]CodexPlusDevice, error)
	SetStatus(ctx context.Context, userID int64, deviceID, status string, revokedAt *time.Time) (*CodexPlusDevice, error)
	UpdateStatus(ctx context.Context, userID int64, deviceID, status string, revokedAt *time.Time) error
}

type CodexPlusManagedProviderKeyRepository interface {
	Upsert(ctx context.Context, input CodexPlusManagedProviderKeyUpsert) (*CodexPlusManagedProviderKey, error)
	GetByUserAndProvider(ctx context.Context, userID int64, managedProviderID string) (*CodexPlusManagedProviderKey, error)
	GetByAPIKeyID(ctx context.Context, apiKeyID int64) (*CodexPlusManagedProviderKey, error)
}

type CodexPlusEventRepository interface {
	Append(ctx context.Context, input CodexPlusEventCreate) (*CodexPlusEvent, error)
	ListByUser(ctx context.Context, userID int64, limit int) ([]CodexPlusEvent, error)
}
