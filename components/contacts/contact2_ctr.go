package contacts

import (
	"fmt"
	"net/http"
	"pesatu/auth"
	"pesatu/jsonrpc2"
	"pesatu/utils"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ContactController struct {
	contactService I_ContactRepo
}

func NewContactController(contactService I_ContactRepo) ContactController {
	return ContactController{contactService}
}

func checkStatus(s Status) bool {
	for _, valid := range ValidStatuses {
		if s == valid {
			return true
		}
	}

	return s == ""
}

func (me *ContactController) CreateContact(validuser *auth.Claims, newContact *CreateContact) (*Contact, *jsonrpc2.RPCError, int) {
	Logger.V(2).Info(fmt.Sprintf("create contact %s to %s", newContact.UID, newContact.ToUsrName))

	if validuser.GetUID() != newContact.UID {
		return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "user uid did not match"}, http.StatusOK
	}

	_, err := utils.IsValidUsername(newContact.ToUsrName)
	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: err.Error()}, http.StatusOK
	}

	targetuser, err := me.contactService.FindUserConnection(validuser.GetUID(), newContact.ToUsrName)
	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: fmt.Sprintf("can not find requested user to check connnection. %s", err.Error())}, http.StatusOK
	}

	user, err := me.contactService.FindUserConnection(targetuser.UID, validuser.GetUsername())
	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: fmt.Sprintf("jwt error, username. %s", err.Error())}, http.StatusOK
	}

	if validuser.GetUID() != user.UID {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "user uid did not match 2"}, http.StatusOK
	}

	if user.Contact != nil {
		switch user.Contact.Status {
		case Pending:
			return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "friend request already sent"}, http.StatusOK
		case Accepted:
			return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "you are already be friends"}, http.StatusOK
		case Waiting:
			//accept friend request
			Logger.Info(fmt.Sprintf("%s accepting friend request from %s", user.Username, targetuser.Username))
		default:
			return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "unknown status request"}, http.StatusOK
		}
	}

	// Logger.V(2).Info("user:" + user.Username)
	// Logger.V(2).Info("target:" + targetuser.Username)

	nc := &Contact{
		Owner:     user.UID,
		To:        targetuser.UID,
		Status:    Pending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	var res *DBContact
	if targetuser.Contact != nil {
		nc.Status = Accepted

		targetuser.Contact.Status = Accepted
		targetuser.Contact.UpdatedAt = time.Now()
		res, err = me.contactService.UpdateContact(targetuser.Contact.Id, targetuser.Contact)
		if err != nil {
			Logger.Error(err, "error updating user target contact")
			return nil, &jsonrpc2.RPCError{Code: http.StatusInternalServerError, Message: err.Error()}, http.StatusInternalServerError
		}

		//todo notif friend request accepted

	} else {
		tnc := &Contact{
			Owner:     targetuser.UID,
			To:        user.UID,
			Status:    Waiting,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		res, err = me.contactService.CreateContact(tnc)
		if err != nil {
			Logger.Error(err, "error creating user target contact")
			return nil, &jsonrpc2.RPCError{Code: http.StatusInternalServerError, Message: err.Error()}, http.StatusInternalServerError
		}

		//todo notif target got a friend request
	}

	// if res == nil {
	// 	return nil, &jsonrpc2.RPCError{Code: http.StatusInternalServerError, Message: "can not add target to contact"}, http.StatusInternalServerError
	// }

	if user.Contact != nil {
		user.Contact.Status = nc.Status
		user.Contact.UpdatedAt = nc.UpdatedAt
		_, err = me.contactService.UpdateContact(user.Contact.Id, user.Contact)
		if err != nil {
			return nil, &jsonrpc2.RPCError{Code: http.StatusInternalServerError, Message: err.Error()}, http.StatusInternalServerError
		}

	} else {
		_, err = me.contactService.CreateContact(nc)
		if err != nil {
			return nil, &jsonrpc2.RPCError{Code: http.StatusInternalServerError, Message: err.Error()}, http.StatusInternalServerError
		}
	}

	var result Contact
	utils.CopyStruct(res, &result)
	result.Owner = targetuser.Username

	Logger.V(2).Info("create contact success")
	return &result, nil, http.StatusCreated
}

func (me *ContactController) RemoveContact(validuser *auth.Claims, delContact *CreateContact) (bool, *jsonrpc2.RPCError, int) {
	Logger.V(2).Info(fmt.Sprintf("remove contact %s to %s", delContact.UID, delContact.ToUsrName))

	if validuser.GetUID() != delContact.UID {
		return false, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "user uid did not match"}, http.StatusOK
	}

	_, err := utils.IsValidUsername(delContact.ToUsrName)
	if err != nil {
		return false, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: err.Error()}, http.StatusOK
	}

	targetuser, err := me.contactService.FindUserConnection(validuser.GetUID(), delContact.ToUsrName)
	if err != nil {
		return false, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: fmt.Sprintf("can not find requested user to check connnection. %s", err.Error())}, http.StatusOK
	}

	user, err := me.contactService.FindUserConnection(targetuser.UID, validuser.GetUsername())
	if err != nil {
		return false, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: fmt.Sprintf("jwt error, username. %s", err.Error())}, http.StatusOK
	}

	if validuser.GetUID() != user.UID {
		return false, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "user uid did not match 2"}, http.StatusOK
	}

	var ids []*primitive.ObjectID
	if targetuser.Contact != nil {
		ids = append(ids, &targetuser.Contact.Id)
	}
	if user.Contact != nil {
		ids = append(ids, &user.Contact.Id)
	}

	if len(ids) == 0 {
		return false, &jsonrpc2.RPCError{Code: http.StatusNotFound, Message: "connection not found"}, http.StatusOK
	}

	err = me.contactService.DeleteContacts(ids)
	if err != nil {
		Logger.Error(err, "error deleting contact")
		return false, &jsonrpc2.RPCError{Code: http.StatusInternalServerError, Message: err.Error()}, http.StatusInternalServerError
	}

	Logger.V(2).Info("delete contact success")
	return true, nil, http.StatusCreated
}

