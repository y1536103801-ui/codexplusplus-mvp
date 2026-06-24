package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type fakeCodexPlusSettingRepo struct {
	values map[string]string
	err    error
}

func newFakeCodexPlusSettingRepo() *fakeCodexPlusSettingRepo {
	return &fakeCodexPlusSettingRepo{values: map[string]string{}}
}

func (r *fakeCodexPlusSettingRepo) Get(ctx context.Context, key string) (*Setting, error) {
	value, err := r.GetValue(ctx, key)
	if err != nil {
		return nil, err
	}
	return &Setting{Key: key, Value: value}, nil
}

func (r *fakeCodexPlusSettingRepo) GetValue(ctx context.Context, key string) (string, error) {
	if r.err != nil {
		return "", r.err
	}
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (r *fakeCodexPlusSettingRepo) Set(ctx context.Context, key, value string) error {
	if r.err != nil {
		return r.err
	}
	r.values[key] = value
	return nil
}

func (r *fakeCodexPlusSettingRepo) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	out := map[string]string{}
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (r *fakeCodexPlusSettingRepo) SetMultiple(ctx context.Context, settings map[string]string) error {
	for key, value := range settings {
		r.values[key] = value
	}
	return nil
}

func (r *fakeCodexPlusSettingRepo) GetAll(ctx context.Context) (map[string]string, error) {
	out := map[string]string{}
	for key, value := range r.values {
		out[key] = value
	}
	return out, nil
}

func (r *fakeCodexPlusSettingRepo) Delete(ctx context.Context, key string) error {
	delete(r.values, key)
	return nil
}

func TestCodexPlusConfigServiceEnsureDefault(t *testing.T) {
	repo := newFakeCodexPlusSettingRepo()
	svc := NewCodexPlusConfigService(repo)
	svc.now = func() time.Time { return time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC) }

	cfg, err := svc.EnsureDefault(context.Background())
	if err != nil {
		t.Fatalf("EnsureDefault() error = %v", err)
	}
	if cfg.ConfigVersion != CodexPlusConfigVersionMVP {
		t.Fatalf("ConfigVersion = %q", cfg.ConfigVersion)
	}
	if _, ok := repo.values[CodexPlusConfigSettingKey]; !ok {
		t.Fatalf("default config was not persisted")
	}
}

func TestCodexPlusConfigServiceGetRepairsHiddenDefaultModel(t *testing.T) {
	repo := newFakeCodexPlusSettingRepo()
	cfg := DefaultCodexPlusConfig(time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC))
	cfg.ModelCatalog.Models[0].IsHidden = true
	cfg.UsagePolicy.Policies[0].CopyKeys = CodexPlusUsagePolicyCopyKeys{}
	cfg.UsagePolicy.Policies[0].InsufficientBalanceMessage = "Codex++ entitlement is not active."
	cfg.UsagePolicy.Policies[0].RateLimitedMessage = "Codex++ usage is temporarily limited."
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	repo.values[CodexPlusConfigSettingKey] = string(raw)

	svc := NewCodexPlusConfigService(repo)
	got, err := svc.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.ModelCatalog.Models[0].IsHidden {
		t.Fatalf("default model remained hidden")
	}
}

func TestCodexPlusConfigValidationRejectsDuplicateModel(t *testing.T) {
	cfg := DefaultCodexPlusConfig(time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC))
	cfg.ModelCatalog.Models = append(cfg.ModelCatalog.Models, cfg.ModelCatalog.Models[0])

	err := ValidateCodexPlusConfig(&cfg)
	if !errors.Is(err, ErrCodexPlusConfigInvalid) {
		t.Fatalf("ValidateCodexPlusConfig() error = %v, want ErrCodexPlusConfigInvalid", err)
	}
}

