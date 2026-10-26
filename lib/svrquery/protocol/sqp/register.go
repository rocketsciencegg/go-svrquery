package sqp

import (
	"github.com/multiplay/go-svrquery/lib/svrquery/protocol"
)

func init() {
	protocol.MustRegister("sqp", newCreator)
	protocol.MustRegister("sqp-v2", newCreatorV2)
}
