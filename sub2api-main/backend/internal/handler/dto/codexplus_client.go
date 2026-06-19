package dto

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type ClientBootstrapResponse struct {
	Service       ClientServiceState   `json:"service"`
	Provider      ManagedProvider      `json:"provider"`
	Plan          ClientPlanSummary    `json:"plan"`
	Models        []ClientModel        `json:"models"`
	Usage         ClientUsageSummary   `json:"usage"`
	FeatureFlags  ClientFeatureFlags   `json:"feature_flags"`
	Announcements []ClientAnnouncement `json:"announcements"`
	VersionPolicy ClientVersionPolicy  `json:"version_policy"`
	Device        ClientDevice         `json:"device"`
}

type ClientServiceState struct {
	Status     string  `json:"status"`
	Message    string  `json:"message"`
	MessageKey string  `json:"message_key"`
	ActionHint string  `json:"action_hint"`
	Retryable  bool    `json:"retryable"`
	SupportURL *string `json:"support_url"`
	ErrorCode  *string `json:"error_code"`
}

type ManagedProvider struct {
	ProviderID     string                  `json:"provider_id"`
	DisplayName    string                  `json:"display_name"`
	GatewayBaseURL string                  `json:"gateway_base_url"`
	AuthMode       string                  `json:"auth_mode"`
	APIKey         *string                 `json:"api_key"`
	KeySummary     ManagedProviderKeyBrief `json:"key_summary"`
	DefaultModel   string                  `json:"default_model"`
}

