package dhttp

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/muka/peer"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func getOpts() peer.Options {
	opts := peer.NewOptions()

	opts.Secure = false
	opts.Host = "localhost"
	opts.Port = 9000
	opts.Path = "/myapp"
	opts.Debug = 0

	return opts
}

func createServer(serverHost string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
		logrus.Debugf("Echo %s", body)
		rw.Write(body)
	})
	server := &http.Server{Addr: serverHost, Handler: mux}
	go func() {
		if err := server.ListenAndServe(); err != nil {
			// handle err
		}
	}()

	return server
}

func initServer(t *testing.T, peerOpts peer.Options, serverHost string) (*Proxy, func()) {

	proxyOpts := NewProxyOptions()
	proxyOpts.Peer = peerOpts

	proxy, err := NewProxy(proxyOpts)
	assert.NoError(t, err)
	assert.NotNil(t, proxy)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	httpServer := createServer(serverHost)

	close := func() {
		proxy.Close()
		httpServer.Close()
	}

	return proxy, close
}

func initClient(t *testing.T, peerOpts peer.Options, proxyID string) *Client {

	clientOptions := NewClientOptions()
	clientOptions.peer = peerOpts
	clientOptions.serverID = proxyID

	c, err := NewClient(context.Background(), clientOptions)
	assert.NoError(t, err)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	return c
}

func getClientServer(t *testing.T, serverHost string) (*Client, func()) {

	peerOpts := getOpts()

	proxy, close := initServer(t, peerOpts, serverHost)

	c := initClient(t, peerOpts, proxy.GetID())

	return c, close
}

func TestPost(t *testing.T) {

	log.SetLevel(log.DebugLevel)
	serverHost := ":54321"
	c, close := getClientServer(t, serverHost)
	defer close()

	payload := []byte(`{"message": "hello world"}`)
	res, err := c.Post(
		fmt.Sprintf("http://localhost%s/", serverHost),
		payload,
		http.Header{
			"Content-Type": []string{"application/json"},
		},
	)
	assert.NoError(t, err)
	if err != nil {
		t.FailNow()
	}

	log.Debugf("Response: %s", res)
	assert.Equal(t, payload, res)
}

func TestGet(t *testing.T) {

	log.SetLevel(log.DebugLevel)
	serverHost := ":54321"
	c, close := getClientServer(t, serverHost)
	defer close()

	res, err := c.Get(
		fmt.Sprintf("http://localhost%s/", serverHost),
		http.Header{
			"Content-Type": []string{"application/json"},
		},
	)
	assert.NoError(t, err)
	if err != nil {
		t.FailNow()
	}

	log.Debugf("Response: %s", res)
}

func TestParallelRequests(t *testing.T) {

	log.SetLevel(log.DebugLevel)
	serverHost := ":54321"
	c, close := getClientServer(t, serverHost)
	defer close()

	count := 1000
	total := 0
	for i := 0; i < count; i++ {

		go func() {

			payload := []byte(`{"message": "hello world"}`)
			res, err := c.Post(
				fmt.Sprintf("http://localhost%s/", serverHost),
				payload,
				http.Header{
					"Content-Type": []string{"application/json"},
				},
			)
			assert.NoError(t, err)
			if err != nil {
				t.FailNow()
			}

			log.Debugf("Response: %s", res)
			assert.Equal(t, payload, res)
			total++
		}()

	}

	for {
		if total == count {
			break
		}
	}

	assert.Equal(t, count, total)
}
