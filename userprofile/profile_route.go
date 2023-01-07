package userprofile

import (
	"context"
	"encoding/json"
	"errors"
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
	statuscode := http.StatusBadRequest
	var jreq jsonrpc2.RPCRequest
	if err := ctx.ShouldBindJSON(&jreq); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "jsonrpc fail", "message": err.Error()})
		return
	} else {
		Logger.V(2).Info(fmt.Sprintf("RPCHandle %s", jreq.Method))
	}

	jres := &jsonrpc2.RPCResponse{
		JSONRPC: "2.0",
		ID:      jreq.ID,
	}

	switch jreq.Method {
	case "GetMyProfile":
		statuscode = me.GetMyProfile(ctx, statuscode, &jreq, jres)
	case "UpdateMyProfile":
		statuscode = me.UpdateMyProfile(ctx, statuscode, &jreq, jres)
	default:
		jres.Error = &jsonrpc2.RPCError{
			Code:    http.StatusMethodNotAllowed,
			Message: "method not allowed",
		}
	}

	if jres.Error != nil {
		Logger.Error(errors.New(jres.Error.Message), "response with error")
	}
	ctx.JSON(statuscode, jres)
}

func (me *ProfileRoute) GetMyProfile(ctx *gin.Context, statuscode int, jreq *jsonrpc2.RPCRequest, jres *jsonrpc2.RPCResponse) int {
	cookieJwt, errCookieJwt := ctx.Cookie("jwt")
	var reg *GetProfileRequest
	var err error
	iserror := false
	err = json.Unmarshal(jreq.Params, &reg)
	if err == nil {
		if errCookieJwt == nil {
			Logger.V(2).Info(fmt.Sprintf("use cookie token %s", cookieJwt))
			reg.JWT = cookieJwt
		} else {
			Logger.V(2).Error(err, "token anavailable")
		}
		var validuser *auth.Claims
		validuser, err = auth.ValidateToken(reg.JWT)
		if err == nil {
			res, e, code := me.controller.FindMyProfile(validuser, reg.UID)
			jres.Result, _ = utils.ToRawMessage(res)
			jres.Error = e
			statuscode = code

		} else {
			iserror = true
		}
	} else {
		iserror = true
	}

	if iserror {
		statuscode = http.StatusBadRequest
		jres.Error = &jsonrpc2.RPCError{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		}
	}

	return statuscode
}

func (me *ProfileRoute) UpdateMyProfile(ctx *gin.Context, statuscode int, jreq *jsonrpc2.RPCRequest, jres *jsonrpc2.RPCResponse) int {
	cookieJwt, errCookieJwt := ctx.Cookie("jwt")
	var reg *UpdateUserProfile
	var err error
	iserror := false
	err = json.Unmarshal(jreq.Params, &reg)
	if err == nil {
		if errCookieJwt == nil {
			reg.JWT = cookieJwt
		}
		var validuser *auth.Claims
		validuser, err = auth.ValidateToken(reg.JWT)
		if err == nil {
			res, e, code := me.controller.UpdateMyProfile(validuser, reg)
			jres.Result, _ = utils.ToRawMessage(res)
			jres.Error = e
			statuscode = code

		} else {
			iserror = true
		}
	} else {
		iserror = true
	}

	if iserror {
		statuscode = http.StatusBadRequest
		jres.Error = &jsonrpc2.RPCError{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		}
	}

	return statuscode
}
