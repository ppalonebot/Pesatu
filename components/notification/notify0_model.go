package notification

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Notification struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Title      string             `bson:"title" json:"title"`
	Content    string             `bson:"content" json:"content"`
	Timestamp  time.Time          `bson:"timestamp" json:"timestamp"`
	Recipient  string             `bson:"recipient" json:"recipient"`
	Type       string             `bson:"type" json:"type"`
	Action     string             `bson:"action" json:"action"`
	ReadStatus bool               `bson:"read_status" json:"read_status"`
	Priority   int                `bson:"priority" json:"priority"`
	SourceLink string             `bson:"source_link,omitempty" json:"source_link,omitempty"`
	Image      string             `bson:"image,omitempty" json:"image,omitempty"`
}
