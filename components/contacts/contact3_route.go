package contacts

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

type ContactRoute struct {
	contactController ContactController
	limiter           *ratelimit.Bucket
}

func NewContactRoute(mongoclient *mongo.Client, ctx context.Context, l logr.Logger, limiter *ratelimit.Bucket, userService user.I_UserRepo) ContactRoute {
	Logger = l
	Logger.V(2).Info("NewContactRoute created")
	contactCollection := mongoclient.Database("pesatu").Collection("contact")
	contactService := NewContactService(userService.GetCollection(), contactCollection, ctx)
	contactController := NewContactController(userService, contactService)
	return ContactRoute{contactController, limiter}
}

func (me *ContactRoute) InitRouteTo(rg *gin.Engine) {
	router := rg.Group("/contacts")
	router.POST("/rpc", me.RateLimit, me.RPCHandle)
}

func (me *ContactRoute) RateLimit(ctx *gin.Context) {
	// Check if the request is allowed by the rate limiter
	if me.limiter.TakeAvailable(1) == 0 {
		// The request is not allowed, so return an error
		ctx.AbortWithStatus(http.StatusTooManyRequests)
		return
	}
	ctx.Next()
}

func (me *ContactRoute) GetContactService() I_ContactRepo {
	return me.contactController.contactService
}

func (me *ContactRoute) RPCHandle(ctx *gin.Context) {
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
	case "SearchUser":
		statuscode = me.method_SearchUser(ctx, &jreq, jres)
	case "AddContact":
		statuscode = me.method_AddContact(ctx, &jreq, jres)
	// case "GetContacts":
	// 	statuscode = me.method_GetContacts(ctx, &jreq, jres)
	default:
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusMethodNotAllowed, Message: "method not allowed"}
	}

	if jres.Error != nil {
		Logger.Error(fmt.Errorf(jres.Error.Message), "response with error")
	}
	ctx.JSON(statuscode, jres)
}

func (me *ContactRoute) method_SearchUser(ctx *gin.Context, jreq *jsonrpc2.RPCRequest, jres *jsonrpc2.RPCResponse) int {
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

	var reg *SearchUser
	err := json.Unmarshal(jreq.Params, &reg)
	if err != nil {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: err.Error()}
		return http.StatusBadRequest
	}

	if validuser.GetUID() != reg.UID {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "ilegal jwt"}
		return http.StatusBadRequest
	}

	page := reg.Page
	if page == "" {
		page = "1"
	}

	limit := reg.Limit
	if limit == "" {
		limit = "10"
	}

	res, e, code := me.contactController.SearchUsers(reg.Keyword, page, limit, reg.UID, reg.Status)
	jres.Result, _ = utils.ToRawMessage(res)
	jres.Error = e

	return code
}

func (me *ContactRoute) method_AddContact(ctx *gin.Context, jreq *jsonrpc2.RPCRequest, jres *jsonrpc2.RPCResponse) int {
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

	var reg *CreateContact
	err := json.Unmarshal(jreq.Params, &reg)
	if err != nil {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: err.Error()}
		return http.StatusBadRequest
	}

	if validuser.GetUID() != reg.UID {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "ilegal jwt"}
		return http.StatusBadRequest
	}

	res, e, code := me.contactController.CreateContact(validuser, reg)
	jres.Result, _ = utils.ToRawMessage(res)
	jres.Error = e

	return code
}
