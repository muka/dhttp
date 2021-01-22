package dhttp

import (
	"bytes"
	"io/ioutil"

	log "github.com/sirupsen/logrus"

	"net/http"

	"github.com/golang/protobuf/proto"
	pb "github.com/muka/dhttp/protobuf"
	"github.com/muka/peer"
)

func client(id string, opts peer.Options) (*peer.Peer, error) {
	peer1, err := peer.NewPeer(id, opts)
	if err != nil {
		return nil, err
	}

	return peer1, nil
}

func requestErrorResponse(id string, code int, msg string) *pb.Response {
	return &pb.Response{
		Id:         id,
		StatusCode: int32(code),
		Body:       []byte(msg),
	}
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

func service(id string, opts peer.Options) (*peer.Peer, error) {
	peer1, err := peer.NewPeer(id, opts)
	if err != nil {
		return nil, err
	}

	peer1.On("connection", func(data interface{}) {
		conn1 := data.(*peer.DataConnection)
		conn1.On("data", func(data interface{}) {

			// create request
			req := &pb.Request{}
			if err := proto.Unmarshal(data.([]byte), req); err != nil {
				msg := "Failed to parse request"
				log.Errorf("%s: %s", msg, err)
				sendResponse(conn1, requestErrorResponse(req.Id, 500, msg))
				return
			}
			log.Debugf("Received request: %v", req)

			c := &http.Client{}

			httpReq, err := http.NewRequest(req.Method, req.Url, bytes.NewReader(req.Body))
			if err != nil {
				msg := "Failed to create HTTP request"
				log.Errorf("%s: %s", msg, err)
				sendResponse(conn1, requestErrorResponse(req.Id, 500, msg))
				return
			}

			for _, header := range req.Headers {
				for _, value := range header.Values {
					httpReq.Header.Add(header.Key, value)
				}
			}

			if req.Protocol != "" {
				httpReq.Proto = req.Protocol
			}

			httpRes, err := c.Do(httpReq)
			if err != nil {
				msg := "Failed to send HTTP request"
				log.Errorf("%s: %s", msg, err)
				sendResponse(conn1, requestErrorResponse(req.Id, 500, msg))
				return
			}

			// prepare response
			res := new(pb.Response)
			res.Id = req.Id
			res.StatusCode = int32(httpRes.StatusCode)
			body, err := ioutil.ReadAll(httpRes.Body)
			if err != nil {
				msg := "Failed to read HTTP response body"
				log.Errorf("%s: %s", msg, err)
				sendResponse(conn1, requestErrorResponse(req.Id, 500, msg))
				return
			}
			res.Body = body
			for key, values := range httpRes.Header {
				res.Headers = append(res.Headers, &pb.Header{
					Key:    key,
					Values: values,
				})
			}

			err = sendResponse(conn1, res)
			if err != nil {
				log.Errorf("Faield  to send repsonse: %s", err)
			}

		})
	})

	return peer1, nil
}
