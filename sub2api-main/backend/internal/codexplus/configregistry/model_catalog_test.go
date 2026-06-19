package configregistry

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestDefaultAndSampleModelCatalogValidate(t *testing.T) {
	for name, catalog := range map[string]ModelCatalog{
		"default": DefaultModelCatalog(),
		"sample":  SampleModelCatalog(),
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateModelCatalog(catalog); err != nil {
				t.Fatalf("ValidateModelCatalog() error = %v", err)
			}
		})
	}
}

func TestModelCatalogJSONIncludesRequiredNullableFields(t *testing.T) {
	payload, err := json.Marshal(DefaultModelCatalog())
	if err != nil {
		t.Fatalf("json.Marshal(DefaultModelCatalog()) error = %v", err)
	}
	raw := string(payload)
	for _, requiredNullable := range []string{
		`"disabled_reason":null`,
		`"fallback_model_id":null`,
		`"deprecation_at":null`,
		`"disabled_replacement_model_id":null`,
		`"disabled_message_key":null`,
	} {
		if !strings.Contains(raw, requiredNullable) {
			t.Fatalf("marshaled catalog missing %s in %s", requiredNullable, raw)
		}
	}
}

func TestValidateModelCatalogAllowsSchemaDraftStatuses(t *testing.T) {
	for _, status := range []string{
		ModelCatalogDraftStatusEditing,
		ModelCatalogDraftStatusReadyForReview,
		ModelCatalogDraftStatusApproved,
		ModelCatalogDraftStatusPublished,
		ModelCatalogDraftStatusArchived,
	} {
		t.Run(status, func(t *testing.T) {
			catalog := DefaultModelCatalog()
			catalog.DraftStatus = status
			if err := ValidateModelCatalog(catalog); err != nil {
				t.Fatalf("ValidateModelCatalog() error = %v", err)
			}
		})
	}
}

func TestValidateModelCatalogRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*ModelCatalog)
		wantField string
	}{
		{
			name: "missing model_id",
			mutate: func(c *ModelCatalog) {
				c.Models[0].ModelID = ""
			},
			wantField: "models[0].model_id",
		},
		{
			name: "duplicate model_id",
			mutate: func(c *ModelCatalog) {
				c.Models[1].ModelID = c.Models[0].ModelID
			},
			wantField: "models[1].model_id",
		},
		{
			name: "missing display_name",
			mutate: func(c *ModelCatalog) {
				c.Models[0].DisplayName = " "
			},
			wantField: "models[0].display_name",
		},
		{
			name: "missing route_model",
			mutate: func(c *ModelCatalog) {
				c.Models[0].RouteModel = ""
			},
			wantField: "models[0].route_model",
		},
		{
			name: "missing model_group",
			mutate: func(c *ModelCatalog) {
				c.Models[0].ModelGroup = " "
			},
			wantField: "models[0].model_group",
		},
		{
			name: "context too small",
			mutate: func(c *ModelCatalog) {
				c.Models[0].ContextWindow = ModelCatalogMinContextWindow - 1
			},
			wantField: "models[0].context_window",
		},
		{
			name: "billing multiplier non-positive",
			mutate: func(c *ModelCatalog) {
				c.Models[0].BillingMultiplier = 0
			},
			wantField: "models[0].billing_multiplier",
		},
		{
			name: "multiple defaults",
			mutate: func(c *ModelCatalog) {
				c.Models[1].IsDefault = true
			},
			wantField: "models",
		},
		{
			name: "default disabled",
			mutate: func(c *ModelCatalog) {
				c.Models[0].IsEnabled = false
				c.Models[0].DisabledReason = stringRef("Temporarily disabled for maintenance")
				c.Models[0].DisabledMessageKey = stringRef("model.codex-standard.disabled")
			},
			wantField: "models[0].is_enabled",
		},
		{
			name: "default hidden",
			mutate: func(c *ModelCatalog) {
				c.Models[0].IsHidden = true
			},
			wantField: "models[0].is_hidden",
		},
		{
			name: "invalid rollout channel",
			mutate: func(c *ModelCatalog) {
				c.Models[0].RolloutChannel = "beta"
			},
			wantField: "models[0].rollout_channel",
		},
		{
			name: "invalid quality tier",
			mutate: func(c *ModelCatalog) {
				c.Models[0].QualityTier = "gold"
			},
			wantField: "models[0].quality_tier",
		},
		{
			name: "fallback model missing",
			mutate: func(c *ModelCatalog) {
				c.Models[1].FallbackModelID = stringRef("missing-model")
			},
			wantField: "models[1].fallback_model_id",
		},
		{
			name: "deprecated model missing deprecation_at",
			mutate: func(c *ModelCatalog) {
				c.Models[1].RolloutChannel = RolloutChannelDeprecated
				c.Models[1].DeprecationAt = nil
			},
			wantField: "models[1].deprecation_at",
		},
		{
			name: "disabled model missing message key",
			mutate: func(c *ModelCatalog) {
				c.Models[1].IsEnabled = false
				c.Models[1].DisabledReason = stringRef("Disabled until upstream routing recovers")
				c.Models[1].DisabledMessageKey = nil
			},
			wantField: "models[1].disabled_message_key",
		},
		{
			name: "disabled replacement model missing",
			mutate: func(c *ModelCatalog) {
				c.Models[1].IsEnabled = false
				c.Models[1].DisabledReason = stringRef("Disabled until upstream routing recovers")
				c.Models[1].DisabledMessageKey = stringRef("model.codex-pro.disabled")
				c.Models[1].DisabledReplacementModelID = stringRef("missing-model")
			},
			wantField: "models[1].disabled_replacement_model_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			catalog := DefaultModelCatalog()
			tt.mutate(&catalog)
			err := ValidateModelCatalog(catalog)
			requireValidationField(t, err, tt.wantField)
		})
	}
}

func TestModelCatalogHelpers(t *testing.T) {
	catalog := SampleModelCatalog()

	defaultModel, ok := catalog.DefaultModel()
	if !ok {
		t.Fatal("DefaultModel() ok = false")
	}
	if defaultModel.ModelID != "codex-standard" {
		t.Fatalf("DefaultModel() model_id = %q", defaultModel.ModelID)
	}

	if model, ok := catalog.ModelByID(" codex-pro "); !ok || model.ModelID != "codex-pro" {
		t.Fatalf("ModelByID() = (%+v, %v), want codex-pro", model, ok)
	}

	enabled := catalog.EnabledModels()
	if len(enabled) != 3 {
		t.Fatalf("EnabledModels() len = %d, want 3", len(enabled))
	}
	visible := catalog.VisibleEnabledModels()
	if len(visible) != 2 {
		t.Fatalf("VisibleEnabledModels() len = %d, want 2", len(visible))
	}

	groups := catalog.ModelsByGroup()
	if got := len(groups["codex"]); got != 3 {
		t.Fatalf("ModelsByGroup()[codex] len = %d, want 3", got)
	}
	if got := len(groups["legacy"]); got != 1 {
		t.Fatalf("ModelsByGroup()[legacy] len = %d, want 1", got)
	}
}

func requireValidationField(t *testing.T, err error, wantField string) {
	t.Helper()
	if err == nil {
		t.Fatalf("ValidateModelCatalog() error = nil, want field %s", wantField)
	}
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("ValidateModelCatalog() error = %T, want *ValidationError", err)
	}
	for _, field := range validationErr.Fields {
		if field.Field == wantField {
			return
		}
	}
	t.Fatalf("ValidateModelCatalog() fields = %+v, want %s", validationErr.Fields, wantField)
}
