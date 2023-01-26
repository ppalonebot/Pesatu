package chat

import (
	"encoding/json"
	"log"
	"pesatu/auth"
	"pesatu/utils"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Client represents the websocket client at the server
type Client struct {
	// The actual websocket connection.
	conn     *websocket.Conn
	wsServer *WsServer
	send     chan []byte
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	rooms    map[*Room]bool
}

func newClient(conn *websocket.Conn, wsServer *WsServer, username string, ID string) *Client {
	client := &Client{
		Name:     username,
		conn:     conn,
		wsServer: wsServer,
		send:     make(chan []byte, 256),
		rooms:    make(map[*Room]bool),
	}

	if ID != "" {
		client.ID, _ = uuid.Parse(ID)
	}

	return client
}

// ServeWs handles websocket requests from clients requests.
func ServeWs(wsServer *WsServer, c *gin.Context) {
	userCtxValue, ok := c.Get("validuser")
	if !ok {
		utils.Log().Info("Not authenticated")
		return
	}

	user := userCtxValue.(*auth.Claims)

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		utils.Log().Error(err, "error while upgrading to websocket")
		return
	}

	client := newClient(conn, wsServer, user.GetUsername(), user.GetUID())

	go client.writeThread()
	go client.readThread()

	wsServer.register <- client
	utils.Log().Info("ServeWs")
}

func (me *Client) GetUID() string {
	return me.ID.String()
}

func (me *Client) GetUsername() string {
	return me.Name
}

func (me *Client) readThread() {
	defer func() {
		me.disconnect()
	}()

	me.conn.SetReadLimit(maxMessageSize)
	me.conn.SetReadDeadline(time.Now().Add(pongWait))
	me.conn.SetPongHandler(func(string) error {
		// keep connection alive
		me.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Start endless read loop, waiting for messages from client
	for {
		_, jsonMessage, err := me.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				utils.Log().Error(err, ("unexpected websocket close error"))
			}
			break
		}

		me.handleNewMessage(jsonMessage)
	}
}

func (me *Client) writeThread() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		me.conn.Close()
	}()
	for {
		select {
		case message, ok := <-me.send:
			me.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The WsServer closed the channel.
				me.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := me.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Attach queued chat messages to the current websocket message.
			n := len(me.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-me.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			me.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := me.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (me *Client) disconnect() {
	me.wsServer.unregister <- me
	for room := range me.rooms {
		room.unregister <- me
	}
	close(me.send)
	me.conn.Close()
}

func (me *Client) handleNewMessage(jsonMessage []byte) {
	var message Message
	if err := json.Unmarshal(jsonMessage, &message); err != nil {
		utils.Log().Error(err, ("error on unmarshal JSON message"))
		return
	}

	message.Sender = me

	switch message.Action {
	case SendMessageAction:
		roomID := message.Target.GetId()
		if room := me.wsServer.findRoomByID(roomID); room != nil {
			utils.Log().Info("new msg in room " + room.Name)
			room.broadcast <- &message
		}

	case JoinRoomAction:
		me.handleJoinRoomMessage(message)

	case LeaveRoomAction:
		me.handleLeaveRoomMessage(message)

	case JoinRoomPrivateAction:
		me.handleJoinRoomPrivateMessage(message)
	}
}

func (me *Client) handleJoinRoomMessage(message Message) {
	//message was a room name
	roomName := message.Message
	me.joinRoom(roomName, nil)
}

func (me *Client) handleLeaveRoomMessage(message Message) {
	utils.Log().V(2).Info("get request Leave Room from", me.Name)

	//message was room's name
	room := me.wsServer.findRoomByID(message.Message)
	if room == nil {
		return
	}

	if _, ok := me.rooms[room]; ok {
		delete(me.rooms, room)
		utils.Log().V(2).Info(me.Name, "leave room", room.Name)
	}

	room.unregister <- me
}

func (me *Client) handleJoinRoomPrivateMessage(message Message) {
	target := me.wsServer.findUserByID(message.Message)

	//todo next
	//target := me.wsServer.findUserByID(message.Message)

	if target == nil {
		utils.Log().V(2).Info("get request Join Room Private from", me.GetUsername(), "to none, target unavailable")
		return
	}

	roomName := utils.JoinAndSort(me.ID.String(), message.Message, "-")

	_ = me.joinRoom(roomName, target)
	_ = target.joinRoom(roomName, me)

	//todo next Join room
	//joinedRoom := me.joinRoom(roomName, target)

	// Invite target user
	// if joinedRoom != nil {
	// 	utils.Log().V(2).Info("get request Join Room Private from", me.GetUsername(), "to", target.GetName(), "room id:", joinedRoom.GetId())
	// 	me.inviteTargetUser(target, joinedRoom)
	// }
}

func (me *Client) joinRoom(roomName string, sender I_User) *Room {
	room := me.wsServer.findRoomByName(roomName)
	if room == nil {
		room = me.wsServer.createRoom(roomName, sender != nil)
		if room == nil {
			return nil
		}
	}

	// Don't allow to join private rooms through public room message
	if sender == nil && room.Private {
		return nil
	}

	if sender != nil {
		utils.Log().V(2).Info(me.Name, "Join Room", roomName, "id:", room.GetId(), "sender:", sender.GetUsername())
	} else {
		utils.Log().V(2).Info(me.Name, "Join Room", roomName, "id:", room.GetId())
	}

	if !me.isInRoom(room) {
		me.rooms[room] = true
		room.register <- me
		me.notifyRoomJoined(room, sender)
	}

	return room
}

func (me *Client) isInRoom(room *Room) bool {
	if _, ok := me.rooms[room]; ok {
		return true
	}

	return false
}

//todo next
// func (me *Client) inviteTargetUser(target I_User, room *Room) {
// 	utils.Log().V(2).Info(me.Name, "inviteTargetUser", target.GetUsername(), "to room name:", room.GetName(), ",id:", room.GetId())
// 	inviteMessage := &Message{
// 		Action:  JoinRoomPrivateAction,
// 		Message: target.GetUID(),
// 		Target:  room,
// 		Sender:  me,
// 	}

// 	if err := config.Redis.Publish(ctx, PubSubGeneralChannel, inviteMessage.encode()).Err(); err != nil {
// 		log.Println(err)
// 	}
// }

func (me *Client) notifyRoomJoined(room *Room, sender I_User) {
	message := Message{
		Action: RoomJoinedAction,
		Target: room,
		Sender: sender,
	}
	log.Println("notify Room Joined,", me.Name, "is registered in room", room.Name)
	me.send <- message.encode()
}
