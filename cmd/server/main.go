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
		return true
	},
}

func main() {
	cfg := config.Load()
	log.Printf("Starting server on port %s", cfg.Port)
	log.Printf("Allowed origins: %v", cfg.AllowedOrigins)

	hub := websocket.NewHub()
	go hub.Run()

	router := gin.Default()

	// CORS middleware
	router.Use(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

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
			"status":           "ok",
			"connectedClients": hub.GetClientCount(),
		})
	})

	// WebSocket endpoint
	router.GET("/ws", middleware.WebSocketAuthMiddleware(cfg.JWTSecret), func(c *gin.Context) {
		userID, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
			return
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("Failed to upgrade connection: %v", err)
			return
		}

		client := websocket.NewClient(hub, conn, userID.(string))
		hub.RegisterClient(client)
		client.Start()
	})

	// Broadcast task updates to all connected clients
	router.POST("/api/broadcast", middleware.AuthMiddleware(cfg.JWTSecret), func(c *gin.Context) {
		var payload struct {
			Type string      `json:"type" binding:"required"`
			Data interface{} `json:"data" binding:"required"`
		}

		if err := c.BindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		msgType := websocket.MessageType(payload.Type)

		if err := hub.BroadcastTaskUpdate(msgType, payload.Data); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to broadcast message"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"clients": hub.GetClientCount(),
		})
	})

	// Broadcast message to a specific room / channel
	router.POST("/api/broadcast/channel", middleware.AuthMiddleware(cfg.JWTSecret), func(c *gin.Context) {
		var payload struct {
			ChannelID string      `json:"channelId" binding:"required"`
			Type      string      `json:"type" binding:"required"`
			Data      interface{} `json:"data" binding:"required"`
		}

		if err := c.BindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		roomID := "channel:" + payload.ChannelID
		msgType := websocket.MessageType(payload.Type)

		if err := hub.BroadcastToRoom(roomID, msgType, payload.Data); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to broadcast to channel"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	// Broadcast message directly to a specific user (DMs and notifications)
	router.POST("/api/broadcast/user", middleware.AuthMiddleware(cfg.JWTSecret), func(c *gin.Context) {
		var payload struct {
			UserID string      `json:"userId" binding:"required"`
			Type   string      `json:"type" binding:"required"`
			Data   interface{} `json:"data" binding:"required"`
		}

		if err := c.BindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		msgType := websocket.MessageType(payload.Type)

		if err := hub.SendToUser(payload.UserID, msgType, payload.Data); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send message to user"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
