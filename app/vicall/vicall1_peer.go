package vicall

import (
	"errors"
	"fmt"
	"pesatu/utils"
	"sync"
	"sync/atomic"

	"github.com/lucsky/cuid"

	"github.com/pion/ion-sfu/pkg/sfu"
	"github.com/pion/webrtc/v3"
)

const (
	publisher  = 0
	subscriber = 1
)

var (
	// ErrTransportExists join is called after a peerconnection is established
	ErrTransportExists = errors.New("rtc transport already exists for this connection")
	// ErrNoTransportEstablished cannot signal before join
	ErrNoTransportEstablished = errors.New("no rtc transport exists for this Peer")
	// ErrOfferIgnored if offer received in unstable state
	ErrOfferIgnored = errors.New("offered ignored")
)

type Peer interface {
	ID() string
	Session() sfu.Session
	Publisher() *sfu.Publisher
	Subscriber() *sfu.Subscriber
	Close() error
	SendDCMessage(label string, msg []byte) error
}

// JoinConfig allow adding more control to the peers joining a SessionLocal.
type JoinConfig struct {
	// If true the peer will not be allowed to publish tracks to SessionLocal.
	NoPublish bool
	// If true the peer will not be allowed to subscribe to other peers in SessionLocal.
	NoSubscribe bool
	// If true the peer will not automatically subscribe all tracks,
	// and then the peer can use peer.Subscriber().AddDownTrack/RemoveDownTrack
	// to customize the subscrbe stream combination as needed.
	// this parameter depends on NoSubscribe=false.
	NoAutoSubscribe bool
}

// SessionProvider provides the SessionLocal to the sfu.Peer
// This allows the sfu.SFU{} implementation to be customized / wrapped by another package
type SessionProvider interface {
	GetSession(sid string) (sfu.Session, sfu.WebRTCTransportConfig)
}

type ChannelAPIMessage struct {
	Method string      `json:"method"`
	Params interface{} `json:"params,omitempty"`
}

type atomicBool int32

func (a *atomicBool) set(value bool) (swapped bool) {
	if value {
		return atomic.SwapInt32((*int32)(a), 1) == 0
	}
	return atomic.SwapInt32((*int32)(a), 0) == 1
}

func (a *atomicBool) get() bool {
	return atomic.LoadInt32((*int32)(a)) != 0
}

// PeerLocal represents a pair peer connection
type PeerLocal struct {
	sync.Mutex
	id       string
	closed   atomicBool
	session  sfu.Session
	provider SessionProvider

	publisher  *sfu.Publisher
	subscriber *sfu.Subscriber

	OnOffer                    func(*webrtc.SessionDescription)
	OnIceCandidate             func(*webrtc.ICECandidateInit, int)
	OnICEConnectionStateChange func(webrtc.ICEConnectionState)

	remoteAnswerPending bool
	negotiationPending  bool
}

// NewPeer creates a new PeerLocal for signaling with the given SFU
func NewPeer(provider SessionProvider) *PeerLocal {
	return &PeerLocal{
		provider: provider,
	}
}

// Join initializes this peer for a given sessionID
func (p *PeerLocal) Join(sid, uid string, config ...JoinConfig) error {
	var conf JoinConfig
	if len(config) > 0 {
		conf = config[0]
	}

	if p.session != nil {
		for i := 0; i < len(p.session.Peers()); i++ {
			if p.session.Peers()[i].ID() == uid || uid == "" {
				utils.Log().V(1).Info(fmt.Sprintf("peer already exists %d", len(p.session.Peers())))
				return ErrTransportExists
			}
		}
	}

	p.closed.set(false)

	if uid == "" {
		uid = cuid.New()
	}
	p.id = uid
	var err error

	s, cfg := p.provider.GetSession(sid)
	p.session = s

	if !conf.NoSubscribe {
		p.subscriber, err = sfu.NewSubscriber(uid, cfg)
		if err != nil {
			return fmt.Errorf("error creating transport: %v", err)
		}

		// p.subscriber.noAutoSubscribe = conf.NoAutoSubscribe

		p.subscriber.OnNegotiationNeeded(func() {
			p.Lock()
			defer p.Unlock()

			if p.remoteAnswerPending {
				p.negotiationPending = true
				return
			}

			utils.Log().V(1).Info("Negotiation needed", "peer_id", p.id)
			offer, err := p.subscriber.CreateOffer()
			if err != nil {
				utils.Log().Error(err, "CreateOffer error")
				return
			}

			p.remoteAnswerPending = true
			if p.OnOffer != nil && !p.closed.get() {
				utils.Log().V(1).Info("Send offer", "peer_id", p.id)
				p.OnOffer(&offer)
			}
		})

		p.subscriber.OnICECandidate(func(c *webrtc.ICECandidate) {
			utils.Log().V(1).Info("On subscriber ice candidate called for peer", "peer_id", p.id)
			if c == nil {
				return
			}

			if p.OnIceCandidate != nil && !p.closed.get() {
				json := c.ToJSON()
				p.OnIceCandidate(&json, subscriber)
			}
		})
	}

	if !conf.NoPublish {
		p.publisher, err = sfu.NewPublisher(uid, p.session, &cfg)
		if err != nil {
			return fmt.Errorf("error creating transport: %v", err)
		}
		if !conf.NoSubscribe {
			for _, dc := range p.session.GetDCMiddlewares() {
				if err := p.subscriber.AddDatachannel(p, dc); err != nil {
					return fmt.Errorf("setting subscriber default dc datachannel: %w", err)
				}
			}
		}

		p.publisher.OnICECandidate(func(c *webrtc.ICECandidate) {
			utils.Log().V(1).Info("on publisher ice candidate called for peer", "peer_id", p.id)
			if c == nil {
				return
			}

			if p.OnIceCandidate != nil && !p.closed.get() {
				json := c.ToJSON()
				p.OnIceCandidate(&json, publisher)
			}
		})

		p.publisher.OnICEConnectionStateChange(func(s webrtc.ICEConnectionState) {
			if p.OnICEConnectionStateChange != nil && !p.closed.get() {
				p.OnICEConnectionStateChange(s)
			}
		})
	}

	p.session.AddPeer(p)

	utils.Log().V(1).Info("PeerLocal join SessionLocal", "peer_id", p.id, "session_id", sid)

	if !conf.NoSubscribe {
		p.session.Subscribe(p)
	}
	return nil
}

