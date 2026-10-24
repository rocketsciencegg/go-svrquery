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
	expected := append([]byte{0xff, 0xff, 0xff, 0xff, 0x50, 0x1, 0x90, 0x1f}, []byte(runtime.GOOS)...)
	expected = append(expected, []byte{0x0, 0x6e, 0x2f, 0x61, 0x0, 0x0, 0x0, 0x0, 0x0, 0x6e, 0x2f, 0x61, 0x0, 0x1, 0x2, 0x6d, 0x79, 0x20, 0x6d, 0x61, 0x70, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}...)
	require.Equal(t, expected, resp)
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
		InstanceInfo: titanfall.InstanceInfo{
			RandomServerID: 123456,
		},
		InstanceInfoV8: titanfall.InstanceInfoV8{
			HealthFlags:    127,
			RandomServerID: 123456,
		},
		BuildName:  "my server",
		Datacenter: "dc",
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

func TestMatchStateRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		proto   string
		version int8
	}{
		{"tf2e", 3},
		{"tf2e-v7", 7},
		{"tf2e-v8", 8},
	} {
		t.Run(tc.proto, func(t *testing.T) {
			teams := uint16(0x1234)
			f := queryWireFormat{
				Header: -1, ResponseType: titanfall.ServerInfoResponse, Version: tc.version,
				InstanceInfo: &instanceInfo{}, BuildName: new(string),
				Datacenter: new(string), GameMode: new(string),
				MatchState: &matchState{
					Phase: 2, MaxRounds: 3, MaxRoundsIMC: 4, MaxRoundsMilitia: 5,
					TimeLimitSeconds: 600, TimePassedSeconds: 123, MaxScore: 500,
					Team: 255,
				},
			}
			if tc.version > 5 {
				f.PerformanceInfo = &performanceInfo{}
				f.MatchState.TeamsLeftWithPlayersNum = &teams
			}
			if tc.version > 6 {
				f.PlatformNum = new(byte)
			}
			if tc.version > 7 {
				f.InstanceInfo = nil
				f.InstanceInfoV8 = &instanceInfoV8{}
			}
			buf := &bytes.Buffer{}
			require.NoError(t, common.WireWrite(buf, &encoder{}, f))
			p, err := protocol.Get(tc.proto)
			require.NoError(t, err)
			response, err := p(&queryerFromBytes{buf}).Query()
			require.NoError(t, err)
			info, ok := response.(*titanfall.Info)
			require.True(t, ok)
			expected := titanfall.MatchStateV6{
				MatchStateV2: titanfall.MatchStateV2{
					Phase: 2, MaxRounds: 3, RoundsWonIMC: 4, RoundsWonMilitia: 5,
					TimeLimit: 600, TimePassed: 123, MaxScore: 500,
				},
			}
			if tc.version > 5 {
				expected.TeamsLeftWithPlayersNum = teams
			}
			require.Equal(t, expected, info.MatchStateV6)
			require.Empty(t, info.Teams)
			require.Empty(t, info.Clients)
		})
	}
}
