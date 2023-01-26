package chat

import (
	"context"
	"pesatu/components/room"
	"pesatu/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/mongo"
)

type WsServer struct {
	users          []I_User
	clients        map[*Client]bool
	register       chan *Client
	unregister     chan *Client
	broadcast      chan []byte
	rooms          map[*Room]bool
	roomRepository room.I_RoomRepo
	// userRepository user.UserService
}

// NewWebsocketServer creates a new WsServer type
func NewWebsocketServer(mongoclient *mongo.Client, ctx context.Context /*, userRepository user.UserService*/) *WsServer {
	collectionRoom := mongoclient.Database("pesatu").Collection("rooms")
	roomRepository := room.NewRoomRepository(collectionRoom, ctx)

	wsServer := &WsServer{
		clients:        make(map[*Client]bool),
		register:       make(chan *Client),
		unregister:     make(chan *Client),
		broadcast:      make(chan []byte),
		rooms:          make(map[*Room]bool),
		roomRepository: roomRepository,
		// userRepository: userRepository,
	}

	// Add users from database to server
	//wsServer.users = userRepository.GetAllUsers()
	return wsServer
}

func (me *WsServer) InitRouteTo(rg *gin.Engine) {
	rg.GET("/ws", func(c *gin.Context) {
		ServeWs(me, c)
	})
}

// Run our websocket server, accepting various requests
func (server *WsServer) Run() {
	//todo next
	//go server.listenPubSubChannel()
	for {
		select {

		case client := <-server.register:
			server.registerClient(client)

		case client := <-server.unregister:
			server.unregisterClient(client)

		case message := <-server.broadcast:
			server.broadcastToClients(message)
		}

	}
}

// add new client connection
func (server *WsServer) registerClient(client *Client) {
	utils.Log().V(2).Info("new connection", client.Name, "@", client.conn.RemoteAddr().String())

	server.notifyClientJoined(client)

	//todo Publish user in PubSub
	//server.publishClientJoined(client)

	server.listOnlineClients(client)
	server.clients[client] = true

	utils.Log().V(2).Info("registered", client.Name, "id:", client.ID.String())
}

// remove client connection
func (server *WsServer) unregisterClient(client *Client) {
	if _, ok := server.clients[client]; ok {
		delete(server.clients, client)
		// Remove user from repo
		// server.userRepository.RemoveUser(client)

		server.notifyClientLeft(client)

		//todo: Publish user left in PubSub
		//server.publishClientLeft(client)

		utils.Log().V(2).Info("del connection ", client.Name, "@", client.conn.RemoteAddr().String())
	}
}

//todo next:
// func (server *WsServer) publishClientJoined(client *Client) {
// 	message := &Message{
// 		Action: UserJoinedAction,
// 		Sender: client,
// 	}
// 	log.Println("publishClientJoined", client.Name)
// 	if err := config.Redis.Publish(ctx, PubSubGeneralChannel, message.encode()).Err(); err != nil {
// 		log.Println(err)
// 	}
// }

func (server *WsServer) notifyClientJoined(client *Client) {
	message := &Message{
		Action: UserJoinedAction,
		Sender: client,
	}
	utils.Log().V(2).Info("notify User Joined", client.Name, "to")
	server.broadcastToClients(message.encode())
}

//todo next
// func (server *WsServer) publishClientLeft(client *Client) {

// 	message := &Message{
// 		Action: UserLeftAction,
// 		Sender: client,
// 	}

// 	log.Println("publishClientLeft", client.Name)
// 	if err := config.Redis.Publish(ctx, PubSubGeneralChannel, message.encode()).Err(); err != nil {
// 		log.Println(err)
// 	}
// }

func (server *WsServer) notifyClientLeft(client *Client) {
	message := &Message{
		Action: UserLeftAction,
		Sender: client,
	}

	utils.Log().V(2).Info("notify User Left", client.Name, "to")
	server.broadcastToClients(message.encode())
}

//todo next
// func (server *WsServer) listenPubSubChannel() {

// 	pubsub := config.Redis.Subscribe(ctx, PubSubGeneralChannel)

// 	ch := pubsub.Channel()

// 	for msg := range ch {

// 		var message Message
// 		if err := json.Unmarshal([]byte(msg.Payload), &message); err != nil {
// 			log.Printf("Error on unmarshal JSON message %s", err)
// 			return
// 		}