// Answer an offer from remote
func (p *PeerLocal) Answer(sdp webrtc.SessionDescription) (*webrtc.SessionDescription, error) {
	if p.publisher == nil {
		return nil, ErrNoTransportEstablished
	}

	utils.Log().V(1).Info("PeerLocal got offer", "peer_id", p.id)

	if p.publisher.SignalingState() != webrtc.SignalingStateStable {
		return nil, ErrOfferIgnored
	}

	answer, err := p.publisher.Answer(sdp)
	if err != nil {
		return nil, fmt.Errorf("error creating answer: %v", err)
	}

	utils.Log().V(1).Info("PeerLocal send answer", "peer_id", p.id)

	return &answer, nil
}

// SetRemoteDescription when receiving an answer from remote
func (p *PeerLocal) SetRemoteDescription(sdp webrtc.SessionDescription) error {
	if p.subscriber == nil {
		return ErrNoTransportEstablished
	}
	p.Lock()
	defer p.Unlock()

	utils.Log().V(1).Info("PeerLocal got answer", "peer_id", p.id)
	if err := p.subscriber.SetRemoteDescription(sdp); err != nil {
		return fmt.Errorf("setting remote description: %w", err)
	}

	p.remoteAnswerPending = false

	if p.negotiationPending {
		p.negotiationPending = false
		p.subscriber.Negotiate()
	}

	return nil
}

// Trickle candidates available for this peer
func (p *PeerLocal) Trickle(candidate webrtc.ICECandidateInit, target int) error {
	if p.subscriber == nil || p.publisher == nil {
		return ErrNoTransportEstablished
	}
	utils.Log().V(1).Info("PeerLocal trickle", "peer_id", p.id)
	switch target {
	case publisher:
		if err := p.publisher.AddICECandidate(candidate); err != nil {
			return fmt.Errorf("setting ice candidate: %w", err)
		}
	case subscriber:
		if err := p.subscriber.AddICECandidate(candidate); err != nil {
			return fmt.Errorf("setting ice candidate: %w", err)
		}
	}
	return nil
}

func (p *PeerLocal) SendDCMessage(label string, msg []byte) error {
	if p.subscriber == nil {
		return fmt.Errorf("no subscriber for this peer")
	}
	dc := p.subscriber.DataChannel(label)

	if dc == nil {
		return fmt.Errorf("data channel %s doesn't exist", label)
	}

	if err := dc.SendText(string(msg)); err != nil {
		return fmt.Errorf("failed to send message: %v", err)
	}
	return nil
}

// Close shuts down the peer connection and sends true to the done channel
func (p *PeerLocal) Close() error {
	p.Lock()
	defer p.Unlock()

	if !p.closed.set(true) {
		return nil
	}

	if p.session != nil {
		p.session.RemovePeer(p)
		utils.Log().V(2).Info("RemovePeer")
	}
	if p.publisher != nil {
		p.publisher.Close()
		utils.Log().V(2).Info("close publisher")
	}
	if p.subscriber != nil {
		if err := p.subscriber.Close(); err != nil {
			return err
		}
		utils.Log().V(2).Info("close subscriber")
	}
	return nil
}

func (p *PeerLocal) Subscriber() *sfu.Subscriber {
	return p.subscriber
}

func (p *PeerLocal) Publisher() *sfu.Publisher {
	return p.publisher
}

func (p *PeerLocal) Session() sfu.Session {
	return p.session
}

// ID return the peer id
func (p *PeerLocal) ID() string {
	return p.id
}
