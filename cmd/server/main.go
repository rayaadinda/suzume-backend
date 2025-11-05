package main

import (
	"log"
	"net/http"
	"task-management-backend/internal/config"
	"task-management-backend/internal/middleware"
	"task-management-backend/internal/websocket"

	"github.com/gin-gonic/gin"
	gorilla_websocket "github.com/gorilla/websocket"
)

var upgrader = gorilla_websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for development
		// In production, check against cfg.AllowedOrigins
		return true
	},
}

func main() {
	// Load configuration
	cfg := config.Load()
	log.Printf("Starting server on port %s", cfg.Port)
	log.Printf("Allowed origins: %v", cfg.AllowedOrigins)

	// Create WebSocket hub
	hub := websocket.NewHub()
	go hub.Run()

	// Create Gin router
	router := gin.Default()

	// CORS middleware
	router.Use(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range cfg.AllowedOrigins {
			if origin == allowedOrigin || allowedOrigin == "*" {
				allowed = true
				break
			}
		}

		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":          "ok",
			"connectedClients": hub.GetClientCount(),
		})
	})

	// WebSocket endpoint (requires authentication)
	router.GET("/ws", middleware.WebSocketAuthMiddleware(cfg.JWTSecret), func(c *gin.Context) {
		// Get user ID from context (set by auth middleware)
		userID, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
			return
		}

		// Upgrade HTTP connection to WebSocket
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("Failed to upgrade connection: %v", err)
			return
		}

		// Create and register new client
		client := websocket.NewClient(hub, conn, userID.(string))
		hub.RegisterClient(client)

		// Start client's read and write pumps
		client.Start()
	})

	// API endpoint to broadcast task updates (for Next.js Server Actions to call)
	router.POST("/api/broadcast", middleware.AuthMiddleware(cfg.JWTSecret), func(c *gin.Context) {
		var payload struct {
			Type string      `json:"type" binding:"required"`
			Data interface{} `json:"data" binding:"required"`
		}

		if err := c.BindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Convert type string to MessageType
		msgType := websocket.MessageType(payload.Type)

		// Broadcast the message
		if err := hub.BroadcastTaskUpdate(msgType, payload.Data); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to broadcast message"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Message broadcasted",
			"clients": hub.GetClientCount(),
		})
	})

	// Start server
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
