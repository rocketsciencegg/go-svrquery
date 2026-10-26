package sqp

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"testing"

	"github.com/multiplay/go-svrquery/lib/svrquery/clienttest"
	"github.com/multiplay/go-svrquery/lib/svrquery/protocol"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testDir = "testdata"
)

func performanceChunk(t *testing.T, flags byte, bits uint32, gauges []float32, extra []byte) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	require.NoError(t, binary.Write(buf, binary.BigEndian, uint32(6+4*len(gauges)+len(extra))))
	require.NoError(t, buf.WriteByte(flags))
	require.NoError(t, binary.Write(buf, binary.BigEndian, bits))
	require.NoError(t, buf.WriteByte(byte(len(gauges))))
	require.NoError(t, binary.Write(buf, binary.BigEndian, gauges))
	_, err := buf.Write(extra)
	require.NoError(t, err)
	return buf.Bytes()
}

func TestQueryer_Query(t *testing.T) {
	setup := func(t *testing.T, name string, version, responseVersion uint16, chunks byte) protocol.Queryer {
		t.Helper()
		m := clienttest.NewMockClient(t)
		m.On("Address").Return("127.0.0.1:8000").Maybe()
		creator, err := protocol.Get(name)
		require.NoError(t, err)
		m.On("Write", []byte{0, 0, 0, 0, 0}).Return(5, nil).Once()
		m.On("Read", mock.Anything).Return([]byte{0, 0, 0, 0, 1}, nil).Once()
		request := []byte{1, 0, 0, 0, 1, 0, byte(version), chunks}
		m.On("Write", request).Return(len(request), nil).Once()
		info := clienttest.LoadData(t, testDir, "info_single_response")[11:]
		payload := append([]byte{}, info...)

		if responseVersion == 2 {
			payload = append(payload, performanceChunk(t, 3, 5, []float32{60, -1.5}, nil)...)
		}

		response := []byte{1, 0, 0, 0, 1, 0, byte(responseVersion), 0, 0, byte(len(payload) >> 8), byte(len(payload))}
		response = append(response, payload...)
		m.On("Read", mock.Anything).Return(response, nil).Once()
		return creator(m)
	}

	t.Run("v1-server-info", func(t *testing.T) {
		q := setup(t, "sqp", 1, 1, ServerInfo)
		result, err := q.Query()
		require.NoError(t, err)
		require.Equal(t, &QueryResponse{
			Version: 1,
			Address: "127.0.0.1:8000",
			ServerInfo: &ServerInfoChunk{
				ChunkLength:    26,
				CurrentPlayers: 5,
				MaxPlayers:     10,
				ServerName:     "my server",
				GameType:       "ctf",
				BuildID:        "1",
				Map:            "map",
				Port:           1025,
			},
		}, result)
	})

	t.Run("v2-performance-without-stdout", func(t *testing.T) {
		q := setup(t, "sqp-v2", 2, 2, ServerInfo|PerformanceInfo)
		old := os.Stdout
		r, w, err := os.Pipe()
		require.NoError(t, err)
		writerClosed := false
		t.Cleanup(func() {
			os.Stdout = old
			require.NoError(t, r.Close())
			if !writerClosed {
				require.NoError(t, w.Close())
			}
		})
		os.Stdout = w
		result, err := q.Query()
		require.NoError(t, err)
		os.Stdout = old
		require.NoError(t, w.Close())
		writerClosed = true
		output, err := io.ReadAll(r)
		require.NoError(t, err)
		require.Empty(t, output)
		qr, ok := result.(*QueryResponse)
		require.True(t, ok)
		require.Equal(t, &QueryResponse{
			Version: 2,
			Address: "127.0.0.1:8000",
			ServerInfo: &ServerInfoChunk{
				ChunkLength:    26,
				CurrentPlayers: 5,
				MaxPlayers:     10,
				ServerName:     "my server",
				GameType:       "ctf",
				BuildID:        "1",
				Map:            "map",
				Port:           1025,
			},
			PerformanceInfo: &PerformanceInfoChunk{ChunkLength: 14, NumFlags: 3, Flags: 5, Gauges: []float32{60, -1.5}},
		}, qr)
	})

	t.Run("rejects-v1-response-to-v2-request", func(t *testing.T) {
		q := setup(t, "sqp-v2", 2, 1, ServerInfo|PerformanceInfo)
		_, err := q.Query()
		require.ErrorContains(t, err, "expected SQP version 2, got 1")
	})
	t.Run("rejects-v2-response-to-v1-request", func(t *testing.T) {
		q := setup(t, "sqp", 1, 2, ServerInfo)
		_, err := q.Query()
		require.ErrorContains(t, err, "expected SQP version 1, got 2")
	})
	t.Run("rejects-unsupported-version", func(t *testing.T) {
		q := setup(t, "sqp", 1, 3, ServerInfo)
		_, err := q.Query()
		require.ErrorContains(t, err, "unsupported SQP version: 3")
	})
}

