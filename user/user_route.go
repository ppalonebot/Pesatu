package user

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/juju/ratelimit"
	"go.mongodb.org/mongo-driver/mongo"
)

var Logger logr.Logger = logr.Discard()

type UserRouteController struct {
	userController UserController
	limiter        *ratelimit.Bucket
}

func NewUserControllerRoute(mongoclient *mongo.Client, ctx context.Context, l logr.Logger, limiter *ratelimit.Bucket) UserRouteController {
	Logger = l
	Logger.V(2).Info("NewUserControllerRoute created")
	userCollection := mongoclient.Database("pesatu").Collection("users")
	userService := NewUserService(userCollection, ctx)
	userController := NewUserController(userService)
	return UserRouteController{userController, limiter}
}

func (r *UserRouteController) InitRouteTo(rg *gin.Engine) {
	router := rg.Group("/usr")
	router.POST("/rpc", func(ctx *gin.Context) {
		// Check if the request is allowed by the rate limiter
		if r.limiter.TakeAvailable(1) == 0 {
			// The request is not allowed, so return an error
			ctx.AbortWithStatus(http.StatusTooManyRequests)
			return
		}

		r.userController.RPCHandle(ctx)
	})
}
