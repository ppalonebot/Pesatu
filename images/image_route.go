package images

import (
	"context"
	"net/http"
	"pesatu/user"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/juju/ratelimit"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
)

var Logger logr.Logger = logr.Discard()

const MAX_IMAGE_SIZE = 200 * 1024 // 200KB

type UploadImageRoute struct {
	controller ImageController
	limiter    *ratelimit.Bucket
	ctx        context.Context
}

func NewUploadImageRoute(mongoclient *mongo.Client, ctx context.Context, l logr.Logger, limiter *ratelimit.Bucket, userService user.I_UserRepo) UploadImageRoute {
	Logger = l
	Logger.V(2).Info("New Upload Image Route created")
	db := mongoclient.Database("pesatu")
	gridfsBucket, err := gridfs.NewBucket(db)
	if err != nil {
		Logger.Error(err, "Error creating GridFS bucket")
	}
	service := NewImageService(gridfsBucket, ctx)
	controller := NewImageController(userService, service)
	return UploadImageRoute{controller, limiter, ctx}
}

func (me *UploadImageRoute) InitRouteTo(rg *gin.Engine) {
	router := rg.Group("/image")
	router.POST("/upload", me.RateLimit, me.controller.ImageUploadHandler)
	router.GET("/:id", me.RateLimit, me.controller.GetImageHandler)
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
