package sharing

import (
	"encoding/json"
	"time"
)

type Share struct {
	ID         string
	ShareToken string
	ProjectID  string
	Title      string
	IsActive   bool
	ViewCount  int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Project struct {
	ID           string
	UserID       string
	Title        string
	DiagramType  string
	ProjectMode  string
	Metadata     json.RawMessage
	IsPinned     bool
	CurrentNodes json.RawMessage
	CurrentEdges json.RawMessage
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Message struct {
	ID            string
	ProjectID     string
	Role          string
	Content       string
	TargetNodeIDs json.RawMessage
	Metadata      json.RawMessage
	CreatedAt     time.Time
}

type NewShare struct {
	Token     string
	ProjectID string
	Title     string
}

type CloneInput struct {
	Original Project
	Messages []Message
	Title    string
	UserID   string
}
