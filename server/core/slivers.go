package core

import (
	"errors"
	"sync"
	"time"

	clientpb "github.com/bishopfox/sliver/protobuf/client"
	implantpb "github.com/bishopfox/sliver/protobuf/implant"
)

var (
	// Hive - Manages sliver connections
	Hive = &SliverHive{
		Slivers: &map[uint32]*Sliver{},
		mutex:   &sync.RWMutex{},
	}
	hiveID = new(uint32)
)

// Sliver implant
type Sliver struct {
	ID            uint32
	Name          string
	Hostname      string
	Username      string
	UID           string
	GID           string
	Os            string
	Arch          string
	Transport     string
	RemoteAddress string
	PID           int32
	Filename      string
	LastCheckin   *time.Time
	Send          chan *implantpb.Envelope
	Resp          map[uint64]chan *implantpb.Envelope
	RespMutex     *sync.RWMutex
	ActiveC2      string
}

// ToProtobuf - Get the protobuf version of the object
func (s *Sliver) ToProtobuf() *clientpb.Sliver {
	var lastCheckin string
	if s.LastCheckin == nil {
		lastCheckin = time.Now().Format(time.RFC1123) // Stateful connections have a nil .LastCheckin
	} else {
		lastCheckin = s.LastCheckin.Format(time.RFC1123)
	}
	return &clientpb.Sliver{
		ID:            uint32(s.ID),
		Name:          s.Name,
		Hostname:      s.Hostname,
		Username:      s.Username,
		UID:           s.UID,
		GID:           s.GID,
		OS:            s.Os,
		Arch:          s.Arch,
		Transport:     s.Transport,
		RemoteAddress: s.RemoteAddress,
		PID:           int32(s.PID),
		Filename:      s.Filename,
		LastCheckin:   lastCheckin,
		ActiveC2:      s.ActiveC2,
	}
}

// Config - Get the config the sliver was generated with
func (s *Sliver) Config() error {

	return nil
}

// Request - Sends a protobuf request to the active sliver and returns the response
func (s *Sliver) Request(msgType uint32, timeout time.Duration, data []byte) ([]byte, error) {

	resp := make(chan *implantpb.Envelope)
	reqID := EnvelopeID()
	s.RespMutex.Lock()
	s.Resp[reqID] = resp
	s.RespMutex.Unlock()
	defer func() {
		s.RespMutex.Lock()
		defer s.RespMutex.Unlock()
		// close(resp)
		delete(s.Resp, reqID)
	}()
	s.Send <- &implantpb.Envelope{
		ID:   reqID,
		Type: msgType,
		Data: data,
	}

	var respEnvelope *implantpb.Envelope
	select {
	case respEnvelope = <-resp:
	case <-time.After(timeout):
		return nil, errors.New("timeout")
	}
	return respEnvelope.Data, nil
}

// SliverHive - Mananges the slivers, provides atomic access
type SliverHive struct {
	mutex   *sync.RWMutex
	Slivers *map[uint32]*Sliver
}

// Sliver - Get Sliver by ID
func (h *SliverHive) Sliver(sliverID uint32) *Sliver {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	return (*h.Slivers)[sliverID]
}

// AddSliver - Add a sliver to the hive (atomically)
func (h *SliverHive) AddSliver(sliver *Sliver) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	(*h.Slivers)[sliver.ID] = sliver
}

// RemoveSliver - Add a sliver to the hive (atomically)
func (h *SliverHive) RemoveSliver(sliver *Sliver) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	delete((*h.Slivers), sliver.ID)
}

// GetHiveID - Returns an incremental nonce as an id
func GetHiveID() uint32 {
	newID := (*hiveID) + 1
	(*hiveID)++
	return newID
}
