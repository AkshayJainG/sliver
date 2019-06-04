package core

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log"

	"github.com/bishopfox/sliver/client/assets"
	clientpb "github.com/bishopfox/sliver/protobuf/client"
	implantpb "github.com/bishopfox/sliver/protobuf/implant"

	"sync"
	"time"

	"github.com/golang/protobuf/proto"
)

const (
	randomIDSize = 16 // 64bits
)

type tunnels struct {
	server  *SliverServer
	tunnels *map[uint64]*tunnel
	mutex   *sync.RWMutex
}

func (t *tunnels) bindTunnel(SliverID uint32, TunnelID uint64) *tunnel {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	(*t.tunnels)[TunnelID] = &tunnel{
		server:   t.server,
		SliverID: SliverID,
		ID:       TunnelID,
		Recv:     make(chan []byte),
	}

	return (*t.tunnels)[TunnelID]
}

// RecvTunnelData - Routes a TunnelData protobuf msg to the correct tunnel object
func (t *tunnels) RecvTunnelData(tunnelData *implantpb.TunnelData) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	tunnel := (*t.tunnels)[tunnelData.TunnelID]
	if tunnel != nil {
		(*tunnel).Recv <- tunnelData.Data
	} else {
		log.Printf("No client tunnel with ID %d", tunnelData.TunnelID)
	}
}

func (t *tunnels) Close(ID uint64) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	close((*t.tunnels)[ID].Recv)
	delete(*t.tunnels, ID)
}

// tunnel - Duplex data tunnel
type tunnel struct {
	server   *SliverServer
	SliverID uint32
	ID       uint64
	Recv     chan []byte
}

func (t *tunnel) Send(data []byte) {
	log.Printf("Sending %d bytes on tunnel %d (sliver %d)", len(data), t.ID, t.SliverID)
	tunnelData := &implantpb.TunnelData{
		SliverID: t.SliverID,
		TunnelID: t.ID,
		Data:     data,
	}
	rawTunnelData, _ := proto.Marshal(tunnelData)
	t.server.Send <- &implantpb.Envelope{
		Type: implantpb.MsgTunnelData,
		Data: rawTunnelData,
	}
}

// SliverServer - Server info
type SliverServer struct {
	Send      chan *implantpb.Envelope
	recv      chan *implantpb.Envelope
	responses *map[uint64]chan *implantpb.Envelope
	mutex     *sync.RWMutex
	Config    *assets.ClientConfig
	Events    chan *clientpb.Event
	Tunnels   *tunnels
}

// CreateTunnel - Create a new tunnel on the server, returns tunnel metadata
func (ss *SliverServer) CreateTunnel(sliverID uint32, defaultTimeout time.Duration) (*tunnel, error) {
	tunReq := &clientpb.TunnelCreateReq{SliverID: sliverID}
	tunReqData, _ := proto.Marshal(tunReq)

	tunResp := <-ss.RPC(&implantpb.Envelope{
		Type: clientpb.MsgTunnelCreate,
		Data: tunReqData,
	}, defaultTimeout)
	if tunResp.Err != "" {
		return nil, fmt.Errorf("Error: %s", tunResp.Err)
	}

	tunnelCreated := &clientpb.TunnelCreate{}
	proto.Unmarshal(tunResp.Data, tunnelCreated)

	tunnel := ss.Tunnels.bindTunnel(tunnelCreated.SliverID, tunnelCreated.TunnelID)

	log.Printf("Created new tunnel with ID %d", tunnel.ID)

	return tunnel, nil
}

// ResponseMapper - Maps recv'd envelopes to response channels
func (ss *SliverServer) ResponseMapper() {
	for envelope := range ss.recv {
		if envelope.ID != 0 {
			ss.mutex.Lock()
			if resp, ok := (*ss.responses)[envelope.ID]; ok {
				resp <- envelope
			}
			ss.mutex.Unlock()
		} else {
			// If the message does not have an envelope ID then we route it based on type
			switch envelope.Type {

			case clientpb.MsgEvent:
				event := &clientpb.Event{}
				err := proto.Unmarshal(envelope.Data, event)
				if err != nil {
					log.Printf("Failed to decode event envelope")
					continue
				}
				// log.Printf("[client] Routing event message")
				ss.Events <- event

			case implantpb.MsgTunnelData:
				tunnelData := &implantpb.TunnelData{}
				err := proto.Unmarshal(envelope.Data, tunnelData)
				if err != nil {
					log.Printf("Failed to decode tunnel data envelope")
					continue
				}
				// log.Printf("[client] Routing tunnel data with id %d", tunnelData.TunnelID)
				ss.Tunnels.RecvTunnelData(tunnelData)

			case implantpb.MsgTunnelClose:
				tunnelClose := &implantpb.TunnelClose{}
				err := proto.Unmarshal(envelope.Data, tunnelClose)
				if err != nil {
					log.Printf("Failed to decode tunnel data envelope")
					continue
				}
				ss.Tunnels.Close(tunnelClose.TunnelID)

			}
		}
	}
}

// RPC - Send a request envelope and wait for a response (blocking)
func (ss *SliverServer) RPC(envelope *implantpb.Envelope, timeout time.Duration) chan *implantpb.Envelope {
	reqID := EnvelopeID()
	envelope.ID = reqID
	envelope.Timeout = timeout.Nanoseconds()
	resp := make(chan *implantpb.Envelope)
	ss.AddRespListener(reqID, resp)
	ss.Send <- envelope
	respCh := make(chan *implantpb.Envelope)
	go func() {
		defer ss.RemoveRespListener(reqID)
		select {
		case respEnvelope := <-resp:
			respCh <- respEnvelope
		case <-time.After(timeout + time.Second):
			respCh <- &implantpb.Envelope{Err: "Timeout"}
		}
	}()
	return respCh
}

// AddRespListener - Add a response listener
func (ss *SliverServer) AddRespListener(envelopeID uint64, resp chan *implantpb.Envelope) {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()
	(*ss.responses)[envelopeID] = resp
}

// RemoveRespListener - Remove a listener
func (ss *SliverServer) RemoveRespListener(envelopeID uint64) {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()
	close((*ss.responses)[envelopeID])
	delete((*ss.responses), envelopeID)
}

// BindSliverServer - Bind send/recv channels to a server
func BindSliverServer(send, recv chan *implantpb.Envelope) *SliverServer {
	server := &SliverServer{
		Send:      send,
		recv:      recv,
		responses: &map[uint64]chan *implantpb.Envelope{},
		mutex:     &sync.RWMutex{},
		Events:    make(chan *clientpb.Event, 1),
	}
	server.Tunnels = &tunnels{
		server:  server,
		tunnels: &map[uint64]*tunnel{},
		mutex:   &sync.RWMutex{},
	}
	return server
}

// EnvelopeID - Generate random ID
func EnvelopeID() uint64 {
	randBuf := make([]byte, 8) // 64 bits of randomness
	rand.Read(randBuf)
	return binary.LittleEndian.Uint64(randBuf)
}
