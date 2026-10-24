package sqp

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/multiplay/go-svrquery/lib/svrsample/common"
	"github.com/stretchr/testify/require"
)

func TestSQPServer(t *testing.T) {
	testcases := []struct {
		name       string
		serverInfo bool
		metrics    bool
		expPayload []byte
	}{
		{
			name:       "empty",
			expPayload: []byte{},
		},
		{
			name:       "server_info_only",
			serverInfo: true,
			expPayload: []byte{
				0x0, 0x0, 0x0, 0xa, // chunk length
				0x0, 0x1, // current players
				0x0, 0x2, // max players
				0x0,      // server name length
				0x0,      // game type length
				0x0,      // build ID length
				0x0,      // map length
				0x0, 0x0, // port
			},
		},
		{
			name:    "metrics_only",
			metrics: true,
			expPayload: []byte{
				0x00, 0x00, 0x00, 0x19, // chunk length
				0x06,                   // metric count
				0x3f, 0x80, 0x00, 0x00, // metric 1
				0x00, 0x00, 0x00, 0x00, // metric 2
				0x40, 0x49, 0x0f, 0xd0, // metric 3
				0x42, 0x5e, 0x47, 0xae, // metric 4
				0x43, 0xdb, 0x20, 0x48, // metric 5
				0xc2, 0xf6, 0xe9, 0x79, // metric 6
			},
		},
		{
			name:       "server_info_and_metrics",
			serverInfo: true,
			metrics:    true,
			expPayload: []byte{
				// server info
				0x0, 0x0, 0x0, 0xa, // chunk length
				0x0, 0x1, // current players
				0x0, 0x2, // max players
				0x0,      // server name length
				0x0,      // game type length
				0x0,      // build ID length
				0x0,      // map length
				0x0, 0x0, // port

				// metrics
				0x00, 0x00, 0x00, 0x19, // chunk length
				0x06,                   // metric count
				0x3f, 0x80, 0x00, 0x00, // metric 1
				0x00, 0x00, 0x00, 0x00, // metric 2
				0x40, 0x49, 0x0f, 0xd0, // metric 3
				0x42, 0x5e, 0x47, 0xae, // metric 4
				0x43, 0xdb, 0x20, 0x48, // metric 5
				0xc2, 0xf6, 0xe9, 0x79, // metric 6
			},
		},
	}

	addr := "client-addr:65534"
	state := common.QueryState{
		CurrentPlayers: 1,
		MaxPlayers:     2,
		Metrics:        []float32{1, 0, 3.14159, 55.57, 438.2522, -123.456},
	}

	q, err := NewQueryResponder(state)
	require.NoError(t, err)
	require.NotNil(t, q)

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			// Challenge packet
			resp, err := q.Respond(addr, []byte{0, 0, 0, 0, 0})
			require.NoError(t, err)
			require.Equal(t, byte(0), resp[0])

			// Requested chunks
			var chunks byte
			if tc.serverInfo {
				chunks |= 0x1
			}
			if tc.metrics {
				chunks |= 0x10
			}

			// Query packet
			resp, err = q.Respond(
				addr,
				bytes.Join(
					[][]byte{
						{1},       // query request
						resp[1:5], // challenge
						{0, 1},    // SQP version
						{chunks},  // Request chunks
					},
					nil,
				),
			)
			require.NoError(t, err)

			// convert packet length to []byte
			pl := make([]byte, 2)
			binary.BigEndian.PutUint16(pl, uint16(len(tc.expPayload)))

			require.Equal(
				t,
				bytes.Join(
					[][]byte{
						{1},           // query response
						resp[1:5],     // challenge
						resp[5:7],     // SQP version
						{0},           // current packet
						{0},           // last packet
						pl,            // packet length
						tc.expPayload, // payload (chunks data)
					},
					nil,
				),
				resp,
			)
		})
	}
}

func TestShortPackets(t *testing.T) {
	for n := 0; n < 8; n++ {
		for _, header := range []byte{0, 1, 2} {
			q, err := NewQueryResponder(common.QueryState{})
			require.NoError(t, err)
			packet := make([]byte, n)
			if n > 0 {
				packet[0] = header
			}
			require.NotPanics(t, func() {
				_, err := q.Respond("client", packet)
				if header == 0 && n >= 5 {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
				}
			})
		}
	}
}

func TestPerformanceStateLimits(t *testing.T) {
	for _, tc := range []struct {
		name      string
		flags     int
		gauges    int
		wantError bool
	}{
		{"empty", 0, 0, false},
		{"maximum", 32, 255, false},
		{"flags overflow", 33, 0, true},
		{"gauges overflow", 0, 256, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q, err := NewQueryResponder(common.QueryState{
				PerformanceFlags:  make([]bool, tc.flags),
				PerformanceGauges: make([]float32, tc.gauges),
			})
			if tc.wantError {
				require.Error(t, err)
				require.Nil(t, q)
			} else {
				require.NoError(t, err)
				require.NotNil(t, q)
			}
		})
	}
}
