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

// CodexPlusEvent is the append-only audit/event stream for Codex++.
type CodexPlusEvent struct {
	ent.Schema
}

func (CodexPlusEvent) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "codexplus_events"},
	}
}

func (CodexPlusEvent) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (CodexPlusEvent) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id").Optional().Nillable(),
		field.String("device_id").MaxLen(128).Optional().Nillable(),
		field.String("event_type").MaxLen(80).NotEmpty(),
		field.String("severity").MaxLen(24).Default("info"),
		field.String("request_id").MaxLen(128).Optional().Nillable(),
		field.String("config_version").MaxLen(64).Optional().Nillable(),
		field.JSON("payload", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
	}
}

func (CodexPlusEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("device_id"),
		index.Fields("event_type"),
		index.Fields("severity"),
		index.Fields("config_version"),
		index.Fields("created_at"),
	}
}