type ManagedProviderKeyBrief struct {
	KeyID      string     `json:"key_id"`
	MaskedKey  string     `json:"masked_key"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

type ClientPlanSummary struct {
	PlanID         string       `json:"plan_id"`
	Name           string       `json:"name"`
	Status         string       `json:"status"`
	ExpiresAt      *time.Time   `json:"expires_at"`
	RenewURL       *string      `json:"renew_url"`
	CommerceAction ClientAction `json:"commerce_action"`
}

type ClientModel struct {
	ModelID        string  `json:"model_id"`
	RouteModel     string  `json:"route_model,omitempty"`
	Label          string  `json:"label"`
	IsDefault      bool    `json:"is_default"`
	IsAvailable    bool    `json:"is_available"`
	DisabledReason *string `json:"disabled_reason"`
}

type ClientUsageSummary struct {
	BalanceDisplay     string       `json:"balance_display"`
	LowBalance         bool         `json:"low_balance"`
	PeriodUsageDisplay string       `json:"period_usage_display"`
	RateLimitState     string       `json:"rate_limit_state"`
	RenewAction        ClientAction `json:"renew_action"`
}

type ClientAction struct {
	ActionType    string  `json:"action_type"`
	MessageKey    *string `json:"message_key"`
	ActionCopyKey *string `json:"action_copy_key"`
	Label         string  `json:"label"`
	URL           *string `json:"url"`
}

type ClientUsageSnapshot struct {
	ServiceStatus   string                   `json:"service_status"`
	BalanceSummary  ClientBalanceSummary     `json:"balance_summary"`
	PeriodUsage     ClientPeriodUsageSummary `json:"period_usage"`
	RateLimitState  string                   `json:"rate_limit_state"`
	RenewAction     ClientAction             `json:"renew_action"`
	LastUpdatedAt   time.Time                `json:"last_updated_at"`
	SnapshotVersion string                   `json:"snapshot_version"`
}

type ClientBalanceSummary struct {
	BalanceDisplay string `json:"balance_display"`
	LowBalance     bool   `json:"low_balance"`
}

type ClientPeriodUsageSummary struct {
	PeriodID     string     `json:"period_id"`
	UsageDisplay string     `json:"usage_display"`
	ResetAt      *time.Time `json:"reset_at"`
}

type ClientFeatureFlags struct {
	AdvancedProviderConfig  bool `json:"advanced_provider_config"`
	InstallAssistant        bool `json:"install_assistant"`
	NewUserTutorial         bool `json:"new_user_tutorial"`
	ModelSelector           bool `json:"model_selector"`
	DiagnosticExport        bool `json:"diagnostic_export"`
	Announcements           bool `json:"announcements"`
	ForceUpdatePrompt       bool `json:"force_update_prompt"`
	StrictDeviceEnforcement bool `json:"strict_device_enforcement"`
}

type ClientAnnouncement struct {
	ID       string  `json:"id"`
	Severity string  `json:"severity"`
	Message  string  `json:"message"`
	URL      *string `json:"url"`
}

type ClientVersionPolicy struct {
	ConfigVersion        string `json:"config_version"`
	SnapshotVersion      string `json:"snapshot_version"`
	RefreshTTLSeconds    int    `json:"refresh_ttl_seconds"`
	ForceRefresh         bool   `json:"force_refresh"`
	MinimumClientVersion string `json:"minimum_client_version"`
}

type ClientDevice struct {
	DeviceID        string `json:"device_id"`
	Status          string `json:"status"`
	Message         string `json:"message"`
	SnapshotVersion string `json:"snapshot_version"`
}

type ClientDeviceRegisterRequest struct {
	DeviceID     string    `json:"device_id" binding:"required,min=8,max=128"`
	Platform     string    `json:"platform" binding:"required,oneof=windows macos linux unknown"`
	AppVersion   string    `json:"app_version" binding:"required"`
	CodexVersion *string   `json:"codex_version"`
	LastSeenAt   time.Time `json:"last_seen_at" binding:"required"`
}

type ClientRedeemRequest struct {
	Code     string  `json:"code" binding:"required,min=4,max=128"`
	DeviceID *string `json:"device_id"`
}

type ClientRedeemResponse struct {
	RedeemStatus            string `json:"redeem_status"`
	EntitlementDeltaSummary string `json:"entitlement_delta_summary"`
	ServiceStatusAfter      string `json:"service_status_after"`
	SnapshotVersion         string `json:"snapshot_version"`
	Message                 string `json:"message"`
}

func ClientBootstrapFromService(in *service.CodexPlusBootstrapSnapshot) *ClientBootstrapResponse {
	if in == nil {
		return nil
	}
	models := make([]ClientModel, 0, len(in.Models))
	for _, model := range in.Models {
		models = append(models, ClientModel{
			ModelID:        model.ModelID,
			RouteModel:     model.RouteModel,
			Label:          model.Label,
			IsDefault:      model.IsDefault,
			IsAvailable:    model.IsAvailable,
			DisabledReason: model.DisabledReason,
		})
	}
	announcements := make([]ClientAnnouncement, 0, len(in.Announcements))
	for _, item := range in.Announcements {
		announcements = append(announcements, ClientAnnouncement{
			ID:       item.ID,
			Severity: item.Severity,
			Message:  item.Message,
			URL:      item.URL,
		})
	}
	return &ClientBootstrapResponse{
		Service: ClientServiceState{
			Status:     in.Service.Status,
			Message:    in.Service.Message,
			MessageKey: in.Service.MessageKey,
			ActionHint: in.Service.ActionHint,
			Retryable:  in.Service.Retryable,
			SupportURL: in.Service.SupportURL,
			ErrorCode:  in.Service.ErrorCode,
		},
		Provider: ManagedProvider{
			ProviderID:     in.Provider.ProviderID,
			DisplayName:    in.Provider.DisplayName,
			GatewayBaseURL: in.Provider.GatewayBaseURL,
			AuthMode:       in.Provider.AuthMode,
			APIKey:         in.Provider.APIKey,
			KeySummary: ManagedProviderKeyBrief{
				KeyID:      in.Provider.KeySummary.KeyID,
				MaskedKey:  in.Provider.KeySummary.MaskedKey,
				CreatedAt:  in.Provider.KeySummary.CreatedAt,
				LastUsedAt: in.Provider.KeySummary.LastUsedAt,
			},
			DefaultModel: in.Provider.DefaultModel,
		},
		Plan: ClientPlanSummary{
			PlanID:         in.Plan.PlanID,
			Name:           in.Plan.Name,
			Status:         in.Plan.Status,
			ExpiresAt:      in.Plan.ExpiresAt,
			RenewURL:       in.Plan.RenewURL,
			CommerceAction: clientActionFromService(in.Plan.CommerceAction),
		},
		Models: models,
		Usage: ClientUsageSummary{
			BalanceDisplay:     in.Usage.BalanceDisplay,
			LowBalance:         in.Usage.LowBalance,
			PeriodUsageDisplay: in.Usage.PeriodUsageDisplay,
			RateLimitState:     in.Usage.RateLimitState,
			RenewAction:        clientActionFromService(in.Usage.RenewAction),
		},
		FeatureFlags: ClientFeatureFlags{
			AdvancedProviderConfig:  in.FeatureFlags.AdvancedProviderConfig,
			InstallAssistant:        in.FeatureFlags.InstallAssistant,
			NewUserTutorial:         in.FeatureFlags.NewUserTutorial,
			ModelSelector:           in.FeatureFlags.ModelSelector,
			DiagnosticExport:        in.FeatureFlags.DiagnosticExport,
			Announcements:           in.FeatureFlags.Announcements,
			ForceUpdatePrompt:       in.FeatureFlags.ForceUpdatePrompt,
			StrictDeviceEnforcement: in.FeatureFlags.StrictDeviceEnforcement,
		},
		Announcements: announcements,
		VersionPolicy: ClientVersionPolicy{
			ConfigVersion:        in.VersionPolicy.ConfigVersion,
			SnapshotVersion:      in.VersionPolicy.SnapshotVersion,
			RefreshTTLSeconds:    in.VersionPolicy.RefreshTTLSeconds,
			ForceRefresh:         in.VersionPolicy.ForceRefresh,
			MinimumClientVersion: in.VersionPolicy.MinimumClientVersion,
		},
		Device: ClientDevice{
			DeviceID:        in.Device.DeviceID,
			Status:          in.Device.Status,
			Message:         in.Device.Message,
			SnapshotVersion: in.Device.SnapshotVersion,
		},
	}
}

func ClientUsageFromService(in *service.CodexPlusUsageSnapshot) *ClientUsageSnapshot {
	if in == nil {
		return nil
	}
	return &ClientUsageSnapshot{
		ServiceStatus: in.ServiceStatus,
		BalanceSummary: ClientBalanceSummary{
			BalanceDisplay: in.BalanceSummary.BalanceDisplay,
			LowBalance:     in.BalanceSummary.LowBalance,
		},
		PeriodUsage: ClientPeriodUsageSummary{
			PeriodID:     in.PeriodUsage.PeriodID,
			UsageDisplay: in.PeriodUsage.UsageDisplay,
			ResetAt:      in.PeriodUsage.ResetAt,
		},
		RateLimitState:  in.RateLimitState,
		RenewAction:     clientActionFromService(in.RenewAction),
		LastUpdatedAt:   in.LastUpdatedAt,
		SnapshotVersion: in.SnapshotVersion,
	}
}

func clientActionFromService(in service.CodexPlusClientAction) ClientAction {
	return ClientAction{
		ActionType:    in.ActionType,
		MessageKey:    in.MessageKey,
		ActionCopyKey: in.ActionCopyKey,
		Label:         in.Label,
		URL:           in.URL,
	}
}

func ClientDeviceFromService(in *service.CodexPlusDeviceSnapshot) *ClientDevice {
	if in == nil {
		return nil
	}
	return &ClientDevice{
		DeviceID:        in.DeviceID,
		Status:          in.Status,
		Message:         in.Message,
		SnapshotVersion: in.SnapshotVersion,
	}
}

func ClientRedeemFromService(in *service.CodexPlusRedeemResult) *ClientRedeemResponse {
	if in == nil {
		return nil
	}
	return &ClientRedeemResponse{
		RedeemStatus:            in.RedeemStatus,
		EntitlementDeltaSummary: in.EntitlementDeltaSummary,
		ServiceStatusAfter:      in.ServiceStatusAfter,
		SnapshotVersion:         in.SnapshotVersion,
		Message:                 in.Message,
	}
}
