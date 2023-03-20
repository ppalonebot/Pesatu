package user

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"pesatu/auth"
	"pesatu/jsonrpc2"
	"pesatu/utils"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/juju/ratelimit"
	"go.mongodb.org/mongo-driver/mongo"
)

var Logger logr.Logger = logr.Discard()

type UserRoute struct {
	userController UserController
	limiter        *ratelimit.Bucket
}

func NewUserRoute(mongoclient *mongo.Client, ctx context.Context, l logr.Logger, limiter *ratelimit.Bucket) UserRoute {
	Logger = l
	Logger.V(2).Info("NewUserRoute created")
	userCollection := mongoclient.Database("pesatu").Collection("users")
	userService := NewUserService(userCollection, ctx)
	userController := NewUserController(userService)
	return UserRoute{userController, limiter}
}

func CheckAllowCredentials(ctx *gin.Context, res *ResponseUser, code int) *ResponseUser {
	if res != nil {
		a := ctx.GetHeader("Access-Control-Allow-Credentials")
		c := ctx.GetHeader("credentials")
		// Logger.V(2).Info(fmt.Sprintf("Access-Control-Allow-Credentials : %s", a))
		// Logger.V(2).Info(fmt.Sprintf("credentials : %s", c))
		if Logger.V(2).Enabled() {
			msg := "request header :"
			for k, v := range ctx.Request.Header {
				msg = (fmt.Sprintf("%s\n%s: %s", msg, k, v))
			}
			Logger.V(2).Info(msg)
		}

		if a == "true" || c == "true" {
			Logger.V(2).Info("Set the JWT as an HTTP-only cookie")
			// Set the JWT as an HTTP-only cookie
			http.SetCookie(ctx.Writer, &http.Cookie{
				Name:     "jwt",
				Value:    res.JWT,
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
				Expires:  time.Now().Add(25 * time.Hour),
				Path:     "/",
				// Domain: ".localhost",
			})

			res.JWT = "#included"
		}
	}

	return res
}

func (me *UserRoute) InitRouteTo(rg *gin.RouterGroup) {
	router := rg.Group("/usr")
	router.POST("/rpc", me.RateLimit, me.RPCHandle)
	router.GET("/resetpwd", me.RateLimit, me.ResetPwdHandler)
}

func (me *UserRoute) RateLimit(ctx *gin.Context) {
	// Check if the request is allowed by the rate limiter
	if me.limiter.TakeAvailable(1) == 0 {
		// The request is not allowed, so return an error
		ctx.AbortWithStatus(http.StatusTooManyRequests)
		return
	}
	ctx.Next()
}

func (me *UserRoute) GetUserService() I_UserRepo {
	return me.userController.userService
}

func (me *UserRoute) ResetPwdHandler(c *gin.Context) {
	// Parse the template file
	t, err := utils.GetTemplateData("resetpassword.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "Error parsing template: %v", err)
		return
	}

	// Render the template, writing the resulting HTML to the response body
	t.Execute(c.Writer, nil)
}

func (me *UserRoute) RPCHandle(ctx *gin.Context) {
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
	case "Login":
		statuscode = me.method_Login(ctx, &jreq, jres)
	// case "Logout":
	// 	statuscode = me.method_Logout(ctx, &jreq, jres)
	case "RefreshToken":
		statuscode = me.method_RefreshToken(ctx, &jreq, jres)
	case "Register":
		statuscode = me.method_Register(ctx, &jreq, jres)
	case "ConfirmRegistration":
		statuscode = me.method_ConfirmRegistration(ctx, &jreq, jres)
	case "ResendCode":
		statuscode = me.method_ResendCode(ctx, &jreq, jres)
	case "SendPwdReset":
		statuscode = me.method_SendPwdReset(ctx, &jreq, jres)
	case "ResetPassword":
		statuscode = me.method_ResetPassword(ctx, &jreq, jres)
	case "GetSelf":
		statuscode = me.method_GetSelf(ctx, &jreq, jres)
	case "SearchUser":
		statuscode = me.method_SearchUser(ctx, &jreq, jres)
	default:
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusMethodNotAllowed, Message: "method not allowed"}
	}

	if jres.Error != nil {
		Logger.Error(fmt.Errorf(jres.Error.Message), "response with error")
	}
	ctx.JSON(statuscode, jres)
}

func (me *UserRoute) method_Login(ctx *gin.Context, jreq *jsonrpc2.RPCRequest, jres *jsonrpc2.RPCResponse) int {
	var login *Login
	err := json.Unmarshal(jreq.Params, &login)
	if err != nil {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: err.Error()}
		return http.StatusBadRequest
	}

	res, e, code := me.userController.UserLogin(login)
	res = CheckAllowCredentials(ctx, res, code)
	jres.Result, _ = utils.ToRawMessage(res)
	jres.Error = e

	return code
}

func (me *UserRoute) method_Logout(ctx *gin.Context, jreq *jsonrpc2.RPCRequest, jres *jsonrpc2.RPCResponse) int {
	//todo, this function is not implemented yet on the client
	vuser, ok := ctx.Get("validuser")
	if !ok {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusUnauthorized, Message: "unauthorized"}
		return http.StatusUnauthorized
	}

	validuser := vuser.(*auth.Claims)

	var reg *GetUserRequest
	err := json.Unmarshal(jreq.Params, &reg)
	if err != nil {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: err.Error()}
		return http.StatusBadRequest
	}

	if validuser.GetUID() != reg.UID {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "ilegal jwt"}
		return http.StatusBadRequest
	}

	res, e, code := me.userController.UserLogout(reg.UID, validuser.GetCode())
	jres.Result, _ = utils.ToRawMessage(res)
	jres.Error = e

	return code
}

