package service

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"time"

	"github.com/logan/cloudcode/internal/ent"
	"github.com/logan/cloudcode/internal/ent/chatmessage"
	"github.com/logan/cloudcode/internal/ent/conversation"
	entuser "github.com/logan/cloudcode/internal/ent/user"
)

// ConversationService manages chat conversations and messages.
type ConversationService struct {
	db *ent.Client
}

// NewConversationService creates a new ConversationService.
func NewConversationService(db *ent.Client) *ConversationService {
	return &ConversationService{db: db}
}

// ConversationResponse is the API response for a conversation.
type ConversationResponse struct {
	ID          int       `json:"id"`
	ProjectPath string    `json:"project_path"`
	Title       string    `json:"title"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ChatMessageResponse is the API response for a chat message.
type ChatMessageResponse struct {
	ID         int             `json:"id"`
	Role       string          `json:"role"`
	Content    string          `json:"content"`
	ToolEvents json.RawMessage `json:"tool_events,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

func toConversationResponse(c *ent.Conversation) *ConversationResponse {
	return &ConversationResponse{
		ID:          c.ID,
		ProjectPath: c.ProjectPath,
		Title:       c.Title,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

func toChatMessageResponse(m *ent.ChatMessage) *ChatMessageResponse {
	resp := &ChatMessageResponse{
		ID:        m.ID,
		Role:      string(m.Role),
		Content:   m.Content,
		CreatedAt: m.CreatedAt,
	}
	if m.ToolEvents != nil && *m.ToolEvents != "" {
		resp.ToolEvents = json.RawMessage(*m.ToolEvents)
	}
	return resp
}

// GetOrCreateByProject returns the conversation for a user+project, creating it if needed.
func (s *ConversationService) GetOrCreateByProject(ctx context.Context, userID int, projectPath string) (*ConversationResponse, error) {
	// Try to find existing
	conv, err := s.db.Conversation.Query().
		Where(
			conversation.HasOwnerWith(entuser.IDEQ(userID)),
			conversation.ProjectPathEQ(projectPath),
		).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, fmt.Errorf("query conversation: %w", err)
	}
	if conv != nil {
		return toConversationResponse(conv), nil
	}

	// Create new
	title := "General"
	if projectPath != "" {
		title = path.Base(projectPath)
	}

	conv, err = s.db.Conversation.Create().
		SetProjectPath(projectPath).
		SetTitle(title).
		SetOwnerID(userID).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create conversation: %w", err)
	}
	return toConversationResponse(conv), nil
}

// ListByUser returns all conversations for a user.
func (s *ConversationService) ListByUser(ctx context.Context, userID int) ([]*ConversationResponse, error) {
	convs, err := s.db.Conversation.Query().
		Where(conversation.HasOwnerWith(entuser.IDEQ(userID))).
		Order(ent.Desc(conversation.FieldUpdatedAt)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}

	result := make([]*ConversationResponse, len(convs))
	for i, c := range convs {
		result[i] = toConversationResponse(c)
	}
	return result, nil
}

// GetMessages returns all messages for a conversation, ordered by creation time.
func (s *ConversationService) GetMessages(ctx context.Context, conversationID int, userID int) ([]*ChatMessageResponse, error) {
	// Verify ownership
	exists, err := s.db.Conversation.Query().
		Where(
			conversation.IDEQ(conversationID),
			conversation.HasOwnerWith(entuser.IDEQ(userID)),
		).
		Exist(ctx)
	if err != nil {
		return nil, fmt.Errorf("check ownership: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("conversation not found")
	}

	msgs, err := s.db.ChatMessage.Query().
		Where(chatmessage.HasConversationWith(conversation.IDEQ(conversationID))).
		Order(ent.Asc(chatmessage.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}

	result := make([]*ChatMessageResponse, len(msgs))
	for i, m := range msgs {
		result[i] = toChatMessageResponse(m)
	}
	return result, nil
}

// AddMessage adds a message to a conversation. Returns the saved message.
func (s *ConversationService) AddMessage(ctx context.Context, conversationID int, userID int, role string, content string, toolEvents *string) (*ChatMessageResponse, error) {
	// Verify ownership
	exists, err := s.db.Conversation.Query().
		Where(
			conversation.IDEQ(conversationID),
			conversation.HasOwnerWith(entuser.IDEQ(userID)),
		).
		Exist(ctx)
	if err != nil {
		return nil, fmt.Errorf("check ownership: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("conversation not found")
	}

	create := s.db.ChatMessage.Create().
		SetRole(chatmessage.Role(role)).
		SetContent(content).
		SetConversationID(conversationID)
	if toolEvents != nil && *toolEvents != "" {
		create = create.SetToolEvents(*toolEvents)
	}

	msg, err := create.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("save message: %w", err)
	}

	// Touch conversation updated_at
	_ = s.db.Conversation.UpdateOneID(conversationID).Exec(ctx)

	return toChatMessageResponse(msg), nil
}

// DeleteConversation deletes a conversation and all its messages.
func (s *ConversationService) DeleteConversation(ctx context.Context, conversationID int, userID int) error {
	// Verify ownership
	conv, err := s.db.Conversation.Query().
		Where(
			conversation.IDEQ(conversationID),
			conversation.HasOwnerWith(entuser.IDEQ(userID)),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return fmt.Errorf("conversation not found")
		}
		return fmt.Errorf("query conversation: %w", err)
	}

	// Delete all messages first
	_, err = s.db.ChatMessage.Delete().
		Where(chatmessage.HasConversationWith(conversation.IDEQ(conv.ID))).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete messages: %w", err)
	}

	// Delete conversation
	return s.db.Conversation.DeleteOneID(conv.ID).Exec(ctx)
}
