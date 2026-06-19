package configregistry

import (
	"fmt"
	"strings"
	"time"
)

const (
	PublishScopeDraft      = "draft"
	PublishScopeInternal   = "internal"
	PublishScopeCanary     = "canary"
	PublishScopeProduction = "production"

	DraftStatusDraft          = "draft"
	DraftStatusEditing        = "editing"
	DraftStatusReadyForReview = "ready_for_review"
	DraftStatusApproved       = "approved"
	DraftStatusPublished      = "published"
	DraftStatusArchived       = "archived"
)

type DocumentMeta struct {
	ConfigVersion string  `json:"config_version"`
	DraftStatus   string  `json:"draft_status"`
	PublishScope  string  `json:"publish_scope"`
	RollbackFrom  *string `json:"rollback_from"`
	UpdatedBy     string  `json:"updated_by"`
	UpdatedAt     string  `json:"updated_at"`
	ChangeReason  string  `json:"change_reason"`
}

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ValidationError struct {
	Scope  string       `json:"scope"`
	Fields []FieldError `json:"fields"`
}

func (e *ValidationError) Error() string {
	if e == nil {
		return "codexplus config validation failed"
	}
	if len(e.Fields) == 0 {
		return fmt.Sprintf("codexplus %s validation failed", e.Scope)
	}
	return fmt.Sprintf("codexplus %s validation failed: %s", e.Scope, e.Fields[0].Message)
}

func NewValidation(scope string) *ValidationError {
	return &ValidationError{Scope: scope}
}

func (e *ValidationError) Add(field, message string) {
	e.Fields = append(e.Fields, FieldError{Field: field, Message: message})
}

func (e *ValidationError) Err() error {
	if e == nil || len(e.Fields) == 0 {
		return nil
	}
	return e
}

func ValidateMeta(scope string, meta DocumentMeta) error {
	ve := NewValidation(scope)
	if strings.TrimSpace(meta.ConfigVersion) == "" {
		ve.Add("config_version", "config_version is required")
	}
	if !ValidDraftStatus(meta.DraftStatus) {
		ve.Add("draft_status", "draft_status must be draft, editing, ready_for_review, approved, published, or archived")
	}
	if !ValidPublishScope(meta.PublishScope) {
		ve.Add("publish_scope", "publish_scope must be draft, internal, canary, or production")
	}
	if strings.TrimSpace(meta.UpdatedBy) == "" {
		ve.Add("updated_by", "updated_by is required")
	}
	if strings.TrimSpace(meta.ChangeReason) == "" {
		ve.Add("change_reason", "change_reason is required")
	}
	if strings.TrimSpace(meta.UpdatedAt) == "" {
		ve.Add("updated_at", "updated_at is required")
	} else if _, err := time.Parse(time.RFC3339, meta.UpdatedAt); err != nil {
		ve.Add("updated_at", "updated_at must be RFC3339")
	}
	return ve.Err()
}

func ValidDraftStatus(value string) bool {
	switch strings.TrimSpace(value) {
	case DraftStatusDraft, DraftStatusEditing, DraftStatusReadyForReview, DraftStatusApproved, DraftStatusPublished, DraftStatusArchived:
		return true
	default:
		return false
	}
}

func ValidPublishScope(value string) bool {
	switch strings.TrimSpace(value) {
	case PublishScopeDraft, PublishScopeInternal, PublishScopeCanary, PublishScopeProduction:
		return true
	default:
		return false
	}
}

func TrimmedSet(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
