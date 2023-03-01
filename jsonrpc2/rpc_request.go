package jsonrpc2

import (
	"encoding/json"
	"pesatu/utils"
)

// Notify is like Call, but it returns when the notification request
// is sent (without waiting for a response, because JSON-RPC
// notifications do not have responses).
func Notify(method string, params interface{}) (*RPCRequest, error) {
	req := &RPCRequest{ID: utils.GetRandomUUID(), Method: method, JSONRPC: "2.0", Notif: true}
	if err := req.SetParams(params); err != nil {
		return nil, err
	}
	return req, nil
}

func (me *RPCRequest) Encode() []byte {
	json, err := json.Marshal(me)
	if err != nil {
		utils.Log().Error(err, "error while marshaling RPCRequest")
	}
	return json
}

// SetParams sets r.Params to the JSON representation of v. If JSON
// marshaling fails, it returns an error.
func (me *RPCRequest) SetParams(v interface{}) error {
	if v == nil {
		me.Params = nil
		return nil
	}

	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	me.Params = (json.RawMessage)(b)
	return nil
}
