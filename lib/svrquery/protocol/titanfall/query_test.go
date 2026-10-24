package titanfall

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/multiplay/go-svrquery/lib/svrquery/clienttest"
	"github.com/multiplay/go-svrquery/lib/svrquery/common"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testDir           = "testdata"
	testKey           = "AABBCCddeeffgghhkkllmmNN"
	testEncryptionKey = "Z2ZkZ3Nnbmpza2U0cnRyZQ=="
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
			key:         testEncryptionKey,
			expEncypted: true,
		},
		{
			name:        "v9",
			version:     9,
			request:     "request-v9",
			response:    "response-v9",
			expected:    v9,
			key:         testEncryptionKey,
			expEncypted: true,
		},
		{
			name:        "v10",
			version:     10,
			request:     "request-v10",
			response:    "response-v10",
			expected:    v10,
			key:         testEncryptionKey,
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
			mc := &clienttest.MockClient{}
			mc.On("Key").Return(tc.key)
			p := queryer{
				c:       mc,
				version: tc.version,
			}

			req := clienttest.LoadData(t, testDir, tc.request)
			resp := clienttest.LoadData(t, testDir, tc.response)

			if tc.expEncypted {
				req[5] = tc.version
				copy(req[6:], tc.key)
				key, err := base64.StdEncoding.DecodeString(tc.key)
				require.NoError(t, err)
				block, err := aes.NewCipher(key)
				require.NoError(t, err)
				gcm, err := cipher.NewGCM(block)
				require.NoError(t, err)
				nonce := bytes.Repeat([]byte{0x42}, gcm.NonceSize())
				sealed := gcm.Seal(nil, nonce, resp, gcmAdditionalData)
				resp = append(nonce, sealed[len(sealed)-gcm.Overhead():]...)
				resp = append(resp, sealed[:len(sealed)-gcm.Overhead()]...)
				mc.On("Write", mock.MatchedBy(func(packet []byte) bool {
					if len(packet) != len(req)+gcm.NonceSize()+gcm.Overhead() {
						return false
					}
					ciphertext := append([]byte(nil), packet[gcm.NonceSize()+gcm.Overhead():]...)
					ciphertext = append(ciphertext, packet[gcm.NonceSize():gcm.NonceSize()+gcm.Overhead()]...)
					plain, err := gcm.Open(nil, packet[:gcm.NonceSize()], ciphertext, gcmAdditionalData)
					return err == nil && bytes.Equal(plain, req)
				})).Return(len(req)+gcm.NonceSize()+gcm.Overhead(), nil).Once()
			} else {
				mc.On("Write", req).Return(len(req), nil).Once()
			}

			mc.On("Read", mock.AnythingOfType("[]uint8")).Return(resp, nil)

			i, err := p.Query()
			require.NoError(t, err)
			require.IsType(t, &Info{}, i)
			require.Equal(t, &tc.expected, i)
			mc.AssertExpectations(t)
		})
	}
}

