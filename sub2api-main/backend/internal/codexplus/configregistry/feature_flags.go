package configregistry

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

const (
	FeatureFlagAdvancedProviderConfig  FeatureFlagName = "advanced_provider_config"
	FeatureFlagInstallAssistant        FeatureFlagName = "install_assistant"
	FeatureFlagNewUserTutorial         FeatureFlagName = "new_user_tutorial"
	FeatureFlagModelSelector           FeatureFlagName = "model_selector"
	FeatureFlagDiagnosticExport        FeatureFlagName = "diagnostic_export"
	FeatureFlagAnnouncements           FeatureFlagName = "announcements"
	FeatureFlagForceUpdatePrompt       FeatureFlagName = "force_update_prompt"
	FeatureFlagStrictDeviceEnforcement FeatureFlagName = "strict_device_enforcement"

	FeatureFlagsDraftStatusEditing        = "editing"
	FeatureFlagsDraftStatusReadyForReview = "ready_for_review"
	FeatureFlagsDraftStatusApproved       = "approved"
	FeatureFlagsDraftStatusPublished      = "published"
	FeatureFlagsDraftStatusArchived       = "archived"
)

var (
	allFeatureFlagNames = []FeatureFlagName{
		FeatureFlagAdvancedProviderConfig,
		FeatureFlagInstallAssistant,
		FeatureFlagNewUserTutorial,
		FeatureFlagModelSelector,
		FeatureFlagDiagnosticExport,
		FeatureFlagAnnouncements,
		FeatureFlagForceUpdatePrompt,
		FeatureFlagStrictDeviceEnforcement,
	}

	clientVisibleFeatureFlags = []FeatureFlagName{
		FeatureFlagAdvancedProviderConfig,
		FeatureFlagInstallAssistant,
		FeatureFlagNewUserTutorial,
		FeatureFlagModelSelector,
		FeatureFlagDiagnosticExport,
		FeatureFlagAnnouncements,
		FeatureFlagForceUpdatePrompt,
	}

	featureFlagsCopyKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{2,127}$`)
)

type FeatureFlagName string

type FeatureFlags struct {
	DocumentMeta
	Flags     FeatureFlagValues            `json:"flags"`
	Exposure  FeatureFlagExposure          `json:"exposure"`
	CopyKeys  FeatureFlagCopyKeys          `json:"copy_keys"`
	Semantics FeatureFlagsRuntimeSemantics `json:"-"`
}

type FeatureFlagValues struct {
	AdvancedProviderConfig  bool `json:"advanced_provider_config"`
	InstallAssistant        bool `json:"install_assistant"`
	NewUserTutorial         bool `json:"new_user_tutorial"`
	ModelSelector           bool `json:"model_selector"`
	DiagnosticExport        bool `json:"diagnostic_export"`
	Announcements           bool `json:"announcements"`
	ForceUpdatePrompt       bool `json:"force_update_prompt"`
	StrictDeviceEnforcement bool `json:"strict_device_enforcement"`
}

type FeatureFlagExposure struct {
	ClientVisible []FeatureFlagName `json:"client_visible"`
	ServerOnly    []FeatureFlagName `json:"server_only"`
}

type FeatureFlagCopyKeys struct {
	ForceUpdatePrompt     string `json:"force_update_prompt"`
	InstallAssistantEntry string `json:"install_assistant_entry"`
	NewUserTutorialEntry  string `json:"new_user_tutorial_entry"`
	DiagnosticExportEntry string `json:"diagnostic_export_entry"`
	AnnouncementEntry     string `json:"announcement_entry"`
}

type FeatureFlagsRuntimeSemantics struct {
	DiagnosticExportRedactionReady bool
}

func DefaultFeatureFlags(now time.Time) FeatureFlags {
	metaTime := featureFlagsTimestamp(now)
	cfg := FeatureFlags{
		DocumentMeta: DocumentMeta{
			ConfigVersion: "feature-flags-default-v1",
			DraftStatus:   FeatureFlagsDraftStatusPublished,
			PublishScope:  PublishScopeProduction,
			UpdatedBy:     "system",
			UpdatedAt:     metaTime.Format(time.RFC3339),
			ChangeReason:  "default safe Codex++ feature flag config",
		},
		Flags: FeatureFlagValues{
			AdvancedProviderConfig:  false,
			InstallAssistant:        true,
			NewUserTutorial:         true,
			ModelSelector:           true,
			DiagnosticExport:        true,
			Announcements:           true,
			ForceUpdatePrompt:       false,
			StrictDeviceEnforcement: false,
		},
		Exposure: FeatureFlagExposure{
			ClientVisible: append([]FeatureFlagName(nil), clientVisibleFeatureFlags...),
			ServerOnly:    []FeatureFlagName{FeatureFlagStrictDeviceEnforcement},
		},
		CopyKeys: FeatureFlagCopyKeys{
			ForceUpdatePrompt:     "codexplus.update.force_prompt",
			InstallAssistantEntry: "codexplus.install_assistant.entry",
			NewUserTutorialEntry:  "codexplus.tutorial.new_user_entry",
			DiagnosticExportEntry: "codexplus.diagnostics.redacted_export_entry",
			AnnouncementEntry:     "codexplus.announcements.entry",
		},
		Semantics: FeatureFlagsRuntimeSemantics{
			DiagnosticExportRedactionReady: true,
		},
	}
	return cfg
}

func SampleFeatureFlags(now time.Time) FeatureFlags {
	cfg := DefaultFeatureFlags(now)
	cfg.ConfigVersion = "feature-flags-sample-v1"
	cfg.DraftStatus = FeatureFlagsDraftStatusReadyForReview
	cfg.PublishScope = PublishScopeInternal
	cfg.ChangeReason = "sample Codex++ feature flag config for admin review"
	cfg.Flags.AdvancedProviderConfig = true
	cfg.Flags.ForceUpdatePrompt = true
	cfg.Flags.StrictDeviceEnforcement = true
	return cfg
}

func AllFeatureFlagNames() []FeatureFlagName {
	return append([]FeatureFlagName(nil), allFeatureFlagNames...)
}

func ValidateFeatureFlags(cfg FeatureFlags) error {
	ve := NewValidation("feature_flags")
	validateFeatureFlagsMeta(ve, cfg.DocumentMeta)
	validateFeatureFlagExposure(ve, cfg.Exposure)
	validateFeatureFlagCopyKeys(ve, cfg.CopyKeys)
	if cfg.Flags.DiagnosticExport && !cfg.Semantics.DiagnosticExportRedactionReady {
		ve.Add("flags.diagnostic_export", "diagnostic_export requires redaction-ready diagnostics before it can be enabled")
	}
	return ve.Err()
}

func (v *FeatureFlagValues) UnmarshalJSON(data []byte) error {
	var raw struct {
		AdvancedProviderConfig  *bool `json:"advanced_provider_config"`
		InstallAssistant        *bool `json:"install_assistant"`
		NewUserTutorial         *bool `json:"new_user_tutorial"`
		ModelSelector           *bool `json:"model_selector"`
		DiagnosticExport        *bool `json:"diagnostic_export"`
		Announcements           *bool `json:"announcements"`
		ForceUpdatePrompt       *bool `json:"force_update_prompt"`
		StrictDeviceEnforcement *bool `json:"strict_device_enforcement"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	missing := []string{}
	if raw.AdvancedProviderConfig == nil {
		missing = append(missing, "advanced_provider_config")
	}
	if raw.InstallAssistant == nil {
		missing = append(missing, "install_assistant")
	}
	if raw.NewUserTutorial == nil {
		missing = append(missing, "new_user_tutorial")
	}
	if raw.ModelSelector == nil {
		missing = append(missing, "model_selector")
	}
	if raw.DiagnosticExport == nil {
		missing = append(missing, "diagnostic_export")
	}
	if raw.Announcements == nil {
		missing = append(missing, "announcements")
	}
	if raw.ForceUpdatePrompt == nil {
		missing = append(missing, "force_update_prompt")
	}
	if raw.StrictDeviceEnforcement == nil {
		missing = append(missing, "strict_device_enforcement")
	}
	if len(missing) > 0 {
		return &ValidationError{
			Scope: "feature_flags.flags",
			Fields: []FieldError{{
				Field:   "flags",
				Message: "missing required feature flags: " + strings.Join(missing, ", "),
			}},
		}
	}
	v.AdvancedProviderConfig = *raw.AdvancedProviderConfig
	v.InstallAssistant = *raw.InstallAssistant
	v.NewUserTutorial = *raw.NewUserTutorial
	v.ModelSelector = *raw.ModelSelector
	v.DiagnosticExport = *raw.DiagnosticExport
	v.Announcements = *raw.Announcements
	v.ForceUpdatePrompt = *raw.ForceUpdatePrompt
	v.StrictDeviceEnforcement = *raw.StrictDeviceEnforcement
	return nil
}

