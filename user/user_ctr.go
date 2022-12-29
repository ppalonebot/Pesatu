package user

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"pesatu/auth"
	"pesatu/jsonrpc2"
	"pesatu/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UserController struct {
	userService I_UserRepo
}

func NewUserController(userService I_UserRepo) UserController {
	return UserController{userService}
}

func SendCodeEmail(email string, code *Registration) {
	//todo: go send email list goroutine
	err := auth.SendCodeMail(email, code)
	if err != nil {
		Logger.Error(err, "send email error")
	}
}

func (uc *UserController) Register(user *CreateUserRequest) (*ResponseUser, *jsonrpc2.RPCError, int) {
	if Logger.V(2).Enabled() {
		jsonBytes, err := json.Marshal(user)
		if err == nil {
			Logger.V(2).Info(fmt.Sprintf("register %s", string(jsonBytes)))
		}
	}

	errres := make([]*jsonrpc2.InputFieldError, 0, 4)

	_, err := utils.IsValidName(user.Name)
	if err != nil {
		errres = append(errres, &jsonrpc2.InputFieldError{Error: err.Error(), Field: "name"})
	}

	_, err = utils.IsValidUsername(user.Username)
	if err != nil {
		errres = append(errres, &jsonrpc2.InputFieldError{Error: err.Error(), Field: "username"})
	} else {
		exist, _ := uc.userService.FindUserByUsername(user.Username)
		if exist != nil {
			errres = append(errres, &jsonrpc2.InputFieldError{Error: "username unavailable", Field: "username"})
		}
	}

	_, err = utils.IsValidPassword(user.Password)
	if err != nil {
		errres = append(errres, &jsonrpc2.InputFieldError{Error: err.Error(), Field: "password"})
	}

	ok := utils.IsValidEmail(user.Email)
	if !ok {
		errres = append(errres, &jsonrpc2.InputFieldError{Error: "email invalid", Field: "email"})
	} else {
		exist, _ := uc.userService.FindUserByEmail(user.Email)
		if exist != nil {
			errres = append(errres, &jsonrpc2.InputFieldError{Error: "email already registered", Field: "email"})
		}
	}

	if len(errres) > 0 {
		x, err := utils.ToRawMessage(errres)
		if err != nil {
			return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: err.Error()}, http.StatusForbidden
		}
		return nil, &jsonrpc2.RPCError{
			Code:    http.StatusForbidden,
			Message: "forbidden, invalid input",
			Params:  x,
		}, http.StatusForbidden
	}

	password, _ := auth.GeneratePassword("password")
	nu := &CreateUser{
		UID:      uuid.New().String(),
		Name:     user.Name,
		Username: user.Username,
		Email:    user.Email,
		Password: password,
		Reg: &Registration{
			Registered: false,
			Code:       utils.GenerateRandomNumber(),
			SendCodeAt: time.Now(),
		},
	}

	newUser, err := uc.userService.CreateUser(nu)

	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return nil, &jsonrpc2.RPCError{Code: http.StatusConflict, Message: err.Error()}, http.StatusConflict
		}
		return nil, &jsonrpc2.RPCError{Code: http.StatusBadGateway, Message: err.Error()}, http.StatusBadGateway
	}

	// Create a JWT
	token, err := auth.CreateJWTToken(newUser.UID, newUser.Username)

	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusNotFound, Message: err.Error()}, http.StatusNotFound
	}

	var resUser ResponseUser
	utils.CopyStruct(newUser, &resUser)
	resUser.IsRegistered = newUser.Reg.Registered
	resUser.JWT = token

	go SendCodeEmail(newUser.Email, newUser.Reg)

	Logger.V(2).Info("register success")
	return &resUser, nil, http.StatusCreated
}

// func (uc *UserController) UpdateUser(ctx *gin.Context) {
// 	userId := ctx.Param("userId")

// 	var user *entity.UpdateUser
// 	if err := ctx.ShouldBindJSON(&user); err != nil {
// 		ctx.JSON(http.StatusBadGateway, gin.H{"status": "fail", "message": err.Error()})
// 		return
// 	}

