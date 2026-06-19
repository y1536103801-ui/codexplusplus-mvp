package configregistry

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	ModelCatalogScope            = "model_catalog"
	ModelCatalogMinContextWindow = 1024

	ModelCatalogDraftStatusEditing        = "editing"
	ModelCatalogDraftStatusReadyForReview = "ready_for_review"
	ModelCatalogDraftStatusApproved       = "approved"
	ModelCatalogDraftStatusPublished      = "published"
	ModelCatalogDraftStatusArchived       = "archived"

	RolloutChannelInternal   = "internal"
	RolloutChannelCanary     = "canary"
	RolloutChannelStable     = "stable"
	RolloutChannelDeprecated = "deprecated"

	QualityTierStandard     = "standard"
	QualityTierPremium      = "premium"
	QualityTierExperimental = "experimental"
	QualityTierLegacy       = "legacy"
)

var (
	modelCatalogModelIDPattern    = regexp.MustCompile(`^[a-zA-Z0-9._:-]{2,128}$`)
	modelCatalogModelGroupPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,63}$`)
	modelCatalogCopyKeyPattern    = regexp.MustCompile(`^[a-z][a-z0-9_.-]{2,127}$`)
)

type ModelCatalog struct {
	DocumentMeta
	Models []ModelCatalogModel `json:"models"`
}

type ModelCatalogModel struct {
	ModelID                    string   `json:"model_id"`
	DisplayName                string   `json:"display_name"`
	RouteModel                 string   `json:"route_model"`
	ModelGroup                 string   `json:"model_group"`
	ContextWindow              int      `json:"context_window"`
	BillingMultiplier          float64  `json:"billing_multiplier"`
	IsDefault                  bool     `json:"is_default"`
	IsEnabled                  bool     `json:"is_enabled"`
	IsHidden                   bool     `json:"is_hidden"`
	DisabledReason             *string  `json:"disabled_reason"`
	RolloutChannel             string   `json:"rollout_channel"`
	QualityTier                string   `json:"quality_tier"`
	FallbackModelID            *string  `json:"fallback_model_id"`
	DeprecationAt              *string  `json:"deprecation_at"`
	DisabledReplacementModelID *string  `json:"disabled_replacement_model_id"`
	DisabledMessageKey         *string  `json:"disabled_message_key"`
	SortOrder                  int      `json:"sort_order"`
	OperatorTags               []string `json:"operator_tags,omitempty"`
}

func DefaultModelCatalog() ModelCatalog {
	return ModelCatalog{
		DocumentMeta: DocumentMeta{
			ConfigVersion: "model-catalog-v1",
			DraftStatus:   ModelCatalogDraftStatusPublished,
			PublishScope:  PublishScopeProduction,
			RollbackFrom:  nil,
			UpdatedBy:     "system",
			UpdatedAt:     "2026-01-01T00:00:00Z",
			ChangeReason:  "Initial Codex++ model catalog defaults",
		},
		Models: []ModelCatalogModel{
			{
				ModelID:                    "codex-standard",
				DisplayName:                "Codex Standard",
				RouteModel:                 "gpt-5-mini",
				ModelGroup:                 "codex",
				ContextWindow:              128000,
				BillingMultiplier:          1,
				IsDefault:                  true,
				IsEnabled:                  true,
				IsHidden:                   false,
				DisabledReason:             nil,
				RolloutChannel:             RolloutChannelStable,
				QualityTier:                QualityTierStandard,
				FallbackModelID:            nil,
				DeprecationAt:              nil,
				DisabledReplacementModelID: nil,
				DisabledMessageKey:         nil,
				SortOrder:                  10,
				OperatorTags:               []string{"default", "public"},
			},
			{
				ModelID:                    "codex-pro",
				DisplayName:                "Codex Pro",
				RouteModel:                 "gpt-5",
				ModelGroup:                 "codex",
				ContextWindow:              256000,
				BillingMultiplier:          2.5,
				IsDefault:                  false,
				IsEnabled:                  true,
				IsHidden:                   false,
				DisabledReason:             nil,
				RolloutChannel:             RolloutChannelStable,
				QualityTier:                QualityTierPremium,
				FallbackModelID:            stringRef("codex-standard"),
				DeprecationAt:              nil,
				DisabledReplacementModelID: nil,
				DisabledMessageKey:         nil,
				SortOrder:                  20,
				OperatorTags:               []string{"premium"},
			},
		},
	}
}

func SampleModelCatalog() ModelCatalog {
	catalog := DefaultModelCatalog()
	catalog.ConfigVersion = "model-catalog-sample-v1"
	catalog.ChangeReason = "Sample model catalog covering hidden, deprecated, fallback, and disabled fields"
	catalog.Models = append(catalog.Models,
		ModelCatalogModel{
			ModelID:                    "codex-lab",
			DisplayName:                "Codex Lab",
			RouteModel:                 "gpt-5-codex-lab",
			ModelGroup:                 "codex",
			ContextWindow:              192000,
			BillingMultiplier:          1.75,
			IsDefault:                  false,
			IsEnabled:                  true,
			IsHidden:                   true,
			DisabledReason:             nil,
			RolloutChannel:             RolloutChannelCanary,
			QualityTier:                QualityTierExperimental,
			FallbackModelID:            stringRef("codex-standard"),
			DeprecationAt:              nil,
			DisabledReplacementModelID: nil,
			DisabledMessageKey:         nil,
			SortOrder:                  30,
			OperatorTags:               []string{"canary", "operator-preview"},
		},
		ModelCatalogModel{
			ModelID:                    "codex-legacy",
			DisplayName:                "Codex Legacy",
			RouteModel:                 "gpt-4.1",
			ModelGroup:                 "legacy",
			ContextWindow:              64000,
			BillingMultiplier:          0.75,
			IsDefault:                  false,
			IsEnabled:                  false,
			IsHidden:                   true,
			DisabledReason:             stringRef("Deprecated route retired after migration to Codex Standard"),
			RolloutChannel:             RolloutChannelDeprecated,
			QualityTier:                QualityTierLegacy,
			FallbackModelID:            stringRef("codex-standard"),
			DeprecationAt:              stringRef("2026-03-01T00:00:00Z"),
			DisabledReplacementModelID: stringRef("codex-standard"),
			DisabledMessageKey:         stringRef("model.codex-legacy.disabled"),
			SortOrder:                  90,
			OperatorTags:               []string{"deprecated"},
		},
	)
	return catalog
}

func ValidateModelCatalog(catalog ModelCatalog) error {
	ve := NewValidation(ModelCatalogScope)
	validateModelCatalogMeta(ve, catalog.DocumentMeta)

	if len(catalog.Models) == 0 {
		ve.Add("models", "models must contain at least one model")
		return ve.Err()
	}

	modelIDs := make(map[string]int, len(catalog.Models))
	defaultCount := 0
	for i, model := range catalog.Models {
		path := fmt.Sprintf("models[%d]", i)
		validateModelCatalogModelShape(ve, path, model)

		modelID := strings.TrimSpace(model.ModelID)
		if modelID != "" && modelCatalogModelIDPattern.MatchString(modelID) {
			if firstIndex, ok := modelIDs[modelID]; ok {
				ve.Add(path+".model_id", fmt.Sprintf("model_id duplicates models[%d].model_id", firstIndex))
			} else {
				modelIDs[modelID] = i
			}
		}

		if model.IsDefault {
			defaultCount++
			if !model.IsEnabled {
				ve.Add(path+".is_enabled", "default model must be enabled")
			}
			if model.IsHidden {
				ve.Add(path+".is_hidden", "default model must not be hidden")
			}
		}
	}

	switch {
	case defaultCount == 0:
		ve.Add("models", "exactly one model must be marked as default")
	case defaultCount > 1:
		ve.Add("models", "only one model can be marked as default")
	}

	for i, model := range catalog.Models {
		path := fmt.Sprintf("models[%d]", i)
		validateModelCatalogReferences(ve, path, model, modelIDs)
	}

	return ve.Err()
}

func (catalog ModelCatalog) DefaultModel() (ModelCatalogModel, bool) {
	for _, model := range catalog.Models {
		if model.IsDefault {
			return model, true
		}
	}
	return ModelCatalogModel{}, false
}

func (catalog ModelCatalog) ModelByID(modelID string) (ModelCatalogModel, bool) {
	modelID = strings.TrimSpace(modelID)
	for _, model := range catalog.Models {
		if model.ModelID == modelID {
			return model, true
		}
	}
	return ModelCatalogModel{}, false
}

func (catalog ModelCatalog) EnabledModels() []ModelCatalogModel {
	models := make([]ModelCatalogModel, 0, len(catalog.Models))
	for _, model := range catalog.Models {
		if model.IsEnabled {
			models = append(models, model)
		}
	}
	return models
}

func (catalog ModelCatalog) VisibleEnabledModels() []ModelCatalogModel {
	models := make([]ModelCatalogModel, 0, len(catalog.Models))
	for _, model := range catalog.Models {
		if model.IsEnabled && !model.IsHidden {
			models = append(models, model)
		}
	}
	return models
}

func (catalog ModelCatalog) ModelsByGroup() map[string][]ModelCatalogModel {
	groups := make(map[string][]ModelCatalogModel)
	for _, model := range catalog.Models {
		group := strings.TrimSpace(model.ModelGroup)
		if group == "" {
			continue
		}
		groups[group] = append(groups[group], model)
	}
	return groups
}

func validateModelCatalogMeta(ve *ValidationError, meta DocumentMeta) {
	if strings.TrimSpace(meta.ConfigVersion) == "" {
		ve.Add("config_version", "config_version is required")
	}
	if !ValidModelCatalogDraftStatus(meta.DraftStatus) {
		ve.Add("draft_status", "draft_status must be editing, ready_for_review, approved, published, or archived")
	}
	if !ValidPublishScope(meta.PublishScope) {
		ve.Add("publish_scope", "publish_scope must be draft, internal, canary, or production")
	}
	if strings.TrimSpace(meta.UpdatedBy) == "" {
		ve.Add("updated_by", "updated_by is required")
	}
	if strings.TrimSpace(meta.UpdatedAt) == "" {
		ve.Add("updated_at", "updated_at is required")
	} else if _, err := time.Parse(time.RFC3339, meta.UpdatedAt); err != nil {
		ve.Add("updated_at", "updated_at must be RFC3339")
	}
	reason := strings.TrimSpace(meta.ChangeReason)
	if reason == "" {
		ve.Add("change_reason", "change_reason is required")
	} else if utf8.RuneCountInString(reason) > 500 {
		ve.Add("change_reason", "change_reason must be at most 500 characters")
	}
}

func validateModelCatalogModelShape(ve *ValidationError, path string, model ModelCatalogModel) {
	modelID := strings.TrimSpace(model.ModelID)
	if modelID == "" {
		ve.Add(path+".model_id", "model_id is required")
	} else if !modelCatalogModelIDPattern.MatchString(modelID) {
		ve.Add(path+".model_id", "model_id must match ^[a-zA-Z0-9._:-]{2,128}$")
	}

	displayName := strings.TrimSpace(model.DisplayName)
	if displayName == "" {
		ve.Add(path+".display_name", "display_name is required")
	} else if utf8.RuneCountInString(displayName) > 80 {
		ve.Add(path+".display_name", "display_name must be at most 80 characters")
	}

	if strings.TrimSpace(model.RouteModel) == "" {
		ve.Add(path+".route_model", "route_model is required")
	}
	modelGroup := strings.TrimSpace(model.ModelGroup)
	if modelGroup == "" {
		ve.Add(path+".model_group", "model_group is required")
	} else if !modelCatalogModelGroupPattern.MatchString(modelGroup) {
		ve.Add(path+".model_group", "model_group must match ^[a-z][a-z0-9_-]{1,63}$")
	}
	if model.ContextWindow < ModelCatalogMinContextWindow {
		ve.Add(path+".context_window", "context_window must be at least 1024")
	}
	if model.BillingMultiplier <= 0 || math.IsNaN(model.BillingMultiplier) || math.IsInf(model.BillingMultiplier, 0) {
		ve.Add(path+".billing_multiplier", "billing_multiplier must be a positive finite number")
	}
	if model.DisabledReason != nil && utf8.RuneCountInString(*model.DisabledReason) > 160 {
		ve.Add(path+".disabled_reason", "disabled_reason must be at most 160 characters")
	}
	if !model.IsEnabled {
		if model.DisabledReason == nil || strings.TrimSpace(*model.DisabledReason) == "" {
			ve.Add(path+".disabled_reason", "disabled model requires disabled_reason")
		}
		if model.DisabledMessageKey == nil || !modelCatalogCopyKeyPattern.MatchString(strings.TrimSpace(*model.DisabledMessageKey)) {
			ve.Add(path+".disabled_message_key", "disabled model requires a valid disabled_message_key")
		}
	} else if model.DisabledMessageKey != nil && strings.TrimSpace(*model.DisabledMessageKey) != "" && !modelCatalogCopyKeyPattern.MatchString(strings.TrimSpace(*model.DisabledMessageKey)) {
		ve.Add(path+".disabled_message_key", "disabled_message_key must match ^[a-z][a-z0-9_.-]{2,127}$")
	}
	if !ValidModelCatalogRolloutChannel(model.RolloutChannel) {
		ve.Add(path+".rollout_channel", "rollout_channel must be internal, canary, stable, or deprecated")
	}
	if !ValidModelCatalogQualityTier(model.QualityTier) {
		ve.Add(path+".quality_tier", "quality_tier must be standard, premium, experimental, or legacy")
	}
	if model.RolloutChannel == RolloutChannelDeprecated {
		if model.DeprecationAt == nil || strings.TrimSpace(*model.DeprecationAt) == "" {
			ve.Add(path+".deprecation_at", "deprecated model requires deprecation_at")
		}
	}
	if model.DeprecationAt != nil && strings.TrimSpace(*model.DeprecationAt) != "" {
		if _, err := time.Parse(time.RFC3339, strings.TrimSpace(*model.DeprecationAt)); err != nil {
			ve.Add(path+".deprecation_at", "deprecation_at must be RFC3339")
		}
	}
	validateOptionalModelID(ve, path+".fallback_model_id", model.FallbackModelID)
	validateOptionalModelID(ve, path+".disabled_replacement_model_id", model.DisabledReplacementModelID)
	if model.SortOrder < 0 {
		ve.Add(path+".sort_order", "sort_order must be at least 0")
	}
	validateOperatorTags(ve, path+".operator_tags", model.OperatorTags)
}

func validateModelCatalogReferences(ve *ValidationError, path string, model ModelCatalogModel, modelIDs map[string]int) {
	modelID := strings.TrimSpace(model.ModelID)
	validateModelCatalogReference(ve, path+".fallback_model_id", modelID, model.FallbackModelID, modelIDs)
	validateModelCatalogReference(ve, path+".disabled_replacement_model_id", modelID, model.DisabledReplacementModelID, modelIDs)
}

func validateModelCatalogReference(ve *ValidationError, field, sourceModelID string, targetRef *string, modelIDs map[string]int) {
	if targetRef == nil {
		return
	}
	targetModelID := strings.TrimSpace(*targetRef)
	if targetModelID == "" || !modelCatalogModelIDPattern.MatchString(targetModelID) {
		return
	}
	if _, ok := modelIDs[targetModelID]; !ok {
		ve.Add(field, "referenced model_id must exist in models")
		return
	}
	if sourceModelID != "" && targetModelID == sourceModelID {
		ve.Add(field, "referenced model_id must not point to itself")
	}
}

func validateOptionalModelID(ve *ValidationError, field string, modelID *string) {
	if modelID == nil {
		return
	}
	value := strings.TrimSpace(*modelID)
	if value == "" {
		ve.Add(field, "model_id reference must be null or non-empty")
		return
	}
	if !modelCatalogModelIDPattern.MatchString(value) {
		ve.Add(field, "model_id reference must match ^[a-zA-Z0-9._:-]{2,128}$")
	}
}

func validateOperatorTags(ve *ValidationError, field string, tags []string) {
	seen := make(map[string]int, len(tags))
	for i, tag := range tags {
		tagPath := fmt.Sprintf("%s[%d]", field, i)
		tag = strings.TrimSpace(tag)
		if tag == "" {
			ve.Add(tagPath, "operator tag must be non-empty")
			continue
		}
		if utf8.RuneCountInString(tag) > 40 {
			ve.Add(tagPath, "operator tag must be at most 40 characters")
		}
		if firstIndex, ok := seen[tag]; ok {
			ve.Add(tagPath, fmt.Sprintf("operator tag duplicates %s[%d]", field, firstIndex))
		} else {
			seen[tag] = i
		}
	}
}

func ValidModelCatalogDraftStatus(value string) bool {
	switch strings.TrimSpace(value) {
	case ModelCatalogDraftStatusEditing,
		ModelCatalogDraftStatusReadyForReview,
		ModelCatalogDraftStatusApproved,
		ModelCatalogDraftStatusPublished,
		ModelCatalogDraftStatusArchived:
		return true
	default:
		return false
	}
}

func ValidModelCatalogRolloutChannel(value string) bool {
	switch strings.TrimSpace(value) {
	case RolloutChannelInternal, RolloutChannelCanary, RolloutChannelStable, RolloutChannelDeprecated:
		return true
	default:
		return false
	}
}

func ValidModelCatalogQualityTier(value string) bool {
	switch strings.TrimSpace(value) {
	case QualityTierStandard, QualityTierPremium, QualityTierExperimental, QualityTierLegacy:
		return true
	default:
		return false
	}
}

func stringRef(value string) *string {
	return &value
}
