package chat

import (
	"fmt"
	"pesatu/components/messageDB"
	"pesatu/components/roommember"
	"pesatu/utils"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Room struct {
	// ctx        context.Context
	wsServer     *WsServer
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	clients      map[*Client]bool
	register     chan *Client
	unregister   chan *Client
	broadcast    chan *Message
	writeMsgToDB chan *Message
	Private      bool `json:"private"`
	wg           *sync.WaitGroup
	disposed     bool
}

// NewRoom creates a new Room
func NewRoom(wsServer *WsServer, name string, private bool) *Room {
	inputs := make(chan *Message)
	var wg sync.WaitGroup
	wg.Add(1)

	r := &Room{
		// ctx:        ctx,
		wsServer:     wsServer,
		ID:           uuid.New(),
		Name:         name,
		clients:      make(map[*Client]bool),
		register:     make(chan *Client),
		unregister:   make(chan *Client),
		broadcast:    make(chan *Message),
		writeMsgToDB: inputs,
		Private:      private,
		wg:           &wg,
		disposed:     false,
	}

	go r.writeToDBLoop()

	return r
}

func (r *Room) writeToDBLoop() {
	defer r.wg.Done()
	var inputMessages []*Message
	for {
		select {
		case msg := <-r.writeMsgToDB:
			inputMessages = append(inputMessages, msg)
		case <-time.After(1 * time.Second):
			if len(inputMessages) > 0 {
				var messages []*messageDB.CreateMessage
				var update []string
				for _, msg := range inputMessages {
					if msg.Action == HasBeenRead {
						update = append(update, msg.Message)
						continue
					}

					CreatedAt, err := time.Parse(time.RFC3339, msg.Time)
					if err != nil {
						CreatedAt = time.Now()
					}

					status := msg.Status
					if msg.Status == "acc" {
						status = "delv"
					}

					messages = append(messages, &messageDB.CreateMessage{
						Action:  msg.Action,
						Message: msg.Message,
						RoomId:  r.GetId(),
						Sender:  msg.Sender.(I_User).GetUID(),
						Status:  status,
						Time:    CreatedAt,
					})
				} //end loop

				inputMessages = nil

				if len(messages) > 0 {
					res, err := r.wsServer.msgRepository.AddMessages(messages)
					if err != nil {
						utils.Log().Error(err, "error while save messages into database")
					}

					message := &Messages{
						Action:   Delivered,
						Target:   r,
						Messages: res,
					}

					utils.Log().V(2).Info("message delivered count: %d", len(res))
					r.broadcastToClientsInRoom(message.encode())
				}

				if len(update) > 0 {
					var msgIds []*primitive.ObjectID
					var status []string
					for i := range update {
						objectID, err := primitive.ObjectIDFromHex(update[i])
						if err != nil {
							continue
						}
						msgIds = append(msgIds, &objectID)
						status = append(status, HasBeenRead)
					}

					if len(msgIds) > 0 {
						err := r.wsServer.msgRepository.UpdateStatus(msgIds, status)
						if err != nil {
							utils.Log().Error(err, "error while save status msessages into database")
						}
					}
				}
			}
		}

		if r.disposed {
			break
		}
	}
}

func (r *Room) GetClients() map[*Client]bool {
	return r.clients
}

func (r *Room) AddMemberID(id string) error {
	// for _, memberID := range r.memberids {
	// 	if memberID == id {
	// 		return fmt.Errorf("member ID %s already exists", id)
	// 	}
	// }
	// r.memberids = append(r.memberids, id)
	// return nil
	_, err := r.wsServer.roomRepository.AddMember(&roommember.Member{RoomID: r.GetId(), UserID: id})
	return err
}

func (r *Room) RemoveMemberID(id string) error {
	// var index int
	// var found bool
	// for i, memberID := range r.memberids {
	// 	if memberID == id {
	// 		index = i
	// 		found = true
	// 		break
	// 	}
	// }
	// if !found {
	// 	return fmt.Errorf("member ID %s not found", id)
	// }
	// r.memberids = append(r.memberids[:index], r.memberids[index+1:]...)
	// return nil
	err := r.wsServer.roomRepository.RemoveMember(&roommember.Member{RoomID: r.GetId(), UserID: id})
	return err
}

func (r *Room) CheckMemberID(id string) bool {
	// for _, memberID := range r.memberids {
	// 	if memberID == id {
	// 		return true
	// 	}
	// }

	// return false
	ok, _ := r.wsServer.roomRepository.CheckMemberExist(&roommember.Member{RoomID: r.GetId(), UserID: id})
	return ok
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
	if !room.Private {
		room.notifyClientJoined(client)
	}
	room.clients[client] = true

	utils.Log().V(2).Info(fmt.Sprintf("%s is registered in room %s", client.Name, room.Name))
}

func (room *Room) unregisterClientInRoom(client *Client) {
	if _, ok := room.clients[client]; ok {
		delete(room.clients, client)
		utils.Log().V(2).Info(fmt.Sprintf("del client %s from room %s", client.Name, room.Name))

		if len(room.clients) == 0 {
			room.disposed = true
			room.wg.Wait()
			delete(room.wsServer.rooms, room)
			utils.Log().V(2).Info(fmt.Sprintf("del room %s from room server", room.Name))

			// }else {
			// 	//optional broadcast leave room
		}
	}
}

func (room *Room) broadcastToClientsInRoom(message []byte) {
	for client := range room.clients {
		utils.Log().V(2).Info(fmt.Sprintf("\tBroadcast []byte : %s", client.Name))
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