// 		switch message.Action {
// 		case UserJoinedAction:
// 			server.handleUserJoined(message)
// 		case UserLeftAction:
// 			server.handleUserLeft(message)
// 		case JoinRoomPrivateAction:
// 			server.handleUserJoinPrivate(message)
// 		}

// 	}
// }

func (server *WsServer) handleUserJoined(message Message) {
	// Add the user to the slice
	server.users = append(server.users, message.Sender)
	server.broadcastToClients(message.encode())
}

func (server *WsServer) handleUserLeft(message Message) {
	// Remove the user from the slice
	for i, user := range server.users {
		if user.GetUID() == message.Sender.GetUID() {
			server.users[i] = server.users[len(server.users)-1]
			server.users = server.users[:len(server.users)-1]
			break // added this break to only remove the first occurrence
		}
	}

	server.broadcastToClients(message.encode())
}

func (server *WsServer) handleUserJoinPrivate(message Message) {
	// Find client for given user, if found add the user to the room.
	targetClients := server.findClientByID(message.Message)
	// if targetClient != nil {
	// 	targetClient.joinRoom(message.Target.GetName(), message.Sender)
	// }
	for _, targetClient := range targetClients {
		_ = targetClient.joinRoom(message.Target.GetName(), message.Sender)
	}
}

func (server *WsServer) listOnlineClients(client *Client) {
	var uniqueUsers = make(map[string]bool)
	for _, user := range server.users {
		if ok := uniqueUsers[user.GetUID()]; !ok {
			message := &Message{
				Action: UserJoinedAction,
				Sender: user,
			}
			utils.Log().V(2).Info("Tell", client.Name, "existing User Joined", user.GetUsername(), "id:", user.GetUID())
			uniqueUsers[user.GetUID()] = true
			client.send <- message.encode()
		}
	}
}

func (server *WsServer) broadcastToClients(message []byte) {
	for client := range server.clients {
		utils.Log().V(2).Info("\tBroadcast []byte :", client.Name, "@", client.conn.RemoteAddr().String())
		client.send <- message
	}

	if len(server.clients) <= 0 {
		utils.Log().V(2).Info("\tnone")
	}
}

func (server *WsServer) findRoomByName(name string) *Room {
	var foundRoom *Room
	for room := range server.rooms {
		if room.GetName() == name {
			foundRoom = room
			break
		}
	}

	// NEW: if there is no room, try to create it from the repo
	if foundRoom == nil {
		// Try to run the room from the repository, if it is found.
		utils.Log().V(2).Info("Try to run the room from the repository...")
		foundRoom = server.runRoomFromRepository(name)
	}

	return foundRoom
}

func (server *WsServer) runRoomFromRepository(name string) *Room {
	var room *Room
	dbRoom, _ := server.roomRepository.FindRoomByName(name)
	if dbRoom != nil {
		room = NewRoom(dbRoom.GetName(), dbRoom.GetPrivate())
		room.ID, _ = uuid.Parse(dbRoom.GetId())

		go room.RunRoom()
		server.rooms[room] = true
	}

	return room
}

func (server *WsServer) findRoomByID(ID string) *Room {
	var foundRoom *Room
	for room := range server.rooms {
		if room.GetId() == ID {
			foundRoom = room
			break
		}
	}

	return foundRoom
}

func (server *WsServer) createRoom(name string, private bool) *Room {
	newRoom := NewRoom(name, private)

	createroom := &room.CreateRoom{
		UID:     newRoom.GetId(),
		Name:    newRoom.GetName(),
		Private: newRoom.GetPrivate(),
	}
	_, err := server.roomRepository.AddRoom(createroom)
	if err != nil {
		utils.Log().Error(err, "error while adding room to repository")
		return nil
	}

	go newRoom.RunRoom()
	server.rooms[newRoom] = true
	utils.Log().V(2).Info("room", name, "is created, id:", newRoom.GetId())

	return newRoom
}

func (server *WsServer) findUserByID(ID string) I_User {
	var foundUser I_User
	for _, client := range server.users {
		if client.GetUID() == ID {
			foundUser = client
			break
		}
	}

	return foundUser
}

func (server *WsServer) findClientByID(ID string) []*Client {
	var foundClients []*Client
	for client := range server.clients {
		if client.GetUID() == ID {
			foundClients = append(foundClients, client)
		}
	}

	return foundClients
}
