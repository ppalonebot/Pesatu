package messageDB

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

type MessageRepository struct {
	msgCollection *mongo.Collection
	ctx           context.Context
}

type I_MessageRepo interface {
	GetRoomCollection() *mongo.Collection
}

func NewRoomRepository(msgCollection *mongo.Collection, ctx context.Context) I_MessageRepo {
	return &MessageRepository{msgCollection, ctx}
}

func (me *MessageRepository) GetRoomCollection() *mongo.Collection {
	return me.msgCollection
}

//todo!