func validateFeatureFlagsMeta(ve *ValidationError, meta DocumentMeta) {
	if strings.TrimSpace(meta.ConfigVersion) == "" {
		ve.Add("config_version", "config_version is required")
	}
	if !validFeatureFlagsDraftStatus(meta.DraftStatus) {
		ve.Add("draft_status", "draft_status must be editing, ready_for_review, approved, published, or archived")
	}
	if !ValidPublishScope(meta.PublishScope) {
		ve.Add("publish_scope", "publish_scope must be draft, internal, canary, or production")
	}
	if strings.TrimSpace(meta.UpdatedBy) == "" {
		ve.Add("updated_by", "updated_by is required")
	}
	changeReason := strings.TrimSpace(meta.ChangeReason)
	if changeReason == "" {
		ve.Add("change_reason", "change_reason is required")
	} else if len(changeReason) > 500 {
		ve.Add("change_reason", "change_reason must be at most 500 characters")
	}
	if strings.TrimSpace(meta.UpdatedAt) == "" {
		ve.Add("updated_at", "updated_at is required")
	} else if _, err := time.Parse(time.RFC3339, meta.UpdatedAt); err != nil {
		ve.Add("updated_at", "updated_at must be RFC3339")
	}
}

func validateFeatureFlagExposure(ve *ValidationError, exposure FeatureFlagExposure) {
	seen := map[FeatureFlagName]string{}
	for _, flag := range exposure.ClientVisible {
		if !validFeatureFlagName(flag) {
			ve.Add("exposure.client_visible", "client_visible contains unknown feature flag "+string(flag))
			continue
		}
		if prev, ok := seen[flag]; ok {
			ve.Add("exposure.client_visible", string(flag)+" is already listed in "+prev)
			continue
		}
		seen[flag] = "client_visible"
	}
	for _, flag := range exposure.ServerOnly {
		if !validFeatureFlagName(flag) {
			ve.Add("exposure.server_only", "server_only contains unknown feature flag "+string(flag))
			continue
		}
		if prev, ok := seen[flag]; ok {
			ve.Add("exposure.server_only", string(flag)+" is already listed in "+prev)
			continue
		}
		seen[flag] = "server_only"
	}
	for _, flag := range allFeatureFlagNames {
		if _, ok := seen[flag]; !ok {
			ve.Add("exposure", string(flag)+" must be classified as client_visible or server_only")
		}
	}
	for _, flag := range clientVisibleFeatureFlags {
		if seen[flag] != "client_visible" {
			ve.Add("exposure.client_visible", string(flag)+" must remain client_visible because clients only use it to show or hide entry points")
		}
	}
	if seen[FeatureFlagStrictDeviceEnforcement] != "server_only" {
		ve.Add("exposure.server_only", "strict_device_enforcement must remain server_only because it is a gateway rollout switch")
	}
}