func TestQueryer_ReadQuery(t *testing.T) {
	setup := func(t *testing.T, response []byte) *queryer {
		t.Helper()
		m := clienttest.NewMockClient(t)
		m.On("Read", mock.Anything).Return(response, nil).Once()
		return newQueryer(ServerInfo, DefaultMaxPacketSize, m)
	}

	t.Run("unsupported-version-before-body", func(t *testing.T) {
		q := setup(t, []byte{1, 0, 0, 0, 0, 0, 3, 0, 0, 0, 1, 0xaa})
		result, err := q.readQuery(ServerInfo)
		require.ErrorContains(t, err, "unsupported SQP version: 3")
		require.Nil(t, result)
		remaining := make([]byte, 5)
		_, err = io.ReadFull(q.reader, remaining)
		require.NoError(t, err)
		require.Equal(t, []byte{0, 0, 0, 1, 0xaa}, remaining, "packet metadata and body must remain unread")
	})
	t.Run("unsupported-version-with-empty-body", func(t *testing.T) {
		q := setup(t, []byte{1, 0, 0, 0, 0, 0, 3, 0, 0, 0, 0})
		result, err := q.readQuery(0)
		require.ErrorContains(t, err, "unsupported SQP version: 3")
		require.Nil(t, result)
	})
	t.Run("unsupported-version-with-truncated-header", func(t *testing.T) {
		q := setup(t, []byte{1, 0, 0, 0, 0, 0, 3})
		result, err := q.readQuery(0)
		require.ErrorContains(t, err, "unsupported SQP version: 3")
		require.Nil(t, result)
	})
}

func requirePerformanceSuccess(t *testing.T, data []byte, packetLen uint32, expected *PerformanceInfoChunk) {
	t.Helper()
	m := clienttest.NewMockClient(t)
	m.On("Address").Return("127.0.0.1:8000").Once()
	q := newQueryer(PerformanceInfo, DefaultMaxPacketSize, m)
	stream := bytes.NewBuffer(append(append([]byte{}, data...), 0xaa))
	result, err := q.readQuerySinglePacket(newPacketReader(stream), 2, PerformanceInfo, packetLen)
	require.NoError(t, err)
	require.Equal(t, &QueryResponse{Version: 2, Address: "127.0.0.1:8000", PerformanceInfo: expected}, result)
	require.Equal(t, []byte{0xaa}, stream.Bytes())
}

func requirePerformanceError(t *testing.T, data []byte, packetLen uint32) {
	t.Helper()
	m := clienttest.NewMockClient(t)
	m.On("Address").Return("127.0.0.1:8000").Once()
	q := newQueryer(PerformanceInfo, DefaultMaxPacketSize, m)
	stream := bytes.NewBuffer(append(append([]byte{}, data...), 0xaa))
	result, err := q.readQuerySinglePacket(newPacketReader(stream), 2, PerformanceInfo, packetLen)
	require.Error(t, err)
	require.Nil(t, result)
	require.GreaterOrEqual(t, stream.Len(), 1, "must not consume the next packet")
}

