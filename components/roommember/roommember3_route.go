package roommember

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"pesatu/auth"
	"pesatu/jsonrpc2"
	"pesatu/utils"

	"github.com/gin-gonic/gin"
	"github.com/juju/ratelimit"
	"go.mongodb.org/mongo-driver/mongo"
)

type RoomMemberRoute struct {
	controller RoomMemberController
	limiter    *ratelimit.Bucket
}

func NewRoomMemberRoute(mongoclient *mongo.Client, ctx context.Context, limiter *ratelimit.Bucket) RoomMemberRoute {
	utils.Log().V(2).Info("NewRoomMemberRoute created")
	collection := mongoclient.Database("pesatu").Collection("roommembers")
	service := NewRoomMemberService(nil, collection, ctx)
	controller := NewRoomMemberController(service)
	return RoomMemberRoute{controller, limiter}
}

func (me *RoomMemberRoute) InitRouteTo(rg *gin.RouterGroup) {
	router := rg.Group("/rm")
	router.POST("/rpc", me.RateLimit, me.RPCHandle)
}

func (me *RoomMemberRoute) RateLimit(ctx *gin.Context) {
	// Check if the request is allowed by the rate limiter
	if me.limiter.TakeAvailable(1) == 0 {
		// The request is not allowed, so return an error
		ctx.AbortWithStatus(http.StatusTooManyRequests)
		return
	}
	ctx.Next()
}

func (me *RoomMemberRoute) RPCHandle(ctx *gin.Context) {
	var jreq jsonrpc2.RPCRequest
	if err := ctx.ShouldBindJSON(&jreq); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "jsonrpc fail", "message": err.Error()})
		return
	}

	utils.Log().V(2).Info(fmt.Sprintf("RPCHandle %s", jreq.Method))

	jres := &jsonrpc2.RPCResponse{
		JSONRPC: "2.0",
		ID:      jreq.ID,
	}

	statuscode := http.StatusBadRequest
	switch jreq.Method {
	case "GetLastMessages":
		statuscode = me.method_GetLastMessages(ctx, &jreq, jres)
	default:
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusMethodNotAllowed, Message: "method not allowed"}
	}

	if jres.Error != nil {
		utils.Log().Error(fmt.Errorf(jres.Error.Message), "response with error")
	}
	ctx.JSON(statuscode, jres)
}

func (me *RoomMemberRoute) method_GetLastMessages(ctx *gin.Context, jreq *jsonrpc2.RPCRequest, jres *jsonrpc2.RPCResponse) int {
	vuser, ok := ctx.Get("validuser")
	if !ok {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusUnauthorized, Message: "unauthorized"}
		return http.StatusUnauthorized
	}

	var reg *SearchLastMessage
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

	res, e, code := me.controller.FindLastMessages(validuser, reg)
	jres.Result, _ = utils.ToRawMessage(res)
	jres.Error = e

	return code
}
