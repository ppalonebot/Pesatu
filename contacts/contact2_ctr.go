package contacts

import (
	"fmt"
	"net/http"
	"pesatu/auth"
	"pesatu/jsonrpc2"
	"pesatu/user"
	"pesatu/utils"
	"strconv"
	"strings"
	"time"
)

type ContactController struct {
	userService    user.I_UserRepo
	contactService I_ContactRepo
}

func NewContactController(userService user.I_UserRepo, contactService I_ContactRepo) ContactController {
	return ContactController{userService, contactService}
}

func (me *ContactController) CreateContact(validuser *auth.Claims, newContact *CreateContact) (*ResponseStatus, *jsonrpc2.RPCError, int) {
	Logger.V(2).Info(fmt.Sprintf("create contact %s to %s", newContact.UID, newContact.ToUsrName))

	if validuser.GetUID() != newContact.UID {
		return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "user uid did not match"}, http.StatusOK
	}

	_, err := utils.IsValidUsername(newContact.ToUsrName)
	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: err.Error()}, http.StatusOK
	}

	targetuserconn, err := me.contactService.FindUserConnection(newContact.UID, newContact.ToUsrName)
	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: fmt.Sprintf("can not find requested user to check connnection. %s", err.Error())}, http.StatusOK
	}

	user, err := me.contactService.FindUserConnection(targetuserconn.UID, validuser.GetUsername())
	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: fmt.Sprintf("jwt error, username. %s", err.Error())}, http.StatusOK
	}

	if validuser.GetUID() != user.UID {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "user uid did not match"}, http.StatusOK
	}

	if user.Contact != nil {
		switch user.Contact.Status {
		case Pending:
			return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "friend request already sent"}, http.StatusOK
		case Accepted:
			return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "you are already be friends"}, http.StatusOK
		default:
			return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "unknown status request"}, http.StatusOK
		}
	}

	nc := &Contact{
		Owner:     user.UID,
		To:        targetuserconn.UID,
		Status:    Pending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if targetuserconn.Contact != nil {
		nc.Status = Accepted
		if targetuserconn.Contact.Status == Pending {
			targetuserconn.Contact.Status = Accepted
			_, err = me.contactService.UpdateContact(targetuserconn.Contact.Id, targetuserconn.Contact)
			if err != nil {
				Logger.Error(err, "error updating user target contact")
			}

			//todo notif friend request accepted
		}
	}

	_, err = me.contactService.CreateContact(nc)
	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: err.Error()}, http.StatusOK
	}

	Logger.V(2).Info("create contact success")
	return &ResponseStatus{Status: "success"}, nil, http.StatusCreated
}

func (me *ContactController) SearchUsers(keyword, pageStr, limitStr, userUID, code string) ([]*UserContact, *jsonrpc2.RPCError, int) {
	Logger.V(2).Info(fmt.Sprintf("search users %s", keyword))
	var page = pageStr
	var limit = limitStr

	intPage, err := strconv.Atoi(page)
	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "invalid page input"}, http.StatusOK
	}

	intLimit, err := strconv.Atoi(limit)
	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "invalid limit input"}, http.StatusOK
	}

	//handle for keyword empty
	if len(keyword) == 0 || keyword == "@" {
		return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "invalid search input"}, http.StatusOK
	}

	ok := utils.IsValidUid(userUID)
	if !ok {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "uid invalid"}, http.StatusOK
	}

	user, err := me.userService.FindUserById(userUID)
	if err != nil {
		if strings.Contains(err.Error(), "exists") {
			return nil, &jsonrpc2.RPCError{Code: http.StatusNotFound, Message: err.Error()}, http.StatusOK
		}
		return nil, &jsonrpc2.RPCError{Code: http.StatusBadGateway, Message: err.Error()}, http.StatusOK
	}

	if user.Reg.Code != code {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "invalid jwt"}, http.StatusOK
	}

	var usercontacts []*UserContact
	if strings.HasPrefix(keyword, "@") {
		keyword = keyword[1:]
		keyword = strings.ToLower(keyword)
		_, err = utils.IsValidUsername(keyword)
		if err != nil {
			return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "invalid username input"}, http.StatusOK
		}
		usercontacts, err = me.contactService.FindUsersByUsername(user.UID, keyword, intPage, intLimit)
	} else {
		_, err = utils.IsValidName(keyword)
		if err != nil {
			return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "invalid name input"}, http.StatusOK
		}
		usercontacts, err = me.contactService.FindUsersByName(user.UID, keyword, intPage, intLimit)
	}

	if len(usercontacts) == 0 {
		usercontacts = make([]*UserContact, 0)
	}

	Logger.V(2).Info(fmt.Sprintf("search count %d", len(usercontacts)))
	return usercontacts, nil, http.StatusOK
}
