package user

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SearchUser struct {
	UID     string `json:"uid"`
	Keyword string `json:"keyword"`
	Page    string `json:"page"`
	Limit   string `json:"limit"`
}

type ForgotPwdRequest struct {
	Email string `json:"email"`
}

type PwdResetRequest struct {
	JWT      string `json:"jwt"`
	Password string `json:"password"`
}

type ResponseStatus struct {
	UID    string `json:"uid"`
	Status string `json:"status"`
}

type Login struct {
	Username string `json:"username" bson:"username"`
	Password string `json:"password" bson:"password"`
}

type GetUserRequest struct {
	UID string `json:"uid"`
	JWT string `json:"jwt"`
}

type Registration struct {
	Registered bool      `json:"registered" bson:"registered"`
	Code       string    `json:"code" bson:"code"`
	SendCodeAt time.Time `json:"sendcode_at,omitepty" bson:"sendcode_at,omitempty"`
}

type EmailRegistration struct {
	User string `json:"user" bson:"user"`
	Code string `json:"code" bson:"code"`
}

type EmailPwdResetRequest struct {
	User string `json:"user" bson:"user"`
	JWT  string `json:"jwt"`
}

type ConfirmRegCode struct {
	JWT  string `json:"jwt"`
	UID  string `json:"uid"`
	Code string `json:"code"`
}

type CreateUserRequest struct {
	Name     string `json:"name" bson:"name"`
	Username string `json:"username" bson:"username"`
	Password string `json:"password" bson:"password"`
	Email    string `json:"email" bson:"email"`
}

type ResponseUser struct {
	UID      string `json:"uid"`
	Name     string `json:"name"`
	Username string `json:"username"`
	// Email        string `json:"email"`
	JWT          string `json:"jwt"`
	IsRegistered bool   `json:"isregistered"`
	Avatar       string `json:"avatar"`
}

type ResponseUserShort struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}

type CreateUser struct {
	UID       string        `json:"uid" bson:"uid"`
	Name      string        `json:"name" bson:"name"`
	Password  string        `json:"password" bson:"password"`
	Username  string        `json:"username" bson:"username"`
	Email     string        `json:"email" bson:"email"`
	Reg       *Registration `json:"reg" bson:"reg"`
	CreatedAt time.Time     `json:"created_at,omitempty" bson:"created_at,omitempty"`
	UpdatedAt time.Time     `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
	Avatar    string        `json:"avatar" bson:"avatar"`
}

type DBUser struct {
	Id        primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	UID       string             `json:"uid" bson:"uid"`
	Name      string             `json:"name" bson:"name"`
	Username  string             `json:"username" bson:"username"`
	Password  string             `json:"password" bson:"password"`
	Reg       *Registration      `json:"reg" bson:"reg"`
	Email     string             `json:"email" bson:"email"`
	CreatedAt time.Time          `json:"created_at,omitempty" bson:"created_at,omitempty"`
	UpdatedAt time.Time          `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
	Avatar    string             `json:"avatar" bson:"avatar"`
}
