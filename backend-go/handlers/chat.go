package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"backend/models"
)

// GET /api/psy/conversations/:patientId
func (h *Handler) LoadConversation(c *gin.Context) {
	userID := c.GetString("userID")
	patientID := c.Param("patientId")

	rows, err := h.DB.Query(`
        SELECT role, content, timestamp
        FROM messages
        WHERE patient_id = $1 AND user_id = $2
        ORDER BY created_at ASC`,
		patientID, userID,
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

// POST /api/psy/conversations
func (h *Handler) SaveConversation(c *gin.Context) {
	userID := c.GetString("userID")

	var req struct {
		PatientID string                  `json:"patientId" binding:"required"`
		Messages  []models.MessagePayload `json:"messages" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	// Delete existing messages for this patient+user, then re-insert
	_, err := h.DB.Exec(`
        DELETE FROM messages WHERE patient_id = $1 AND user_id = $2`,
		req.PatientID, userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error clearing old messages"})
		return
	}

	for _, msg := range req.Messages {
		_, err := h.DB.Exec(`
            INSERT INTO messages (id, patient_id, user_id, role, content, timestamp, created_at)
            VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			uuid.New().String(), req.PatientID, userID,
			msg.Role, msg.Content, msg.Timestamp, time.Now(),
		)
		if err != nil {
			continue
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Conversation saved"})
}

// DELETE /api/psy/conversations/:patientId
func (h *Handler) ClearConversation(c *gin.Context) {
	userID := c.GetString("userID")
	patientID := c.Param("patientId")

	_, err := h.DB.Exec(`
        DELETE FROM messages WHERE patient_id = $1 AND user_id = $2`,
		patientID, userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error clearing conversation"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Conversation cleared"})
}
