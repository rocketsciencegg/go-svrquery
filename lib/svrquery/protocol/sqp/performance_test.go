package sqp

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"math"
	"os"
	"testing"

	"github.com/multiplay/go-svrquery/lib/svrquery/clienttest"
	"github.com/multiplay/go-svrquery/lib/svrquery/protocol"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func performanceChunk(t *testing.T, flags byte, bits uint32, gauges []float32, extra []byte) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	require.NoError(t, binary.Write(buf, binary.BigEndian, uint32(6+4*len(gauges)+len(extra))))
	buf.WriteByte(flags)
	require.NoError(t, binary.Write(buf, binary.BigEndian, bits))
	buf.WriteByte(byte(len(gauges)))
	require.NoError(t, binary.Write(buf, binary.BigEndian, gauges))
	buf.Write(extra)
	return buf.Bytes()
}

func TestQueryPerformanceProtocol(t *testing.T) {
	for _, tc := range []struct {
		name            string
		version         uint16
		responseVersion uint16
		chunks          byte
		wantError       bool
	}{
		{"sqp", 1, 1, ServerInfo, false},
		{"sqp-v2", 2, 2, ServerInfo | PerformanceInfo, false},
		{"sqp-v2", 2, 1, ServerInfo | PerformanceInfo, true},
	} {
		t.Run(tc.name+"/"+string(rune('0'+tc.responseVersion)), func(t *testing.T) {
			m := &clienttest.MockClient{}
			m.On("Address").Return("127.0.0.1:8000").Maybe()
			creator, err := protocol.Get(tc.name)
			require.NoError(t, err)
			q := creator(m)
			challenge := []byte{0, 0, 0, 0, 1}
			m.On("Write", []byte{0, 0, 0, 0, 0}).Return(5, nil).Once()
			m.On("Read", mock.Anything).Return(challenge, nil).Once()
			request := []byte{1, 0, 0, 0, 1, 0, byte(tc.version), tc.chunks}
			m.On("Write", request).Return(len(request), nil).Once()

			info := clienttest.LoadData(t, testDir, "info_single_response")[11:]
			payload := append([]byte{}, info...)
			if tc.responseVersion == 2 {
				payload = append(payload, performanceChunk(t, 3, 5, []float32{60, -1.5}, nil)...)
			}
			response := &bytes.Buffer{}
			response.Write([]byte{1, 0, 0, 0, 1})
			require.NoError(t, binary.Write(response, binary.BigEndian, tc.responseVersion))
			response.Write([]byte{0, 0})
			require.NoError(t, binary.Write(response, binary.BigEndian, uint16(len(payload))))
			response.Write(payload)
			m.On("Read", mock.Anything).Return(response.Bytes(), nil).Once()

			// Library queries must not corrupt the CLI's JSON stdout.
			old := os.Stdout
			r, w, err := os.Pipe()
			require.NoError(t, err)
			t.Cleanup(func() {
				os.Stdout = old
				r.Close()
				w.Close()
			})
			os.Stdout = w
			result, queryErr := q.Query()
			os.Stdout = old
			require.NoError(t, w.Close())
			output, err := io.ReadAll(r)
			require.NoError(t, err)
			require.Empty(t, output)
			m.AssertExpectations(t)
			if tc.wantError {
				require.Error(t, queryErr)
				return
			}
			require.NoError(t, queryErr)
			qr := result.(*QueryResponse)
			require.Equal(t, "my server", qr.ServerInfo.ServerName)
			require.Nil(t, qr.Metrics)
			data, err := json.Marshal(qr)
			require.NoError(t, err)
			if tc.version == 2 {
				require.Equal(t, []float32{60, -1.5}, qr.PerformanceInfo.Gauges)
				require.Contains(t, string(data), `"performance_info":{"flag_0":true,"flag_1":false,"flag_2":true,"gauge_0":60,"gauge_1":-1.5}`)
			} else {
				require.Nil(t, qr.PerformanceInfo)
				require.NotContains(t, string(data), "performance_info")
			}
		})
	}
}

func TestPerformanceChunkBoundaries(t *testing.T) {
	valid := performanceChunk(t, 32, 1<<31, []float32{1.25}, nil)
	tooManyGauges := append([]byte{}, valid...)
	tooManyGauges[9] = 2
	for _, tc := range []struct {
		name      string
		data      []byte
		packetLen uint32
		wantError bool
	}{
		{"valid", valid, uint32(len(valid)), false},
		{"maximum gauges", performanceChunk(t, 32, 1<<31, make([]float32, 255), nil), 1030, false},
		{"empty", performanceChunk(t, 0, 0, nil, nil), 10, false},
		{"future fields", performanceChunk(t, 1, 1, nil, []byte{4, 5}), 12, false},
		{"too many flags", performanceChunk(t, 33, 0, nil, nil), 10, true},
		{"too many gauges", tooManyGauges, uint32(len(tooManyGauges)), true},
		{"short chunk header", []byte{0, 0, 0, 5, 0, 0, 0, 0, 0}, 9, true},
		{"truncated gauge", valid[:len(valid)-1], uint32(len(valid) - 1), true},
		{"chunk exceeds packet", valid, uint32(len(valid) - 1), true},
		{"truncated future fields", performanceChunk(t, 0, 0, nil, []byte{1})[:10], 10, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, q := newClient(PerformanceInfo)
			stream := bytes.NewBuffer(append(append([]byte{}, tc.data...), 0xaa))
			qr, err := q.readQuerySinglePacket(newPacketReader(stream), 2, PerformanceInfo, tc.packetLen)
			if tc.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, qr.PerformanceInfo)
				require.Nil(t, qr.Metrics)
			}
			require.GreaterOrEqual(t, stream.Len(), 1, "must not consume the next packet")
		})
	}
}

func TestPerformanceJSON(t *testing.T) {
	for _, tc := range []struct {
		name      string
		info      PerformanceInfoChunk
		want      string
		wantError bool
	}{
		{"empty", PerformanceInfoChunk{}, `{}`, false},
		{"high bit", PerformanceInfoChunk{NumFlags: 32, Flags: 1 << 31}, "", false},
		{"invalid flags", PerformanceInfoChunk{NumFlags: 33}, "", true},
		{"nan", PerformanceInfoChunk{Gauges: []float32{float32(math.NaN())}}, "", true},
		{"infinity", PerformanceInfoChunk{Gauges: []float32{float32(math.Inf(1))}}, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.info)
			if tc.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tc.want != "" {
				require.JSONEq(t, tc.want, string(data))
			} else {
				var values map[string]bool
				require.NoError(t, json.Unmarshal(data, &values))
				require.Len(t, values, 32)
				require.True(t, values["flag_31"])
				require.False(t, values["flag_0"])
			}
		})
	}
}