// 	updatedUser, err := uc.userService.UpdateUser(userId, user)
// 	if err != nil {
// 		if strings.Contains(err.Error(), "Id exists") {
// 			ctx.JSON(http.StatusNotFound, gin.H{"status": "fail", "message": err.Error()})
// 			return
// 		}
// 		ctx.JSON(http.StatusBadGateway, gin.H{"status": "fail", "message": err.Error()})
// 		return
// 	}

//		ctx.JSON(http.StatusOK, gin.H{"status": "success", "data": updatedUser})
//	}

func (uc *UserController) ResetPassword(uid, newPassword string) (*ResponseStatus, *jsonrpc2.RPCError, int) {
	Logger.V(2).Info(fmt.Sprintf("reset password %s", uid))

	ok := utils.IsValidUid(uid)
	if !ok {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "uid invalid"}, http.StatusForbidden
	}

	_, err := utils.IsValidPassword(newPassword)
	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: err.Error()}, http.StatusForbidden
	}

	user, err := uc.userService.FindUserById(uid)
	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: err.Error()}, http.StatusForbidden
	}

	password, _ := auth.GeneratePassword(newPassword)
	user.Password = password

	user, err = uc.userService.UpdateUser(user.Id, user)
	if err != nil {
		Logger.Error(err, "internal error, while update user in ResendCode")
	}

	Logger.V(2).Info("reset password success")
	return &ResponseStatus{UID: user.UID, Status: "success"}, nil, http.StatusOK
}

func (uc *UserController) ForgotPassword(req *ForgotPwdRequest) (*ResponseStatus, *jsonrpc2.RPCError, int) {
	Logger.V(2).Info(fmt.Sprintf("forgot password prosedure for %s", req.Email))

	if isemail := utils.IsValidEmail(req.Email); !isemail {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "email invalid"}, http.StatusForbidden
	}

	user, err := uc.userService.FindUserByEmail(req.Email)
	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: err.Error()}, http.StatusForbidden
	}

	if !user.Reg.Registered {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "user is not registered"}, http.StatusForbidden
	}

	delta := time.Since(user.Reg.SendCodeAt)
	if delta.Seconds() < 50.0 {
		// Time delta is less than 30 seconds
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: fmt.Sprintf("please try again after %.1f seconds", 50.0-delta.Seconds())}, http.StatusForbidden
	}

	jwt, err := auth.CreateJWTWithExpire(user.UID, user.Username, auth.AnHour)
	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusInternalServerError, Message: "create JWT failed"}, http.StatusInternalServerError
	}

	//todo: go send email list goroutine
	go func(email, token string) {
		data := &PwdResetRequest{JWT: token}
		err := auth.SendCodeMail(email, data)
		if err != nil {
			Logger.Error(err, "send email error")
		}
	}(user.Email, jwt)

	user.Reg.SendCodeAt = time.Now()
	user, err = uc.userService.UpdateUser(user.Id, user)
	if err != nil {
		Logger.Error(err, "internal error, while update user in ForgotPassword")
	}

	Logger.V(2).Info("send success")
	return &ResponseStatus{UID: "", Status: "success"}, nil, http.StatusOK
}

func (uc *UserController) ResendCode(req *GetUserRequest) (*ResponseStatus, *jsonrpc2.RPCError, int) {
	Logger.V(2).Info(fmt.Sprintf("resend code for %s", req.UID))

	ok := utils.IsValidUid(req.UID)
	if !ok {
		return nil, &jsonrpc2.RPCError{
			Code:    http.StatusForbidden,
			Message: "uid invalid",
		}, http.StatusForbidden
	}

	user, err := uc.userService.FindUserById(req.UID)
	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: err.Error()}, http.StatusForbidden
	}

	if user.Reg.Registered {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "user already registered"}, http.StatusForbidden
	}

	delta := time.Since(user.Reg.SendCodeAt)
	if delta.Seconds() < 50.0 {
		// Time delta is less than 30 seconds
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: fmt.Sprintf("please try again after %.1f seconds", 50.0-delta.Seconds())}, http.StatusForbidden
	}

	go SendCodeEmail(user.Email, user.Reg)
	user.Reg.SendCodeAt = time.Now()
	user, err = uc.userService.UpdateUser(user.Id, user)
	if err != nil {
		Logger.Error(err, "internal error, while update user in ResendCode")
	}

	Logger.V(2).Info("send success")
	return &ResponseStatus{UID: user.UID, Status: "success"}, nil, http.StatusOK
}

