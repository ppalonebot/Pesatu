package images

import (
	"context"
	"net/http"
	"pesatu/auth"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/juju/ratelimit"
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
	controller ImageController
	limiter    *ratelimit.Bucket
	ctx        context.Context
}

func NewUploadImageRoute(mongoclient *mongo.Client, ctx context.Context, l logr.Logger, limiter *ratelimit.Bucket) UploadImageRoute {
	Logger = l
	Logger.V(2).Info("New Upload Image Route created")
	db := mongoclient.Database("pesatu")
	gridfsBucket, err := gridfs.NewBucket(db)
	if err != nil {
		Logger.Error(err, "Error creating GridFS bucket")
	}
	service := NewImageService(gridfsBucket, ctx)
	controller := NewImageController(service)
	return UploadImageRoute{controller, limiter, ctx}
}

func (me *UploadImageRoute) InitRouteTo(rg *gin.Engine) {
	router := rg.Group("/image").Use(auth.AuthMiddleware())
	router.POST("/upload", me.RateLimit, me.controller.ImageUploadHandler)
	router.GET("/:id", me.RateLimit, me.controller.GetImageHandler)

	// router.DELETE("/:id", me.RateLimit, func(c *gin.Context) {
	// 	vuser, ok := c.Get("validuser")
	// 	if !ok {
	// 		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
	// 		return
	// 	}

	// 	validuser := vuser.(*auth.Claims)
	// 	if validuser.IsExpired() {
	// 		c.JSON(http.StatusUnauthorized, gin.H{"error": "session expired"})
	// 		return
	// 	}

	// 	// Retrieve image ID from request parameters
	// 	imageID := c.Param("id")

	// 	err := me.controller.DeleteImage(imageID)
	// 	if err != nil {
	// 		c.JSON(http.StatusBadRequest, gin.H{"error": "image " + err.Error()})
	// 		return
	// 	}

	// 	// Return success response to client
	// 	c.JSON(http.StatusOK, gin.H{"message": "image successfully deleted"})
	// })
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
