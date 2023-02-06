package chat

import (
	"encoding/json"
	"pesatu/utils"
)

type Message struct {
	Action  string      `json:"action"`
	Message string      `json:"message"`
	Target  *Room       `json:"target"`
	Sender  interface{} `json:"sender"`
}

func (message *Message) encode() []byte {
	json, err := json.Marshal(message)
	if err != nil {
		utils.Log().Error(err, "error while marshaling message")
	}

	return json
}

// func (message *Message) UnmarshalJSON(data []byte) error {
// 	type Alias Message
// 	msg := &struct {
// 		Sender Client `json:"sender"`
// _		*Alias
// 	}{
// 		Alias: (*Alias)(message),
// 	}
// 	if err := json.Unmarshal(data, &msg); err != nil {
// 		return err
// 	}
// 	message.Sender = &msg.Sender
// 	return nil
// }
