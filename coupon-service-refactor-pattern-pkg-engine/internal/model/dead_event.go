package model

import "time"

type DeadEvent struct {
	ID        int64
	EventID   string
	EventType string
	Payload   []byte
	Reason    string
	CreatedAt time.Time
}
