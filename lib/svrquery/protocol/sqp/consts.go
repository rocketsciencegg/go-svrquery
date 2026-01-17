package sqp

const (
	// TODO(steve): remove this?
	// DefaultMaxPacketSize is the default maximum size of a packet (MTU 1500 - UDP+IP header size)
	DefaultMaxPacketSize = 1472

	// MaxMetrics is the maximum number of metrics supported in a request.
	MaxMetrics = byte(25)
)