func TestQueryer_ReadQuerySinglePacket(t *testing.T) {
	valid := performanceChunk(t, 32, 1<<31, []float32{1.25}, nil)
	t.Run("valid", func(t *testing.T) {
		requirePerformanceSuccess(t, valid, 14, &PerformanceInfoChunk{ChunkLength: 10, NumFlags: 32, Flags: 1 << 31, Gauges: []float32{1.25}})
	})
	t.Run("maximum-gauges", func(t *testing.T) {
		data := performanceChunk(t, 32, 1<<31, make([]float32, 255), nil)
		requirePerformanceSuccess(t, data, 1030, &PerformanceInfoChunk{ChunkLength: 1026, NumFlags: 32, Flags: 1 << 31, Gauges: make([]float32, 255)})
	})
	t.Run("empty", func(t *testing.T) {
		requirePerformanceSuccess(t, performanceChunk(t, 0, 0, nil, nil), 10, &PerformanceInfoChunk{ChunkLength: 6, Gauges: []float32{}})
	})
	t.Run("future-fields", func(t *testing.T) {
		requirePerformanceSuccess(t, performanceChunk(t, 1, 1, nil, []byte{4, 5}), 12, &PerformanceInfoChunk{ChunkLength: 8, NumFlags: 1, Flags: 1, Gauges: []float32{}})
	})
	t.Run("future-packet-fields", func(t *testing.T) {
		data := append(append([]byte{}, valid...), 4, 5)
		requirePerformanceSuccess(t, data, 16, &PerformanceInfoChunk{ChunkLength: 10, NumFlags: 32, Flags: 1 << 31, Gauges: []float32{1.25}})
	})
	t.Run("truncated-packet-fields", func(t *testing.T) {
		m := clienttest.NewMockClient(t)
		m.On("Address").Return("127.0.0.1:8000").Once()
		q := newQueryer(PerformanceInfo, DefaultMaxPacketSize, m)
		result, err := q.readQuerySinglePacket(newPacketReader(bytes.NewReader(valid)), 2, PerformanceInfo, 16)
		require.ErrorIs(t, err, io.EOF)
		require.Nil(t, result)
	})
	t.Run("too-many-flags", func(t *testing.T) {
		requirePerformanceError(t, performanceChunk(t, 33, 0, nil, nil), 10)
	})
	t.Run("too-many-gauges", func(t *testing.T) {
		data := append([]byte{}, valid...)
		data[9] = 2
		requirePerformanceError(t, data, 14)
	})
	t.Run("short-chunk-header", func(t *testing.T) {
		requirePerformanceError(t, []byte{0, 0, 0, 5, 0, 0, 0, 0, 0}, 9)
	})
	t.Run("truncated-gauge", func(t *testing.T) {
		requirePerformanceError(t, valid[:len(valid)-1], 13)
	})
	t.Run("chunk-exceeds-packet", func(t *testing.T) {
		requirePerformanceError(t, valid, 13)
	})
	t.Run("truncated-future-fields", func(t *testing.T) {
		requirePerformanceError(t, performanceChunk(t, 0, 0, nil, []byte{1})[:10], 10)
	})
}

func newClient(requestedChunks byte) (*clienttest.MockClient, *queryer) {
	m := &clienttest.MockClient{}
	m.On("Address").Return("127.0.0.1:8000")
	c := newQueryer(requestedChunks, DefaultMaxPacketSize, m)

	return m, c
}

func TestQuery(t *testing.T) {
	cases := []struct {
		name   string
		chunks byte
		f      func(t *testing.T, challengeID uint32, c *queryer)
	}{
		{
			name:   "info_single",
			chunks: ServerInfo,
			f:      testQueryInfoSinglePacket,
		},
		{
			name:   "info_single_malformed",
			chunks: ServerInfo,
			f:      testQueryServerInfoSinglePacketMalformed,
		},
		{
			name:   "rules",
			chunks: ServerRules,
			f:      testQueryServerRulesSinglePacket,
		},
		{
			name:   "player",
			chunks: PlayerInfo,
			f:      testQueryPlayerInfoSinglePacket,
		},
		{
			name:   "team",
			chunks: TeamInfo,
			f:      testQueryTeamInfoSinglePacket,
		},
		{
			name:   "metrics",
			chunks: Metrics,
			f:      testQueryMetricsSinglePacket,
		},
	}

	buf := &bytes.Buffer{}
	chalReq := clienttest.LoadData(t, testDir, "challenge_success_request")

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cid := uint32(i + 1)
			req := clienttest.LoadData(t, testDir, tc.name+"_request")

			m, c := newClient(tc.chunks)
			// Challenge
			buf.Reset()
			buf.WriteByte(ChallengeResponseType)
			require.NoError(t, binary.Write(buf, binary.BigEndian, cid))
			chalResp := buf.Bytes()
			m.On("Write", chalReq).Return(len(chalReq), nil).Once()
			m.On("Read", mock.AnythingOfType("[]uint8")).Return(chalResp, nil).Once()
			// Request
			testSetChallenge(req, chalResp)
			m.On("Write", req).Return(len(req), nil).Once()

			resp := clienttest.LoadData(t, testDir, tc.name+"_response")
			testSetChallenge(resp, chalResp)
			m.On("Read", mock.AnythingOfType("[]uint8")).Return(resp, nil).Once()

			tc.f(t, cid, c)
		})
	}
}

