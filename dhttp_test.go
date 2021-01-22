package dhttp

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	pb "github.com/muka/dhttp/protobuf"
	"github.com/muka/peer"
	"github.com/rs/xid"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func getOpts() peer.Options {
	opts := peer.NewOptions()

	opts.Secure = false
	opts.Host = "localhost"
	opts.Port = 9000
	opts.Path = "/myapp"
	opts.Debug = 2

	return opts
}

func createServer(serverHost string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Add("content-type", "text/html")
		rw.Write([]byte(`<html><body><h1>Hello world!</h1></body></html>`))
	})
	mux.HandleFunc("/json", func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Add("content-type", "application/json")
		rw.Write([]byte(`{ "hello": "world" }`))
	})

	server := &http.Server{Addr: serverHost, Handler: mux}
	go func() {
		if err := server.ListenAndServe(); err != nil {
			// handle err
		}
	}()

	return server
}

func TestStart(t *testing.T) {

	log.SetLevel(log.DebugLevel)

	serviceID := xid.New().String()
	clientID := xid.New().String()
	opts := getOpts()

	peer1, err := service(serviceID, opts)
	assert.NoError(t, err)
	defer peer1.Close()

	serverHost := ":12345"
	httpServer := createServer(serverHost)
	defer httpServer.Close()

	c, err := client(clientID, opts)
	assert.NoError(t, err)

	conn1, err := c.Connect(serviceID, nil)
	assert.NoError(t, err)

	done := make(chan bool)

	conn1.On("open", func(data interface{}) {

		conn1.On("data", func(data interface{}) {
			res := new(pb.Response)
			err := proto.Unmarshal(data.([]byte), res)

			assert.NoError(t, err)
			log.Debugf("Got response: %s", res.Body)

		})

		req := &pb.Request{
			Method:   "POST",
			Url:      fmt.Sprintf("http://localhost%s/json", serverHost),
			Id:       xid.New().String(),
			Protocol: "HTTP/1.1",
			Headers: []*pb.Header{
				{
					Key:    "Content-Type",
					Values: []string{"application/json"},
				},
			},
			Body: []byte(`{"message": "hello world"}`),
		}
		raw, err := proto.Marshal(req)
		assert.NoError(t, err)
		conn1.Send(raw, false)
		done <- true

	})

	<-time.After(time.Millisecond * 500)
	<-done
}