func (uc *UserController) ConfirmRegistration(confirm *ConfirmRegCode) (*ResponseUser, *jsonrpc2.RPCError, int) {
	if Logger.V(2).Enabled() {
		jsonBytes, err := json.Marshal(confirm)
		if err == nil {
			Logger.V(2).Info(fmt.Sprintf("confirm registration %s", string(jsonBytes)))
		}
	}

	_, err := utils.IsValidActivationCode(confirm.Code)
	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: err.Error()}, http.StatusForbidden
	}

	ok := utils.IsValidUid(confirm.UID)
	if !ok {
		return nil, &jsonrpc2.RPCError{
			Code:    http.StatusForbidden,
			Message: "uid invalid",
		}, http.StatusForbidden
	}

	user, err := uc.userService.FindUserById(confirm.UID)
	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: err.Error()}, http.StatusForbidden
	}

	if user.Reg.Registered {
		return nil, &jsonrpc2.RPCError{Code: http.StatusNotAcceptable, Message: "user already registered"}, http.StatusNotAcceptable
	}

	if user.Reg.Code == confirm.Code {
		user.Reg.Registered = true
		user, err = uc.userService.UpdateUser(user.Id, user)
		if err != nil {
			return nil, &jsonrpc2.RPCError{Code: http.StatusInternalServerError, Message: err.Error()}, http.StatusInternalServerError
		}
	} else {
		return nil, &jsonrpc2.RPCError{Code: http.StatusNotFound, Message: "wrong code"}, http.StatusNotFound
	}

	var resUser ResponseUser
	utils.CopyStruct(user, &resUser)
	resUser.IsRegistered = user.Reg.Registered

	Logger.V(2).Info("registration confirmed")
	return &resUser, nil, http.StatusOK

}

func (uc *UserController) UserLogin(login *Login) (*ResponseUser, *jsonrpc2.RPCError, int) {
	Logger.V(2).Info(fmt.Sprintf("Login attempt from %s", login.Username))

	var user *DBUser
	var err error
	if isemail := utils.IsValidEmail(login.Username); isemail {
		Logger.V(2).Info("login with email")
		user, err = uc.userService.FindUserByEmail(login.Username)
	} else {
		Logger.V(2).Info("login with username")
		_, err = utils.IsValidUsername(login.Username)
		if err != nil {
			return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: err.Error()}, http.StatusForbidden
		}

		user, err = uc.userService.FindUserByUsername(login.Username)
	}

	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusNotFound, Message: err.Error()}, http.StatusNotFound
	}

	//check password
	ok, err := auth.ComparePassword(login.Password, user.Password)
	if !ok || err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "wrong password"}, http.StatusForbidden
	}

	// Create a JWT
	token, err := auth.CreateJWTToken(user.UID, user.Username)
	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusInternalServerError, Message: err.Error()}, http.StatusInternalServerError
	}

	var resUser ResponseUser
	utils.CopyStruct(user, &resUser)
	resUser.IsRegistered = user.Reg.Registered
	resUser.JWT = token

	Logger.V(2).Info("logged in")
	return &resUser, nil, http.StatusOK
}

func (uc *UserController) ValidateToken(jwt string) (*auth.Claims, error) {
	if len(jwt) >= 1 {
		user, err := auth.ValidateToken(jwt)
		if err != nil {
			return nil, err
		} else {
			return user, nil
		}
	} else {
		return nil, errors.New("Please login")
	}
}

func (uc *UserController) FindUserById(userUID string) (*ResponseUser, *jsonrpc2.RPCError, int) {
	Logger.V(2).Info(fmt.Sprintf("find a user by uid %s", userUID))

	ok := utils.IsValidUid(userUID)
	if !ok {
		return nil, &jsonrpc2.RPCError{
			Code:    http.StatusForbidden,
			Message: "uid invalid",
		}, http.StatusForbidden
	}

	user, err := uc.userService.FindUserById(userUID)

	if err != nil {
		if strings.Contains(err.Error(), "exists") {
			return nil, &jsonrpc2.RPCError{
				Code:    http.StatusNotFound,
				Message: err.Error(),
			}, http.StatusNotFound
		}

		return nil, &jsonrpc2.RPCError{
			Code:    http.StatusBadGateway,
			Message: err.Error(),
		}, http.StatusBadGateway
	}

	var resUser ResponseUser
	utils.CopyStruct(user, &resUser)
	resUser.IsRegistered = user.Reg.Registered

	Logger.V(2).Info("found ", user.Username)
	return &resUser, nil, http.StatusOK
}

