package contact

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type DBContact struct {
	Id        primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Owner     string             `json:"owner" bson:"owner"`
	Friends   []string           `json:"friends" bson:"friends"`
	Pending   []string           `json:"pending" bson:"pending"`
	Request   []string           `json:"request" bson:"request"`
	Blocked   []string           `json:"blocked" bson:"blocked"`
	CreatedAt time.Time          `json:"created_at,omitempty" bson:"created_at,omitempty"`
	UpdatedAt time.Time          `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}
