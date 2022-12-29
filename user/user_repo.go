package user

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

type I_UserRepo interface {
	CreateUser(*CreateUser) (*DBUser, error)
	UpdateUser(primitive.ObjectID, *DBUser) (*DBUser, error)
	FindUserById(string) (*DBUser, error)
	FindUserByUsername(string) (*DBUser, error)
	FindUserByEmail(string) (*DBUser, error)
	FindUsers(page int, limit int) ([]*DBUser, error)
	DeleteUser(string) error
}

type UserService struct {
	userCollection *mongo.Collection
	ctx            context.Context
}

func NewUserService(userCollection *mongo.Collection, ctx context.Context) I_UserRepo {
	return &UserService{userCollection, ctx}
}

func (usr *UserService) CreateUser(user *CreateUser) (*DBUser, error) {
	user.CreatedAt = time.Now()
	user.UpdatedAt = user.CreatedAt

	res, err := usr.userCollection.InsertOne(usr.ctx, user)
	if err != nil {
		if er, ok := err.(mongo.WriteException); ok && er.WriteErrors[0].Code == 11000 {
			return nil, errors.New("uid already exists")
		}
		return nil, err
	}

	opt := options.Index()
	opt.SetUnique(true)

	index := mongo.IndexModel{Keys: bson.M{"uid": 1}, Options: opt}

	if _, err := usr.userCollection.Indexes().CreateOne(usr.ctx, index); err != nil {
		return nil, err
	}

	var newUser *DBUser
	query := bson.M{"_id": res.InsertedID}
	if err = usr.userCollection.FindOne(usr.ctx, query).Decode(&newUser); err != nil {
		return nil, err
	}

	return newUser, nil
}

func (usr *UserService) UpdateUser(obId primitive.ObjectID, user *DBUser) (*DBUser, error) {
	user.UpdatedAt = time.Now()
	doc, err := utils.ToDoc(user)
	if err != nil {
		return nil, err
	}

	// obId, _ := primitive.ObjectIDFromHex(id)
	query := bson.D{{Key: "_id", Value: obId}}
	update := bson.D{{Key: "$set", Value: doc}}
	res := usr.userCollection.FindOneAndUpdate(usr.ctx, query, update, options.FindOneAndUpdate().SetReturnDocument(1))

	var updatedUser *DBUser

	if err := res.Decode(&updatedUser); err != nil {
		return nil, errors.New("no user with that Id exists")
	}

	return updatedUser, nil
}

func (usr *UserService) FindUserById(uid string) (*DBUser, error) {
	query := bson.M{"uid": uid}

	var user *DBUser

	if err := usr.userCollection.FindOne(usr.ctx, query).Decode(&user); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("no document with that UID exists")
		}
		return nil, err
	}

	return user, nil
}

func (usr *UserService) FindUserByUsername(username string) (*DBUser, error) {
	query := bson.M{"username": username}

	var user *DBUser

	if err := usr.userCollection.FindOne(usr.ctx, query).Decode(&user); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("user unavailable")
		}
		return nil, err
	}

	return user, nil
}

func (usr *UserService) FindUserByEmail(email string) (*DBUser, error) {
	query := bson.M{"email": email}

	var user *DBUser

	if err := usr.userCollection.FindOne(usr.ctx, query).Decode(&user); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("user unavailable")
		}
		return nil, err
	}

	return user, nil
}

func (usr *UserService) FindUsers(page int, limit int) ([]*DBUser, error) {
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

	query := bson.M{}

	cursor, err := usr.userCollection.Find(usr.ctx, query, &opt)
	if err != nil {
		return nil, err
	}

	defer cursor.Close(usr.ctx)

	var users []*DBUser

	for cursor.Next(usr.ctx) {
		post := &DBUser{}
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
		return []*DBUser{}, nil
	}

	return users, nil
}

func (usr *UserService) DeleteUser(id string) error {
	obId, _ := primitive.ObjectIDFromHex(id)
	query := bson.M{"_id": obId}

	res, err := usr.userCollection.DeleteOne(usr.ctx, query)
	if err != nil {
		return err
	}

	if res.DeletedCount == 0 {
		return errors.New("no user with that Id exists")
	}

	return nil
}
