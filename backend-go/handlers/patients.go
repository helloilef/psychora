package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"backend/models"
)

// GET /api/psy/patients
func (h *Handler) GetPatients(c *gin.Context) {
	userID := c.GetString("userID")

	rows, err := h.DB.Query(`
		SELECT id, user_id, name, age, condition, last_seen, sessions_count, created_at
		FROM patients
		WHERE user_id = $1
		ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error fetching patients"})
		return
	}
	defer rows.Close()

	patients := []models.Patient{}
	for rows.Next() {
		var p models.Patient
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.Age, &p.Condition, &p.LastSeen, &p.SessionsCount, &p.CreatedAt); err != nil {
			continue
		}
		patients = append(patients, p)
	}

	c.JSON(http.StatusOK, patients)
}

// GET /api/psy/patients/:id
func (h *Handler) GetPatient(c *gin.Context) {
	userID := c.GetString("userID")
	patientID := c.Param("id")

	var p models.Patient
	err := h.DB.QueryRow(`
		SELECT id, user_id, name, age, condition, last_seen, sessions_count, created_at
		FROM patients
		WHERE id = $1 AND user_id = $2`,
		patientID, userID,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.Age, &p.Condition, &p.LastSeen, &p.SessionsCount, &p.CreatedAt)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"message": "Patient not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database error"})
		return
	}

	c.JSON(http.StatusOK, p)
}

// POST /api/psy/patients
func (h *Handler) CreatePatient(c *gin.Context) {
	userID := c.GetString("userID")

	var req models.CreatePatientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	p := models.Patient{
		ID:            uuid.New().String(),
		UserID:        userID,
		Name:          req.Name,
		Age:           req.Age,
		Condition:     req.Condition,
		LastSeen:      req.LastSeen,
		SessionsCount: req.SessionsCount,
		CreatedAt:     time.Now(),
	}

	_, err := h.DB.Exec(`
		INSERT INTO patients (id, user_id, name, age, condition, last_seen, sessions_count, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		p.ID, p.UserID, p.Name, p.Age, p.Condition, p.LastSeen, p.SessionsCount, p.CreatedAt,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error creating patient"})
		return
	}

	c.JSON(http.StatusCreated, p)
}

// PUT /api/psy/patients/:id
func (h *Handler) UpdatePatient(c *gin.Context) {
	userID := c.GetString("userID")
	patientID := c.Param("id")

	var ownerID string
	err := h.DB.QueryRow(`SELECT user_id FROM patients WHERE id = $1`, patientID).Scan(&ownerID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"message": "Patient not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database error"})
		return
	}
	if ownerID != userID {
		c.JSON(http.StatusForbidden, gin.H{"message": "Access denied"})
		return
	}

	var req models.UpdatePatientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	var p models.Patient
	err = h.DB.QueryRow(`
		UPDATE patients
		SET name = COALESCE(NULLIF($1, ''), name),
		    age = CASE WHEN $2 = 0 THEN age ELSE $2 END,
		    condition = COALESCE(NULLIF($3, ''), condition),
		    last_seen = COALESCE($4, last_seen),
		    sessions_count = CASE WHEN $5 = 0 THEN sessions_count ELSE $5 END
		WHERE id = $6
		RETURNING id, user_id, name, age, condition, last_seen, sessions_count, created_at`,
		req.Name, req.Age, req.Condition, req.LastSeen, req.SessionsCount, patientID,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.Age, &p.Condition, &p.LastSeen, &p.SessionsCount, &p.CreatedAt)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error updating patient"})
		return
	}

	c.JSON(http.StatusOK, p)
}

