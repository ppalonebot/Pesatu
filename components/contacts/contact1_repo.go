package contacts

import (
	"context"
	"errors"
	"fmt"
	"pesatu/components/user"
	"pesatu/utils"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type I_ContactRepo interface {
	user.I_UserRepo
	CreateContact(contact *Contact) (*DBContact, error)
	UpdateContact(id primitive.ObjectID, contact *DBContact) (*DBContact, error)
	DeleteContact(id primitive.ObjectID) error
	DeleteContacts(ids []*primitive.ObjectID) error
	FindMyContacts(myUid, status string, page, limit int) ([]*DBContact, error)
	FindMyContactTo(myUid, to string) (*DBContact, error)
	FindContactsRequest(toUid string, page, limit int) ([]*DBContact, error)
	FindUserConnection(uidOwner, toUsername string) (*DBUserContact, error)
	FindUsersByName(uidOwner, name, username, status string, page, limit int) ([]*UserContact, error)
	FindUsersByUsername(uidOwner, name, status string, page, limit int) ([]*UserContact, error)
	FindUserCountByName(uidOwner, name, username, status string) (int64, error)
	FindUserCountByUsername(uidOwner, username, status string) (int64, error)
}

type ContactService struct {
	user.I_UserRepo
	contactCollection *mongo.Collection
	ctx               context.Context
}

func NewContactService(userCollection *mongo.Collection, contactCollection *mongo.Collection, ctx context.Context) I_ContactRepo {
	userService := user.NewUserService(userCollection, ctx)
	return &ContactService{userService, contactCollection, ctx}
}

func (me *ContactService) CreateContact(contact *Contact) (*DBContact, error) {
	contact.CreatedAt = time.Now()
	contact.UpdatedAt = contact.CreatedAt

	res, err := me.contactCollection.InsertOne(me.ctx, contact)
	if err != nil {
		if er, ok := err.(mongo.WriteException); ok && er.WriteErrors[0].Code == 11000 {
			return nil, errors.New("already exists")
		}
		return nil, err
	}

	var newContact *DBContact
	query := bson.M{"_id": res.InsertedID}
	if err = me.contactCollection.FindOne(me.ctx, query).Decode(&newContact); err != nil {
		return nil, err
	}

	return newContact, nil
}

func (me *ContactService) UpdateContact(id primitive.ObjectID, contact *DBContact) (*DBContact, error) {
	contact.UpdatedAt = time.Now()
	doc, err := utils.ToDoc(contact)
	if err != nil {
		return nil, err
	}

	query := bson.D{{Key: "_id", Value: id}}
	update := bson.D{{Key: "$set", Value: doc}}
	res := me.contactCollection.FindOneAndUpdate(me.ctx, query, update, options.FindOneAndUpdate().SetReturnDocument(1))

	var updatedContact *DBContact

	if err := res.Decode(&updatedContact); err != nil {
		return nil, errors.New("contact doesn't exist")
	}

	return updatedContact, nil
}

func (me *ContactService) DeleteContact(id primitive.ObjectID) error {
	query := bson.M{"_id": id}

	res, err := me.contactCollection.DeleteOne(me.ctx, query)
	if err != nil {
		return err
	}

	if res.DeletedCount == 0 {
		return errors.New("no contact with that Id exists")
	}

	return nil
}

func (me *ContactService) DeleteContacts(ids []*primitive.ObjectID) error {
	query := bson.M{"_id": bson.M{"$in": ids}}

	res, err := me.contactCollection.DeleteMany(me.ctx, query)
	if err != nil {
		return err
	}

	if res.DeletedCount == 0 {
		return errors.New("no contacts with those Ids exist")
	}

	return nil
}

func (me *ContactService) FindMyContacts(myUid, status string, page, limit int) ([]*DBContact, error) {
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

	query := bson.M{"owner": myUid, "status": status}
	// query := bson.M{"owner": myUid,
	// 	"status": bson.M{
	// 		"$regex": fmt.Sprintf(".*%s.*", status),
	// 	}}

	cursor, err := me.contactCollection.Find(me.ctx, query, &opt)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(me.ctx)

	var contacts []*DBContact
	for cursor.Next(me.ctx) {
		ctt := &DBContact{}
		err := cursor.Decode(ctt)

		if err != nil {
			return nil, err
		}

		contacts = append(contacts, ctt)
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	if len(contacts) == 0 {
		return []*DBContact{}, nil
	}

	return contacts, nil
}

func (me *ContactService) FindMyContactTo(myUid, to string) (*DBContact, error) {
	opt := options.FindOptions{}
	opt.SetLimit(1)
	opt.SetSkip(0)
	query := bson.M{"owner": myUid, "to": to}
	cursor, err := me.contactCollection.Find(me.ctx, query, &opt)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(me.ctx)

	var contacts []*DBContact
	for cursor.Next(me.ctx) {
		ctt := &DBContact{}
		err := cursor.Decode(ctt)

		if err != nil {
			return nil, err
		}

		contacts = append(contacts, ctt)
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	if len(contacts) == 0 {
		return nil, fmt.Errorf("doesn't exist")
	}

	return contacts[0], nil
}

func (me *ContactService) FindContactsRequest(toUid string, page, limit int) ([]*DBContact, error) {
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

	query := bson.M{"to": toUid, "status": "pending"}

	cursor, err := me.contactCollection.Find(me.ctx, query, &opt)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(me.ctx)

	var contacts []*DBContact
	for cursor.Next(me.ctx) {
		ctt := &DBContact{}
		err := cursor.Decode(ctt)

		if err != nil {
			return nil, err
		}

		contacts = append(contacts, ctt)
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	if len(contacts) == 0 {
		return []*DBContact{}, nil
	}

	return contacts, nil
}

func (me *ContactService) FindUserConnection(uidOwner, toUsername string) (*DBUserContact, error) {
	pipeline := []bson.M{
		{"$match": bson.M{"username": bson.M{"$eq": toUsername}}},
		{"$lookup": bson.M{
			"from": "contact",
			"let":  bson.M{"userUID": "$uid"},
			"pipeline": []bson.M{
				{"$match": bson.M{"$expr": bson.M{
					"$and": []bson.M{
						{"$eq": []interface{}{"$owner", "$$userUID"}},
						{"$eq": []interface{}{"$to", uidOwner}},
					},
				}}},
			},
			"as": "temp_contact",
		}},
		{"$addFields": bson.M{
			"contact": bson.M{"$arrayElemAt": []interface{}{"$temp_contact", 0}},
		}},
		{"$project": bson.M{
			"temp_contact": 0,
		}},
		{"$limit": 1},
	}

	cursor, err := me.GetCollection().Aggregate(me.ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(me.ctx)

	var ucontacts []*DBUserContact
	for cursor.Next(me.ctx) {
		ctt := &DBUserContact{}
		err := cursor.Decode(ctt)

		if err != nil {
			return nil, err
		}
		ucontacts = append(ucontacts, ctt)
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	if len(ucontacts) == 0 {
		return nil, fmt.Errorf("doesn't exist")
	}

	return ucontacts[0], nil
}

func (me *ContactService) findUsers(pipeline []primitive.M) ([]*UserContact, error) {
	cursor, err := me.GetCollection().Aggregate(me.ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(me.ctx)

	var ucontacts []*UserContact
	for cursor.Next(me.ctx) {
		ctt := &UserContact{}
		err := cursor.Decode(ctt)

		if err != nil {
			return nil, err
		}
		ucontacts = append(ucontacts, ctt)
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	if len(ucontacts) == 0 {
		return []*UserContact{}, nil
	}

	return ucontacts, nil
}

func (me *ContactService) FindUsersByName(uidOwner, name, username, status string, page, limit int) ([]*UserContact, error) {
	if page == 0 {
		page = 1
	}

	if limit == 0 {
		limit = 10
	}

	skip := (page - 1) * limit

	temp := []bson.M{
		{"$eq": []interface{}{"$owner", "$$userUID"}},
		{"$eq": []interface{}{"$to", uidOwner}},
	}

	if status != "" {
		temp = append(temp, bson.M{"$eq": []interface{}{"$status", status}})
	}

	pipeline := []bson.M{
		{"$match": bson.M{
			"$or": []bson.M{
				{"name": bson.M{"$regex": fmt.Sprintf(".*%s.*", name), "$options": "i"}},
				{"username": bson.M{"$regex": fmt.Sprintf(".*%s.*", username)}},
			},
			"uid": bson.M{"$ne": uidOwner},
		}},
		{"$lookup": bson.M{
			"from": "contact",
			"let":  bson.M{"userUID": "$uid"},
			"pipeline": []bson.M{
				{"$match": bson.M{"$expr": bson.M{
					"$and": temp,
				}}},
			},
			"as": "temp_contact",
		}},
		{"$addFields": bson.M{
			"contact": bson.M{"$arrayElemAt": []interface{}{"$temp_contact", 0}},
		}},
		{"$sort": bson.M{
			"contact.status": -1,
			"name":           1,
		}},
		{"$project": bson.M{
			"temp_contact": 0,
		}},
		{"$skip": skip},
		{"$limit": limit},
	}

	if status != "" {
		pipeline = append(pipeline, bson.M{"$match": bson.M{"contact": bson.M{"$ne": nil}}})
	}

	return me.findUsers(pipeline)
}

func (me *ContactService) FindUsersByUsername(uidOwner, username, status string, page, limit int) ([]*UserContact, error) {
	if page == 0 {
		page = 1
	}

	if limit == 0 {
		limit = 10
	}

	skip := (page - 1) * limit

	temp := []bson.M{
		{"$eq": []interface{}{"$owner", "$$userUID"}},
		{"$eq": []interface{}{"$to", uidOwner}},
	}

	if status != "" {
		temp = append(temp, bson.M{"$eq": []interface{}{"$status", status}})
	}

	pipeline := []bson.M{
		{"$match": bson.M{"username": bson.M{"$regex": fmt.Sprintf(".*%s.*", username)}, "uid": bson.M{"$ne": uidOwner}}},
		{"$lookup": bson.M{
			"from": "contact",
			"let":  bson.M{"userUID": "$uid"},
			"pipeline": []bson.M{
				{"$match": bson.M{"$expr": bson.M{
					"$and": temp,
				}}},
			},
			"as": "temp_contact",
		}},
		{"$addFields": bson.M{
			"contact": bson.M{"$arrayElemAt": []interface{}{"$temp_contact", 0}},
		}},
		{"$sort": bson.M{
			"contact.status": -1,
		}},
		{"$project": bson.M{
			"temp_contact": 0,
		}},

		{"$skip": skip},
		{"$limit": limit},
	}

	if status != "" {
		pipeline = append(pipeline, bson.M{"$match": bson.M{"contact": bson.M{"$ne": nil}}})
	}

	return me.findUsers(pipeline)
}

func (me *ContactService) FindUserCountByName(uidOwner, name, username, status string) (int64, error) {
	temp := []bson.M{
		{"$eq": []interface{}{"$owner", "$$userUID"}},
		{"$eq": []interface{}{"$to", uidOwner}},
	}

	if status != "" {
		temp = append(temp, bson.M{"$eq": []interface{}{"$status", status}})
	}

	pipeline := []bson.M{
		{"$match": bson.M{
			"$or": []bson.M{
				{"name": bson.M{"$regex": fmt.Sprintf(".*%s.*", name), "$options": "i"}},
				{"username": bson.M{"$regex": fmt.Sprintf(".*%s.*", username)}},
			},
			"uid": bson.M{"$ne": uidOwner},
		}},
		{"$lookup": bson.M{
			"from": "contact",
			"let":  bson.M{"userUID": "$uid"},
			"pipeline": []bson.M{
				{"$match": bson.M{"$expr": bson.M{
					"$and": temp,
				}}},
			},
			"as": "temp_contact",
		}},
		{"$addFields": bson.M{
			"contact": bson.M{"$arrayElemAt": []interface{}{"$temp_contact", 0}},
		}},
		{"$match": bson.M{"contact": bson.M{"$ne": nil}}},
		{"$count": "total_count"},
	}

	cursor, err := me.GetCollection().Aggregate(me.ctx, pipeline)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(context.Background())

	var result struct {
		TotalCount int64 `bson:"total_count"`
	}
	if cursor.Next(context.Background()) {
		if err := cursor.Decode(&result); err != nil {
			return 0, err
		}
	}

	return result.TotalCount, nil
}

func (me *ContactService) FindUserCountByUsername(uidOwner, username, status string) (int64, error) {
	temp := []bson.M{
		{"$eq": []interface{}{"$owner", "$$userUID"}},
		{"$eq": []interface{}{"$to", uidOwner}},
	}

	if status != "" {
		temp = append(temp, bson.M{"$eq": []interface{}{"$status", status}})
	}

	pipeline := []bson.M{
		{"$match": bson.M{"username": bson.M{"$regex": fmt.Sprintf(".*%s.*", username)}, "uid": bson.M{"$ne": uidOwner}}},
		{"$lookup": bson.M{
			"from": "contact",
			"let":  bson.M{"userUID": "$uid"},
			"pipeline": []bson.M{
				{"$match": bson.M{"$expr": bson.M{
					"$and": temp,
				}}},
			},
			"as": "temp_contact",
		}},
		{"$addFields": bson.M{
			"contact": bson.M{"$arrayElemAt": []interface{}{"$temp_contact", 0}},
		}},
		{"$match": bson.M{"contact": bson.M{"$ne": nil}}},
		{"$count": "total_count"},
	}

	cursor, err := me.GetCollection().Aggregate(me.ctx, pipeline)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(context.Background())

	var result struct {
		TotalCount int64 `bson:"total_count"`
	}
	if cursor.Next(context.Background()) {
		if err := cursor.Decode(&result); err != nil {
			return 0, err
		}
	}

	return result.TotalCount, nil
}
