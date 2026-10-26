package sqp

const (
	// TODO(steve): remove this?
	// DefaultMaxPacketSize is the default maximum size of a packet (MTU 1500 - UDP+IP header size)
	DefaultMaxPacketSize = 1472

	// Version1 is the SQP version with metrics data.
	Version1 = uint16(1)

	// Version2 is the SQP version with performance flags and gauges.
	Version2 = uint16(2)

	// Version is the default query protocol version.
	Version = Version1

	// MaxMetrics is the maximum number of metrics supported in a request.
	MaxMetrics = byte(25)
)
