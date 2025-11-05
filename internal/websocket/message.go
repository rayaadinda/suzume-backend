package websocket

import "encoding/json"

// MessageType represents the type of WebSocket message
type MessageType string

const (
	MessageTypeTaskCreated MessageType = "task_created"
	MessageTypeTaskUpdated MessageType = "task_updated"
	MessageTypeTaskDeleted MessageType = "task_deleted"
	MessageTypeTaskStatusChanged MessageType = "task_status_changed"
	MessageTypePing MessageType = "ping"
	MessageTypePong MessageType = "pong"
)

// Message represents a WebSocket message
type Message struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// TaskPayload represents the payload for task-related messages
type TaskPayload struct {
	ID          string   `json:"id"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	StatusID    string   `json:"statusId,omitempty"`
	Priority    string   `json:"priority,omitempty"`
	Assignees   []string `json:"assignees,omitempty"`
	Labels      []string `json:"labels,omitempty"`
}

// NewTaskMessage creates a new task message
func NewTaskMessage(msgType MessageType, payload interface{}) (*Message, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &Message{
		Type:    msgType,
		Payload: payloadBytes,
	}, nil
}
