package model

import "time"

type OutboxEvent struct {
	ID            int64
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       []byte
	Status        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Type          string
}

type OutboxInsert struct {
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       interface{}
}
