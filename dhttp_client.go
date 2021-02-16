package dhttp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	pb "github.com/muka/dhttp/protobuf"
	"github.com/muka/peer"
	"github.com/rs/xid"
	"github.com/sirupsen/logrus"
)

// NewClient initialize a client
func NewClient(ctx context.Context, opts ClientOptions) (*Client, error) {
	c := new(Client)
	c.opts = opts
	err := c.init(ctx)
	if err != nil {
		return nil, err
	}
	return c, nil
}

//NewClientOptions init default client options
func NewClientOptions() ClientOptions {
	c := ClientOptions{}
	c.id = xid.New().String()
	c.peer = peer.NewOptions()
	c.connectionTimeout = time.Second * 2
	return c
}

//ClientOptions store client configurations
type ClientOptions struct {
	id                string
	serverID          string
	connectionTimeout time.Duration
	peer              peer.Options
}

type messageResponse struct {
	id  string
	res *pb.Response
	fn  func(*pb.Response)
	ev  chan error
}

//Client is a dhttp client
type Client struct {
	opts              ClientOptions
	peer              *peer.Peer
	conn              *peer.DataConnection
	messageQueueMutex sync.Mutex
	messageQueue      map[string]messageResponse
}

func (c *Client) init(ctx context.Context) error {

	c.clearMessageQueue()

	// go func() {
	// 	for {
	// 		select {
	// 		case mres := <-c.queueMatch:
	// 			if fn, ok := c.messageQueue[mres.id]; ok {
	// 				fn(mres.res)
	// 			}
	// 			break
	// 		}
	// 	}
	// }()

	p, err := createPeer(c.opts.id, c.opts.peer)
	if err != nil {
		logrus.Errorf("createPeer: %s", err)
		return err
	}
	c.peer = p

	conn, err := c.peer.Connect(c.opts.serverID, nil)
	if err != nil {
		logrus.Errorf("Connect: %s", err)
		return err
	}
	c.conn = conn

	// ctx1, cancel := context.WithTimeout(ctx, c.opts.connectionTimeout)
	// defer cancel()

	connected := make(chan error, 1)

	c.peer.On("error", func(data interface{}) {
		logrus.Errorf("Client peer error: %s", data.(error))
		// connected <- data.(error)
	})

	conn.On("open", func(data interface{}) {
		conn.On("data", func(data interface{}) {

			// TODO handle reconciliation with request by ID

			res := new(pb.Response)
			err := proto.Unmarshal(data.([]byte), res)
			if err != nil {
				logrus.Errorf("Unmarshal: %s", err)
				return
			}

			logrus.Debugf("Got response: %s", res.Body)

			c.messageQueueMutex.Lock()
			defer c.messageQueueMutex.Unlock()
			for id, mres := range c.messageQueue {
				if id == res.Id {
					mres.fn(res)
					mres.ev <- nil
					break
				}
			}

		})
		logrus.Debugf("Peer connected")
		connected <- nil
	})

	return <-connected
}

func (c *Client) clearMessageQueue() error {
	c.messageQueueMutex.Lock()
	defer c.messageQueueMutex.Unlock()
	for _, mres := range c.messageQueue {
		if mres.ev != nil {
			c.removeMessage(mres.id)
		}
	}
	c.messageQueue = map[string]messageResponse{}
	return nil
}

func (c *Client) addMessage(id string, callback func(*pb.Response)) chan error {
	c.messageQueueMutex.Lock()
	defer c.messageQueueMutex.Unlock()

	waitChan := make(chan error)

	if _, ok := c.messageQueue[id]; ok {
		waitChan <- fmt.Errorf("Duplicated message id %s", id)
		return waitChan
	}

	c.messageQueue[id] = messageResponse{
		id:  id,
		res: nil,
		fn:  callback,
		ev:  waitChan,
	}

	return waitChan
}

func (c *Client) removeMessage(id string) error {
	c.messageQueueMutex.Lock()
	defer c.messageQueueMutex.Unlock()
	if mres, ok := c.messageQueue[id]; ok {
		if mres.ev != nil {
			mres.ev <- nil
			close(mres.ev)
			mres.ev = nil
		}
		mres.fn = nil
		mres.res = nil
		delete(c.messageQueue, id)
	}
	return nil
}

// Do executes the request
func (c *Client) Do(r *http.Request) ([]byte, error) {

	headers := []*pb.Header{}
	for key, values := range r.Header {
		headers = append(headers, &pb.Header{
			Key:    key,
			Values: values,
		})
	}

	var body []byte = nil
	var err error

	if r.Body != nil {
		body, err = ioutil.ReadAll(r.Body)
		if err != nil {
			logrus.Errorf("ReadAll: %s", err)
			return nil, err
		}
	}

	req := &pb.Request{
		Method:   r.Method,
		Url:      r.URL.String(),
		Id:       xid.New().String(),
		Protocol: r.Proto,
		Headers:  headers,
		Body:     body,
	}

	raw, err := proto.Marshal(req)
	if err != nil {
		logrus.Errorf("Marshal: %s", err)
		return nil, err
	}

	if c.conn == nil {
		return nil, errors.New("Peer connection is not available")
	}

	var resBody []byte = nil
	waitChan := c.addMessage(req.Id, func(res *pb.Response) {
		logrus.Infof("Got response message %s", string(res.Body))
		resBody = res.Body
		return
	})

	err = c.conn.Send(raw, false)
	if err != nil {
		logrus.Errorf("Do: %s", err)
		c.removeMessage(req.Id)
		return nil, err
	}

	return resBody, <-waitChan
}

// Post perform a POST request
func (c *Client) Post(url string, body []byte, headers http.Header) ([]byte, error) {
	r, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	return c.Do(r)
}

// Get perform a GET request
func (c *Client) Get(url string, headers http.Header) ([]byte, error) {
	r, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(r)
}
