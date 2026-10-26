package common

// QueryResponder represents an interface to a concrete type which responds
// to query requests.
type QueryResponder interface {
	Respond(clientAddress string, buf []byte) ([]byte, error)
}

// QueryState represents the state of a currently running game.
type QueryState struct {
	CurrentPlayers    int32
	MaxPlayers        int32
	ServerName        string
	GameType          string
	Map               string
	Port              uint16
	Metrics           []float32
	PerformanceFlags  []bool
	PerformanceGauges []float32
}
