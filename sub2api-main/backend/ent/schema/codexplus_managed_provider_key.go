package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// CodexPlusManagedProviderKey maps a user to the API key used by Codex++ Cloud.
type CodexPlusManagedProviderKey struct {
	ent.Schema
}

func (CodexPlusManagedProviderKey) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "codexplus_managed_provider_keys"},
	}
}

func (CodexPlusManagedProviderKey) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (CodexPlusManagedProviderKey) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.Int64("api_key_id"),
		field.String("managed_provider_id").MaxLen(80).Default("codex-plus-cloud"),
		field.String("display_name").MaxLen(100).Default("Codex++ Cloud"),
		field.String("key_prefix").MaxLen(32).Optional().Nillable(),
		field.String("status").MaxLen(32).Default("active"),
		field.Time("last_used_at").Optional().Nillable(),
	}
}

func (CodexPlusManagedProviderKey) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "managed_provider_id").
			Unique().
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("api_key_id").
			Unique().
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("status"),
	}
}
