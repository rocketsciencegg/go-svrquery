package sqp

import (
	"encoding/json"
	"strconv"
)

// ServerInfoChunk is the response chunk for server info data
type ServerInfoChunk struct {
	ChunkLength    uint32 `json:"-"`
	CurrentPlayers uint16 `json:"current_players"`
	MaxPlayers     uint16 `json:"max_players"`
	ServerName     string `json:"server_name"`
	GameType       string `json:"game_type"`
	BuildID        string `json:"build_id"`
	Map            string `json:"map"`
	Port           uint16 `json:"port"`
}

// ServerRulesChunk is the response chunk for server rules data
type ServerRulesChunk struct {
	ChunkLength uint32 `json:"-"`
	Rules       map[string]*DynamicValue
}

// MarshalJSON returns the JSON representation of the server rules
func (src *ServerRulesChunk) MarshalJSON() ([]byte, error) {
	return json.Marshal(src.Rules)
}

// PlayerInfoChunk is the response chunk for player data
type PlayerInfoChunk struct {
	ChunkLength uint32 `json:"-"`
	Players     []map[string]*DynamicValue
}

// MarshalJSON returns the JSON representation of the player info
func (pic *PlayerInfoChunk) MarshalJSON() ([]byte, error) {
	return json.Marshal(pic.Players)
}

// TeamInfoChunk is the response chunk for team data
type TeamInfoChunk struct {
	ChunkLength uint32 `json:"-"`
	Teams       []map[string]*DynamicValue
}

// MarshalJSON returns the JSON representation of the team info
func (tic *TeamInfoChunk) MarshalJSON() ([]byte, error) {
	return json.Marshal(tic.Teams)
}

// MetricsChunk is the response chunk for metrics data
type MetricsChunk struct {
	ChunkLength uint32 `json:"-"`
	MetricCount byte   `json:"-"`
	Metrics     []float32
}

// MarshalJSON returns the JSON representation of the metrics
func (mc *MetricsChunk) MarshalJSON() ([]byte, error) {
	return json.Marshal(mc.Metrics)
}

// PerformanceInfoChunk contains SQP v2 performance flags and gauges.
type PerformanceInfoChunk struct {
	ChunkLength uint32    `json:"-"`
	NumFlags    byte      `json:"-"`
	Flags       uint32    `json:"-"`
	Gauges      []float32 `json:"-"`
}

// MarshalJSON returns indexed performance flags and gauges.
func (pi PerformanceInfoChunk) MarshalJSON() ([]byte, error) {
	if pi.NumFlags > 32 {
		return nil, ErrMalformedPacket("performance flag count exceeds 32")
	}
	obj := make(map[string]interface{}, int(pi.NumFlags)+len(pi.Gauges))
	for i := 0; i < int(pi.NumFlags); i++ {
		obj["flag_"+strconv.Itoa(i)] = (pi.Flags>>i)&1 == 1
	}
	for i, value := range pi.Gauges {
		obj["gauge_"+strconv.Itoa(i)] = value
	}
	return json.Marshal(obj)
}

// QueryResponse is the combined response to a query request
type QueryResponse struct {
	Version         uint16                `json:"version"`
	Address         string                `json:"address"`
	ServerInfo      *ServerInfoChunk      `json:"server_info,omitempty"`
	ServerRules     *ServerRulesChunk     `json:"server_rules,omitempty"`
	PlayerInfo      *PlayerInfoChunk      `json:"player_info,omitempty"`
	TeamInfo        *TeamInfoChunk        `json:"team_info,omitempty"`
	Metrics         *MetricsChunk         `json:"metrics,omitempty"`
	PerformanceInfo *PerformanceInfoChunk `json:"performance_info,omitempty"`
}

// MaxClients returns the maximum number of clients.
func (q *QueryResponse) MaxClients() int64 {
	if q.ServerInfo == nil {
		// No server info chunk, use 0
		return 0
	}
	return int64(q.ServerInfo.MaxPlayers)
}

// NumClients returns the number of clients.
func (q *QueryResponse) NumClients() int64 {
	if q.ServerInfo == nil {
		// No server info chunk, use 0
		return 0
	}
	return int64(q.ServerInfo.CurrentPlayers)
}

// Map implements protocol.Mapper.
func (q *QueryResponse) Map() string {
	return q.ServerInfo.Map
}

type infoHeader struct {
	Name string
	Type DataType
}
