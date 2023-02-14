package chat

import (
	"encoding/json"
	"pesatu/utils"
)

type Message struct {
	Action  string      `json:"action" bson:"action"`
	Message string      `json:"message" bson:"message"`
	Target  *Room       `json:"target" bson:"target"`
	Sender  interface{} `json:"sender" bson:"sender"`
	Status  string      `json:"status" bson:"status"`
	Time    string      `json:"time" bson:"time"`
}

type Messages struct {
	Action   string      `json:"action" bson:"action"`
	Messages interface{} `json:"messages" bson:"messages"`
	Target   *Room       `json:"target" bson:"target"`
	Sender   interface{} `json:"sender" bson:"sender"`
	Status   string      `json:"status" bson:"status"`
	Time     string      `json:"time" bson:"time"`
}

func (message *Message) encode() []byte {
	json, err := json.Marshal(message)
	if err != nil {
		utils.Log().Error(err, "error while marshaling message")
	}

	return json
}

func (message *Messages) encode() []byte {
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
