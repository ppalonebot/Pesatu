package messageDB

import (
	"context"
	"fmt"
	"pesatu/components/user"
	"pesatu/utils"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type MessageRepository struct {
	user.I_UserRepo
	msgCollection *mongo.Collection
	ctx           context.Context
}

type I_MessageRepo interface {
	user.I_UserRepo
	GetMsgCollection() *mongo.Collection
	AddMessages(messages []*CreateMessage) error
	AddMessage(message *CreateMessage) (*DBMessage, error)
	FindMessagesByRoom(roomId string, page, limit int) ([]*DBMessage, error)
	RemoveMessage(msgId string) error
	RemoveMessages(msgIds []string) error
}

func NewMsgRepository(userCollection, msgCollection *mongo.Collection, ctx context.Context) I_MessageRepo {
	userService := user.NewUserService(userCollection, ctx)
	return &MessageRepository{userService, msgCollection, ctx}
}

func (me *MessageRepository) GetMsgCollection() *mongo.Collection {
	return me.msgCollection
}

func (me *MessageRepository) AddMessages(messages []*CreateMessage) error {
	docs := make([]interface{}, 0)
	for i := range messages {
		messages[i].UpdatedAt = time.Now()
		doc, err := utils.ToDoc(messages[i])
		if err != nil {
			return err
		}
		docs = append(docs, doc)
	}

	_, err := me.msgCollection.InsertMany(me.ctx, docs)
	if err != nil {
		return err
	}

	return nil
}

func (me *MessageRepository) AddMessage(message *CreateMessage) (*DBMessage, error) {
	message.UpdatedAt = time.Now()
	doc, err := utils.ToDoc(message)
	if err != nil {
		return nil, err
	}

	res, err := me.msgCollection.InsertOne(me.ctx, doc)
	if err != nil {
		return nil, err
	}

	var msg *DBMessage
	query := bson.M{"_id": res.InsertedID}
	if err = me.msgCollection.FindOne(me.ctx, query).Decode(&msg); err != nil {
		return nil, err
	}

	return msg, nil
}

func (me *MessageRepository) FindMessagesByRoom(roomId string, page, limit int) ([]*DBMessage, error) {
	if page == 0 {
		page = 1
	}

	if limit == 0 {
		limit = 10
	}

	skip := (page - 1) * limit

	pipeline := []bson.M{
		{"$match": bson.M{
			"room": roomId,
		}},
		{"$lookup": bson.M{
			"from":         "users",
			"localField":   "sender",
			"foreignField": "uid",
			"as":           "sender_user",
		}},
		{"$addFields": bson.M{
			"sender": bson.M{
				"$arrayElemAt": []interface{}{"$sender_user.username", 0},
			},
		}},
		{"$project": bson.M{
			"sender_user": 0,
		}},
		{"$sort": bson.M{
			"time": -1,
		}},
		{"$skip": skip},
		{"$limit": limit},
	}

	cursor, err := me.msgCollection.Aggregate(me.ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(me.ctx)

	var results []*DBMessage
	for cursor.Next(me.ctx) {
		rr := &DBMessage{}
		err := cursor.Decode(rr)

		if err != nil {
			return nil, err
		}
		results = append(results, rr)
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return []*DBMessage{}, nil
	}

	return results, nil
}

func (me *MessageRepository) RemoveMessage(msgId string) error {
	objectID, err := primitive.ObjectIDFromHex(msgId)
	if err != nil {
		return fmt.Errorf("error creating ObjectID: %s", err.Error())
	}

	_, err = me.msgCollection.DeleteOne(me.ctx, &bson.M{"_id": objectID})
	if err != nil {
		return err
	}

	return nil
}

func (me *MessageRepository) RemoveMessages(msgIds []string) error {
	var objectIDs []primitive.ObjectID
	for _, id := range msgIds {
		objectID, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			return fmt.Errorf("error creating ObjectID: %s, id: %s", err.Error(), id)
		}
		objectIDs = append(objectIDs, objectID)
	}

	filter := bson.M{"_id": bson.M{"$in": objectIDs}}

	_, err := me.msgCollection.DeleteMany(context.Background(), filter)
	if err != nil {
		return err
	}

	return nil
}
