package notification

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CreateNotification struct {
	Title      string    `bson:"title" json:"title"`
	Content    string    `bson:"content" json:"content"`
	Recipient  string    `bson:"recipient" json:"recipient"`
	Type       string    `bson:"type" json:"type"`
	Action     string    `bson:"action" json:"action"`
	ReadStatus bool      `bson:"read_status" json:"read_status"`
	Priority   int       `bson:"priority" json:"priority"`
	SourceLink string    `bson:"source_link,omitempty" json:"source_link,omitempty"`
	Image      string    `bson:"image" json:"image"`
	CreatedAt  time.Time `json:"created_at,omitempty" bson:"created_at,omitempty"`
	UpdatedAt  time.Time `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}

type DBNotification struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Title      string             `bson:"title" json:"title"`
	Content    string             `bson:"content" json:"content"`
	Recipient  string             `bson:"recipient" json:"recipient"`
	Type       string             `bson:"type" json:"type"`
	Action     string             `bson:"action" json:"action"`
	ReadStatus bool               `bson:"read_status" json:"read_status"`
	Priority   int                `bson:"priority" json:"priority"`
	SourceLink string             `bson:"source_link,omitempty" json:"source_link,omitempty"`
	Image      string             `bson:"image" json:"image"`
	CreatedAt  time.Time          `json:"created_at,omitempty" bson:"created_at,omitempty"`
	UpdatedAt  time.Time          `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}
