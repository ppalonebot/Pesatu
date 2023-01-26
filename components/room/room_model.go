package room

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CreateRoom struct {
	UID       string    `json:"uid" bson:"uid"`
	Name      string    `json:"name" bson:"name"`
	Private   bool      `json:"private" bson:"private"`
	CreatedAt time.Time `json:"created_at,omitempty" bson:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}

type Room struct {
	Id        primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	UID       string             `json:"uid" bson:"uid"`
	Name      string             `json:"name" bson:"name"`
	Private   bool               `json:"private" bson:"private"`
	CreatedAt time.Time          `json:"created_at,omitempty" bson:"created_at,omitempty"`
	UpdatedAt time.Time          `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}

func (room *Room) GetId() string {
	return room.UID
}

func (room *Room) GetName() string {
	return room.Name
}

func (room *Room) GetPrivate() bool {
	return room.Private
}
