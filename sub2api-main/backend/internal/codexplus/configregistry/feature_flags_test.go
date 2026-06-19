package configregistry

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDefaultFeatureFlagsValidateAndCoverSchemaFlags(t *testing.T) {
	cfg := DefaultFeatureFlags(time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC))

	if err := ValidateFeatureFlags(cfg); err != nil {
		t.Fatalf("ValidateFeatureFlags(default) error = %v", err)
	}
	if cfg.Flags.AdvancedProviderConfig {
		t.Fatalf("AdvancedProviderConfig = true, want safe default false")
	}
	if !cfg.Flags.InstallAssistant {
		t.Fatalf("InstallAssistant = false, want true")
	}
	if !cfg.Flags.NewUserTutorial {
		t.Fatalf("NewUserTutorial = false, want true")
	}
	if !cfg.Flags.ModelSelector {
		t.Fatalf("ModelSelector = false, want true")
	}
	if !cfg.Flags.DiagnosticExport {
		t.Fatalf("DiagnosticExport = false, want schema default true")
	}
	if !cfg.Flags.Announcements {
		t.Fatalf("Announcements = false, want true")
	}
	if cfg.Flags.ForceUpdatePrompt {
		t.Fatalf("ForceUpdatePrompt = true, want safe default false")
	}
	if cfg.Flags.StrictDeviceEnforcement {
		t.Fatalf("StrictDeviceEnforcement = true, want rollout default false")
	}
	assertExposure(t, cfg, FeatureFlagStrictDeviceEnforcement, "server_only")
}

func TestSampleFeatureFlagsValidate(t *testing.T) {
	cfg := SampleFeatureFlags(time.Date(2026, 6, 16, 1, 2, 3, 0, time.UTC))

	if err := ValidateFeatureFlags(cfg); err != nil {
		t.Fatalf("ValidateFeatureFlags(sample) error = %v", err)
	}
	if !cfg.Flags.AdvancedProviderConfig || !cfg.Flags.ForceUpdatePrompt || !cfg.Flags.StrictDeviceEnforcement {
		t.Fatalf("sample should enable advanced, force update, and strict rollout flags")
	}
	assertExposure(t, cfg, FeatureFlagStrictDeviceEnforcement, "server_only")
}

func TestValidateFeatureFlagsRejectsMissingCopyKey(t *testing.T) {
	cfg := DefaultFeatureFlags(time.Now())
	cfg.CopyKeys.DiagnosticExportEntry = ""

	err := ValidateFeatureFlags(cfg)
	requireFeatureFlagValidationField(t, err, "copy_keys.diagnostic_export_entry")
}

func TestValidateFeatureFlagsRejectsInvalidCopyKey(t *testing.T) {
	cfg := DefaultFeatureFlags(time.Now())
	cfg.CopyKeys.ForceUpdatePrompt = "Force update now"

	err := ValidateFeatureFlags(cfg)
	requireFeatureFlagValidationField(t, err, "copy_keys.force_update_prompt")
}

func TestValidateFeatureFlagsRejectsInvalidExposure(t *testing.T) {
	cfg := DefaultFeatureFlags(time.Now())
	cfg.Exposure.ClientVisible = append(cfg.Exposure.ClientVisible, FeatureFlagName("pricing_table"))

	err := ValidateFeatureFlags(cfg)
	requireFeatureFlagValidationField(t, err, "exposure.client_visible")
}

func TestValidateFeatureFlagsRejectsExposureOverlap(t *testing.T) {
	cfg := DefaultFeatureFlags(time.Now())
	cfg.Exposure.ServerOnly = append(cfg.Exposure.ServerOnly, FeatureFlagDiagnosticExport)

	err := ValidateFeatureFlags(cfg)
	requireFeatureFlagValidationField(t, err, "exposure.server_only")
}

func TestValidateFeatureFlagsRejectsMissingExposureClassification(t *testing.T) {
	cfg := DefaultFeatureFlags(time.Now())
	cfg.Exposure.ClientVisible = []FeatureFlagName{
		FeatureFlagAdvancedProviderConfig,
		FeatureFlagInstallAssistant,
		FeatureFlagNewUserTutorial,
		FeatureFlagModelSelector,
		FeatureFlagDiagnosticExport,
		FeatureFlagAnnouncements,
	}

	err := ValidateFeatureFlags(cfg)
	requireFeatureFlagValidationField(t, err, "exposure")
}

func TestValidateFeatureFlagsRejectsStrictDeviceClientExposure(t *testing.T) {
	cfg := DefaultFeatureFlags(time.Now())
	cfg.Exposure.ClientVisible = append(cfg.Exposure.ClientVisible, FeatureFlagStrictDeviceEnforcement)
	cfg.Exposure.ServerOnly = nil

	err := ValidateFeatureFlags(cfg)
	requireFeatureFlagValidationField(t, err, "exposure.server_only")
}

func TestValidateFeatureFlagsRejectsDiagnosticExportWithoutRedactionReady(t *testing.T) {
	cfg := DefaultFeatureFlags(time.Now())
	cfg.Semantics.DiagnosticExportRedactionReady = false

	err := ValidateFeatureFlags(cfg)
	requireFeatureFlagValidationField(t, err, "flags.diagnostic_export")
}

func TestValidateFeatureFlagsAllowsDiagnosticExportDisabledBeforeRedactionReady(t *testing.T) {
	cfg := DefaultFeatureFlags(time.Now())
	cfg.Flags.DiagnosticExport = false
	cfg.Semantics.DiagnosticExportRedactionReady = false

	if err := ValidateFeatureFlags(cfg); err != nil {
		t.Fatalf("ValidateFeatureFlags() error = %v", err)
	}
}

func TestFeatureFlagValuesUnmarshalRequiresAllFlags(t *testing.T) {
	raw := `{
		"advanced_provider_config": false,
		"install_assistant": true,
		"new_user_tutorial": true,
		"model_selector": true,
		"diagnostic_export": true,
		"announcements": true,
		"force_update_prompt": false
	}`
	var flags FeatureFlagValues

	err := json.Unmarshal([]byte(raw), &flags)
	if err == nil {
		t.Fatalf("json.Unmarshal() error = nil, want missing strict_device_enforcement")
	}
	if !strings.Contains(err.Error(), "strict_device_enforcement") {
		t.Fatalf("json.Unmarshal() error = %v, want strict_device_enforcement", err)
	}
}

func assertExposure(t *testing.T, cfg FeatureFlags, flag FeatureFlagName, want string) {
	t.Helper()
	got := ""
	for _, candidate := range cfg.Exposure.ClientVisible {
		if candidate == flag {
			got = "client_visible"
		}
	}
	for _, candidate := range cfg.Exposure.ServerOnly {
		if candidate == flag {
			got = "server_only"
		}
	}
	if got != want {
		t.Fatalf("exposure for %s = %q, want %q", flag, got, want)
	}
}

func requireFeatureFlagValidationField(t *testing.T, err error, field string) {
	t.Helper()
	if err == nil {
		t.Fatalf("ValidateFeatureFlags() error = nil, want field %s", field)
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("ValidateFeatureFlags() error = %T, want *ValidationError", err)
	}
	for _, item := range ve.Fields {
		if item.Field == field {
			return
		}
	}
	t.Fatalf("ValidateFeatureFlags() fields = %#v, want %s", ve.Fields, field)
}
