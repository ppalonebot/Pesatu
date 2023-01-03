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

func (uc *UserController) Register(regUser *CreateUserRequest) (*ResponseUser, *jsonrpc2.RPCError, int) {
	if Logger.V(2).Enabled() {
		jsonBytes, err := json.Marshal(regUser)
		if err == nil {
			Logger.V(2).Info(fmt.Sprintf("register %s", string(jsonBytes)))
		}
	}

	errres := make([]*jsonrpc2.InputFieldError, 0, 4)

	_, err := utils.IsValidName(regUser.Name)
	if err != nil {
		errres = append(errres, &jsonrpc2.InputFieldError{Error: err.Error(), Field: "name"})
	}

	regUser.Username = strings.ToLower(regUser.Username)
	_, err = utils.IsValidUsername(regUser.Username)
	if err != nil {
		errres = append(errres, &jsonrpc2.InputFieldError{Error: err.Error(), Field: "username"})
	} else {
		exist, _ := uc.userService.FindUserByUsername(regUser.Username)
		if exist != nil {
			errres = append(errres, &jsonrpc2.InputFieldError{Error: "username unavailable", Field: "username"})
		}
	}

	_, err = utils.IsValidPassword(regUser.Password)
	if err != nil {
		errres = append(errres, &jsonrpc2.InputFieldError{Error: err.Error(), Field: "password"})
	}

	regUser.Email = strings.ToLower(regUser.Email)
	ok := utils.IsValidEmail(regUser.Email)
	if !ok {
		errres = append(errres, &jsonrpc2.InputFieldError{Error: "email invalid", Field: "email"})
	} else {
		exist, _ := uc.userService.FindUserByEmail(regUser.Email)
		if exist != nil {
			errres = append(errres, &jsonrpc2.InputFieldError{Error: "email already registered", Field: "email"})
		}
	}

	if len(errres) > 0 {
		// x, err := utils.ToRawMessage(errres)
		// if err != nil {
		// 	return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: err.Error()}, http.StatusOK
		// }
		return nil, &jsonrpc2.RPCError{
			Code:    http.StatusForbidden,
			Message: "forbidden, invalid input",
			Params:  errres,
		}, http.StatusOK
	}

	password, _ := auth.GeneratePassword("password")
	nu := &CreateUser{
		UID:      uuid.New().String(),
		Name:     regUser.Name,
		Username: regUser.Username,
		Email:    regUser.Email,
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
			return nil, &jsonrpc2.RPCError{Code: http.StatusConflict, Message: err.Error()}, http.StatusOK
		}
		return nil, &jsonrpc2.RPCError{Code: http.StatusBadGateway, Message: err.Error()}, http.StatusOK
	}

	// Create a JWT
	token, err := auth.CreateJWTToken(newUser.UID, newUser.Username, newUser.Reg.Code)

	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusInternalServerError, Message: err.Error()}, http.StatusOK
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

func (uc *UserController) ResetPassword(uid, newPassword, code string) (*ResponseStatus, *jsonrpc2.RPCError, int) {
	Logger.V(2).Info(fmt.Sprintf("reset password %s", uid))

	ok := utils.IsValidUid(uid)
	if !ok {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "uid invalid"}, http.StatusOK
	}

	_, err := utils.IsValidPassword(newPassword)
	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: err.Error()}, http.StatusOK
	}

	user, err := uc.userService.FindUserById(uid)
	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusNotFound, Message: err.Error()}, http.StatusOK
	}

	if user.Reg.Code != code {
		return nil, &jsonrpc2.RPCError{Code: http.StatusNotFound, Message: "invalid jwt"}, http.StatusOK
	}

	password, _ := auth.GeneratePassword(newPassword)
	user.Password = password

	if user.Reg.Registered {
		user.Reg.Code = utils.GenerateRandomNumber()
	}

	user, err = uc.userService.UpdateUser(user.Id, user)
	if err != nil {
		Logger.Error(err, "internal error, while update user in ResendCode")
	}

	Logger.V(2).Info("reset password success")
	return &ResponseStatus{UID: user.UID, Status: "success"}, nil, http.StatusOK
}

