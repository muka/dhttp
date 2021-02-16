package dhttp

import (
	"net"
	"net/http"
	"time"

	"github.com/golang/protobuf/proto"
	pb "github.com/muka/dhttp/protobuf"
	"github.com/muka/peer"
	"github.com/rs/xid"
	log "github.com/sirupsen/logrus"
)

//NewProxy initialize a new Proxy
func NewProxy(opts ProxyOptions) (*Proxy, error) {

	p := new(Proxy)
	p.opts = opts

	err := p.init()
	if err != nil {
		return nil, err
	}

	return p, nil
}

// NewProxyOptions provide defaults for the proxy timeout
func NewProxyOptions() ProxyOptions {
	return ProxyOptions{
		Peer:                  peer.NewOptions(),
		Timeout:               10 * time.Second,
		KeepAlive:             10 * time.Second,
		ClientTimeout:         10 * time.Second,
		TLSHandshakeTimeout:   2 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		MaxIdleConns:          10,
		MaxBodyBytes:          10 * 1024 * 1024,
	}
}

//ProxyOptions wraps options for the Proxy instance
type ProxyOptions struct {
	Peer                  peer.Options
	Timeout               time.Duration
	KeepAlive             time.Duration
	ClientTimeout         time.Duration
	TLSHandshakeTimeout   time.Duration
	ResponseHeaderTimeout time.Duration
	MaxIdleConns          int
	MaxBodyBytes          int64
}

//Proxy is a HTTP over WEBRTC instance
type Proxy struct {
	id         string
	opts       ProxyOptions
	peer       *peer.Peer
	httpClient *http.Client
}

//Close close the proxy instance
func (p *Proxy) Close() error {

	if p.peer != nil {
		p.peer.Close()
	}

	p.peer = nil
	p.httpClient = nil

	return nil
}

//GetID return the proxy ID
func (p *Proxy) GetID() string {
	return p.id
}

func (p *Proxy) init() error {

	p.id = xid.New().String()

	peer1, err := createPeer(p.id, p.opts.Peer)
	if err != nil {
		return err
	}
	p.peer = peer1

	// dialer is taken from DefaultTransport
	dialer := &net.Dialer{
		Timeout:   p.opts.Timeout,
		KeepAlive: p.opts.KeepAlive,
		DualStack: true,
	}
	p.httpClient = &http.Client{
		Timeout: p.opts.ClientTimeout,
		Transport: &http.Transport{
			Dial:                  dialer.Dial,
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   p.opts.TLSHandshakeTimeout,
			ResponseHeaderTimeout: p.opts.ResponseHeaderTimeout,
			ExpectContinueTimeout: time.Second,
			MaxIdleConns:          int(p.opts.MaxIdleConns),
			DisableCompression:    true,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	p.peer.On("error", func(data interface{}) {
		log.Errorf("Peer error: %s", data.(error))
	})

	p.peer.On("connection", func(data interface{}) {
		conn1 := data.(*peer.DataConnection)
		conn1.On("data", func(data interface{}) {

			// parse request
			req, err := parseRequestProto(data.([]byte))
			if err != nil {
				msg := "Failed to parse request"
				log.Errorf("%s: %s", msg, err)
				sendResponse(conn1, requestErrorResponse(req.Id, 500, msg))
				return
			}

			httpReq, err := createHTTPRequest(req)
			if err != nil {
				msg := "Failed to create HTTP request"
				log.Errorf("%s: %s", msg, err)
				sendResponse(conn1, requestErrorResponse(req.Id, 500, msg))
				return
			}

			httpRes, err := p.httpClient.Do(httpReq)
			if err != nil {
				msg := "Failed to send HTTP request"
				log.Errorf("%s: %s", msg, err)
				sendResponse(conn1, requestErrorResponse(req.Id, 500, msg))
				return
			}

			// prepare response
			res, err := createHTTPResponse(req.Id, httpRes, p.opts.MaxBodyBytes)
			if err != nil {
				msg := "Failed to read HTTP response body"
				if _, ok := err.(BodyTooLargeError); ok {
					msg = "HTTP body is too large"
				}
				log.Errorf("%s: %s", msg, err)
				sendResponse(conn1, requestErrorResponse(req.Id, 500, msg))
				return
			}

			err = sendResponse(conn1, res)
			if err != nil {
				log.Errorf("Failed to send response: %s", err)
			}

		})
	})

	return nil
}

func sendResponse(conn *peer.DataConnection, res *pb.Response) error {

	raw, err := proto.Marshal(res)
	if err != nil {
		log.Errorf("Failed to marshal response: %s", err)
		return err
	}

	conn.Send(raw, false)
	return nil
}
