package roommember

import (
	"fmt"
	"net/http"
	"pesatu/auth"
	"pesatu/jsonrpc2"
	"pesatu/utils"
	"strconv"
)

type RoomMemberController struct {
	roomService I_RoomMember
}

func NewRoomMemberController(roomService I_RoomMember) RoomMemberController {
	return RoomMemberController{roomService}
}

func (me *RoomMemberController) FindLastMessages(validuser *auth.Claims, o *SearchLastMessage) (*LastMessages, *jsonrpc2.RPCError, int) {
	utils.Log().V(2).Info(fmt.Sprintf("find last messages by user id: %s", validuser.GetUID()))

	ok := utils.IsValidUid(validuser.GetUID()) && validuser.GetUID() == o.UID
	if !ok {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "uid invalid"}, http.StatusOK
	}

	var page = o.Page
	var limit = o.Limit

	intPage, err := strconv.Atoi(page)
	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "invalid page input"}, http.StatusOK
	}

	intLimit, err := strconv.Atoi(limit)
	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusBadRequest, Message: "invalid limit input"}, http.StatusOK
	}

	results, err := me.roomService.FindLastMessagesGroupingByRoom(validuser.GetUID(), intPage, intLimit)

	if err != nil {
		return nil, &jsonrpc2.RPCError{Code: http.StatusNotFound, Message: err.Error()}, http.StatusOK
	}

	return results, nil, http.StatusOK
}
