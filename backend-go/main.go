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
	// Load .env
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	// Connect to Supabase
	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal("Error connecting to database:", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal("Cannot reach database:", err)
	}
	log.Println("✅ Connected to database")

	// Setup handlers
	h := handlers.NewHandler(db)

	// Setup router
	r := gin.Default()

	// CORS — needed for Flutter
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
	protected := r.Group("/api")
	protected.Use(middleware.AuthMiddleware())
	{
		protected.POST("/conversations", h.CreateConversation)
		protected.GET("/conversations", h.GetConversations)
		protected.POST("/conversations/:id/messages", h.SendMessage)
		protected.GET("/conversations/:id/messages", h.GetMessages)
	}

	port := os.Getenv("PORT")
	log.Println("🚀 Server running on port", port)
	r.Run(":" + port)
}