func (me *UserRoute) method_RefreshToken(ctx *gin.Context, jreq *jsonrpc2.RPCRequest, jres *jsonrpc2.RPCResponse) int {
	vuser, ok := ctx.Get("validuser")
	if !ok {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusUnauthorized, Message: "unauthorized"}
		return http.StatusUnauthorized
	}

	var reg *GetUserRequest
	err := json.Unmarshal(jreq.Params, &reg)
	if err != nil {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: err.Error()}
		return http.StatusBadRequest
	}

	validuser := vuser.(*auth.Claims)
	expiresAt := time.Unix(validuser.ExpiresAt, 0)
	//check if token has been expired more than duration
	if !time.Now().Add(time.Hour * 12).After(expiresAt) {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusUnauthorized, Message: "session expired"}
		return http.StatusUnauthorized
	}

	if validuser.GetUID() != reg.UID {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "ilegal jwt"}
		return http.StatusBadRequest
	}

	res, e, code := me.userController.FindUserById(reg.UID)
	if e == nil {
		res.JWT, _ = auth.CreateJWTToken(reg.UID, res.Username, "")
	}
	res = CheckAllowCredentials(ctx, res, code)
	jres.Result, _ = utils.ToRawMessage(res)
	jres.Error = e

	return code
}

func (me *UserRoute) method_Register(ctx *gin.Context, jreq *jsonrpc2.RPCRequest, jres *jsonrpc2.RPCResponse) int {
	var reg *CreateUserRequest
	err := json.Unmarshal(jreq.Params, &reg)
	if err != nil {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: err.Error()}
		return http.StatusBadRequest
	}

	res, e, code := me.userController.Register(reg)
	res = CheckAllowCredentials(ctx, res, code)
	jres.Result, _ = utils.ToRawMessage(res)
	jres.Error = e

	return code
}

func (me *UserRoute) method_ConfirmRegistration(ctx *gin.Context, jreq *jsonrpc2.RPCRequest, jres *jsonrpc2.RPCResponse) int {
	vuser, ok := ctx.Get("validuser")
	if !ok {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusUnauthorized, Message: "unauthorized"}
		return http.StatusUnauthorized
	}

	var reg *ConfirmRegCode
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

	if validuser.GetUID() != reg.UID {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "ilegal jwt"}
		return http.StatusBadRequest
	}

	res, e, code := me.userController.ConfirmRegistration(reg)
	jres.Result, _ = utils.ToRawMessage(res)
	jres.Error = e

	return code
}

func (me *UserRoute) method_ResendCode(ctx *gin.Context, jreq *jsonrpc2.RPCRequest, jres *jsonrpc2.RPCResponse) int {
	vuser, ok := ctx.Get("validuser")
	if !ok {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusUnauthorized, Message: "unauthorized"}
		return http.StatusUnauthorized
	}

	var reg *GetUserRequest
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

	if validuser.GetUID() != reg.UID {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "ilegal jwt"}
		return http.StatusBadRequest
	}

	res, e, code := me.userController.ResendCode(reg)
	jres.Result, _ = utils.ToRawMessage(res)
	jres.Error = e

	return code
}

func (me *UserRoute) method_SendPwdReset(ctx *gin.Context, jreq *jsonrpc2.RPCRequest, jres *jsonrpc2.RPCResponse) int {
	var reg *ForgotPwdRequest
	err := json.Unmarshal(jreq.Params, &reg)
	if err != nil {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: err.Error()}
		return http.StatusBadRequest
	}

	res, e, code := me.userController.ForgotPassword(reg)
	jres.Result, _ = utils.ToRawMessage(res)
	jres.Error = e

	return code
}

func (me *UserRoute) method_ResetPassword(ctx *gin.Context, jreq *jsonrpc2.RPCRequest, jres *jsonrpc2.RPCResponse) int {
	var reg *PwdResetRequest
	var err error
	err = json.Unmarshal(jreq.Params, &reg)
	if err != nil {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: err.Error()}
		return http.StatusBadRequest
	}

	var validuser *auth.Claims
	validuser, err = me.userController.ValidateToken(reg.JWT)
	if err != nil {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: err.Error()}
		return http.StatusBadRequest
	}

	if validuser.GetCmd() != "ResetPassword" {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "ilegal reset password token"}
		return http.StatusBadRequest
	}

	res, e, code := me.userController.ResetPassword(validuser.GetUID(), reg.Password)
	jres.Result, _ = utils.ToRawMessage(res)
	jres.Error = e

	return code
}

func (me *UserRoute) method_GetSelf(ctx *gin.Context, jreq *jsonrpc2.RPCRequest, jres *jsonrpc2.RPCResponse) int {
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

	var reg *GetUserRequest
	err := json.Unmarshal(jreq.Params, &reg)
	if err != nil {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: err.Error()}
		return http.StatusBadRequest
	}

	if validuser.GetUID() != reg.UID {
		jres.Error = &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "ilegal jwt"}
		return http.StatusBadRequest
	}

	res, e, code := me.userController.FindUserById(reg.UID)
	jres.Result, _ = utils.ToRawMessage(res)
	jres.Error = e

	return code
}

func (me *UserRoute) method_SearchUser(ctx *gin.Context, jreq *jsonrpc2.RPCRequest, jres *jsonrpc2.RPCResponse) int {
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

	res, e, code := me.userController.SearchUsers(reg.Keyword, page, limit, reg.UID)
	jres.Result, _ = utils.ToRawMessage(res)
	jres.Error = e

	return code
}
