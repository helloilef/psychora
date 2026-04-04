package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"backend/models"
)

func (h *Handler) CreateConversation(c *gin.Context) {
	userID := c.GetString("userID")

	id := uuid.New().String()
	_, err := h.DB.Exec(`
		INSERT INTO conversations (id, user_id, title, created_at)
		VALUES ($1, $2, $3, $4)`,
		id, userID, "New Conversation", time.Now(),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error creating conversation"})
		return
	}

	c.JSON(http.StatusCreated, models.Conversation{
		ID:        id,
		UserID:    userID,
		Title:     "New Conversation",
		CreatedAt: time.Now(),
	})
}

func (h *Handler) GetConversations(c *gin.Context) {
	userID := c.GetString("userID")

	rows, err := h.DB.Query(`
		SELECT id, user_id, title, created_at 
		FROM conversations 
		WHERE user_id=$1 
		ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error fetching conversations"})
		return
	}
	defer rows.Close()

	conversations := []models.Conversation{}
	for rows.Next() {
		var conv models.Conversation
		if err := rows.Scan(&conv.ID, &conv.UserID, &conv.Title, &conv.CreatedAt); err != nil {
			continue
		}
		conversations = append(conversations, conv)
	}

	c.JSON(http.StatusOK, conversations)
}

func (h *Handler) GetMessages(c *gin.Context) {
	conversationID := c.Param("id")

	rows, err := h.DB.Query(`
		SELECT id, conversation_id, role, content, created_at 
		FROM messages 
		WHERE conversation_id=$1 
		ORDER BY created_at ASC`,
		conversationID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error fetching messages"})
		return
	}
	defer rows.Close()

	messages := []models.Message{}
	for rows.Next() {
		var msg models.Message
		if err := rows.Scan(&msg.ID, &msg.ConversationID, &msg.Role, &msg.Content, &msg.CreatedAt); err != nil {
			continue
		}
		messages = append(messages, msg)
	}

	c.JSON(http.StatusOK, messages)
}

func (h *Handler) SendMessage(c *gin.Context) {
	conversationID := c.Param("id")

	var req models.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	// Save user message
	userMsgID := uuid.New().String()
	_, err := h.DB.Exec(`
		INSERT INTO messages (id, conversation_id, role, content, created_at)
		VALUES ($1, $2, $3, $4, $5)`,
		userMsgID, conversationID, "user", req.Content, time.Now(),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error saving message"})
		return
	}

	// Call Python FastAPI
	aiResponse, err := callPythonAI(req.Content, conversationID)
	if err != nil {
		aiResponse = "I'm sorry, I'm having trouble responding right now."
	}

	// Save assistant message
	assistantMsgID := uuid.New().String()
	_, err = h.DB.Exec(`
		INSERT INTO messages (id, conversation_id, role, content, created_at)
		VALUES ($1, $2, $3, $4, $5)`,
		assistantMsgID, conversationID, "assistant", aiResponse, time.Now(),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error saving response"})
		return
	}

	c.JSON(http.StatusOK, models.ChatResponse{
		UserMessage: models.Message{
			ID:             userMsgID,
			ConversationID: conversationID,
			Role:           "user",
			Content:        req.Content,
			CreatedAt:      time.Now(),
		},
		AssistantMessage: models.Message{
			ID:             assistantMsgID,
			ConversationID: conversationID,
			Role:           "assistant",
			Content:        aiResponse,
			CreatedAt:      time.Now(),
		},
	})
}
