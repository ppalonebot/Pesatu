package roommember

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SearchLastMessage struct {
	UID   string `json:"uid"`
	Page  string `json:"page"`
	Limit string `json:"limit"`
}

type Member struct {
	RoomID string `json:"room_id" bson:"room_id"`
	UserID string `json:"usr_id" bson:"usr_id"`
}

type DBMember struct {
	Id     primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	RoomID string             `json:"room_id" bson:"room_id"`
	UserID string             `json:"usr_id" bson:"usr_id"`
}

type Message struct {
	Id        primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Action    string             `json:"action" bson:"action"`
	Message   string             `json:"message" bson:"message"`
	RoomId    string             `json:"room" bson:"room"`
	Status    string             `json:"status" bson:"status"`
	Time      time.Time          `json:"time,omitempty" bson:"time,omitempty"`
	UpdatedAt time.Time          `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}

type Icon struct {
	Name   string `json:"name" bson:"name"`
	At     string `json:"at" bson:"at"`
	Image  string `json:"image" bson:"image"`
	RoomId string `json:"room_id" bson:"roomId"`
}

type DBLastMessage struct {
	RoomId      string   `json:"room_id" bson:"_id"`
	LastMsg     *Message `json:"last_msg" bson:"latestMessage"`
	UnreadCount int      `json:"unread_c" bson:"unreadCount"`
	Private     bool     `json:"private" bson:"private"`
	Sender      string   `json:"sender" bson:"sender"`
}

type LastMessages struct {
	Rooms []*DBLastMessage `json:"rooms"`
	Icons []*Icon          `json:"icons"`
}
