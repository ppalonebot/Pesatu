package chat

import (
	"encoding/json"
	"fmt"
	"net/http"
	"pesatu/app/vicall"
	"pesatu/auth"
	"pesatu/components/contacts"
	"pesatu/jsonrpc2"
	"pesatu/utils"
	"strconv"
	"strings"
	"sync"
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
	id             uuid.UUID
	Name           string `json:"name"`
	Username       string `json:"username"`
	Avatar         string `json:"avatar"`
	rooms          map[*Room]bool
	contactService contacts.I_ContactRepo
	vicall         *vicall.JSONSignal
	wg             *sync.WaitGroup
	disposed       bool
}

func newClient(conn *websocket.Conn, wsServer *WsServer, username string, ID string, contactRepo contacts.I_ContactRepo) (*Client, error) {
	user, err := contactRepo.FindUserByUsername(username)
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	wg.Add(1)

	client := &Client{
		Name:           user.Name,
		Username:       user.Username,
		Avatar:         user.Avatar,
		conn:           conn,
		wsServer:       wsServer,
		send:           make(chan []byte, 256),
		rooms:          make(map[*Room]bool),
		contactService: contactRepo,
		wg:             &wg,
		disposed:       false,
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
			return strings.HasPrefix(origin, "http://192.168.") || strings.HasPrefix(origin, "http://localhost") //||origin == "http://localhost:3000"
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
	client.vicall = vicall.NewJSONSignal(vicall.NewPeer(wsServer.ionsfu), utils.Log())

	go client.writeThread()
	go client.readThread()

	wsServer.register <- client
	utils.Log().Info("ServeWs " + user.GetUsername())
}

func (me *Client) GetUID() string {
	return me.id.String()
}

func (me *Client) GetUsername() string {
	return me.Username
}

func (me *Client) readThread() {
	defer func() {
		me.wg.Done()
		me.disconnect()
	}()

	// me.conn.SetReadLimit(maxMessageSize)
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
				utils.Log().Error(err, "unexpected websocket close error")
				break
			}

			if strings.Contains(err.Error(), "close") {
				utils.Log().V(2).Info(fmt.Sprintf("client @%s close connection", me.GetUsername()))
				break
			}

			utils.Log().Error(err, "error while reading message")
			break
		}

		me.handleNewMessage(jsonMessage)

		if me.disposed {
			break
		}
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

			//Attach queued chat messages to the current websocket message.
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
	utils.Log().Info("disconnect " + me.Username)
	me.vicall.Close()
	me.wsServer.unregister <- me
	for room := range me.rooms {
		room.unregister <- me
	}
	close(me.send)
	me.conn.Close()
}

func (me *Client) handleNewMessage(jsonMessage []byte) {
	utils.Log().V(2).Info("handleNewMessage " + string(jsonMessage))
	var rpc jsonrpc2.RPCRequest
	if err := json.Unmarshal(jsonMessage, &rpc); err != nil {
		utils.Log().Error(err, ("error on unmarshal JSON rpc"))
		return
	}

	var message Message
	if err := json.Unmarshal(rpc.Params, &message); err != nil {
		// utils.Log().Error(err, ("error on unmarshal rpc message"))
		message = Message{}
	}

	message.Sender = me

	switch rpc.Method {
	case SendMessageAction:
		me.handleSendMessageAction(message)

	case JoinRoomAction:
		me.handleJoinRoomMessage(message)

	case LeaveRoomAction:
		me.handleLeaveRoomMessage(message)

	case JoinRoomPrivateAction:
		me.handleJoinRoomPrivateMessage(message)

	case GetMessages:
		me.handleGetMessages(message)

	case HasBeenRead:
		me.handleHasBeenRead(message)

	default:
		me.handleVicall(&rpc)
		// me.vicall.Handle(me.send, &rpc)
	}
}

func (me *Client) handleVicall(rpc *jsonrpc2.RPCRequest) {
	switch rpc.Method {
	case vicall.JoinVicall:
		replyError := func(err error) {
			utils.Log().V(2).Info(fmt.Sprintf("ReplyWithError, %s", err))
			resErr, err := jsonrpc2.ReplyWithError(rpc.ID, nil, http.StatusBadRequest, err)

			if err != nil {
				utils.Log().Error(err, "error while sending reply with error")
				return
			}

			me.SendMsg(resErr.Encode())
		}

		var join vicall.Join
		err := json.Unmarshal(rpc.Params, &join)
		if err != nil {
			replyError(err)
			return
		}

		ok := utils.IsValidUid(join.SID)
		if !ok {
			replyError(fmt.Errorf("error, invalid room id %s", join.SID))
			return
		}

		if me.Username != join.UID {
			replyError(fmt.Errorf("error, uid didn't match with current user %s", join.UID))
			return
		}

		room := me.wsServer.findRoomByID(join.SID)
		if room == nil {
			replyError(fmt.Errorf("error, can't find room id %s", join.SID))
			return
		}

		ok = room.CheckMemberID(me.GetUID())
		if !ok {
			replyError(fmt.Errorf("error, client rejected"))
			return
		}

		me.vicall.Handle(me.send, rpc)
	default:
		me.vicall.Handle(me.send, rpc)
	}
}

