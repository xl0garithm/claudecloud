package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// User holds the schema definition for the User entity.
type User struct {
	ent.Schema
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("email").
			Unique().
			NotEmpty(),
		field.String("api_key").
			Unique().
			Optional().
			Nillable(),
		field.String("name").
			Optional().
			Default(""),
		field.String("stripe_customer_id").
			Unique().
			Optional().
			Nillable(),
		field.String("stripe_subscription_id").
			Optional().
			Nillable(),
		field.String("subscription_status").
			Default("inactive"),
		field.String("plan").
			Default("free"),
		field.Float("usage_hours").
			Default(0),
		field.String("anthropic_api_key").
			Optional().
			Nillable().
			Sensitive().
			Comment("User's Anthropic API key for Claude Code (API pay-as-you-go billing)"),
		field.String("claude_oauth_token").
			Optional().
			Nillable().
			Sensitive().
			Comment("User's Claude.ai OAuth token for Claude Code (Pro/Max subscription billing)"),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("instances", Instance.Type),
		edge.To("conversations", Conversation.Type),
	}
}
