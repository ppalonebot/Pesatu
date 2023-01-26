package room

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type RoomRepository struct {
	roomCollection *mongo.Collection
	ctx            context.Context
}

type I_RoomRepo interface {
	AddRoom(room *CreateRoom) (*Room, error)
	FindRoomByName(name string) (*Room, error)
	DeleteRoom(obId primitive.ObjectID) error
}

func NewRoomRepository(roomCollection *mongo.Collection, ctx context.Context) I_RoomRepo {
	return &RoomRepository{roomCollection, ctx}
}

func (me *RoomRepository) AddRoom(room *CreateRoom) (*Room, error) {
	room.CreatedAt = time.Now()
	room.UpdatedAt = room.CreatedAt

	res, err := me.roomCollection.InsertOne(me.ctx, room)
	if err != nil {
		if er, ok := err.(mongo.WriteException); ok && er.WriteErrors[0].Code == 11000 {
			return nil, errors.New("name already exists")
		}
		return nil, err
	}

	opt := options.Index()
	opt.SetUnique(true)

	index := mongo.IndexModel{Keys: bson.M{"name": 1}, Options: opt}

	if _, err := me.roomCollection.Indexes().CreateOne(me.ctx, index); err != nil {
		return nil, err
	}

	var newRoom *Room
	query := bson.M{"_id": res.InsertedID}
	if err = me.roomCollection.FindOne(me.ctx, query).Decode(&newRoom); err != nil {
		return nil, err
	}

	return newRoom, nil
}

func (me *RoomRepository) FindRoomByName(name string) (*Room, error) {
	query := bson.M{"name": name}

	var room *Room
	if err := me.roomCollection.FindOne(me.ctx, query).Decode(&room); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("room unavailable")
		}
		return nil, err
	}

	return room, nil
}

func (me *RoomRepository) DeleteRoom(obId primitive.ObjectID) error {
	query := bson.M{"_id": obId}

	res, err := me.roomCollection.DeleteOne(me.ctx, query)
	if err != nil {
		return err
	}

	if res.DeletedCount == 0 {
		return errors.New("no room with that Id exists")
	}

	return nil
}
