package tf2

import (
	"bytes"
	"runtime"

	"github.com/multiplay/go-svrquery/lib/svrsample/common"
)

type (
	QueryResponder struct {
		enc      *encoder
		extended bool
		state    common.QueryState
		version  int8
	}

	instanceInfo struct {
		Retail         byte
		InstanceType   byte
		ClientDLLCRC   uint32
		NetProtocol    uint16
		RandomServerID uint64
	}

	instanceInfoV8 struct {
		Retail         byte
		InstanceType   byte
		ClientDLLCRC   uint32
		NetProtocol    uint16
		HealthFlags    uint32
		RandomServerID uint32
	}

	performanceInfo struct {
		AverageFrameTime       float32
		MaxFrameTime           float32
		AverageUserCommandTime float32
		MaxUserCommandTime     float32
	}

	matchState struct {
		Phase             byte
		MaxRounds         byte
		MaxRoundsIMC      byte
		MaxRoundsMilitia  byte
		TimeLimitSeconds  uint16
		TimePassedSeconds uint16
		MaxScore          uint16
		Team              byte
	}

	queryWireFormat struct {
		Header                  int32
		ResponseType            byte
		Version                 int8
		InstanceInfo            *instanceInfo
		InstanceInfoV8          *instanceInfoV8
		BuildName               *string
		Datacenter              *string
		GameMode                *string
		Port                    uint16
		Platform                string
		PlaylistVersion         string
		PlaylistNum             uint32
		PlaylistName            string
		PlatformNum             *byte
		NumClients              uint8
		MaxClients              uint8
		Map                     string
		PerformanceInfo         *performanceInfo
		TeamsLeftWithPlayersNum *uint16
		MatchState              *matchState
		EOP                     uint64
	}
)

const (
	notApplicable = "n/a"
)

// NewQueryResponder returns creates a new responder capable of responding
// to tf2-formatted queries.
func NewQueryResponder(state common.QueryState, version int8, extended bool) (common.QueryResponder, error) {
	q := &QueryResponder{
		enc:      &encoder{},
		extended: extended,
		state:    state,
		version:  version,
	}
	return q, nil
}

// Respond writes a query response to the requester in the tf2 wire protocol.
func (q *QueryResponder) Respond(_ string, _ []byte) ([]byte, error) {
	resp := bytes.NewBuffer(nil)
	responseFlag := byte(80)
	dc := "multiplay-dc"

	if q.extended {
		responseFlag = byte(78)
	}

	f := queryWireFormat{
		Header:          -1,
		ResponseType:    responseFlag,
		Version:         q.version,
		Port:            q.state.Port,
		Platform:        runtime.GOOS,
		PlaylistVersion: notApplicable,
		PlaylistName:    notApplicable,
		NumClients:      uint8(q.state.CurrentPlayers),
		MaxClients:      uint8(q.state.MaxPlayers),
		Map:             q.state.Map,
	}

	if q.version > 1 {
		f.BuildName = &q.state.ServerName
		f.Datacenter = &dc
		f.GameMode = &q.state.GameType
	}

	if q.version > 2 {
		f.MatchState = &matchState{
			Team: 255, // Team information omitted
		}
	}

	// Performance Info
	if q.version > 4 {
		f.PerformanceInfo = &performanceInfo{
			AverageFrameTime:       1.2,
			MaxFrameTime:           3.4,
			AverageUserCommandTime: 5.6,
			MaxUserCommandTime:     7.8,
		}
	}

	if q.version > 5 {
		i := uint16(0)
		f.TeamsLeftWithPlayersNum = &i
	}

	if q.version > 6 {
		i := byte(0)
		f.PlatformNum = &i
	}

	if q.version > 7 {
		f.InstanceInfoV8 = &instanceInfoV8{
			HealthFlags: uint32(
				1<<0 | // Packet Loss In
					1<<1 | // Packet Loss Out
					1<<2 | // Packet Choked In
					1<<3 | // Packet Choked Out
					1<<4 | // Slow Server Frames
					1<<5 | // Hitching
					1<<6, // DOS
			),
			RandomServerID: 123456,
		}
	} else if q.version > 1 {
		f.InstanceInfo = &instanceInfo{RandomServerID: 123456}
	}

	if err := common.WireWrite(resp, q.enc, f); err != nil {
		return nil, err
	}

	return resp.Bytes(), nil
}
