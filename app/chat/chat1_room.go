package chat

import (
	"fmt"
	"pesatu/utils"

	"github.com/google/uuid"
)

type Room struct {
	// ctx        context.Context
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan *Message
	Private    bool `json:"private"`
}

// NewRoom creates a new Room
func NewRoom( /*ctx context.Context, */ name string, private bool) *Room {
	return &Room{
		// ctx:        ctx,
		ID:         uuid.New(),
		Name:       name,
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *Message),
		Private:    private,
	}
}

// RunRoom runs our room, accepting various requests
func (room *Room) RunRoom() {
	//todo subscribe to pub/sub messages inside a new goroutine
	//go room.subscribeToRoomMessages()

	for {
		select {

		case client := <-room.register:
			room.registerClientInRoom(client)

		case client := <-room.unregister:
			room.unregisterClientInRoom(client)

		case message := <-room.broadcast:
			room.broadcastToClientsInRoom(message.encode())

			//todo next version using redis:
			//room.publishRoomMessage(message)
		}
	}
}

func (room *Room) registerClientInRoom(client *Client) {
	utils.Log().V(2).Info("new client", client.Name, "in room", room.Name)

	if !room.Private {
		room.notifyClientJoined(client)
	}
	room.clients[client] = true

	utils.Log().V(2).Info(client.Name, "is registered in room", room.Name)
}

func (room *Room) unregisterClientInRoom(client *Client) {
	if _, ok := room.clients[client]; ok {
		delete(room.clients, client)
		utils.Log().V(2).Info("del client ", client.Name, "from room", room.Name)
	}
}

func (room *Room) broadcastToClientsInRoom(message []byte) {
	for client := range room.clients {
		utils.Log().V(2).Info("\tBroadcast []byte :", client.Name)
		client.send <- message
	}

	if len(room.clients) <= 0 {
		utils.Log().V(2).Info("\tnone")
	}
}

//todo next version using redis:
// func (room *Room) publishRoomMessage(message *Message) {
// 	utils.Log().Info("publishRoomMessage ", room.GetName())

// 	json, err := json.Marshal(message)
// 	if err != nil {
// 		utils.Log().Error(err, "error while marshal json")
// 		return
// 	}

// 	err := config.Redis.Publish(room.ctx, room.GetName(), json).Err()
// 	if err != nil {
// 		utils.Log().Error(err, "error while publishing to redis")
// 	}
// }

//todo next version using redis:
// func (room *Room) subscribeToRoomMessages() {
// 	log.Println("subscribeToRoomMessages ", room.GetName())
// 	pubsub := config.Redis.Subscribe(ctx, room.GetName())

// 	ch := pubsub.Channel()

// 	for msg := range ch {
// 		room.broadcastToClientsInRoom([]byte(msg.Payload))
// 	}
// }

func (room *Room) notifyClientJoined(client *Client) {
	message := &Message{
		Action:  SendMessageAction,
		Target:  room,
		Message: fmt.Sprintf("%s joined room", client.GetUsername()),
	}

	utils.Log().V(2).Info("notify", client.Name, "Joined room", room.Name)
	room.broadcastToClientsInRoom(message.encode())
	//todo
	//room.publishRoomMessage(message.encode())
}

func (room *Room) GetId() string {
	return room.ID.String()
}

func (room *Room) GetName() string {
	return room.Name
}

func (room *Room) GetPrivate() bool {
	return room.Private
}
