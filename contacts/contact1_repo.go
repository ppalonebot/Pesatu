package contacts

import (
	"context"
	"errors"
	"pesatu/utils"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type I_ContactRepo interface {
	CreateContact(contact *Contact) (*DBContact, error)
	UpdateContact(id primitive.ObjectID, contact *DBContact) (*DBContact, error)
	DeleteContact(id primitive.ObjectID) error
	FindMyContacts(myUid, status string, page, limit int) ([]*DBContact, error)
	FindContactsRequest(toUid string, page, limit int) ([]*DBContact, error)
}

type ContactService struct {
	collection *mongo.Collection
	ctx        context.Context
}

func NewContactService(contactCollection *mongo.Collection, ctx context.Context) I_ContactRepo {
	return &ContactService{contactCollection, ctx}
}

func (me *ContactService) CreateContact(contact *Contact) (*DBContact, error) {
	contact.CreatedAt = time.Now()
	contact.UpdatedAt = contact.CreatedAt

	res, err := me.collection.InsertOne(me.ctx, contact)
	if err != nil {
		if er, ok := err.(mongo.WriteException); ok && er.WriteErrors[0].Code == 11000 {
			return nil, errors.New("already exists")
		}
		return nil, err
	}

	var newContact *DBContact
	query := bson.M{"_id": res.InsertedID}
	if err = me.collection.FindOne(me.ctx, query).Decode(&newContact); err != nil {
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
	res := me.collection.FindOneAndUpdate(me.ctx, query, update, options.FindOneAndUpdate().SetReturnDocument(1))

	var updatedContact *DBContact

	if err := res.Decode(&updatedContact); err != nil {
		return nil, errors.New("contact doesn't exist")
	}

	return updatedContact, nil
}

func (me *ContactService) DeleteContact(id primitive.ObjectID) error {
	query := bson.M{"_id": id}

	res, err := me.collection.DeleteOne(me.ctx, query)
	if err != nil {
		return err
	}

	if res.DeletedCount == 0 {
		return errors.New("no contact with that Id exists")
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

	cursor, err := me.collection.Find(me.ctx, query, &opt)
	if err != nil {
		return nil, err
	}

	defer cursor.Close(me.ctx)

	var contacts []*DBContact

	for cursor.Next(me.ctx) {
		profile := &DBContact{}
		err := cursor.Decode(profile)

		if err != nil {
			return nil, err
		}

		contacts = append(contacts, profile)
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	if len(contacts) == 0 {
		return []*DBContact{}, nil
	}

	return contacts, nil
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

	cursor, err := me.collection.Find(me.ctx, query, &opt)
	if err != nil {
		return nil, err
	}

	defer cursor.Close(me.ctx)

	var contacts []*DBContact

	for cursor.Next(me.ctx) {
		profile := &DBContact{}
		err := cursor.Decode(profile)

		if err != nil {
			return nil, err
		}

		contacts = append(contacts, profile)
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	if len(contacts) == 0 {
		return []*DBContact{}, nil
	}

	return contacts, nil
}
