package svrsample

import (
	"errors"
	"fmt"

	"github.com/multiplay/go-svrquery/lib/svrsample/common"
	"github.com/multiplay/go-svrquery/lib/svrsample/protocol/sqp"
	"github.com/multiplay/go-svrquery/lib/svrsample/protocol/tf2"
)

var (
	// ErrProtoNotSupported returned when a protocol is not supported
	ErrProtoNotSupported = errors.New("protocol not supported")
)

// GetResponder gets the appropriate responder for the protocol provided
func GetResponder(proto string, state common.QueryState) (common.QueryResponder, error) {
	switch proto {
	case "sqp":
		return sqp.NewQueryResponder(state)
	case "tf2":
		return tf2.NewQueryResponder(state, 1, false)
	case "tf2e":
		return tf2.NewQueryResponder(state, 3, true)
	case "tf2e-v7":
		return tf2.NewQueryResponder(state, 7, true)
	case "tf2e-v8":
		return tf2.NewQueryResponder(state, 8, true)
	}
	return nil, fmt.Errorf("%w: %s", ErrProtoNotSupported, proto)
}
