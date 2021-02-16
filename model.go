package dhttp

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/golang/protobuf/proto"
	pb "github.com/muka/dhttp/protobuf"
)

//BodyTooLargeError indicates the request body is larger than allowed by MaxBodyBytes option
type BodyTooLargeError interface {
	error
}

func createHTTPResponse(reqID string, httpRes *http.Response, maxBodyBytes int64) (*pb.Response, error) {

	res := new(pb.Response)
	res.Id = reqID
	res.Status = int32(httpRes.StatusCode)
	res.StatusText = http.StatusText(httpRes.StatusCode)

	r := http.MaxBytesReader(nil, httpRes.Body, maxBodyBytes)
	body, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, BodyTooLargeError(err)
	}
	res.Body = body

	defer func() {
		io.Copy(ioutil.Discard, httpRes.Body)
		httpRes.Body.Close()
	}()

	for key, values := range httpRes.Header {
		res.Headers = append(res.Headers, &pb.Header{
			Key:    key,
			Values: values,
		})
	}

	return res, nil
}

func createHTTPRequest(req *pb.Request) (*http.Request, error) {

	httpReq, err := http.NewRequest(req.Method, req.Url, bytes.NewReader(req.Body))
	if err != nil {
		return nil, err
	}

	for _, header := range req.Headers {
		for _, value := range header.Values {
			httpReq.Header.Add(header.Key, value)
		}
	}

	if req.Protocol != "" {
		httpReq.Proto = req.Protocol
	}

	return httpReq, nil
}

func parseRequestProto(data []byte) (*pb.Request, error) {
	req := &pb.Request{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, err
	}
	return req, nil
}

func requestErrorResponse(id string, code int, msg string) *pb.Response {
	return &pb.Response{
		Id:         id,
		Status:     int32(code),
		StatusText: http.StatusText(code),
		Body:       []byte(msg),
	}
}