func (uc *UserController) ForgotPassword(req *ForgotPwdRequest) (*ResponseStatus, *jsonrpc2.RPCError, int) {
	Logger.V(2).Info(fmt.Sprintf("forgot password prosedure for %s", req.Email))

	req.Email = strings.ToLower(req.Email)
	var errres []*jsonrpc2.InputFieldError
	if isemail := utils.IsValidEmail(req.Email); !isemail {
		errres = append(errres, &jsonrpc2.InputFieldError{Error: "invalid email format", Field: "email"})
		return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "email error", Params: errres}, http.StatusOK
	}

	user, err := uc.userService.FindUserByEmail(req.Email)
	if err != nil {
		errres = append(errres, &jsonrpc2.InputFieldError{Error: err.Error(), Field: "email"})
		return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: err.Error(), Params: errres}, http.StatusOK
	}

	code := user.Reg.Code
	if !user.Reg.Registered {
		// errres = append(errres, &jsonrpc2.InputFieldError{Error: "user is not registered", Field: "email"})
		// return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "user unknown", Params: errres}, http.StatusOK
	} else {
		code = utils.GenerateRandomNumber()
	}

	delta := time.Since(user.Reg.SendCodeAt)
	if delta.Seconds() < 50.0 {
		// Time delta is less than 30 seconds
		return nil, &jsonrpc2.RPCError{Code: http.StatusTooManyRequests, Message: fmt.Sprintf("please try again after %.1f seconds", 50.0-delta.Seconds())}, http.StatusOK
	}

	jwt, err := auth.CreateJWTWithExpire(user.UID, user.Username, "ResetPassword", code, time.Hour*1)
	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusInternalServerError, Message: "server busy"}, http.StatusOK
	}

	//todo: go send email list goroutine
	go func(email, token string) {
		data := &PwdResetRequest{JWT: token}
		err := auth.SendForgotPwdMail(email, data)
		if err != nil {
			Logger.Error(err, "send email error")
		}
	}(user.Email, jwt)

	user.Reg.SendCodeAt = time.Now()
	user.Reg.Code = code
	_, err = uc.userService.UpdateUser(user.Id, user)
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
		}, http.StatusOK
	}

	user, err := uc.userService.FindUserById(req.UID)
	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: err.Error()}, http.StatusOK
	}

	if user.Reg.Registered {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "user already registered"}, http.StatusOK
	}

	delta := time.Since(user.Reg.SendCodeAt)
	if delta.Seconds() < 50.0 {
		// Time delta is less than 30 seconds
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: fmt.Sprintf("please try again after %.1f seconds", 50.0-delta.Seconds())}, http.StatusOK
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
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: err.Error()}, http.StatusOK
	}

	ok := utils.IsValidUid(confirm.UID)
	if !ok {
		return nil, &jsonrpc2.RPCError{
			Code:    http.StatusForbidden,
			Message: "uid invalid",
		}, http.StatusOK
	}

	user, err := uc.userService.FindUserById(confirm.UID)
	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: err.Error()}, http.StatusOK
	}

	if user.Reg.Registered {
		return nil, &jsonrpc2.RPCError{Code: http.StatusNotAcceptable, Message: "user already registered"}, http.StatusOK
	}

	if user.Reg.Code == confirm.Code {
		user.Reg.Registered = true
		user, err = uc.userService.UpdateUser(user.Id, user)
		if err != nil {
			return nil, &jsonrpc2.RPCError{Code: http.StatusInternalServerError, Message: err.Error()}, http.StatusOK
		}
	} else {
		return nil, &jsonrpc2.RPCError{Code: http.StatusNotFound, Message: "wrong code"}, http.StatusOK
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
	var errres []*jsonrpc2.InputFieldError
	login.Username = strings.ToLower(login.Username)
	if isemail := utils.IsValidEmail(login.Username); isemail {
		Logger.V(2).Info("login with email")
		user, err = uc.userService.FindUserByEmail(login.Username)
	} else {
		Logger.V(2).Info("login with username")
		_, err = utils.IsValidUsername(login.Username)
		if err != nil {
			errres = append(errres, &jsonrpc2.InputFieldError{Error: err.Error(), Field: "username"})
			return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: err.Error(), Params: errres}, http.StatusOK
		}
		user, err = uc.userService.FindUserByUsername(login.Username)
	}

	if err != nil {
		errres = append(errres, &jsonrpc2.InputFieldError{Error: err.Error(), Field: "username"})
		return nil, &jsonrpc2.RPCError{Code: http.StatusNotFound, Message: err.Error(), Params: errres}, http.StatusOK
	}

	_, err = utils.IsValidPassword(login.Password)
	if err != nil {
		errres = append(errres, &jsonrpc2.InputFieldError{Error: err.Error(), Field: "password"})
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: err.Error(), Params: errres}, http.StatusOK
	}

	//check password
	ok, err := auth.ComparePassword(login.Password, user.Password)
	if !ok || err != nil {
		errres = append(errres, &jsonrpc2.InputFieldError{Error: "wrong password", Field: "password"})
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "invalid password", Params: errres}, http.StatusOK
	}

	// Create a JWT
	token, err := auth.CreateJWTToken(user.UID, user.Username, user.Reg.Code)
	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusInternalServerError, Message: err.Error()}, http.StatusOK
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
		claim, err := auth.ValidateToken(jwt)
		return claim, err
	} else {
		return nil, errors.New("jwt can't empty")
	}
}

func (uc *UserController) FindUserById(userUID, code string) (*ResponseUser, *jsonrpc2.RPCError, int) {
	Logger.V(2).Info(fmt.Sprintf("find a user by uid %s", userUID))

	ok := utils.IsValidUid(userUID)
	if !ok {
		return nil, &jsonrpc2.RPCError{
			Code:    http.StatusForbidden,
			Message: "uid invalid",
		}, http.StatusOK
	}

	user, err := uc.userService.FindUserById(userUID)

	if err != nil {
		if strings.Contains(err.Error(), "exists") {
			return nil, &jsonrpc2.RPCError{
				Code:    http.StatusNotFound,
				Message: err.Error(),
			}, http.StatusOK
		}

		return nil, &jsonrpc2.RPCError{
			Code:    http.StatusBadGateway,
			Message: err.Error(),
		}, http.StatusOK
	}

	if user.Reg.Code != code {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "invalid jwt"}, http.StatusOK
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
