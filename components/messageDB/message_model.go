package messageDB

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CreateMessage struct {
	Action    string    `json:"action" bson:"action"`
	Message   string    `json:"message" bson:"message"`
	RoomId    string    `json:"room" bson:"room"`
	Sender    string    `json:"sender" bson:"sender"`
	Status    string    `json:"status" bson:"status"`
	Time      time.Time `json:"time,omitempty" bson:"time,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}

type DBMessage struct {
	Id        primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Action    string             `json:"action" bson:"action"`
	Message   string             `json:"message" bson:"message"`
	RoomId    string             `json:"room" bson:"room"`
	Sender    string             `json:"sender" bson:"sender"`
	Status    string             `json:"status" bson:"status"`
	Time      time.Time          `json:"time,omitempty" bson:"time,omitempty"`
	UpdatedAt time.Time          `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}
