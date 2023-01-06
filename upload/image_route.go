package upload

import (
	"context"
	"io"
	"net/http"
	"pesatu/auth"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/juju/ratelimit"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
)

var Logger logr.Logger = logr.Discard()

const MAX_IMAGE_SIZE = 500 * 1024 // 500KB

type FileMetadata struct {
	ContentType string `bson:"content_type"`
	Filename    string `bson:"filename"`
}

type UploadImageRoute struct {
	dbclient *mongo.Client
	limiter  *ratelimit.Bucket
	ctx      context.Context
}

func NewUploadImageRoute(mongoclient *mongo.Client, ctx context.Context, l logr.Logger, limiter *ratelimit.Bucket) UploadImageRoute {
	Logger = l
	Logger.V(2).Info("New Upload Image Route created")
	return UploadImageRoute{mongoclient, limiter, ctx}
}

func (me *UploadImageRoute) InitRouteTo(rg *gin.Engine) {
	router := rg.Group("/image")
	router.POST("/upload", me.RateLimit, func(c *gin.Context) {
		if c.Request.ContentLength > MAX_IMAGE_SIZE {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Image size exceeds maximum allowed, max 5MB"})
			return
		}

		// Retrieve uploaded image file
		file, err := c.FormFile("image")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Error retrieving image file: " + err.Error()})
			return
		}

		src, err := file.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error opening image file: " + err.Error()})
			return
		}
		defer src.Close()

		db := me.dbclient.Database("pesatu")
		gridfsBucket, err := gridfs.NewBucket(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating GridFS bucket: " + err.Error()})
			return
		}

		// Generate a unique ID for the image
		imageID := primitive.NewObjectID()
		// Save image to GridFS bucket using unique ID as filename
		uploadStream, err := gridfsBucket.OpenUploadStreamWithID(imageID, imageID.Hex())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error saving image to GridFS bucket: " + err.Error()})
			return
		}
		defer uploadStream.Close()

		_, err = io.Copy(uploadStream, src)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error copying image to GridFS bucket: " + err.Error()})
			return
		}

		// Save image metadata to separate collection
		imageMetadata := bson.M{
			"_id":          imageID,
			"filename":     imageID.Hex(),
			"upload_date":  time.Now(),
			"content_type": file.Header.Get("Content-Type"),
		}

		imageCollection := db.Collection("images")
		_, err = imageCollection.InsertOne(me.ctx, imageMetadata)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error saving image metadata: " + err.Error()})
			return
		}

		// Return image metadata as JSON to client
		c.JSON(http.StatusOK, imageMetadata)
	})
	router.GET("/:id", me.RateLimit, func(c *gin.Context) {
		// Retrieve image ID from request parameters
		imageID := c.Param("id")

		// Create a new GridFS bucket
		db := me.dbclient.Database("pesatu")
		gridfsBucket, err := gridfs.NewBucket(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating GridFS bucket: " + err.Error()})
			return
		}

		// Retrieve image from GridFS bucket
		downloadStream, err := gridfsBucket.OpenDownloadStreamByName(imageID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Image not found: " + err.Error()})
			return
		}
		defer downloadStream.Close()

		// Retrieve metadata about the file
		file := downloadStream.GetFile()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting file metadata: " + err.Error()})
			return
		}

		var metadata FileMetadata
		err = bson.Unmarshal(file.Metadata, &metadata)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error unmarshalling metadata: " + err.Error()})
			return
		}

		// Set headers based on metadata
		c.Header("Content-Type", metadata.ContentType)
		c.Header("Content-Length", strconv.Itoa(int(file.Length)))

		// Send image as response to client
		_, err = io.Copy(c.Writer, downloadStream)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error sending image: " + err.Error()})
			return
		}
	})

	//todo! put delete scheme in controller instead
	router.DELETE("/:id", me.RateLimit, func(c *gin.Context) {
		cookieJwt, err := c.Cookie("jwt")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Error getting jwt: " + err.Error()})
			return
		}

		_, err = auth.ValidateToken(cookieJwt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Error validating jwt: " + err.Error()})
			return
		}
		// Retrieve image ID from request parameters
		imageID := c.Param("id")

		// Create a new GridFS bucket
		db := me.dbclient.Database("pesatu")
		gridfsBucket, err := gridfs.NewBucket(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating GridFS bucket: " + err.Error()})
			return
		}

		// Delete image from GridFS bucket
		err = gridfsBucket.Delete(imageID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Image not found: " + err.Error()})
			return
		}

		// Return success response to client
		c.JSON(http.StatusOK, gin.H{"message": "Image successfully deleted"})
	})

}

func (me *UploadImageRoute) RateLimit(ctx *gin.Context) {
	// Check if the request is allowed by the rate limiter
	if me.limiter.TakeAvailable(1) == 0 {
		// The request is not allowed, so return an error
		ctx.AbortWithStatus(http.StatusTooManyRequests)
		return
	}
	ctx.Next()
}
