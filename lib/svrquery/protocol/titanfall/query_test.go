package titanfall

import (
	"testing"

	"github.com/multiplay/go-svrquery/lib/svrquery/clienttest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testDir = "testdata"
	testKey = "AABBCCddeeffgghhkkllmmNN"
)

var (
	base = Info{
		Header: Header{
			Prefix:  -1,
			Command: 78,
			Version: 3,
		},
		InstanceInfo: InstanceInfo{
			Retail:         1,
			InstanceType:   2,
			ClientCRC:      4294967295,
			NetProtocol:    526,
			RandomServerID: 0,
		},
		BuildName:  "R5pc_r5launch_N895_CL450114_2019_10_03_04_00_PM",
		Datacenter: "west europe 2",
		GameMode:   "survival",
		BasicInfo: BasicInfo{
			Port:            37015,
			Platform:        "PC",
			PlaylistVersion: "",
			PlaylistNum:     307,
			PlaylistName:    "des_ranked",
			NumClients:      0,
			MaxClients:      60,
			Map:             "mp_rr_desertlands_64k_x_64k",
		},
		PerformanceInfo: PerformanceInfo{},
		MatchStateV6: MatchStateV6{
			MatchStateV2: MatchStateV2{
				Phase:            2,
				MaxRounds:        1,
				RoundsWonIMC:     0,
				RoundsWonMilitia: 0,
				TimeLimit:        1800,
				TimePassed:       0,
				MaxScore:         50,
			},
			TeamsLeftWithPlayersNum: 0,
		},
	}
)

func TestQuery(t *testing.T) {
	keyed := base
	keyed.Version = 5
	keyed.AverageFrameTime = 1.2347187
	keyed.MaxFrameTime = 1.583148
	keyed.AverageUserCommandTime = 0.9734314
	keyed.MaxUserCommandTime = 7.678111

	v7 := base
	v7.Version = 7
	v7.PlatformPlayers = map[string]byte{
		"ps3": 16,
		"pc":  6,
	}
	v7.PerformanceInfo = PerformanceInfo{
		AverageFrameTime:       1,
		MaxFrameTime:           2,
		AverageUserCommandTime: 3,
		MaxUserCommandTime:     4,
	}
	v7.MatchStateV6.TeamsLeftWithPlayersNum = 6

	v8 := v7
	v8.Version = 8
	v8.InstanceInfoV8 = InstanceInfoV8{
		Retail:         1,
		InstanceType:   2,
		ClientCRC:      4294967295,
		NetProtocol:    526,
		HealthFlags:    0,
		RandomServerID: 0,
	}

	v9 := v8
	v9.Version = 9
	v9.BasicInfo.NumBotClients = 3
	v9.BasicInfo.TotalClientsConnectedEver = 0
	v9.PerformanceInfoV9 = PerformanceInfoV9{
		PerformanceInfo: v8.PerformanceInfo,
		CommitMemory:    8472,
		ResidentMemory:  3901,
	}
	// Newer version of the match state are dramatically different to the older ones. So wipe with a new copy that
	// looks like what will be retained by the compatability code.
	v9.MatchStateV9 = MatchStateV9{
		Phase:                   3,
		TimePassed:              0,
		TeamsLeftWithPlayersNum: 0,
	}
	v9.MatchStateV6 = MatchStateV6{
		MatchStateV2: MatchStateV2{
			Phase:      3,
			TimePassed: 0,
		},
		TeamsLeftWithPlayersNum: 0,
	}

	v10 := v9
	v10.Version = 10
	v10.MatchStateV10 = MatchStateV10{
		MatchStateV9: MatchStateV9{
			Phase:                   3,
			TimePassed:              0,
			TeamsLeftWithPlayersNum: 0,
		},
		CurrentEntityPropertyCount: 2,
		MaxEntityPropertyCount:     5,
	}

	cases := []struct {
		name        string
		version     byte
		request     string
		response    string
		key         string
		expected    Info
		expEncypted bool
	}{
		{
			name:     "v3",
			version:  3,
			request:  "request-v3",
			response: "response-v3",
			expected: base,
		},
		{
			name:     "v7",
			version:  7,
			request:  "request-v7",
			response: "response-v7",
			expected: v7,
		},
		{
			name:        "v8",
			version:     8,
			request:     "request-v8",
			response:    "response-v8",
			expected:    v8,
			key:         testKey,
			expEncypted: true,
		},
		{
			name:        "v9",
			version:     9,
			request:     "request-v9",
			response:    "response-v9",
			expected:    v9,
			key:         testKey,
			expEncypted: true,
		},
		{
			name:        "v10",
			version:     10,
			request:     "request-v10",
			response:    "response-v10",
			expected:    v10,
			key:         testKey,
			expEncypted: true,
		},
		{
			name:     "keyed",
			version:  5,
			request:  "request-key",
			response: "response-key",
			key:      testKey,
			expected: keyed,
		},
		{
			name:     "keyed_upgrades_lower_version",
			version:  3,
			request:  "request-key",
			response: "response-key",
			key:      testKey,
			expected: keyed,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			mc := &clienttest.MockClient{}
			mc.On("Key").Return("Z2ZkZ3Nnbmpza2U0cnRyZQ==")
			p := queryer{
				c:       mc,
				version: tc.version,
			}

			req := clienttest.LoadData(t, testDir, tc.request)
			resp := clienttest.LoadData(t, testDir, tc.response)

			if tc.expEncypted {
				req, err = p.encrypt(req)
				require.NoError(t, err)
				resp, err = p.encrypt(resp)
				require.NoError(t, err)
			}

			mc.On("Write", mock.AnythingOfType("[]uint8")).Return(len(req), nil)
			mc.On("Read", mock.AnythingOfType("[]uint8")).Return(resp, nil)

			i, err := p.Query()
			require.NoError(t, err)
			require.IsType(t, &Info{}, i)
			require.Equal(t, &tc.expected, i)
			mc.AssertExpectations(t)
		})
	}
}

func TestEncryptAndDecrypt(t *testing.T) {
	mc := &clienttest.MockClient{}
	mc.On("Key").Return("Z2ZkZ3Nnbmpza2U0cnRyZQ==")
	p := queryer{
		c: mc,
	}

	text := `Line 1: Some test text to be encrypted and decrypted
Line 2: Some test text to be encrypted and decrypted
Line 3: Some test text to be encrypted and decrypted
Line 4: Some test text to be encrypted and decrypted
Line 5: Some test text to be encrypted and decrypted
Line 6: Some test text to be encrypted and decrypted
Line 7: Some test text to be encrypted and decrypted
Line 8: Some test text to be encrypted and decrypted`

	encoded, err := p.encrypt([]byte(text))
	require.NoError(t, err)

	decoded, err := p.decrypt(encoded)
	require.NoError(t, err)
	require.Equal(t, text, string(decoded))
}
