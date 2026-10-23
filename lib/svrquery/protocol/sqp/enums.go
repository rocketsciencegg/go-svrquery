package sqp

//go:generate go run golang.org/x/tools/cmd/stringer@v0.52.0 -type=DataType -output=enums_string.go

// DataType indicates the type of a response field
type DataType byte

// Size returns the DataTypes size in bytes, or -1 if unknown
func (dt DataType) Size() int {
	switch dt {
	case Byte:
		return 1
	case Uint16:
		return 2
	case Uint32, Float32:
		return 4
	case Uint64:
		return 8
	default:
		return -1
	}
}

// Supported types for response fields
const (
	Byte DataType = iota
	Uint16
	Uint32
	Uint64
	String
	Float32
)

// Request Types
const (
	ChallengeRequestType byte = iota
	QueryRequestType
)

// Response Types
const (
	ChallengeResponseType byte = iota
	QueryResponseType
)

// Query Requested Chunks
const (
	ServerInfo byte = 1 << iota
	ServerRules
	PlayerInfo
	TeamInfo
	Metrics
)
