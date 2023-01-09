package userprofile

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"pesatu/auth"
	"pesatu/jsonrpc2"
	"pesatu/user"
	"pesatu/utils"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/juju/ratelimit"
	"go.mongodb.org/mongo-driver/mongo"
)

var Logger logr.Logger = logr.Discard()

type ProfileRoute struct {
	controller ProfileController
	limiter    *ratelimit.Bucket
}

func NewProfileRoute(mongoclient *mongo.Client, ctx context.Context, l logr.Logger, limiter *ratelimit.Bucket, userService user.I_UserRepo) ProfileRoute {
	Logger = l
	Logger.V(2).Info("NewProfileRoute created")
	collection := mongoclient.Database("pesatu").Collection("profiles")
	service := NewProfileService(collection, ctx)
	controller := NewProfileController(userService, service, mongoclient)
	return ProfileRoute{controller, limiter}
}

func (me *ProfileRoute) InitRouteTo(rg *gin.Engine) {
	router := rg.Group("/prf").Use(auth.AuthMiddleware())
	router.POST("/rpc", me.RateLimit, me.RPCHandle)
}

func (me *ProfileRoute) RateLimit(ctx *gin.Context) {
	// Check if the request is allowed by the rate limiter
	if me.limiter.TakeAvailable(1) == 0 {
		// The request is not allowed, so return an error
		ctx.AbortWithStatus(http.StatusTooManyRequests)
		return
	}
	ctx.Next()
}

func (me *ProfileRoute) RPCHandle(ctx *gin.Context) {
	var jreq jsonrpc2.RPCRequest
	if err := ctx.ShouldBindJSON(&jreq); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "jsonrpc fail", "message": err.Error()})
		return
	}

	Logger.V(2).Info(fmt.Sprintf("RPCHandle %s", jreq.Method))

	jres := &jsonrpc2.RPCResponse{
		JSONRPC: "2.0",
		ID:      jreq.ID,
	}

	statuscode := http.StatusBadRequest
	switch jreq.Method {
	case "GetMyProfile":
		statuscode = me.method_GetMyProfile(ctx, &jreq, jres)
	case "UpdateMyProfile":
		statuscode = me.method_UpdateMyProfile(ctx, &jreq, jres)
	default:
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusMethodNotAllowed, Message: "method not allowed"}
	}

	if jres.Error != nil {
		Logger.Error(fmt.Errorf(jres.Error.Message), "response with error")
	}
	ctx.JSON(statuscode, jres)
}

func (me *ProfileRoute) method_GetMyProfile(ctx *gin.Context, jreq *jsonrpc2.RPCRequest, jres *jsonrpc2.RPCResponse) int {
	vuser, ok := ctx.Get("validuser")
	if !ok {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusUnauthorized, Message: "unauthorized"}
		return http.StatusUnauthorized
	}

	var reg *GetProfileRequest
	err := json.Unmarshal(jreq.Params, &reg)
	if err != nil {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: err.Error()}
		return http.StatusBadRequest
	}

	validuser := vuser.(*auth.Claims)
	if validuser.IsExpired() {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusUnauthorized, Message: "session expired"}
		return http.StatusUnauthorized
	}

	res, e, code := me.controller.FindMyProfile(validuser, reg.UID)
	jres.Result, _ = utils.ToRawMessage(res)
	jres.Error = e

	return code
}

func (me *ProfileRoute) method_UpdateMyProfile(ctx *gin.Context, jreq *jsonrpc2.RPCRequest, jres *jsonrpc2.RPCResponse) int {
	vuser, ok := ctx.Get("validuser")
	if !ok {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusUnauthorized, Message: "unauthorized"}
		return http.StatusUnauthorized
	}

	var reg *UpdateUserProfile
	err := json.Unmarshal(jreq.Params, &reg)
	if err != nil {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: err.Error()}
		return http.StatusBadRequest
	}

	validuser := vuser.(*auth.Claims)
	if validuser.IsExpired() {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusUnauthorized, Message: "session expired"}
		return http.StatusUnauthorized
	}

	res, e, code := me.controller.UpdateMyProfile(validuser, reg)
	jres.Result, _ = utils.ToRawMessage(res)
	jres.Error = e

	return code
}