func validateFeatureFlagCopyKeys(ve *ValidationError, keys FeatureFlagCopyKeys) {
	validateFeatureFlagCopyKey(ve, "copy_keys.force_update_prompt", keys.ForceUpdatePrompt)
	validateFeatureFlagCopyKey(ve, "copy_keys.install_assistant_entry", keys.InstallAssistantEntry)
	validateFeatureFlagCopyKey(ve, "copy_keys.new_user_tutorial_entry", keys.NewUserTutorialEntry)
	validateFeatureFlagCopyKey(ve, "copy_keys.diagnostic_export_entry", keys.DiagnosticExportEntry)
	validateFeatureFlagCopyKey(ve, "copy_keys.announcement_entry", keys.AnnouncementEntry)
}

func validateFeatureFlagCopyKey(ve *ValidationError, field, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		ve.Add(field, field+" is required")
		return
	}
	if !featureFlagsCopyKeyPattern.MatchString(value) {
		ve.Add(field, field+" must be a stable copy key")
	}
}

func validFeatureFlagsDraftStatus(value string) bool {
	switch strings.TrimSpace(value) {
	case FeatureFlagsDraftStatusEditing,
		FeatureFlagsDraftStatusReadyForReview,
		FeatureFlagsDraftStatusApproved,
		FeatureFlagsDraftStatusPublished,
		FeatureFlagsDraftStatusArchived:
		return true
	default:
		return false
	}
}

func validFeatureFlagName(value FeatureFlagName) bool {
	for _, flag := range allFeatureFlagNames {
		if value == flag {
			return true
		}
	}
	return false
}

func featureFlagsTimestamp(now time.Time) time.Time {
	if now.IsZero() {
		return time.Unix(0, 0).UTC()
	}
	return now.UTC()
}
