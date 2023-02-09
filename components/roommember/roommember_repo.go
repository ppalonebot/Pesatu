package roommember

import (
	"context"
	"fmt"
	"pesatu/components/room"
	"pesatu/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type I_RoomMember interface {
	room.I_RoomRepo
	AddMember(newMember *Member) (*DBMember, error)
	RemoveMember(member *Member) error
	FindMembers(roomId string, page int, limit int) ([]*DBMember, error)
	CheckMemberExist(member *Member) (bool, error)
	FindRoomByMemberID(id string, page, limit int) ([]*room.Room, error)
}

type RoomMemberService struct {
	room.I_RoomRepo
	memberCollection *mongo.Collection
	ctx              context.Context
}

func NewRoomMemberService(roomCollection *mongo.Collection, memberCollection *mongo.Collection, ctx context.Context) I_RoomMember {
	roomService := room.NewRoomRepository(roomCollection, ctx)
	return &RoomMemberService{roomService, memberCollection, ctx}
}

func (me *RoomMemberService) AddMember(newMember *Member) (*DBMember, error) {
	filter, err := utils.ToDoc(newMember)
	if err != nil {
		return nil, err
	}

	var dbMember *DBMember
	update := bson.M{"$setOnInsert": filter}
	opts := options.FindOneAndUpdate().SetUpsert(true)
	err = me.memberCollection.FindOneAndUpdate(me.ctx, filter, update, opts).Decode(&dbMember)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("record already exists")
		}
		return nil, err
	}

	return dbMember, nil
}

func (me *RoomMemberService) RemoveMember(member *Member) error {
	filter, err := utils.ToDoc(member)
	if err != nil {
		return err
	}

	_, err = me.memberCollection.DeleteOne(me.ctx, filter)
	if err != nil {
		return err
	}
	// if deleteResult.DeletedCount == 0 {
	// 	return fmt.Errorf("No document with the filter %v was found", filter)
	// }
	// fmt.Println("Document was deleted successfully")
	return nil
}

func (me *RoomMemberService) FindMembers(roomId string, page int, limit int) ([]*DBMember, error) {
	query := &bson.M{"room_id": roomId}
	if page == 0 {
		page = 1
	}

	if limit == 0 {
		limit = 10
	}

	skip := (page - 1) * limit

	opt := options.FindOptions{}
	opt.SetLimit(int64(limit))
	opt.SetSkip(int64(skip))

	cursor, err := me.memberCollection.Find(me.ctx, query, &opt)
	if err != nil {
		return nil, err
	}

	defer cursor.Close(me.ctx)

	var users []*DBMember

	for cursor.Next(me.ctx) {
		post := &DBMember{}
		err := cursor.Decode(post)

		if err != nil {
			return nil, err
		}

		users = append(users, post)
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	if len(users) == 0 {
		return []*DBMember{}, nil
	}

	return users, nil
}

func (me *RoomMemberService) CheckMemberExist(member *Member) (bool, error) {
	filter, err := utils.ToDoc(member)
	if err != nil {
		return false, err
	}

	count, err := me.memberCollection.CountDocuments(me.ctx, filter)
	if err != nil {
		return false, err
	}

	if count > 0 {
		//Record exists
		return true, nil
	}

	return false, nil
}

func (me *RoomMemberService) FindRoomByMemberID(id string, page, limit int) ([]*room.Room, error) {
	if page == 0 {
		page = 1
	}

	if limit == 0 {
		limit = 10
	}

	skip := (page - 1) * limit

	pipeline := []bson.M{
		{"$lookup": bson.M{
			"from":         "roommembers",
			"localField":   "uid",
			"foreignField": "room_id",
			"as":           "member",
		}},
		{"$match": bson.M{"member.usr_id": id}},
		{"$project": bson.M{
			"_id":        1,
			"uid":        1,
			"name":       1,
			"private":    1,
			"created_at": 1,
			"updated_at": 1,
		}},
		{"$skip": skip},
		{"$limit": limit},
	}

	cursor, err := me.GetRoomCollection().Aggregate(me.ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(me.ctx)

	var roomresults []*room.Room
	for cursor.Next(me.ctx) {
		rr := &room.Room{}
		err := cursor.Decode(rr)

		if err != nil {
			return nil, err
		}
		roomresults = append(roomresults, rr)
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	if len(roomresults) == 0 {
		return []*room.Room{}, nil
	}

	return roomresults, nil
}
