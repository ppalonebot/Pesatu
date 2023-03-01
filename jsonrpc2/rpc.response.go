package jsonrpc2

import (
	"encoding/json"
	"fmt"
	"pesatu/utils"
)

// Reply sends a successful response with a result.
func Reply(id string, result interface{}) (*RPCResponse, error) {
	resp := &RPCResponse{ID: id, JSONRPC: "2.0"}
	if err := resp.SetResult(result); err != nil {
		return nil, err
	}
	return resp, nil
}

func ReplyWithError(id string, result interface{}, code int, err error) (*RPCResponse, error) {
	resp := &RPCResponse{
		ID:      id,
		JSONRPC: "2.0",
		Error:   &RPCError{Code: code, Message: fmt.Sprintf("%s", err)},
	}
	if err := resp.SetResult(result); err != nil {
		return nil, err
	}
	return resp, nil
}

func (me *RPCResponse) Encode() []byte {
	json, err := json.Marshal(me)
	if err != nil {
		utils.Log().Error(err, "error while marshaling RPCResponse")
	}

	return json
}

// SetResult sets r.Result to the JSON representation of v. If JSON
// marshaling fails, it returns an error.
func (me *RPCResponse) SetResult(v interface{}) error {
	if v == nil {
		me.Result = nil
		return nil
	}

	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	me.Result = (json.RawMessage)(b)
	return nil
}
