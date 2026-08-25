package ent

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// AuditMixin 提供 Ent 审计字段与回填 hook。
type AuditMixin struct {
	ent.Mixin
}

// Fields 返回审计字段定义。
func (AuditMixin) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("created_by").Default(0).Comment("创建人ID"),
		field.Int64("updated_by").Default(0).Comment("更新人ID"),
		field.Time("created_at").Default(time.Now).Immutable().Comment("创建时间"),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).Comment("更新时间"),
	}
}

// Hooks 返回审计字段回填 hook。
func (AuditMixin) Hooks() []ent.Hook {
	return []ent.Hook{
		AuditHook(),
	}
}

// Annotations 返回审计字段注释落库配置。
func (AuditMixin) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.WithComments(true),
	}
}
