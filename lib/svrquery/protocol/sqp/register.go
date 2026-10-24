package sqp

import (
	"github.com/multiplay/go-svrquery/lib/svrquery/protocol"
)

func init() {
	protocol.MustRegister("sqp", newCreator(ServerInfo, Version))
	protocol.MustRegister("sqp-v2", newCreator(ServerInfo|Metrics, 2))
}
