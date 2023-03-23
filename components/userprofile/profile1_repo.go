package userprofile

import (
	"context"
	"errors"
	"fmt"
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
	SeeOtherProfile(uidOwner, toUsername string) (*ResponseSeeOther, error)
}

type ProfileService struct {
	userCollection *mongo.Collection
	collection     *mongo.Collection
	ctx            context.Context
}

func NewProfileService(userCollection *mongo.Collection, profileCollection *mongo.Collection, ctx context.Context) I_ProfileRepo {
	return &ProfileService{userCollection, profileCollection, ctx}
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

func (me *ProfileService) SeeOtherProfile(uidOwner, toUsername string) (*ResponseSeeOther, error) {
	// pipeline := []bson.M{
	// 	{"$match": bson.M{"username": bson.M{"$eq": toUsername}}},
	// 	{"$lookup": bson.M{
	// 		"from":         "profiles",
	// 		"localField":   "uid",
	// 		"foreignField": "owner",
	// 		"as":           "profile",
	// 	}},
	// 	{"$lookup": bson.M{
	// 		"from": "contact",
	// 		"let":  bson.M{"userUID": "$uid"},
	// 		"pipeline": []bson.M{
	// 			{"$match": bson.M{"$expr": bson.M{
	// 				"$and": []bson.M{
	// 					{"$eq": []interface{}{"$owner", "$$userUID"}},
	// 					{"$eq": []interface{}{"$to", uidOwner}},
	// 				},
	// 			}}},
	// 		},
	// 		"as": "temp_contact",
	// 	}},
	// 	{"$addFields": bson.M{
	// 		"contact": bson.M{"$arrayElemAt": []interface{}{"$temp_contact", 0}},
	// 	}},
	// 	{"$project": bson.M{
	// 		"temp_contact": 0,
	// 	}},
	// 	{"$unwind": "$profile"},
	// 	{"$project": bson.M{
	// 		"_id":        0,
	// 		"name":       "$name",
	// 		"username":   "$username",
	// 		"email":      "$email",
	// 		"status":     "$profile.status",
	// 		"bio":        "$profile.bio",
	// 		"avatar":     "$avatar",
	// 		"created_at": "$created_at",
	// 		"updated_at": "$updated_at",
	// 		"contact":    "$contact",
	// 	}},
	// 	{"$limit": 1},
	// }
	pipeline := []bson.M{
		{"$match": bson.M{"username": bson.M{"$eq": toUsername}}},
		{"$lookup": bson.M{
			"from":         "profiles",
			"localField":   "uid",
			"foreignField": "owner",
			"as":           "profile",
		}},
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
		{"$unwind": bson.M{
			"path":                       "$profile",
			"preserveNullAndEmptyArrays": true,
		}},
		{"$project": bson.M{
			"_id":      0,
			"name":     "$name",
			"username": "$username",
			"email":    "$email",
			"status": bson.M{
				"$cond": []interface{}{
					bson.M{"$eq": []interface{}{"$profile", []interface{}{}}},
					"",
					"$profile.status",
				},
			},
			"bio": bson.M{
				"$cond": []interface{}{
					bson.M{"$eq": []interface{}{"$profile", []interface{}{}}},
					"",
					"$profile.bio",
				},
			},
			"avatar":     "$avatar",
			"created_at": "$created_at",
			"updated_at": "$updated_at",
			"contact":    "$contact",
		}},
		{"$limit": 1},
	}

	cursor, err := me.userCollection.Aggregate(me.ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(me.ctx)

	var result []*ResponseSeeOther
	for cursor.Next(me.ctx) {
		ctt := &ResponseSeeOther{}
		err := cursor.Decode(ctt)

		if err != nil {
			return nil, err
		}
		result = append(result, ctt)
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	if len(result) == 0 {

		return nil, fmt.Errorf("doesn't exist")
	}

	return result[0], nil
}
