package svrsample

import (
	"errors"
	"fmt"

	"github.com/multiplay/go-svrquery/lib/svrsample/common"

	"github.com/multiplay/go-svrquery/lib/svrsample/protocol/sqp"
)

var (
	// ErrProtoNotSupported returned when a protocol is not supported
	ErrProtoNotSupported = errors.New("protocol not supported")
)

// GetResponder gets the appropriate responder for the protocol provided
func GetResponder(proto string, state common.QueryState) (common.QueryResponder, error) {
	switch proto {
	case "sqp", "sqp-v2":
		return sqp.NewQueryResponder(state)
	}
	return nil, fmt.Errorf("%w: %s", ErrProtoNotSupported, proto)
}
