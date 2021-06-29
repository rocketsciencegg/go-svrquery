package tf2

import (
	"bytes"
	"runtime"
	"testing"

	"github.com/multiplay/go-svrquery/lib/svrquery/protocol"
	"github.com/multiplay/go-svrquery/lib/svrquery/protocol/titanfall"
	"github.com/multiplay/go-svrquery/lib/svrsample/common"
	"github.com/stretchr/testify/require"
)

type (
	// queryerFromBytes is a queryer which responds with data in the provided
	// byte buffer.
	queryerFromBytes struct {
		*bytes.Buffer
	}
)

func (q queryerFromBytes) Close() error {
	return nil
}

func (q queryerFromBytes) Key() string {
	return ""
}

func (q queryerFromBytes) Address() string {
	return ""
}

func Test_Respond(t *testing.T) {
	q, err := NewQueryResponder(common.QueryState{
		CurrentPlayers: 1,
		MaxPlayers:     2,
		ServerName:     "my server",
		GameType:       "game type",
		Map:            "my map",
		Port:           8080,
	}, 1, false)
	require.NoError(t, err)
	require.NotNil(t, q)

	resp, err := q.Respond("", nil)
	require.NoError(t, err)
	require.Equal(t, []byte{0xff, 0xff, 0xff, 0xff, 0x50, 0x1, 0x90, 0x1f, 0x64, 0x61, 0x72, 0x77, 0x69, 0x6e, 0x0, 0x6e, 0x2f, 0x61, 0x0, 0x0, 0x0, 0x0, 0x0, 0x6e, 0x2f, 0x61, 0x0, 0x1, 0x2, 0x6d, 0x79, 0x20, 0x6d, 0x61, 0x70, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}, resp)
}

func Test_Respond_tf2e_v8(t *testing.T) {
	q, err := NewQueryResponder(common.QueryState{
		CurrentPlayers: 1,
		MaxPlayers:     2,
		ServerName:     "my server",
		GameType:       "game type",
		Map:            "my map",
		Port:           8080,
	}, 8, true)
	require.NoError(t, err)
	require.NotNil(t, q)

	data, err := q.Respond("", nil)
	require.NoError(t, err)

	p, err := protocol.Get("tf2e-v8")
	require.NoError(t, err)

	client := p(&queryerFromBytes{
		bytes.NewBuffer(data),
	})
	resp, err := client.Query()
	require.NoError(t, err)
	require.Equal(t, &titanfall.Info{
		Header: titanfall.Header{
			Prefix:  -1,
			Command: 78,
			Version: 8,
		},
		InstanceInfoV8: titanfall.InstanceInfoV8{
			HealthFlags:    127,
			RandomServerID: 123456,
		},
		BuildName:  "my server",
		Datacenter: "multiplay-dc",
		GameMode:   "game type",
		BasicInfo: titanfall.BasicInfo{
			Port:            8080,
			Platform:        runtime.GOOS,
			PlaylistName:    "n/a",
			PlaylistVersion: "n/a",
			NumClients:      1,
			MaxClients:      2,
			Map:             "my map",
			PlatformPlayers: map[string]uint8{},
		},
		PerformanceInfo: titanfall.PerformanceInfo{
			AverageFrameTime:       1.2,
			MaxFrameTime:           3.4,
			AverageUserCommandTime: 5.6,
			MaxUserCommandTime:     7.8,
		},
	}, resp)
}
