package userprofile

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

type I_ProfileRepo interface {
	CreateProfile(newProfile *CreateProfile) (*DBProfile, error)
	UpdateProfile(obId primitive.ObjectID, profile *DBProfile) (*DBProfile, error)
	FindProfileByOwner(owner string) (*DBProfile, error)
	FindProfiles(page int, limit int) ([]*DBProfile, error)
	DeleteProfile(obId primitive.ObjectID) error
}

type ProfileService struct {
	collection *mongo.Collection
	ctx        context.Context
}

func NewProfileService(profileCollection *mongo.Collection, ctx context.Context) I_ProfileRepo {
	return &ProfileService{collection: profileCollection, ctx: ctx}
}

func (p *ProfileService) CreateProfile(newProfile *CreateProfile) (*DBProfile, error) {
	newProfile.CreatedAt = time.Now()
	newProfile.UpdatedAt = newProfile.CreatedAt

	res, err := p.collection.InsertOne(p.ctx, newProfile)
	if err != nil {
		if er, ok := err.(mongo.WriteException); ok && er.WriteErrors[0].Code == 11000 {
			return nil, errors.New("owner already exists")
		}
		return nil, err
	}

	opt := options.Index()
	opt.SetUnique(true)

	index := mongo.IndexModel{Keys: bson.M{"owner": 1}, Options: opt}

	if _, err := p.collection.Indexes().CreateOne(p.ctx, index); err != nil {
		return nil, err
	}

	var profile *DBProfile
	query := bson.M{"_id": res.InsertedID}
	if err = p.collection.FindOne(p.ctx, query).Decode(&profile); err != nil {
		return nil, err
	}

	return profile, nil
}

func (p *ProfileService) UpdateProfile(obId primitive.ObjectID, profile *DBProfile) (*DBProfile, error) {
	profile.UpdatedAt = time.Now()
	doc, err := utils.ToDoc(profile)
	if err != nil {
		return nil, err
	}

	// obId, _ := primitive.ObjectIDFromHex(id)
	query := bson.D{{Key: "_id", Value: obId}}
	update := bson.D{{Key: "$set", Value: doc}}
	res := p.collection.FindOneAndUpdate(p.ctx, query, update, options.FindOneAndUpdate().SetReturnDocument(1))

	var updatedProfile *DBProfile

	if err := res.Decode(&updatedProfile); err != nil {
		return nil, errors.New("profile doesn't exist")
	}

	return updatedProfile, nil
}

func (p *ProfileService) FindProfileByOwner(owner string) (*DBProfile, error) {
	query := bson.M{"owner": owner}

	var profile *DBProfile

	if err := p.collection.FindOne(p.ctx, query).Decode(&profile); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("no document with that owner exists")
		}
		return nil, err
	}

	return profile, nil
}

func (p *ProfileService) FindProfiles(page int, limit int) ([]*DBProfile, error) {
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

	cursor, err := p.collection.Find(p.ctx, query, &opt)
	if err != nil {
		return nil, err
	}

	defer cursor.Close(p.ctx)

	var profiles []*DBProfile

	for cursor.Next(p.ctx) {
		profile := &DBProfile{}
		err := cursor.Decode(profile)

		if err != nil {
			return nil, err
		}

		profiles = append(profiles, profile)
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	if len(profiles) == 0 {
		return []*DBProfile{}, nil
	}

	return profiles, nil
}

func (p *ProfileService) DeleteProfile(obId primitive.ObjectID) error {
	query := bson.M{"_id": obId}

	res, err := p.collection.DeleteOne(p.ctx, query)
	if err != nil {
		return err
	}

	if res.DeletedCount == 0 {
		return errors.New("no profile with that Id exists")
	}

	return nil
}
