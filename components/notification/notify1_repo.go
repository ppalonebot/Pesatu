package notification

import (
	"context"
	"pesatu/utils"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type NotifRepository struct {
	notifCollection *mongo.Collection
	ctx             context.Context
}

type I_NotifRepo interface {
	GetNotifCollection() *mongo.Collection
	AddNotif(room *CreateNotification) (*DBNotification, error)
}

func NewNotifRepository(notifCollection *mongo.Collection, ctx context.Context) I_NotifRepo {
	return &NotifRepository{notifCollection, ctx}
}

func (me *NotifRepository) GetNotifCollection() *mongo.Collection {
	return me.notifCollection
}

func (me *NotifRepository) AddNotif(notif *CreateNotification) (*DBNotification, error) {
	notif.UpdatedAt = time.Now()
	doc, err := utils.ToDoc(notif)
	if err != nil {
		return nil, err
	}

	res, err := me.notifCollection.InsertOne(me.ctx, doc)
	if err != nil {
		return nil, err
	}

	var msg *DBNotification
	query := bson.M{"_id": res.InsertedID}
	if err = me.notifCollection.FindOne(me.ctx, query).Decode(&msg); err != nil {
		return nil, err
	}

	return msg, nil

}

func (me *NotifRepository) AddNotifs(messages []*CreateNotification) ([]*DBNotification, error) {
	docs := make([]interface{}, 0)
	for i := range messages {
		messages[i].UpdatedAt = time.Now()
		doc, err := utils.ToDoc(messages[i])
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}

	res, err := me.notifCollection.InsertMany(me.ctx, docs)
	if err != nil {
		return nil, err
	}

	var insertedDocs []*DBNotification // replace MyStruct with your struct type
	for _, id := range res.InsertedIDs {
		var doc *DBNotification
		err := me.notifCollection.FindOne(me.ctx, bson.M{"_id": id}).Decode(&doc)
		if err != nil {
			return nil, err
		}
		insertedDocs = append(insertedDocs, doc)
	}

	return insertedDocs, nil
}

// func (me *NotifRepository) FindNotifsByRecipient(roomId string, page, limit int) ([]*DBNotification, error) {
// 	if page == 0 {
// 		page = 1
// 	}

// 	if limit == 0 {
// 		limit = 10
// 	}

// 	skip := (page - 1) * limit

// 	pipeline := []bson.M{
// 		{"$match": bson.M{
// 			"room": roomId,
// 		}},
// 		{"$lookup": bson.M{
// 			"from":         "users",
// 			"localField":   "sender",
// 			"foreignField": "uid",
// 			"as":           "sender_user",
// 		}},
// 		{"$addFields": bson.M{
// 			"sender": bson.M{
// 				"$arrayElemAt": []interface{}{"$sender_user.username", 0},
// 			},
// 		}},
// 		{"$project": bson.M{
// 			"sender_user": 0,
// 		}},
// 		{"$sort": bson.M{
// 			"time": -1,
// 		}},
// 		{"$skip": skip},
// 		{"$limit": limit},
// 	}

// 	cursor, err := me.msgCollection.Aggregate(me.ctx, pipeline)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer cursor.Close(me.ctx)

// 	var results []*DBMessage
// 	for cursor.Next(me.ctx) {
// 		rr := &DBMessage{}
// 		err := cursor.Decode(rr)

// 		if err != nil {
// 			return nil, err
// 		}
// 		results = append(results, rr)
// 	}

// 	if err := cursor.Err(); err != nil {
// 		return nil, err
// 	}

// 	if len(results) == 0 {
// 		return []*DBMessage{}, nil
// 	}

// 	return results, nil
// }
