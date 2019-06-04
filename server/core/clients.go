package core

import (
	"crypto/x509"
	"sync"

	clientpb "github.com/bishopfox/sliver/protobuf/client"
	implantpb "github.com/bishopfox/sliver/protobuf/implant"
)

var (
	// Clients - Manages client connections
	Clients = &clientConns{
		Connections: &map[int]*Client{},
		mutex:       &sync.RWMutex{},
	}

	clientID = new(int)
)

// Client - Single client connection
type Client struct {
	ID          int
	Operator    string
	Certificate *x509.Certificate
	Send        chan *implantpb.Envelope
	Resp        map[uint64]chan *implantpb.Envelope
	mutex       *sync.RWMutex
}

// ToProtobuf - Get the protobuf version of the object
func (c *Client) ToProtobuf() *clientpb.Client {
	return &clientpb.Client{
		ID:       int32(c.ID),
		Operator: c.Operator,
	}
}

// Response - Drop an evelope into a response channel
func (c *Client) Response(envelope *implantpb.Envelope) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if resp, ok := c.Resp[envelope.ID]; ok {
		resp <- envelope
	}
}

// clientConns - Manage client connections
type clientConns struct {
	mutex       *sync.RWMutex
	Connections *map[int]*Client
}

// AddClient - Add a client struct atomically
func (cc *clientConns) AddClient(client *Client) {
	cc.mutex.Lock()
	defer cc.mutex.Unlock()
	(*cc.Connections)[client.ID] = client
}

// RemoveClient - Remove a client struct atomically
func (cc *clientConns) RemoveClient(clientID int) {
	cc.mutex.Lock()
	defer cc.mutex.Unlock()
	delete((*cc.Connections), clientID)
}

// GetClientID - Get a client ID
func GetClientID() int {
	newID := (*clientID) + 1
	(*clientID)++
	return newID
}

// GetClient - Create a new client object
func GetClient(certificate *x509.Certificate) *Client {
	var operator string
	if certificate != nil {
		operator = certificate.Subject.CommonName
	} else {
		operator = "server"
	}
	return &Client{
		ID:          GetClientID(),
		Operator:    operator,
		Certificate: certificate,
		mutex:       &sync.RWMutex{},
		Send:        make(chan *implantpb.Envelope),
		Resp:        map[uint64]chan *implantpb.Envelope{},
	}
}