func testSetChallenge(dest, src []byte) {
	copy(dest[1:5], src[1:5])
}

func testQueryInfoSinglePacket(t *testing.T, challengeID uint32, c *queryer) {
	r, err := c.Query()
	require.NoError(t, err, "query request failed")

	qr := r.(*QueryResponse)
	require.Equal(t, challengeID, c.challengeID, "expected correct challenge id")

	require.NotNil(t, qr, "expected query response")
	require.NotNil(t, qr.ServerInfo, "expected server info")

	require.Equal(t, uint16(5), qr.ServerInfo.CurrentPlayers)
	require.Equal(t, uint16(10), qr.ServerInfo.MaxPlayers)
	require.Equal(t, "my server", qr.ServerInfo.ServerName)
	require.Equal(t, "ctf", qr.ServerInfo.GameType)
	require.Equal(t, "1", qr.ServerInfo.BuildID)
	require.Equal(t, "map", qr.ServerInfo.Map)
	require.Equal(t, uint16(1025), qr.ServerInfo.Port)
}

func testQueryServerInfoSinglePacketMalformed(t *testing.T, challengeID uint32, c *queryer) {
	_, err := c.Query()
	require.Error(t, err, "query request should have failed")

	_, ok := err.(ErrMalformedPacket)
	require.Truef(t, ok, "expected malformed packet err, got: %v", err)
}

func testQueryServerInfoMultiPacket(t *testing.T, challengeID uint32, c *queryer) {
	r, err := c.Query()
	require.NoError(t, err, "query request should not have failed")
	qr := r.(*QueryResponse)

	require.Equal(t, challengeID, c.challengeID, "expected correct challenge id")

	require.NotNil(t, qr, "expected query response")
	require.NotNil(t, qr.ServerInfo, "expected server info")

	require.Equal(t, uint16(5), qr.ServerInfo.CurrentPlayers)
	require.Equal(t, uint16(10), qr.ServerInfo.MaxPlayers)
	require.Equal(t, "my server", qr.ServerInfo.ServerName)
	require.Equal(t, "ctf", qr.ServerInfo.GameType)
	require.Equal(t, "1", qr.ServerInfo.BuildID)
	require.Equal(t, "map", qr.ServerInfo.Map)
	require.Equal(t, uint16(1025), qr.ServerInfo.Port)
}

func testQueryServerRulesSinglePacket(t *testing.T, challengeID uint32, c *queryer) {
	r, err := c.Query()
	require.NoError(t, err, "query request should not have failed")
	qr := r.(*QueryResponse)

	require.Equal(t, challengeID, c.challengeID, "expected correct challenge id")

	require.NotNil(t, qr, "expected query response")
	require.NotNil(t, qr.ServerRules, "expected server rules")
	require.Len(t, qr.ServerRules.Rules, 5)

	require.Equal(t, byte(128), qr.ServerRules.Rules["rule 1"].Byte())
	require.Equal(t, uint16(257), qr.ServerRules.Rules["rule 2"].Uint16())
	require.Equal(t, uint32(16777217), qr.ServerRules.Rules["rule 3"].Uint32())
	require.Equal(t, uint64(72057594037927937), qr.ServerRules.Rules["rule 4"].Uint64())
	require.Equal(t, "string", qr.ServerRules.Rules["rule 5"].String())
}

