package roommember

import "go.mongodb.org/mongo-driver/bson/primitive"

type Member struct {
	RoomID string `json:"room_id" bson:"room_id"`
	UserID string `json:"usr_id" bson:"usr_id"`
}

type DBMember struct {
	Id     primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	RoomID string             `json:"room_id" bson:"room_id"`
	UserID string             `json:"usr_id" bson:"usr_id"`
}
