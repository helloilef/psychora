package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GET /api/psy/conversations/:patientId
func (h *Handler) LoadConversation(c *gin.Context) {
	userID := c.GetString("userID")
	patientID := c.Param("patientId")

	// Auth: verify the patient belongs to this doctor via patients table
	rows, err := h.DB.Query(`
        SELECT role, content, timestamp
        FROM messages
        WHERE patient_id = $1
          AND EXISTS (
              SELECT 1 FROM patients p
              WHERE p.id = $1 AND p.user_id = $2
          )
        ORDER BY created_at ASC`,
		patientID, userID,
	)
	if err != nil {
		c.JSON(500, gin.H{"message": "Error fetching messages"})
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

	c.JSON(200, messages)
}

// POST /api/psy/conversations
func (h *Handler) SaveConversation(c *gin.Context) {
	userID := c.GetString("userID")

	var req struct {
		PatientID string                   `json:"patientId" binding:"required"`
		Messages  []map[string]interface{} `json:"messages" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}

	// Auth: verify ownership before touching any data
	var ownerID string
	err := h.DB.QueryRow(`SELECT user_id FROM patients WHERE id = $1`, req.PatientID).Scan(&ownerID)
	if err != nil || ownerID != userID {
		c.JSON(403, gin.H{"message": "Access denied"})
		return
	}

	_, err = h.DB.Exec(`DELETE FROM messages WHERE patient_id = $1`, req.PatientID)
	if err != nil {
		c.JSON(500, gin.H{"message": "Error clearing old messages"})
		return
	}

	for _, msg := range req.Messages {
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)
		timestamp, _ := msg["timestamp"].(string)

		_, err := h.DB.Exec(`
            INSERT INTO messages (id, patient_id, role, content, timestamp, created_at)
            VALUES ($1, $2, $3, $4, $5, NOW())`,
			uuid.New().String(), req.PatientID, role, content, timestamp,
		)
		if err != nil {
			c.JSON(500, gin.H{"message": "Error saving messages"})
			return
		}
	}

	c.JSON(200, gin.H{"message": "Conversation saved"})
}

// DELETE /api/psy/conversations/:patientId
func (h *Handler) ClearConversation(c *gin.Context) {
	userID := c.GetString("userID")
	patientID := c.Param("patientId")

	// Auth: verify ownership
	var ownerID string
	err := h.DB.QueryRow(`SELECT user_id FROM patients WHERE id = $1`, patientID).Scan(&ownerID)
	if err != nil || ownerID != userID {
		c.JSON(403, gin.H{"message": "Access denied"})
		return
	}

	_, err = h.DB.Exec(`DELETE FROM messages WHERE patient_id = $1`, patientID)
	if err != nil {
		c.JSON(500, gin.H{"message": "Error clearing conversation"})
		return
	}

	c.JSON(200, gin.H{"message": "Conversation cleared"})
}

// POST /api/psy/conversations/message
func (h *Handler) SendMessage(c *gin.Context) {
	userID := c.GetString("userID")

	var req struct {
		PatientID string                   `json:"patientId" binding:"required"`
		Message   string                   `json:"message" binding:"required"`
		History   []map[string]interface{} `json:"history"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}

	// Auth: verify the patient belongs to this doctor
	var ownerID string
	err := h.DB.QueryRow(`SELECT user_id FROM patients WHERE id = $1`, req.PatientID).Scan(&ownerID)
	if err != nil || ownerID != userID {
		c.JSON(403, gin.H{"message": "Access denied"})
		return
	}

	// Save user message
	_, err = h.DB.Exec(`
        INSERT INTO messages (id, patient_id, role, content, timestamp, created_at)
        VALUES ($1, $2, 'user', $3, NOW()::text, NOW())`,
		uuid.New().String(), req.PatientID, req.Message,
	)
	if err != nil {
		c.JSON(500, gin.H{"message": "Error saving user message"})
		return
	}

	// Call Python AI
	aiReply, err := callPythonAI(req.Message, req.PatientID)
	if err != nil {
		c.JSON(500, gin.H{"message": "AI service error: " + err.Error()})
		return
	}

	// Save AI response
	_, err = h.DB.Exec(`
        INSERT INTO messages (id, patient_id, role, content, timestamp, created_at)
        VALUES ($1, $2, 'assistant', $3, NOW()::text, NOW())`,
		uuid.New().String(), req.PatientID, aiReply,
	)
	if err != nil {
		c.JSON(500, gin.H{"message": "Error saving AI response"})
		return
	}

	c.JSON(200, gin.H{"response": aiReply})
}
