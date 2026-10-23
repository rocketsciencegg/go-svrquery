package titanfall

import (
	"github.com/multiplay/go-svrquery/lib/svrquery/protocol"
)

func init() {
	// TODO(steve): add support for tf2.
	protocol.MustRegister("tf2e", newQueryer(3))
	protocol.MustRegister("tf2e-v7", newQueryer(7))
	protocol.MustRegister("tf2e-v8", newQueryer(8))
	protocol.MustRegister("tf2e-v9", newQueryer(9))
	protocol.MustRegister("tf2e-v10", newQueryer(10))
}
