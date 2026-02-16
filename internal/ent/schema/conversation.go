package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Conversation holds the schema definition for the Conversation entity.
type Conversation struct {
	ent.Schema
}

// Fields of the Conversation.
func (Conversation) Fields() []ent.Field {
	return []ent.Field{
		field.String("project_path").
			Default("").
			Comment("Project directory path (e.g. 'myrepo'). Empty string for general chat."),
		field.String("title").
			Default("").
			Comment("Display title, auto-set from project name"),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the Conversation.
func (Conversation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("owner", User.Type).
			Ref("conversations").
			Unique().
			Required(),
		edge.To("messages", ChatMessage.Type),
	}
}

// Indexes of the Conversation.
func (Conversation) Indexes() []ent.Index {
	return []ent.Index{
		// One conversation per project per user
		index.Edges("owner").
			Fields("project_path").
			Unique(),
	}
}
