package messageDB

import "go.mongodb.org/mongo-driver/bson/primitive"

type CreateMessageDB struct {
	Action  string `json:"action" bson:"action"`
	Message string `json:"message" bson:"message"`
	Target  string `json:"target" bson:"target"`
	Sender  string `json:"sender" bson:"sender"`
	Status  string `json:"status" bson:"status"`
	Time    string `json:"time" bson:"time"`
}

type MessageDB struct {
	Id      primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Action  string             `json:"action" bson:"action"`
	Message string             `json:"message" bson:"message"`
	Target  string             `json:"target" bson:"target"`
	Sender  string             `json:"sender" bson:"sender"`
	Status  string             `json:"status" bson:"status"`
	Time    string             `json:"time" bson:"time"`
}
