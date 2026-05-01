package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GET /api/psy/student/messages
func (h *Handler) LoadStudentConversation(c *gin.Context) {
	userID := c.GetString("userID")

	rows, err := h.DB.Query(`
        SELECT role, content, timestamp
        FROM student_messages
        WHERE user_id = $1
        ORDER BY created_at ASC`,
		userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error fetching messages"})
		return
	}
	defer rows.Close()

	messages := []map[string]interface{}{}
	for rows.Next() {
		var role, content, timestamp string
		if err := rows.Scan(&role, &content, &timestamp); err != nil {
			continue
		}
		messages = append(messages, map[string]interface{}{
			"role":      role,
			"content":   content,
			"timestamp": timestamp,
		})
	}

	c.JSON(http.StatusOK, messages)
}

// POST /api/psy/student/message
// Matches what ChatGeneralServices sends: { message, history }
func (h *Handler) SendStudentMessage(c *gin.Context) {
	userID := c.GetString("userID")

	var req struct {
		Message string                   `json:"message" binding:"required"`
		History []map[string]interface{} `json:"history"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	// Save user message
	_, err := h.DB.Exec(`
        INSERT INTO student_messages (id, user_id, role, content, timestamp, created_at)
        VALUES ($1, $2, 'user', $3, NOW()::text, NOW())`,
		uuid.New().String(), userID, req.Message,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error saving user message"})
		return
	}

	// Call Python AI — use userID as session so the AI has per-student context
	aiReply, err := callPythonAI(req.Message, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "AI service error: " + err.Error()})
		return
	}

	// Save AI response
	_, err = h.DB.Exec(`
        INSERT INTO student_messages (id, user_id, role, content, timestamp, created_at)
        VALUES ($1, $2, 'assistant', $3, NOW()::text, NOW())`,
		uuid.New().String(), userID, aiReply,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error saving AI response"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"response": aiReply})
}

// DELETE /api/psy/student/messages
func (h *Handler) ClearStudentConversation(c *gin.Context) {
	userID := c.GetString("userID")

	_, err := h.DB.Exec(`DELETE FROM student_messages WHERE user_id = $1`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error clearing conversation"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Conversation cleared"})
}