func testQueryPlayerInfoSinglePacket(t *testing.T, challengeID uint32, c *queryer) {
	r, err := c.Query()
	require.NoError(t, err, "query request should not have failed")
	qr := r.(*QueryResponse)

	require.Equal(t, challengeID, c.challengeID, "expected correct challenge id")

	require.NotNil(t, qr, "expected query response")
	require.NotNil(t, qr.PlayerInfo, "expected player info")
	require.Len(t, qr.PlayerInfo.Players, 2)

	require.Len(t, qr.PlayerInfo.Players[0], 5)
	require.Equal(t, byte(128), qr.PlayerInfo.Players[0]["field1"].Byte())
	require.Equal(t, uint16(257), qr.PlayerInfo.Players[0]["field2"].Uint16())
	require.Equal(t, uint32(16777217), qr.PlayerInfo.Players[0]["field3"].Uint32())
	require.Equal(t, uint64(72057594037927937), qr.PlayerInfo.Players[0]["field4"].Uint64())
	require.Equal(t, "string", qr.PlayerInfo.Players[0]["field5"].String())

	require.Len(t, qr.PlayerInfo.Players[1], 5)
	require.Equal(t, byte(129), qr.PlayerInfo.Players[1]["field1"].Byte())
	require.Equal(t, uint16(258), qr.PlayerInfo.Players[1]["field2"].Uint16())
	require.Equal(t, uint32(16777218), qr.PlayerInfo.Players[1]["field3"].Uint32())
	require.Equal(t, uint64(72057594037927938), qr.PlayerInfo.Players[1]["field4"].Uint64())
	require.Equal(t, "STRING", qr.PlayerInfo.Players[1]["field5"].String())
}

func testQueryTeamInfoSinglePacket(t *testing.T, challengeID uint32, c *queryer) {
	r, err := c.Query()
	require.NoError(t, err, "query request should not have failed")
	qr := r.(*QueryResponse)

	require.Equal(t, challengeID, c.challengeID, "expected correct challenge id")

	require.NotNil(t, qr, "expected query response")
	require.NotNil(t, qr.TeamInfo, "expected Team info")
	require.Len(t, qr.TeamInfo.Teams, 2)

	require.Len(t, qr.TeamInfo.Teams[0], 5)
	require.Equal(t, byte(128), qr.TeamInfo.Teams[0]["field1"].Byte())
	require.Equal(t, uint16(257), qr.TeamInfo.Teams[0]["field2"].Uint16())
	require.Equal(t, uint32(16777217), qr.TeamInfo.Teams[0]["field3"].Uint32())
	require.Equal(t, uint64(72057594037927937), qr.TeamInfo.Teams[0]["field4"].Uint64())
	require.Equal(t, "string", qr.TeamInfo.Teams[0]["field5"].String())

	require.Len(t, qr.TeamInfo.Teams[1], 5)
	require.Equal(t, byte(129), qr.TeamInfo.Teams[1]["field1"].Byte())
	require.Equal(t, uint16(258), qr.TeamInfo.Teams[1]["field2"].Uint16())
	require.Equal(t, uint32(16777218), qr.TeamInfo.Teams[1]["field3"].Uint32())
	require.Equal(t, uint64(72057594037927938), qr.TeamInfo.Teams[1]["field4"].Uint64())
	require.Equal(t, "STRING", qr.TeamInfo.Teams[1]["field5"].String())
}

func testQueryMetricsSinglePacket(t *testing.T, challengeID uint32, c *queryer) {
	r, err := c.Query()
	require.NoError(t, err, "query request should not have failed")
	qr := r.(*QueryResponse)

	require.Equal(t, challengeID, c.challengeID, "expected correct challenge id")

	require.NotNil(t, qr, "expected query response")
	require.NotNil(t, qr.Metrics, "expected metrics")

	require.Equal(t, byte(6), qr.Metrics.MetricCount)
	require.Len(t, qr.Metrics.Metrics, int(qr.Metrics.MetricCount))
	require.Equal(t, float32(1), qr.Metrics.Metrics[0])
	require.Equal(t, float32(0), qr.Metrics.Metrics[1])
	require.Equal(t, float32(3.14159), qr.Metrics.Metrics[2])
	require.Equal(t, float32(55.57), qr.Metrics.Metrics[3])
	require.Equal(t, float32(438.2522), qr.Metrics.Metrics[4])
	require.Equal(t, float32(-123.456), qr.Metrics.Metrics[5])
}
