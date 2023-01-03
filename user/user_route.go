package user

import (
	"context"
	"encoding/json"
	"errors"
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

type UserRouteController struct {
	userController UserController
	limiter        *ratelimit.Bucket
}

func NewUserControllerRoute(mongoclient *mongo.Client, ctx context.Context, l logr.Logger, limiter *ratelimit.Bucket) UserRouteController {
	Logger = l
	Logger.V(2).Info("NewUserRoute created")
	userCollection := mongoclient.Database("pesatu").Collection("users")
	userService := NewUserService(userCollection, ctx)
	userController := NewUserController(userService)
	return UserRouteController{userController, limiter}
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
				// Domain: ".localhost",
			})

			res.JWT = "#included"
		}
	}

	return res
}

func (r *UserRouteController) InitRouteTo(rg *gin.Engine) {
	router := rg.Group("/usr")
	router.POST("/rpc", r.RateLimit, r.RPCHandle)
	router.GET("/resetpwd", r.RateLimit, r.ResetPwdHandler)
}

func (r *UserRouteController) RateLimit(ctx *gin.Context) {
	// Check if the request is allowed by the rate limiter
	if r.limiter.TakeAvailable(1) == 0 {
		// The request is not allowed, so return an error
		ctx.AbortWithStatus(http.StatusTooManyRequests)
		return
	}
	ctx.Next()
}

func (r *UserRouteController) ResetPwdHandler(c *gin.Context) {
	// Parse the template file
	t, err := utils.GetTemplateData("resetpassword.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "Error parsing template: %v", err)
		return
	}

	// Render the template, writing the resulting HTML to the response body
	t.Execute(c.Writer, nil)
}

func (r *UserRouteController) RPCHandle(ctx *gin.Context) {
	cookieJwt, errCookieJwt := ctx.Cookie("jwt")
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
	case "Login":
		var login *Login
		err := json.Unmarshal(jreq.Params, &login)
		if err != nil {
			statuscode = http.StatusBadRequest
			jres.Error = &jsonrpc2.RPCError{
				Code:    http.StatusBadRequest,
				Message: err.Error(),
			}
		} else {
			res, e, code := r.userController.UserLogin(login)
			res = CheckAllowCredentials(ctx, res, code)
			jres.Result, _ = utils.ToRawMessage(res)
			jres.Error = e
			statuscode = code
		}
	case "RefreshToken":
		var reg *GetUserRequest
		var err error
		iserror := false
		err = json.Unmarshal(jreq.Params, &reg)
		if err == nil {
			if errCookieJwt == nil {
				reg.JWT = cookieJwt
			}
			var validuser *auth.Claims
			validuser, err = r.userController.ValidateToken(reg.JWT)
			expiresAt := time.Unix(validuser.ExpiresAt, 0)
			//check if token has been expired more than duration
			if time.Now().Add(time.Hour * 12).After(expiresAt) {
				if validuser != nil && validuser.GetUID() == reg.UID {
					res, e, code := r.userController.FindUserById(reg.UID, validuser.GetCode())
					if e == nil {
						res.JWT, _ = auth.CreateJWTToken(reg.UID, res.Username, validuser.GetCode())
					}
					res = CheckAllowCredentials(ctx, res, code)
					jres.Result, _ = utils.ToRawMessage(res)
					jres.Error = e
					statuscode = code
				} else {
					iserror = true
				}
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

	case "Register":
		var reg *CreateUserRequest
		err := json.Unmarshal(jreq.Params, &reg)
		if err != nil {
			statuscode = http.StatusBadRequest
			jres.Error = &jsonrpc2.RPCError{
				Code:    http.StatusBadRequest,
				Message: err.Error(),
			}
		} else {
			res, e, code := r.userController.Register(reg)
			res = CheckAllowCredentials(ctx, res, code)
			jres.Result, _ = utils.ToRawMessage(res)
			jres.Error = e
			statuscode = code
		}
	case "ConfirmRegistration":
		var reg *ConfirmRegCode
		iserror := false
		err := json.Unmarshal(jreq.Params, &reg)
		if err == nil {
			if errCookieJwt == nil {
				reg.JWT = cookieJwt
			}
			var validuser *auth.Claims
			validuser, err = r.userController.ValidateToken(reg.JWT)
			if err == nil && validuser.GetUID() == reg.UID {
				res, e, code := r.userController.ConfirmRegistration(reg)
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

	case "ResendCode":
		var reg *GetUserRequest
		iserror := false
		err := json.Unmarshal(jreq.Params, &reg)
		if err == nil {
			if errCookieJwt == nil {
				reg.JWT = cookieJwt
			}
			var validuser *auth.Claims
			validuser, err = r.userController.ValidateToken(reg.JWT)
			if err == nil && validuser.GetUID() == reg.UID {
				res, e, code := r.userController.ResendCode(reg)
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

	case "SendPwdReset":
		var reg *ForgotPwdRequest
		err := json.Unmarshal(jreq.Params, &reg)
		if err == nil {
			res, e, code := r.userController.ForgotPassword(reg)
			jres.Result, _ = utils.ToRawMessage(res)
			jres.Error = e
			statuscode = code
		} else {
			statuscode = http.StatusBadRequest
			jres.Error = &jsonrpc2.RPCError{
				Code:    http.StatusBadRequest,
				Message: err.Error(),
			}
		}

	case "ResetPassword":
		var reg *PwdResetRequest
		var err error
		iserror := false
		err = json.Unmarshal(jreq.Params, &reg)
		if err == nil {
			var validuser *auth.Claims
			validuser, err = r.userController.ValidateToken(reg.JWT)
			if err == nil {
				if validuser.GetCmd() == "ResetPassword" {
					res, e, code := r.userController.ResetPassword(validuser.GetUID(), reg.Password, validuser.GetCode())
					jres.Result, _ = utils.ToRawMessage(res)
					jres.Error = e
					statuscode = code
				} else {
					err = errors.New("invalid JWT cmd")
					iserror = true
				}
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

	case "GetSelf":
		var reg *GetUserRequest
		iserror := false
		err := json.Unmarshal(jreq.Params, &reg)
		if err == nil {
			if errCookieJwt == nil {
				reg.JWT = cookieJwt
			}
			var validuser *auth.Claims
			validuser, err = r.userController.ValidateToken(reg.JWT)
			if err == nil && validuser.GetUID() == reg.UID {
				res, e, code := r.userController.FindUserById(reg.UID, validuser.GetCode())
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
