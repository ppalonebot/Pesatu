package userprofile

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"pesatu/auth"
	"pesatu/components/user"
	"pesatu/jsonrpc2"
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
	service := NewProfileService(userService.GetCollection(), collection, ctx)
	controller := NewProfileController(userService, service)
	return ProfileRoute{controller, limiter}
}

func (me *ProfileRoute) InitRouteTo(rg *gin.RouterGroup) {
	router := rg.Group("/prf")
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
	case "GetProfile":
		statuscode = me.method_GetProfile(ctx, &jreq, jres)
	case "GetUpdateAvatarToken":
		statuscode = me.method_GetUpdateAvatarToken(ctx, &jreq, jres)
	case "GetWebsocketToken":
		statuscode = me.method_GetWebsocketToken(ctx, &jreq, jres)
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

func (me *ProfileRoute) method_GetProfile(ctx *gin.Context, jreq *jsonrpc2.RPCRequest, jres *jsonrpc2.RPCResponse) int {
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

	res, e, code := me.controller.FindProfile(validuser, reg)
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

	validuser := vuser.(*auth.Claims)
	if validuser.IsExpired() {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusUnauthorized, Message: "session expired"}
		return http.StatusUnauthorized
	}

	var reg *UpdateUserProfile
	err := json.Unmarshal(jreq.Params, &reg)
	if err != nil {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: err.Error()}
		return http.StatusBadRequest
	}

	res, e, code := me.controller.UpdateMyProfile(validuser, reg)
	jres.Result, _ = utils.ToRawMessage(res)
	jres.Error = e

	return code
}

func (me *ProfileRoute) method_GetUpdateAvatarToken(ctx *gin.Context, jreq *jsonrpc2.RPCRequest, jres *jsonrpc2.RPCResponse) int {
	vuser, ok := ctx.Get("validuser")
	if !ok {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusUnauthorized, Message: "unauthorized"}
		return http.StatusUnauthorized
	}

	validuser := vuser.(*auth.Claims)
	if validuser.IsExpired() {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusUnauthorized, Message: "session expired"}
		return http.StatusUnauthorized
	}

	var reg *GetProfileRequest
	err := json.Unmarshal(jreq.Params, &reg)
	if err != nil {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: err.Error()}
		return http.StatusBadRequest
	}

	res, e, code := me.controller.GetUpdateAvatarToken(validuser, reg)
	jres.Result, _ = utils.ToRawMessage(res)
	jres.Error = e

	return code
}

func (me *ProfileRoute) method_GetWebsocketToken(ctx *gin.Context, jreq *jsonrpc2.RPCRequest, jres *jsonrpc2.RPCResponse) int {
	vuser, ok := ctx.Get("validuser")
	if !ok {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusUnauthorized, Message: "unauthorized"}
		return http.StatusUnauthorized
	}

	validuser := vuser.(*auth.Claims)
	if validuser.IsExpired() {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusUnauthorized, Message: "session expired"}
		return http.StatusUnauthorized
	}

	var reg *GetProfileRequest
	err := json.Unmarshal(jreq.Params, &reg)
	if err != nil {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: err.Error()}
		return http.StatusBadRequest
	}

	res, e, code := me.controller.GetWebsocketToken(validuser, reg)
	jres.Result, _ = utils.ToRawMessage(res)
	jres.Error = e

	return code
}
