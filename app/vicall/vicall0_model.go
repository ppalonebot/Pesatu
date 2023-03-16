package vicall

import (
	"github.com/pion/webrtc/v3"
)

// Join message sent when initializing a peer connection
type Join struct {
	SID    string                    `json:"sid"`
	UID    string                    `json:"uid"`
	Offer  webrtc.SessionDescription `json:"offer"`
	Config JoinConfig                `json:"config"`
}

// Negotiation message sent when renegotiating the peer connection
type Negotiation struct {
	Desc webrtc.SessionDescription `json:"desc"`
}

// Trickle message sent when renegotiating the peer connection
type Trickle struct {
	Target    int                     `json:"target"`
	Candidate webrtc.ICECandidateInit `json:"candidate"`
}

type VAction = string

const (
	JoinVicall  VAction = "join"
	LeaveVicall VAction = "leave-vicall"
)
