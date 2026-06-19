package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// CodexPlusDevice stores Codex++ desktop device state.
type CodexPlusDevice struct {
	ent.Schema
}

func (CodexPlusDevice) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "codexplus_devices"},
	}
}

func (CodexPlusDevice) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (CodexPlusDevice) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("device_id").MaxLen(128).NotEmpty(),
		field.String("device_name").MaxLen(160).Optional().Nillable(),
		field.String("platform").MaxLen(40).Optional().Nillable(),
		field.String("app_version").MaxLen(64).Optional().Nillable(),
		field.String("fingerprint_hash").MaxLen(128).Optional().Nillable(),
		field.String("status").MaxLen(32).Default("active"),
		field.Time("last_seen_at").Optional().Nillable(),
		field.Time("revoked_at").Optional().Nillable(),
		field.JSON("metadata", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
	}
}

func (CodexPlusDevice) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "device_id").
			Unique().
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("user_id"),
		index.Fields("status"),
		index.Fields("last_seen_at"),
	}
}
