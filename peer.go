package dhttp

import "github.com/muka/peer"

func createPeer(id string, opts peer.Options) (*peer.Peer, error) {
	peer1, err := peer.NewPeer(id, opts)
	if err != nil {
		return nil, err
	}
	return peer1, err
}
