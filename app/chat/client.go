package chat

import (
	"encoding/json"
	"fmt"
	"net/http"
	"pesatu/auth"
	"pesatu/components/contacts"
	"pesatu/utils"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Client represents the websocket client at the server
type Client struct {
	// The actual websocket connection.
	conn           *websocket.Conn
	wsServer       *WsServer
	send           chan []byte
	id             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	Username       string    `json:"username"`
	Avatar         string    `json:"avatar"`
	rooms          map[*Room]bool
	contactService contacts.I_ContactRepo
}

func newClient(conn *websocket.Conn, wsServer *WsServer, username string, ID string, contactRepo contacts.I_ContactRepo) (*Client, error) {
	user, err := contactRepo.FindUserByUsername(username)
	if err != nil {
		return nil, err
	}

	client := &Client{
		Name:           user.Name,
		Username:       user.Username,
		Avatar:         user.Avatar,
		conn:           conn,
		wsServer:       wsServer,
		send:           make(chan []byte, 256),
		rooms:          make(map[*Room]bool),
		contactService: contactRepo,
	}

	if ID != "" {
		client.id, _ = uuid.Parse(ID)
	}

	return client, nil
}

// ServeWs handles websocket requests from clients requests.
func ServeWs(wsServer *WsServer, c *gin.Context, contactRepo contacts.I_ContactRepo, devmode int) {
	userCtxValue, ok := c.Get("validuser")
	if !ok {
		utils.Log().Info("Not authenticated")
		return
	}

	user := userCtxValue.(*auth.Claims)
	if user.IsExpired() {
		utils.Log().Info("User token expired")
		return
	}

	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	if devmode > 0 {
		upgrader.CheckOrigin = func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			return origin == "http://localhost:3000"
			// return true
		}
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		utils.Log().Error(err, "error while upgrading to websocket")
		return
	}

	client, err := newClient(conn, wsServer, user.GetUsername(), user.GetUID(), contactRepo)
	if err != nil {
		utils.Log().Error(err, "error while creating client")
		return
	}

	go client.writeThread()
	go client.readThread()

	wsServer.register <- client
	utils.Log().Info("ServeWs")
}

func (me *Client) GetUID() string {
	return me.id.String()
}

func (me *Client) GetUsername() string {
	return me.Username
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
			utils.Log().Info(fmt.Sprintf("new msg in room %s", room.GetName()))
			room.broadcast <- &message
		}

	case JoinRoomAction:
		me.handleJoinRoomMessage(message)

	case LeaveRoomAction:
		me.handleLeaveRoomMessage(message)

	case JoinRoomPrivateAction:
		me.handleJoinRoomPrivateMessage(message)

		// case Info:
		// 	me.handleInfoMessage(message)
	}
}

func (me *Client) handleJoinRoomMessage(message Message) {
	//message was a room name
	roomName := message.Message
	me.joinRoom(roomName, nil, false, "")
}

func (me *Client) handleLeaveRoomMessage(message Message) {
	utils.Log().V(2).Info(fmt.Sprintf("get request Leave Room from %s", me.Name))

	//message was room's name
	room := me.wsServer.findRoomByID(message.Message)
	if room == nil {
		return
	}

	if _, ok := me.rooms[room]; ok {
		delete(me.rooms, room)
		utils.Log().V(2).Info(fmt.Sprintf("%s leave room %s", me.Name, room.Name))
	}

	room.unregister <- me
}

// func (me *Client) handleInfoMessage(message Message) {
// 	_, err := utils.IsValidUsername(message.Message)
// 	if err != nil {
// 		utils.Log().Error(err, "get target info error while checking target username")
// 		return
// 	}

// 	targetuser, err := me.contactService.FindUserConnection(me.GetUID(), message.Message)
// 	if err != nil {
// 		utils.Log().Error(err, "get target info can not find requested user to check connnection")
// 		return
// 	}

// 	if targetuser.Contact.Status != contacts.Accepted {
// 		utils.Log().Info("get target info but you are not in their contact")
// 		return
// 	}

// 	userContact := &contacts.UserContact{
// 		Name:     targetuser.Name,
// 		Username: targetuser.Username,
// 		Avatar:   targetuser.Avatar,
// 		Contact: &contacts.ResponseStatus{
// 			Status:    targetuser.Contact.Status,
// 			UpdatedAt: targetuser.Contact.UpdatedAt,
// 			CreatedAt: targetuser.Contact.CreatedAt,
// 		},
// 	}

// 	me.notifyInfo(nil,"target info", userContact)
// }

func (me *Client) handleJoinRoomPrivateMessage(message Message) {
	_, err := utils.IsValidUsername(message.Message)
	if err != nil {
		utils.Log().Error(err, "Join Room Private error while checking target username")
		return
	}

	targetuser, err := me.contactService.FindUserConnection(me.GetUID(), message.Message)
	if err != nil {
		utils.Log().Error(err, "Join Room Private but can not find requested user to check connnection")
		return
	}

	roomName := utils.JoinAndSort(me.GetUsername(), targetuser.Username, "-")

	if targetuser.Contact.Status != contacts.Accepted {
		utils.Log().Info("Join Room Private but you are not in their contact")
		return
	}

	targets := me.wsServer.findClientByID(targetuser.UID)

	//todo next
	//target := me.wsServer.findUserByID(message.Message)

	if len(targets) == 0 || targets == nil {
		utils.Log().Info(fmt.Sprintf("get request Join Room Private from %s to none, target unavailable", me.GetUsername()))
		sender := NewSender(targetuser.UID, targetuser.Name, targetuser.Username, targetuser.Avatar)

		room := me.joinRoom(roomName, sender, true, "offline")
		_ = room.AddMemberID(me.GetUID())
		_ = room.AddMemberID(targetuser.UID)
		return
	}

	var room *Room
	for i := 0; i < len(targets); i++ {
		_ = me.joinRoom(roomName, targets[i], true, "online")
		room = targets[i].joinRoom(roomName, me, true, "online")
	}

	if room != nil {
		_ = room.AddMemberID(me.GetUID())
		_ = room.AddMemberID(targetuser.UID)
	}

	//todo next Join room
	//joinedRoom := me.joinRoom(roomName, target)

	// Invite target user
	// if joinedRoom != nil {
	// 	utils.Log().V(2).Info("get request Join Room Private from", me.GetUsername(), "to", target.GetName(), "room id:", joinedRoom.GetId())
	// 	me.inviteTargetUser(target, joinedRoom)
	// }
}

func (me *Client) joinRoom(roomName string, sender I_User, isPrivate bool, msg string) *Room {
	room := me.wsServer.findRoomByName(roomName)
	if room == nil {
		room = me.wsServer.createRoom(roomName, isPrivate)
		if room == nil {
			return nil
		}
	}

	if !me.isInRoom(room) {
		me.rooms[room] = true
		room.register <- me
	}

	me.notifyRoomJoined(room, sender, msg)

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

func (me *Client) notifyRoomJoined(room *Room, sender I_User, msg string) {
	message := Message{
		Action:  RoomJoinedAction,
		Target:  room,
		Sender:  sender,
		Message: msg,
	}
	utils.Log().V(2).Info(fmt.Sprintf("notify Room Joined, %s is registered in room %s", me.Name, room.Name))
	me.send <- message.encode()
}

func (me *Client) notifyInfo(room *Room, msg string, sender interface{}) {
	message := Message{
		Action:  Info,
		Target:  room,
		Sender:  sender,
		Message: msg,
	}
	me.send <- message.encode()
}
