package contacts

import (
	"pesatu/components/user"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Status = string

const (
	Accepted Status = "50_accepted"
	Pending  Status = "40_pending"
	Waiting  Status = "30_waiting"
	Rejected Status = "20_rejected"
	Blocked  Status = "10_blocked"
)

var ValidStatuses = [5]Status{Waiting, Pending, Accepted, Rejected, Blocked}

type SearchUser struct {
	UID     string `json:"uid"`
	Keyword string `json:"keyword"`
	Status  string `json:"status"`
	Page    string `json:"page"`
	Limit   string `json:"limit"`
}

type SearchUserCount struct {
	UID     string `json:"uid"`
	Keyword string `json:"keyword"`
	Status  string `json:"status"`
}

type ResponseStatus struct {
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at,omitempty" bson:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}

type CreateContact struct {
	UID       string `json:"uid" bson:"uid"`
	ToUsrName string `json:"to_usr" bson:"to_usr"`
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

type UserContact struct {
	Name     string          `json:"name"`
	Username string          `json:"username"`
	Avatar   string          `json:"avatar"`
	Contact  *ResponseStatus `json:"contact"`
}

type DBUserContact struct {
	Id        primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	UID       string             `json:"uid" bson:"uid"`
	Name      string             `json:"name" bson:"name"`
	Username  string             `json:"username" bson:"username"`
	Password  string             `json:"password" bson:"password"`
	Reg       *user.Registration `json:"reg" bson:"reg"`
	Email     string             `json:"email" bson:"email"`
	CreatedAt time.Time          `json:"created_at,omitempty" bson:"created_at,omitempty"`
	UpdatedAt time.Time          `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
	Avatar    string             `json:"avatar" bson:"avatar"`
	Contact   *DBContact
}