func (me *Client) handleHasBeenRead(message Message) {
	roomID := message.Target.GetId()
	if room := me.wsServer.findRoomByID(roomID); room != nil {
		utils.Log().V(2).Info(fmt.Sprintf("msg has been read id: %s", message.Message))
		room.broadcast <- &message
		room.writeMsgToDB <- &message
	}
}

func (me *Client) handleGetMessages(message Message) {
	roomID := message.Target.GetId()
	room := me.wsServer.findRoomByID(roomID)

	flag := false
	if room == nil {

		dbRoom, err := me.wsServer.roomRepository.FindRoomByName(message.Target.GetName())
		if err != nil {
			me.notifyInfo(nil, message.Sender.(I_User), JoinRoomPrivateAction+", can not find room", "error", message.Time)
			return
		}

		inputUUID, err := uuid.Parse(dbRoom.UID)
		if err != nil {
			me.notifyInfo(nil, message.Sender.(I_User), JoinRoomPrivateAction+", invalid uid", "error", message.Time)
			return
		}

		room = &Room{Name: dbRoom.Name, ID: inputUUID, Private: dbRoom.Private}
		flag = true
	}

	if room != nil {
		parts := strings.Split(message.Message, ",")

		page, err := strconv.Atoi(parts[0])
		if err != nil {
			utils.Log().Error(err, "invalid page number: %s\n", parts[0])
			return
		}

		limit, err := strconv.Atoi(parts[1])
		if err != nil {
			utils.Log().Error(err, "invalid limit number: %s\n", parts[1])
			return
		}

		msgs, err := me.wsServer.msgRepository.FindMessagesByRoom(room.GetId(), page, limit)
		retMsg := Messages{
			Action:   message.Action,
			Target:   room,
			Sender:   me,
			Messages: msgs,
		}
		if flag {
			retMsg.Target.ID = uuid.Nil
		}
		utils.Log().V(2).Info(fmt.Sprintf("notify Room Joined, %s is registered in room %s", me.Name, room.Name))
		m, err := jsonrpc2.Notify(message.Action, retMsg)
		if err != nil {
			utils.Log().Error(err, "error while create jsonrpc2 notify")
		}
		me.SendMsg(m.Encode())
	}
}

func (me *Client) handleSendMessageAction(message Message) {
	roomID := message.Target.GetId()
	if room := me.wsServer.findRoomByID(roomID); room != nil {
		utils.Log().V(2).Info(fmt.Sprintf("new msg in room %s", room.GetName()))
		message.Status = "acc"
		room.broadcast <- &message
		room.writeMsgToDB <- &message
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
	utils.Log().V(2).Info("handleJoinRoomPrivateMessage " + message.Message + " " + message.Time)
	_, err := utils.IsValidUsername(message.Message)
	if err != nil {
		utils.Log().Error(err, fmt.Sprintf("Join Room Private error while checking target username: %s", message.Message))
		sender := NewSender("", "", message.Message, JoinRoomPrivateAction)
		me.notifyInfo(nil, sender, JoinRoomPrivateAction+", username error", "error", message.Time)
		return
	}

	targetuser, err := me.contactService.FindUserConnection(me.GetUID(), message.Message)
	if err != nil {
		utils.Log().Error(err, "Join Room Private but can not find requested user to check connnection")
		sender := NewSender("", "", message.Message, JoinRoomPrivateAction)
		me.notifyInfo(nil, sender, JoinRoomPrivateAction+", can not find requested user to connect", "error", message.Time)
		return
	}

	roomName := utils.JoinAndSort(me.GetUsername(), targetuser.Username, "-")

	if targetuser.Contact == nil || targetuser.Contact.Status != contacts.Accepted {
		utils.Log().Info("Join Room Private but you are not in their contact")
		sender := NewSender(targetuser.UID, targetuser.Name, targetuser.Username, targetuser.Avatar)
		dbRoom, err := me.wsServer.roomRepository.FindRoomByName(roomName)
		if err != nil {
			me.notifyInfo(nil, sender, JoinRoomPrivateAction+", can not find room", "error", message.Time)
			return
		}

		// inputUUID, err := uuid.Parse(dbRoom.UID)
		// if err != nil {
		// 	me.notifyInfo(nil, sender, JoinRoomPrivateAction+", invalid uid", "error", message.Time)
		// 	return
		// }

		room := &Room{Name: dbRoom.Name, ID: uuid.Nil, Private: dbRoom.Private}
		me.notifyInfo(room, sender, JoinRoomPrivateAction+", you are not in contact", "error", message.Time)
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

	m, err := jsonrpc2.Notify(RoomJoinedAction, message)
	if err != nil {
		utils.Log().Error(err, "error while create jsonrpc2 notify")
	}
	me.SendMsg(m.Encode())
}

func (me *Client) notifyInfo(room *Room, sender I_User, msg, status, time string) {
	message := Message{
		Action:  Info,
		Target:  room,
		Sender:  sender,
		Message: msg,
		Status:  status,
		Time:    time,
	}
	m, err := jsonrpc2.Notify(Info, message)
	if err != nil {
		utils.Log().Error(err, "error while create jsonrpc2 notify")
	}
	me.SendMsg(m.Encode())
}

func (me *Client) SendMsg(msg []byte) {
	select {
	case me.send <- msg:
		utils.Log().V(2).Info(fmt.Sprintf("send rpc msg"))
	default:
		//channel closed
		utils.Log().Error(nil, "send msg error, chanel closed")
	}
}
