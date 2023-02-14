package chat

import (
	"time"
)

const (
	// Max wait time when writing message to peer
	writeWait = 10 * time.Second

	// Max time till next pong from peer
	pongWait = 60 * time.Second

	// Send ping interval, must be less then pong wait time
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 1024
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

type Action = string

const (
	SendMessageAction     Action = "send-message"
	JoinRoomAction        Action = "join-room"
	LeaveRoomAction       Action = "leave-room"
	UserJoinedAction      Action = "user-join"
	UserLeftAction        Action = "user-left"
	JoinRoomPrivateAction Action = "join-room-private"
	RoomJoinedAction      Action = "room-joined"
	Info                  Action = "info"
	GetMessages           Action = "get-msg"
)

type I_User interface {
	GetUID() string
	GetUsername() string
	// joinRoom(roomName string, sender I_User, isPrivate bool) *Room
}
