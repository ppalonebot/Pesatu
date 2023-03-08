package vicall

import (
	"encoding/json"
	"fmt"
	"net/http"
	"pesatu/jsonrpc2"

	"github.com/go-logr/logr"
	"github.com/pion/ion-sfu/pkg/sfu"
	"github.com/pion/webrtc/v3"
)

type JSONSignal struct {
	*PeerLocal
	logr.Logger
}

func NewJSONSignal(p *PeerLocal, l logr.Logger) *JSONSignal {
	return &JSONSignal{p, l}
}

// Handle incoming RPC call events like join, answer, offer and trickle
// called from jsonrpc2.Conn object
func (me *JSONSignal) Handle(send chan []byte, req *jsonrpc2.RPCRequest) {
	sendMsg := func(msg []byte) {
		select {
		case send <- msg:
			me.V(2).Info(fmt.Sprintf("vc send rpc msg"))
		default:
			//channel closed
			me.Error(nil, "vc send msg error, chanel closed")
		}
	}
	replyError := func(err error) {
		me.V(2).Info(fmt.Sprintf("ReplyWithError, %s", err))
		resErr, err := jsonrpc2.ReplyWithError(req.ID, nil, http.StatusBadRequest, err)

		if err != nil {
			me.Error(err, "error while sending reply with error")
			return
		}

		sendMsg(resErr.Encode())
	}

	// create a logger that is only enabled for trace messages
	me.V(2).Info(fmt.Sprintf("On req method %s, id: %s", req.Method, req.ID))

	switch req.Method {
	case "join":
		var join Join
		err := json.Unmarshal(req.Params, &join)
		if err != nil {
			me.Error(err, "connect: error parsing offer")
			replyError(err)
			break
		}

		me.OnOffer = func(offer *webrtc.SessionDescription) {
			me.V(2).Info(fmt.Sprintf("Notify to peer_id: %s, sdp:%s", me.ID(), offer.SDP))
			rpcReq, err := jsonrpc2.Notify("offer", offer)
			if err != nil {
				me.Error(err, "error sending offer")
				return
			}

			sendMsg(rpcReq.Encode())
		}
		me.OnIceCandidate = func(candidate *webrtc.ICECandidateInit, target int) {
			me.V(2).Info(fmt.Sprintf("Notify trickle to peer_id: %s, %s", me.ID(), candidate.Candidate))
			rpcReq, err := jsonrpc2.Notify("trickle", Trickle{
				Candidate: *candidate,
				Target:    target,
			})
			if err != nil {
				me.Error(err, "error sending ice candidate")
				return
			}

			sendMsg(rpcReq.Encode())
		}

		me.OnICEConnectionStateChange = func(s webrtc.ICEConnectionState) {
			me.V(2).Info(fmt.Sprintf("peer_id: %s OnICEConnectionStateChange! %s", me.ID(), s.String()))
		}

		err = me.Join(join.SID, join.UID, join.Config)
		if err != nil {
			replyError(fmt.Errorf("at join " + err.Error()))
			break
		}

		answer, err := me.Answer(join.Offer)
		if err != nil {
			replyError(fmt.Errorf("at answer " + err.Error()))
			break
		}

		me.V(2).Info(fmt.Sprintf("Reply join to req id:%s, with peer_id: %s@%s, sdp:%s", req.ID, me.ID(), join.SID, answer.SDP))
		resRpc, err := jsonrpc2.Reply(req.ID, answer)
		if err != nil {
			me.Error(err, "error while answering join")
		}
		sendMsg(resRpc.Encode())

	case "offer":
		var negotiation Negotiation
		err := json.Unmarshal(req.Params, &negotiation)
		if err != nil {
			me.Error(err, "connect: error parsing offer")
			replyError(err)
			break
		}

		answer, err := me.Answer(negotiation.Desc)
		if err != nil {
			replyError(err)
			break
		}

		me.V(2).Info(fmt.Sprintf("Reply offer to req id:%s, with answer sdp:%s", req.ID, answer.SDP))
		resRpc, err := jsonrpc2.Reply(req.ID, answer)
		if err != nil {
			me.Error(err, "error while answering offer")
		}
		sendMsg(resRpc.Encode())

	case "answer":
		var negotiation Negotiation
		err := json.Unmarshal(req.Params, &negotiation)
		if err != nil {
			me.Error(err, "connect: error parsing answer")
			replyError(err)
			break
		}

		err = me.SetRemoteDescription(negotiation.Desc)
		if err != nil {
			replyError(err)
		}

	case "trickle":
		var trickle Trickle
		err := json.Unmarshal(req.Params, &trickle)
		if err != nil {
			me.Error(err, "connect: error parsing candidate")
			replyError(err)
			break
		}

		err = me.Trickle(trickle.Candidate, trickle.Target)
		if err != nil {
			replyError(err)
		}

	case "dmessage":
		session := me.Session()
		peers := session.Peers()
		jsonBytes, err := json.Marshal(req.Params)
		if err != nil {
			replyError(err)
		} else {
			me.V(2).Info(fmt.Sprintf("sending %s", string(jsonBytes)))
			for _, peer := range peers {
				if peer.ID() != me.ID() {
					peer.SendDCMessage(sfu.APIChannelLabel, jsonBytes)
				}
			}
		}

	case "leave-vicall":
		if me.closed.get() {
			return
		}
		me.Info(fmt.Sprintf("%s leaving %s", me.ID(), me.Session().ID()))
		err := me.Close()
		if err != nil {
			replyError(err)
		}
		me.V(2).Info(fmt.Sprintf("len of peers %d", len(me.Session().Peers())))
	}
}
