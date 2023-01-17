package contacts

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Status = string

const (
	Pending  Status = "pending"
	Accepted Status = "accepted"
	Rejected Status = "rejected"
	Blocked  Status = "blocked"
)

type ResponseStatus struct {
	Status string `json:"status"`
}

type CreateContact struct {
	Owner string `json:"owner" bson:"owner"`
	ToUsr string `json:"to_usr" bson:"to_usr"`
}

type Contact struct {
	Owner     string    `json:"owner" bson:"owner"`
	To        string    `json:"to" bson:"to"`
	Status    string    `json:"status" bson:"status"`
	CreatedAt time.Time `json:"created_at,omitempty" bson:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}

type DBContact struct {
	Id        primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Owner     string             `json:"owner" bson:"owner"`
	To        string             `json:"to" bson:"to"`
	Status    string             `json:"status" bson:"status"`
	CreatedAt time.Time          `json:"created_at,omitempty" bson:"created_at,omitempty"`
	UpdatedAt time.Time          `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}
