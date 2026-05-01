package main

import (
	"database/sql"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"backend/handlers"
	"backend/middleware"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal("Error connecting to database:", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal("Cannot reach database:", err)
	}
	log.Println("✅ Connected to database")

	h := handlers.NewHandler(db)

	r := gin.Default()

	// CORS
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Public routes
	api := r.Group("/api/psy")
	{
		api.POST("/signup", h.Signup)
		api.POST("/login", h.Login)
	}

	// Protected routes
	protected := r.Group("/api/psy")
	protected.Use(middleware.AuthMiddleware())
	{
		// Auth & User
		protected.GET("/users/me", h.GetCurrentUser)
		protected.PUT("/users/me", h.UpdateCurrentUser)
		protected.POST("/logout", h.Logout)

		// Patients (doctors only)
		protected.GET("/patients", h.GetPatients)
		protected.POST("/patients", h.CreatePatient)
		protected.GET("/patients/:id", h.GetPatient)
		protected.PUT("/patients/:id", h.UpdatePatient)
		protected.DELETE("/patients/:id", h.DeletePatient)

		// Notes — specific routes MUST be registered before /patients/:id to avoid conflicts
		protected.GET("/patients/notes", h.GetAllPatientNotes)
		protected.GET("/patients/notes/:patientId", h.GetPatientNotes)
		protected.POST("/patients/notes", h.SavePatientNote)
		protected.PUT("/patients/notes/:patientId", h.SavePatientNote)
		protected.PUT("/patients/:id/notes", h.SavePatientNote)
		protected.DELETE("/patients/notes/:noteId", h.DeletePatientNote)

		// Doctor conversations (tied to a patient)
		protected.POST("/conversations/message", h.SendMessage) // MUST be before /:patientId
		protected.GET("/conversations/:patientId", h.LoadConversation)
		protected.POST("/conversations", h.SaveConversation)
		protected.DELETE("/conversations/:patientId", h.ClearConversation)

		// Student general chat (no patient — uses student_messages table)
		protected.POST("/student/message", h.SendStudentMessage)
		protected.GET("/student/messages", h.LoadStudentConversation)
		protected.DELETE("/student/messages", h.ClearStudentConversation)
	}

	port := os.Getenv("PORT")
	log.Println("🚀 Server running on port", port)
	r.Run(":" + port)
}