func TestCodexPlusConfigPublishNormalizesMetadata(t *testing.T) {
	repo := newFakeCodexPlusSettingRepo()
	svc := NewCodexPlusConfigService(repo)
	svc.now = func() time.Time { return time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC) }
	cfg := DefaultCodexPlusConfig(time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC))
	cfg.UpdatedBy = ""
	cfg.UpdatedAt = ""
	cfg.ChangeReason = ""

	published, err := svc.Publish(context.Background(), cfg, "admin", "set default")
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if published.UpdatedBy != "admin" {
		t.Fatalf("UpdatedBy = %q", published.UpdatedBy)
	}
	if published.ChangeReason != "set default" {
		t.Fatalf("ChangeReason = %q", published.ChangeReason)
	}
	if _, err := time.Parse(time.RFC3339, published.UpdatedAt); err != nil {
		t.Fatalf("UpdatedAt is not RFC3339: %v", err)
	}
	if published.PlanCatalog.DraftStatus != "published" {
		t.Fatalf("PlanCatalog.DraftStatus = %q", published.PlanCatalog.DraftStatus)
	}
	if published.ModelCatalog.DraftStatus != "published" {
		t.Fatalf("ModelCatalog.DraftStatus = %q", published.ModelCatalog.DraftStatus)
	}
	if published.UsagePolicy.DraftStatus != "published" {
		t.Fatalf("UsagePolicy.DraftStatus = %q", published.UsagePolicy.DraftStatus)
	}
	if published.FeatureFlags.DraftStatus != "published" {
		t.Fatalf("FeatureFlags.DraftStatus = %q", published.FeatureFlags.DraftStatus)
	}
}

func TestCodexPlusConfigDefaultUsesRegistryCatalogs(t *testing.T) {
	cfg := DefaultCodexPlusConfig(time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC))

	if err := ValidateCodexPlusConfig(&cfg); err != nil {
		t.Fatalf("ValidateCodexPlusConfig(default) error = %v", err)
	}
	if len(cfg.PlanCatalog.Plans) == 0 {
		t.Fatalf("default config has no plans")
	}
	if len(cfg.ModelCatalog.Models) == 0 {
		t.Fatalf("default config has no models")
	}
	if len(cfg.UsagePolicy.Policies) == 0 {
		t.Fatalf("default config has no usage policies")
	}

	plan := cfg.PlanCatalog.Plans[0]
	if plan.PriceAmountMinor <= 0 {
		t.Fatalf("Plan.PriceAmountMinor = %d, want backend registry price", plan.PriceAmountMinor)
	}
	if plan.UsagePolicyID == "" || !codexPlusUsagePolicyExists(&cfg, plan.UsagePolicyID) {
		t.Fatalf("Plan.UsagePolicyID = %q does not resolve", plan.UsagePolicyID)
	}
	if !defaultPlanReferencesAConfiguredModelGroup(plan, cfg.ModelCatalog.Models) {
		t.Fatalf("plan model groups %v do not reference configured model groups", plan.ModelGroups)
	}
	if len(cfg.FeatureFlags.Exposure.ClientVisible) == 0 || len(cfg.FeatureFlags.Exposure.ServerOnly) == 0 {
		t.Fatalf("feature flag exposure was not populated from registry defaults")
	}
}

func TestCodexPlusConfigValidationUsesRegistryPlanRules(t *testing.T) {
	cfg := DefaultCodexPlusConfig(time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC))
	cfg.PlanCatalog.Plans[0].DisplayPrice = ""

	err := ValidateCodexPlusConfig(&cfg)
	if !errors.Is(err, ErrCodexPlusConfigInvalid) {
		t.Fatalf("ValidateCodexPlusConfig() error = %v, want ErrCodexPlusConfigInvalid", err)
	}
}

func TestCodexPlusConfigValidationUsesRegistryFeatureExposureRules(t *testing.T) {
	cfg := DefaultCodexPlusConfig(time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC))
	cfg.FeatureFlags.Exposure.ServerOnly = nil

	err := ValidateCodexPlusConfig(&cfg)
	if !errors.Is(err, ErrCodexPlusConfigInvalid) {
		t.Fatalf("ValidateCodexPlusConfig() error = %v, want ErrCodexPlusConfigInvalid", err)
	}
}

func defaultPlanReferencesAConfiguredModelGroup(plan CodexPlusPlan, models []CodexPlusModel) bool {
	modelGroups := map[string]struct{}{}
	for _, model := range models {
		modelGroups[model.ModelGroup] = struct{}{}
	}
	for _, group := range plan.ModelGroups {
		if _, ok := modelGroups[group]; ok {
			return true
		}
	}
	return false
}
