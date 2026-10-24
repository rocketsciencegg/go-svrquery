package sqp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/multiplay/go-svrquery/lib/svrsample/common"
	"github.com/stretchr/testify/require"
)

func TestSQPServerUnsupportedVersion(t *testing.T) {
	for _, version := range []uint16{0, 3, 258} {
		t.Run(fmt.Sprintf("v%d", version), func(t *testing.T) {
			q, err := NewQueryResponder(common.QueryState{})
			require.NoError(t, err)
			challenge, err := q.Respond("client", []byte{0, 0, 0, 0, 0})
			require.NoError(t, err)
			request := []byte{1, 0, 0, 0, 0, 0, 0, 0x11}
			copy(request[1:5], challenge[1:5])
			binary.BigEndian.PutUint16(request[5:7], version)
			response, err := q.Respond("client", request)
			require.EqualError(t, err, fmt.Sprintf("unsupported sqp version: %d", version))
			require.Nil(t, response)
		})
	}
}

func TestQueryResponder_Respond(t *testing.T) {
	setup := func(t *testing.T) *QueryResponder {
		t.Helper()
		q, err := NewQueryResponder(common.QueryState{
			CurrentPlayers: 1,
			MaxPlayers:     2,
			Metrics:        []float32{1, 0, 3.14159, 55.57, 438.2522, -123.456},
		})
		require.NoError(t, err)
		return q
	}
	serverInfo := []byte{0, 0, 0, 10, 0, 1, 0, 2, 0, 0, 0, 0, 0, 0}
	metrics := []byte{
		0, 0, 0, 25, 6,
		0x3f, 0x80, 0, 0,
		0, 0, 0, 0,
		0x40, 0x49, 0x0f, 0xd0,
		0x42, 0x5e, 0x47, 0xae,
		0x43, 0xdb, 0x20, 0x48,
		0xc2, 0xf6, 0xe9, 0x79,
	}

	t.Run("empty", func(t *testing.T) {
		q := setup(t)
		requireQueryPayload(t, q, 0, nil)
	})
	t.Run("server-info-only", func(t *testing.T) {
		q := setup(t)
		requireQueryPayload(t, q, 1, serverInfo)
	})
	t.Run("metrics-only", func(t *testing.T) {
		q := setup(t)
		requireQueryPayload(t, q, 16, metrics)
	})
	t.Run("server-info-and-metrics", func(t *testing.T) {
		q := setup(t)
		requireQueryPayload(t, q, 17, append(append([]byte{}, serverInfo...), metrics...))
	})
	t.Run("nil-input", func(t *testing.T) {
		q := setup(t)
		_, err := q.Respond("client", nil)
		require.ErrorContains(t, err, "unsupported query")
	})
	t.Run("short-challenge", func(t *testing.T) {
		q := setup(t)
		for n := 1; n < 5; n++ {
			_, err := q.Respond("client", make([]byte, n))
			require.ErrorContains(t, err, "unsupported query")
		}
	})
	t.Run("padded-challenge", func(t *testing.T) {
		q := setup(t)
		for n := 5; n < 8; n++ {
			response, err := q.Respond("client", make([]byte, n))
			require.NoError(t, err)
			require.Len(t, response, 5)
		}
	})
	t.Run("short-query", func(t *testing.T) {
		q := setup(t)
		for n := 1; n < 8; n++ {
			_, err := q.Respond("client", []byte{0, 0, 0, 0, 0})
			require.NoError(t, err)
			packet := make([]byte, n)
			packet[0] = 1
			_, err = q.Respond("client", packet)
			require.ErrorContains(t, err, "packet not long enough")
		}
	})
	t.Run("unsupported-header", func(t *testing.T) {
		q := setup(t)
		for n := 1; n < 8; n++ {
			packet := make([]byte, n)
			packet[0] = 2
			_, err := q.Respond("client", packet)
			require.ErrorContains(t, err, "unsupported query")
		}
	})
}

func requireQueryPayload(t *testing.T, q *QueryResponder, chunks byte, payload []byte) {
	t.Helper()
	const address = "client-addr:65534"
	challenge, err := q.Respond(address, []byte{0, 0, 0, 0, 0})
	require.NoError(t, err)
	require.Len(t, challenge, 5)
	require.Equal(t, byte(0), challenge[0])
	request := append([]byte{1}, challenge[1:5]...)
	request = append(request, 0, 1, chunks)
	response, err := q.Respond(address, request)
	require.NoError(t, err)
	expected := bytes.NewBuffer(append([]byte{1}, challenge[1:5]...))
	_, err = expected.Write([]byte{0, 1, 0, 0})
	require.NoError(t, err)
	require.NoError(t, binary.Write(expected, binary.BigEndian, uint16(len(payload))))
	_, err = expected.Write(payload)
	require.NoError(t, err)
	require.Equal(t, expected.Bytes(), response)
}

func TestNewQueryResponder(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		q, err := NewQueryResponder(common.QueryState{})
		require.NoError(t, err)
		require.NotNil(t, q)
	})
	t.Run("maximum", func(t *testing.T) {
		state := common.QueryState{PerformanceFlags: make([]bool, 32), PerformanceGauges: make([]float32, 255)}
		q, err := NewQueryResponder(state)
		require.NoError(t, err)
		require.Equal(t, state, q.state)
	})
	t.Run("flags-overflow", func(t *testing.T) {
		q, err := NewQueryResponder(common.QueryState{PerformanceFlags: make([]bool, 33)})
		require.ErrorContains(t, err, "performance data exceeds wire-format limits")
		require.Nil(t, q)
	})
	t.Run("gauges-overflow", func(t *testing.T) {
		q, err := NewQueryResponder(common.QueryState{PerformanceGauges: make([]float32, 256)})
		require.ErrorContains(t, err, "performance data exceeds wire-format limits")
		require.Nil(t, q)
	})
}