// func (uc *UserController) FindUsers(ctx *gin.Context) {
// 	var page = ctx.DefaultQuery("page", "1")
// 	var limit = ctx.DefaultQuery("limit", "10")

// 	intPage, err := strconv.Atoi(page)
// 	if err != nil {
// 		ctx.JSON(http.StatusBadGateway, gin.H{"status": "fail", "message": err.Error()})
// 		return
// 	}

// 	intLimit, err := strconv.Atoi(limit)
// 	if err != nil {
// 		ctx.JSON(http.StatusBadGateway, gin.H{"status": "fail", "message": err.Error()})
// 		return
// 	}

// 	users, err := uc.userService.FindUsers(intPage, intLimit)
// 	if err != nil {
// 		ctx.JSON(http.StatusBadGateway, gin.H{"status": "fail", "message": err.Error()})
// 		return
// 	}

// 	ctx.JSON(http.StatusOK, gin.H{"status": "success", "results": len(users), "data": users})
// }

// func (uc *UserController) DeleteUser(ctx *gin.Contextgin
// 	userId := ctx.Param("userId")

// 	err := uc.userService.DeleteUser(userId)

// 	if err != nil {
// 		if strings.Contains(err.Error(), "Id exists") {
// 			ctx.JSON(http.StatusNotFound, gin.H{"status": "fail", "message": err.Error()})
// 			return
// 		}
// 		ctx.JSON(http.StatusBadGateway, gin.H{"status": "fail", "message": err.Error()})
// 		return
// 	}

// 	// ctx.JSON(http.StatusNoContent, nil)
// 	ctx.JSON(http.StatusOK, gin.H{"status": "deleted"})
// }

func (uc *UserController) RPCHandle(ctx *gin.Context) {
	statuscode := http.StatusBadRequest
	var jreq jsonrpc2.RPCRequest
	if err := ctx.ShouldBindJSON(&jreq); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "jsonrpc fail", "message": err.Error()})
		return
	} else {
		Logger.V(2).Info("RPCHandle", jreq.Method)
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
			res, e, code := uc.UserLogin(login)
			jres.Result, _ = utils.ToRawMessage(res)
			jres.Error = e
			statuscode = code
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
			res, e, code := uc.Register(reg)
			jres.Result, _ = utils.ToRawMessage(res)
			jres.Error = e
			statuscode = code
		}
	case "ConfirmRegistration":
		var confirm *ConfirmRegCode
		iserror := false
		err := json.Unmarshal(jreq.Params, &confirm)
		var validuser *auth.Claims
		if err == nil {
			validuser, err = uc.ValidateToken(confirm.JWT)
			if err == nil && validuser.GetUID() == confirm.UID {
				res, e, code := uc.ConfirmRegistration(confirm)
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
		var validuser *auth.Claims
		if err == nil {
			validuser, err = uc.ValidateToken(reg.JWT)
			if err == nil && validuser.GetUID() == reg.UID {
				res, e, code := uc.ResendCode(reg)
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
			res, e, code := uc.ForgotPassword(reg)
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
		iserror := false
		err := json.Unmarshal(jreq.Params, &reg)
		if err == nil {
			validuser, err := uc.ValidateToken(reg.JWT)
			if err == nil {
				res, e, code := uc.ResetPassword(validuser.GetUID(), reg.Password)
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

	case "GetSelf":
		var reg *GetUserRequest
		iserror := false
		err := json.Unmarshal(jreq.Params, &reg)
		var validuser *auth.Claims
		if err == nil {
			validuser, err = uc.ValidateToken(reg.JWT)
			if err == nil && validuser.GetUID() == reg.UID {
				res, e, code := uc.FindUserById(reg.UID)
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
		Logger.Error(errors.New(fmt.Sprintf("method not allowed: %s", jreq.Method)), "method error")
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
