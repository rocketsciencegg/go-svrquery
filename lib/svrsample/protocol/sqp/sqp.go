package sqp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"sync"

	"github.com/multiplay/go-svrquery/lib/svrsample/common"
)

// QueryResponder responds to queries
type QueryResponder struct {
	challenges sync.Map
	enc        *common.Encoder
	state      common.QueryState
}

// challengeWireFormat describes the format of an SQP challenge response
type challengeWireFormat struct {
	Header    byte
	Challenge uint32
}

// queryWireFormat describes the format of an SQP query response
type queryWireFormat struct {
	Header           byte
	Challenge        uint32
	SQPVersion       uint16
	CurrentPacketNum byte
	LastPacketNum    byte
	PayloadLength    uint16
	ServerInfoLength *uint32
	ServerInfo       *ServerInfo
	MetricsLength    *uint32
	Metrics          *Metrics
}

// NewQueryResponder returns creates a new responder capable of responding
// to SQP-formatted queries.
func NewQueryResponder(state common.QueryState) (*QueryResponder, error) {
	q := &QueryResponder{
		enc:   &common.Encoder{},
		state: state,
	}
	return q, nil
}

// Respond writes a query response to the requester in the SQP wire protocol.
func (q *QueryResponder) Respond(clientAddress string, buf []byte) ([]byte, error) {
	switch {
	case isChallenge(buf):
		return q.handleChallenge(clientAddress)

	case isQuery(buf):
		return q.handleQuery(clientAddress, buf)
	}

	return nil, errors.New("unsupported query")
}

// isChallenge determines if the input buffer corresponds to a challenge packet.
func isChallenge(buf []byte) bool {
	return bytes.Equal(buf[0:5], []byte{0, 0, 0, 0, 0})
}

// isQuery determines if the input buffer corresponds to a query packet.
func isQuery(buf []byte) bool {
	return buf[0] == 1
}

// handleChallenge handles an incoming challenge packet.
func (q *QueryResponder) handleChallenge(clientAddress string) ([]byte, error) {
	v := rand.Uint32()
	q.challenges.Store(clientAddress, v)

	resp := bytes.NewBuffer(nil)
	err := common.WireWrite(
		resp,
		q.enc,
		challengeWireFormat{
			Header:    0,
			Challenge: v,
		},
	)
	if err != nil {
		return nil, err
	}

	return resp.Bytes(), nil
}

// handleQuery handles an incoming query packet.
func (q *QueryResponder) handleQuery(clientAddress string, buf []byte) ([]byte, error) {
	expectedChallenge, ok := q.challenges.LoadAndDelete(clientAddress)
	if !ok {
		return nil, errors.New("no challenge")
	}

	if len(buf) < 8 {
		return nil, errors.New("packet not long enough")
	}

	// Challenge doesn't match, return with no response
	if binary.BigEndian.Uint32(buf[1:5]) != expectedChallenge.(uint32) {
		return nil, errors.New("challenge mismatch")
	}

	version := binary.BigEndian.Uint16(buf[5:7])
	if version != 1 && version != 2 {
		return nil, fmt.Errorf("unsupported sqp version: %d", version)
	}

	requestedChunks := buf[7]
	wantsServerInfo := requestedChunks&0x1 == 1
	wantsMetrics := requestedChunks&0x10 == 16

	f := queryWireFormat{
		Header:     1,
		Challenge:  expectedChallenge.(uint32),
		SQPVersion: version,
	}

	resp := bytes.NewBuffer(nil)

	if wantsServerInfo {
		f.ServerInfo = ServerInfoFromQueryState(q.state)
		size := f.ServerInfo.Size()
		f.ServerInfoLength = &size
		f.PayloadLength += uint16(*f.ServerInfoLength) + 4
	}

	if wantsMetrics {
		f.Metrics = MetricsFromQueryState(q.state)
		size := f.Metrics.Size()
		f.MetricsLength = &size
		f.PayloadLength += uint16(*f.MetricsLength) + 4
	}

	if err := common.WireWrite(resp, q.enc, f); err != nil {
		return nil, err
	}

	return resp.Bytes(), nil
}