func TestDecryptTruncatedPackets(t *testing.T) {
	for size := 0; size < 28; size++ {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			mc := &clienttest.MockClient{}
			mc.On("Key").Return(testEncryptionKey)
			p := queryer{c: mc, version: 8}
			_, err := p.decrypt(make([]byte, size))
			require.Error(t, err)
			if size >= minLength {
				mc.On("Write", mock.AnythingOfType("[]uint8")).Return(packetSize+28, nil)
				mc.On("Read", mock.AnythingOfType("[]uint8")).Return(make([]byte, size), nil)
				_, err = p.Query()
				require.Error(t, err)
			}
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

func encodeClients(t *testing.T, version byte, clients []Client) []byte {
	t.Helper()
	var b bytes.Buffer
	write := func(v interface{}) {
		require.NoError(t, binary.Write(&b, binary.LittleEndian, v))
	}
	for _, c := range clients {
		write(c.ID)
		b.WriteString(c.Name)
		b.WriteByte(0)
		write(c.TeamID)
		if version > 3 {
			b.WriteString(c.Address)
			b.WriteByte(0)
			write(c.Ping)
			write(c.PacketsReceived)
			write(c.PacketsDropped)
		}
		if version > 2 {
			write(c.Score)
			write(c.Kills)
			write(c.Deaths)
		}
	}
	write(uint64(0))
	return b.Bytes()
}

func testClients(version byte) []Client {
	clients := []Client{
		{ID: 0x1122334455667788, Name: "Alice", TeamID: 1},
		{ID: 0x8877665544332211, Name: "Bob", TeamID: 2},
	}
	for j := range clients {
		if version > 3 {
			clients[j].Address = fmt.Sprintf("192.0.2.%d:37015", j+1)
			clients[j].Ping = uint32(20 + j)
			clients[j].PacketsReceived = uint32(100 + j)
			clients[j].PacketsDropped = uint32(2 + j)
		}
		if version > 2 {
			clients[j].Score = uint32(40 + j)
			clients[j].Kills = uint16(5 + j)
			clients[j].Deaths = uint16(3 + j)
		}
	}
	return clients
}

func TestClients(t *testing.T) {
	for _, version := range []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10} {
		for _, count := range []int{0, 1, 2} {
			t.Run(fmt.Sprintf("v%d/count%d", version, count), func(t *testing.T) {
				var expected []Client
				if count > 0 {
					expected = testClients(version)[:count]
				}
				data := encodeClients(t, version, expected)
				r := common.NewBinaryReader(data, binary.LittleEndian)
				info := &Info{Header: Header{Version: version}}
				err := (&queryer{}).clients(r, info)
				require.NoError(t, err)
				require.Equal(t, expected, info.Clients)
				var trailing byte
				require.Error(t, r.Read(&trailing))
			})
		}
	}
}

func TestClientsTruncated(t *testing.T) {
	for _, version := range []byte{1, 3, 4, 10} {
		data := encodeClients(t, version, testClients(version)[:1])
		for length := 0; length < len(data); length++ {
			t.Run(fmt.Sprintf("v%d/length%d", version, length), func(t *testing.T) {
				r := common.NewBinaryReader(data[:length], binary.LittleEndian)
				info := &Info{Header: Header{Version: version}}
				require.Error(t, (&queryer{}).clients(r, info))
				if length < len(data)-8 {
					require.Empty(t, info.Clients)
				}
			})
		}
	}
}

func TestQueryWithClients(t *testing.T) {
	expected := Info{
		Header: Header{Prefix: -1, Command: ServerInfoResponse, Version: 1},
		BasicInfo: BasicInfo{
			Port: 37015, Platform: "PC", PlaylistNum: 307,
			PlaylistName: "test", NumClients: 2, MaxClients: 60, Map: "test_map",
		},
		Clients: testClients(1),
	}
	var response bytes.Buffer
	write := func(v interface{}) {
		require.NoError(t, binary.Write(&response, binary.LittleEndian, v))
	}
	write(expected.Header)
	write(expected.Port)
	response.WriteString("PC\x00\x00")
	write(expected.PlaylistNum)
	response.WriteString("test\x00")
	write(expected.BasicInfo.NumClients)
	write(expected.BasicInfo.MaxClients)
	response.WriteString("test_map\x00")
	response.Write(encodeClients(t, 1, expected.Clients))

	mc := &clienttest.MockClient{}
	mc.On("Key").Return("")
	mc.On("Write", mock.AnythingOfType("[]uint8")).Return(packetSize, nil).Once()
	mc.On("Read", mock.AnythingOfType("[]uint8")).Return(response.Bytes(), nil).Once()
	q := queryer{c: mc, version: 1}
	info, err := q.Query()
	require.NoError(t, err)
	require.Equal(t, &expected, info)
	mc.AssertExpectations(t)
}
