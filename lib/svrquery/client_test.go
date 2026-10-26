package svrquery

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/multiplay/go-svrquery/lib/svrquery/protocol/sqp"
	"github.com/multiplay/go-svrquery/lib/svrsample"
	"github.com/multiplay/go-svrquery/lib/svrsample/common"
	"github.com/stretchr/testify/require"
)

const (
	// sqpExchangePackets counts the challenge and query in one exchange.
	sqpExchangePackets = 2
	// sqpResponderTimeout bounds the lifetime of the UDP test server.
	sqpResponderTimeout = 5 * time.Second
	// sqpChallengeSize includes the packet type and uint32 challenge.
	sqpChallengeSize = 1 + 4
	// sqpQuerySize adds the uint16 version and requested chunk bits.
	sqpQuerySize = sqpChallengeSize + 2 + 1
)

func TestClient_Query(t *testing.T) {
	state := common.QueryState{
		CurrentPlayers:    1,
		MaxPlayers:        2,
		ServerName:        "sample",
		Map:               "map",
		Metrics:           []float32{-1, 0, 1, 3.14159},
		PerformanceFlags:  []bool{true, false, true},
		PerformanceGauges: []float32{-1, 0, 1, 3.14159},
	}
	serverInfo := &sqp.ServerInfoChunk{
		ChunkLength:    19,
		CurrentPlayers: 1,
		MaxPlayers:     2,
		ServerName:     "sample",
		Map:            "map",
	}
	performanceInfo := &sqp.PerformanceInfoChunk{
		ChunkLength: 22,
		NumFlags:    3,
		Flags:       0b101,
		Gauges:      []float32{-1, 0, 1, 3.14159},
	}
	setup := func(t *testing.T, proto, responderProto string) (*Client, <-chan []byte) {
		t.Helper()
		responder, err := svrsample.GetResponder(responderProto, state)
		require.NoError(t, err)
		conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, conn.Close()) })
		require.NoError(t, conn.SetDeadline(time.Now().Add(sqpResponderTimeout)))

		done := make(chan error, 1)
		requests := make(chan []byte, sqpExchangePackets)
		go func() {
			done <- serveSQPExchange(conn, responder, requests)
		}()
		t.Cleanup(func() { require.NoError(t, <-done, "sample responder failed") })

		c, err := NewClient(proto, conn.LocalAddr().String(), WithTimeout(time.Second))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, c.Close()) })
		return c, requests
	}

	t.Run("v1-with-legacy-responder", func(t *testing.T) {
		c, requests := setup(t, "sqp", "sqp")
		requireSQPQuerySuccess(t, c, requests, sqp.ServerInfo, &sqp.QueryResponse{
			Version:    sqp.Version,
			ServerInfo: serverInfo,
		})
	})
	t.Run("v1-with-v2-responder", func(t *testing.T) {
		c, requests := setup(t, "sqp", "sqp-v2")
		requireSQPQuerySuccess(t, c, requests, sqp.ServerInfo, &sqp.QueryResponse{
			Version:    sqp.Version,
			ServerInfo: serverInfo,
		})
	})
	t.Run("v2-with-legacy-responder", func(t *testing.T) {
		c, requests := setup(t, "sqp-v2", "sqp")
		requireSQPQuerySuccess(t, c, requests, sqp.ServerInfo|sqp.PerformanceInfo, &sqp.QueryResponse{
			Version:         sqp.Version2,
			ServerInfo:      serverInfo,
			PerformanceInfo: performanceInfo,
		})
	})
	t.Run("v2-with-v2-responder", func(t *testing.T) {
		c, requests := setup(t, "sqp-v2", "sqp-v2")
		requireSQPQuerySuccess(t, c, requests, sqp.ServerInfo|sqp.PerformanceInfo, &sqp.QueryResponse{
			Version:         sqp.Version2,
			ServerInfo:      serverInfo,
			PerformanceInfo: performanceInfo,
		})
	})
}

func serveSQPExchange(conn *net.UDPConn, responder common.QueryResponder, requests chan<- []byte) error {
	for i := 0; i < sqpExchangePackets; i++ {
		buf := make([]byte, sqp.DefaultMaxPacketSize)
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			return err
		}

		requests <- buf[:n]
		response, err := responder.Respond(addr.String(), buf[:n])
		if err != nil {
			return err
		}

		if _, err := conn.WriteToUDP(response, addr); err != nil {
			return err
		}
	}
	return nil
}

func requireSQPQuerySuccess(t *testing.T, c *Client, requests <-chan []byte, chunks byte, expected *sqp.QueryResponse) {
	t.Helper()
	response, err := c.Query()
	require.NoError(t, err)
	expected.Address = c.Address()
	require.Equal(t, expected, response)
	require.Equal(t, make([]byte, sqpChallengeSize), <-requests)
	request := <-requests
	require.Len(t, request, sqpQuerySize)
	require.Equal(t, sqp.QueryRequestType, request[0])
	require.Equal(t, expected.Version, binary.BigEndian.Uint16(request[sqpChallengeSize:sqpQuerySize-1]))
	require.Equal(t, chunks, request[sqpQuerySize-1])
}

func TestNewClient(t *testing.T) {
	addr := "localhost:8000"

	t.Run("tf2e", func(t *testing.T) {
		c, err := NewClient("tf2e", addr, WithKey("test"), WithTimeout(time.Second))
		require.NoError(t, err)
		require.NotNil(t, c)
	})

	t.Run("invalid-protocol", func(t *testing.T) {
		c, err := NewClient("my-protocol", addr, WithKey("test"), WithTimeout(time.Second))
		require.Error(t, err)
		require.Nil(t, c)
	})
}

func TestQuery(t *testing.T) {
	addr := os.Getenv("TEST_QUERY_ADDR")
	if addr == "" {
		t.Skip("env TEST_QUERY_ADDR not set")
	}

	proto := os.Getenv("TEST_QUERY_PROTO")
	if proto == "" {
		t.Skip("env TEST_QUERY_PROTO not set")
	}

	c, err := NewClient(proto, addr)
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		r, err := c.Query()
		require.NoError(t, err)
		fmt.Printf("%#v\n", r)
	}
}