func (me *ContactController) SearchUsers(keyword, pageStr, limitStr, userUID, status string) ([]*UserContact, *jsonrpc2.RPCError, int) {
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
	if keyword == "@" {
		return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "invalid search input"}, http.StatusOK
	}

	if ok := utils.IsValidUid(userUID); !ok {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "uid invalid"}, http.StatusOK
	}

	if ok := checkStatus(status); !ok {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "status invalid"}, http.StatusOK
	}

	user, err := me.contactService.FindUserById(userUID)
	if err != nil {
		if strings.Contains(err.Error(), "exists") {
			return nil, &jsonrpc2.RPCError{Code: http.StatusNotFound, Message: err.Error()}, http.StatusOK
		}
		return nil, &jsonrpc2.RPCError{Code: http.StatusBadGateway, Message: err.Error()}, http.StatusOK
	}

	// if user.Reg.Code != code {
	// 	return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "invalid jwt"}, http.StatusOK
	// }

	var usercontacts []*UserContact
	if strings.HasPrefix(keyword, "@") {
		keyword = keyword[1:]
		keyword = strings.ToLower(keyword)
		_, err = utils.IsValidUsername(keyword)
		if err != nil {
			return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: fmt.Sprintf("invalid username input. %s", err.Error())}, http.StatusOK
		}
		usercontacts, err = me.contactService.FindUsersByUsername(user.UID, keyword, status, intPage, intLimit)
	} else {
		_, err = utils.IsValidName(keyword)
		keyword2 := strings.ToLower(keyword)
		keyword2 = strings.Replace(keyword2, " ", "", -1)
		if ok, _ := utils.IsValidUsername(keyword2); !ok {
			keyword2 = "#ojan!"
		}

		if err != nil && keyword != "" {
			return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: fmt.Sprintf("invalid name input. %s", err.Error())}, http.StatusOK
		}
		usercontacts, err = me.contactService.FindUsersByName(user.UID, keyword, keyword2, status, intPage, intLimit)
	}

	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: err.Error()}, http.StatusOK
	}

	// if len(usercontacts) == 0 {
	// 	usercontacts = make([]*UserContact, 0)
	// }

	Logger.V(2).Info(fmt.Sprintf("search count %d", len(usercontacts)))
	return usercontacts, nil, http.StatusOK
}

func (me *ContactController) SearchUserCount(keyword, userUID, status string) (int64, *jsonrpc2.RPCError, int) {
	Logger.V(2).Info(fmt.Sprintf("search user count %s", keyword))
	//handle for keyword empty
	if keyword == "@" {
		return 0, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "invalid search input"}, http.StatusOK
	}

	if ok := utils.IsValidUid(userUID); !ok {
		return 0, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "uid invalid"}, http.StatusOK
	}

	if ok := checkStatus(status); !ok {
		return 0, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "status invalid"}, http.StatusOK
	}

	user, err := me.contactService.FindUserById(userUID)
	if err != nil {
		if strings.Contains(err.Error(), "exists") {
			return 0, &jsonrpc2.RPCError{Code: http.StatusNotFound, Message: err.Error()}, http.StatusOK
		}
		return 0, &jsonrpc2.RPCError{Code: http.StatusBadGateway, Message: err.Error()}, http.StatusOK
	}

	var usercontactcount int64
	if strings.HasPrefix(keyword, "@") {
		keyword = keyword[1:]
		keyword = strings.ToLower(keyword)
		_, err = utils.IsValidUsername(keyword)
		if err != nil {
			return 0, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: fmt.Sprintf("invalid username input. %s", err.Error())}, http.StatusOK
		}
		usercontactcount, err = me.contactService.FindUserCountByUsername(user.UID, keyword, status)
	} else {
		_, err = utils.IsValidName(keyword)
		keyword2 := strings.ToLower(keyword)
		keyword2 = strings.Replace(keyword2, " ", "", -1)
		if ok, _ := utils.IsValidUsername(keyword2); !ok {
			keyword2 = "#ojan!"
		}

		if err != nil && keyword != "" {
			return 0, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: fmt.Sprintf("invalid name input. %s", err.Error())}, http.StatusOK
		}
		usercontactcount, err = me.contactService.FindUserCountByName(user.UID, keyword, keyword2, status)
	}

	if err != nil {
		return 0, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: err.Error()}, http.StatusOK
	}

	Logger.V(2).Info(fmt.Sprintf("search count %d", usercontactcount))
	return usercontactcount, nil, http.StatusOK
}