// DELETE /api/psy/patients/:id
func (h *Handler) DeletePatient(c *gin.Context) {
	userID := c.GetString("userID")
	patientID := c.Param("id")

	result, err := h.DB.Exec(`
		DELETE FROM patients WHERE id = $1 AND user_id = $2`,
		patientID, userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error deleting patient"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"message": "Patient not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Patient deleted successfully"})
}

// POST /api/psy/patients/notes  or  PUT /api/psy/patients/notes/:patientId  or  PUT /api/psy/patients/:id/notes
func (h *Handler) SavePatientNote(c *gin.Context) {
	userID := c.GetString("userID")

	patientID := c.Param("patientId")
	if patientID == "" {
		patientID = c.Param("id")
	}

	var req struct {
		PatientID string `json:"patientId"`
		Note      string `json:"note" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	if patientID == "" {
		patientID = req.PatientID
	}

	// Auth: verify this patient belongs to the requesting doctor
	var ownerID string
	err := h.DB.QueryRow(`SELECT user_id FROM patients WHERE id = $1`, patientID).Scan(&ownerID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"message": "Patient not found"})
		return
	}
	if err != nil || ownerID != userID {
		c.JSON(http.StatusForbidden, gin.H{"message": "Access denied"})
		return
	}

	_, err = h.DB.Exec(`
        INSERT INTO patient_notes (id, patient_id, note, created_at)
        VALUES ($1, $2, $3, NOW())`,
		uuid.New().String(), patientID, req.Note,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error saving note"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Note saved"})
}

// GET /api/psy/patients/notes/:patientId
func (h *Handler) GetPatientNotes(c *gin.Context) {
	userID := c.GetString("userID")
	patientID := c.Param("patientId")

	// Auth via JOIN — only return notes for patients this doctor owns
	rows, err := h.DB.Query(`
        SELECT pn.id, pn.note, pn.created_at
        FROM patient_notes pn
        JOIN patients p ON p.id = pn.patient_id
        WHERE pn.patient_id = $1 AND p.user_id = $2
        ORDER BY pn.created_at DESC`,
		patientID, userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error fetching notes"})
		return
	}
	defer rows.Close()

	notes := []map[string]interface{}{}
	for rows.Next() {
		var id, note, createdAt string
		rows.Scan(&id, &note, &createdAt)
		notes = append(notes, map[string]interface{}{
			"id": id, "note": note, "createdAt": createdAt,
		})
	}
	c.JSON(http.StatusOK, notes)
}

// GET /api/psy/patients/notes  (optionally ?patientId=...)
func (h *Handler) GetAllPatientNotes(c *gin.Context) {
	userID := c.GetString("userID")
	patientID := c.Query("patientId")

	var rows *sql.Rows
	var err error

	if patientID != "" {
		rows, err = h.DB.Query(`
            SELECT pn.id, pn.patient_id, pn.note, pn.created_at
            FROM patient_notes pn
            JOIN patients p ON p.id = pn.patient_id
            WHERE p.user_id = $1 AND pn.patient_id = $2
            ORDER BY pn.created_at DESC`,
			userID, patientID,
		)
	} else {
		rows, err = h.DB.Query(`
            SELECT pn.id, pn.patient_id, pn.note, pn.created_at
            FROM patient_notes pn
            JOIN patients p ON p.id = pn.patient_id
            WHERE p.user_id = $1
            ORDER BY pn.created_at DESC`,
			userID,
		)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error fetching notes"})
		return
	}
	defer rows.Close()

	notes := []map[string]interface{}{}
	for rows.Next() {
		var id, patID, note, createdAt string
		rows.Scan(&id, &patID, &note, &createdAt)
		notes = append(notes, map[string]interface{}{
			"id":        id,
			"patientId": patID,
			"note":      note,
			"createdAt": createdAt,
		})
	}

	c.JSON(http.StatusOK, notes)
}

// DELETE /api/psy/patients/notes/:noteId
func (h *Handler) DeletePatientNote(c *gin.Context) {
	userID := c.GetString("userID")
	noteID := c.Param("noteId")

	// Auth via JOIN — only delete if the note's patient belongs to this doctor
	result, err := h.DB.Exec(`
        DELETE FROM patient_notes
        WHERE id = $1
          AND EXISTS (
              SELECT 1 FROM patients p
              WHERE p.id = patient_notes.patient_id AND p.user_id = $2
          )`,
		noteID, userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error deleting note"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"message": "Note not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Note deleted successfully"})
}
