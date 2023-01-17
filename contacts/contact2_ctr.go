package contacts

import (
	"fmt"
	"net/http"
	"pesatu/auth"
	"pesatu/jsonrpc2"
	"pesatu/user"
	"pesatu/utils"

	"go.mongodb.org/mongo-driver/bson"
)

type ContactController struct {
	userService    user.I_UserRepo
	contactService I_ContactRepo
}

func NewContactController(userService user.I_UserRepo, contactService I_ContactRepo) ContactController {
	return ContactController{userService, contactService}
}

func (me *ContactController) CreateContact(validuser *auth.Claims, newContact *CreateContact) (*ResponseStatus, *jsonrpc2.RPCError, int) {
	Logger.V(2).Info(fmt.Sprintf("create contact %s to ", newContact.Owner, newContact.ToUsr))

	if validuser.GetUID() != newContact.Owner {
		return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "user uid did not match"}, http.StatusOK
	}

	_, err := utils.IsValidUsername(newContact.ToUsr)
	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: err.Error()}, http.StatusOK
	}

	filter := &bson.M{
		"$or": []bson.M{
			bson.M{"uid": newContact.Owner},
			bson.M{"username": newContact.ToUsr},
		},
	}
	users, err := me.userService.FindUsers(filter, 0, 2)
	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "can not find requested user to add contact"}, http.StatusOK
	}
	if len(users) != 2 {
		return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "error find users to add contact"}, http.StatusOK
	}

	var nc *Contact
	nc.Status = Pending
	for i := 0; i < len(users); i++ {
		if users[i].UID == newContact.Owner {
			nc.Owner = users[i].UID
			continue
		}
		if users[i].Username == newContact.ToUsr {
			nc.To = users[i].UID
			continue
		}
	}

	_, err = me.contactService.CreateContact(nc)
	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: err.Error()}, http.StatusOK
	}

	Logger.V(2).Info("create contact pending")
	return &ResponseStatus{Status: Pending}, nil, http.StatusCreated
}
