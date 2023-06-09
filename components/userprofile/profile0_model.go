package userprofile

import (
	"pesatu/components/contacts"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type GetProfileRequest struct {
	UID      string `json:"uid"`
	Username string `json:"username"`
	JWT      string `json:"jwt"`
}
type UpdateUserProfile struct {
	UID  string `json:"uid"`
	Name string `json:"name"`
	// Username string `json:"username"`
	Email  string `json:"email"`
	Status string `json:"status"`
	Bio    string `json:"bio"`
	JWT    string `json:"jwt"`
}

type ResponseUserProfile struct {
	UID          string `json:"uid"`
	Name         string `json:"name"`
	Username     string `json:"username"`
	Email        string `json:"email"`
	IsRegistered bool   `json:"isregistered"`
	Status       string `json:"status"`
	Bio          string `json:"bio"`
	PPic         string `json:"ppic"`
	Avatar       string `json:"avatar"`
}

// type RegProfile struct {
// 	Owner  string `json:"owner"`
// 	Status string `json:"status"`
// 	Bio    string `json:"bio"`
// 	PPic   string `json:"ppic"`
// }

type CreateProfile struct {
	Owner     string    `json:"owner" bson:"owner"`
	Status    string    `json:"status" bson:"status"`
	Bio       string    `json:"bio" bson:"bio"`
	PPic      string    `json:"ppic" bson:"ppic"`
	CreatedAt time.Time `json:"created_at,omitempty" bson:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}

type ResponseSeeOther struct {
	Name      string                   `json:"name"`
	Username  string                   `json:"username"`
	Email     string                   `json:"email"`
	Status    string                   `json:"status"`
	Bio       string                   `json:"bio"`
	Avatar    string                   `json:"avatar"`
	CreatedAt time.Time                `json:"created_at"`
	UpdatedAt time.Time                `json:"updated_at"`
	Contact   *contacts.ResponseStatus `json:"contact"`
}

type DBProfile struct {
	Id        primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Owner     string             `json:"owner" bson:"owner"`
	Status    string             `json:"status" bson:"status"`
	Bio       string             `json:"bio" bson:"bio"`
	PPic      string             `json:"ppic" bson:"ppic"`
	CreatedAt time.Time          `json:"created_at,omitempty" bson:"created_at,omitempty"`
	UpdatedAt time.Time          `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}

type Contact struct {
	Friends []string `json:"friends" bson:"friends"`
	Pending []string `json:"pending" bson:"pending"`
	Request []string `json:"request" bson:"request"`
	Blocked []string `json:"blocked" bson:"blocked"`
}
