package sqp

import "github.com/multiplay/go-svrquery/lib/svrsample/common"

// Performance holds the SQP v2 performance chunk data.
type Performance struct {
	FlagCount  byte
	Flags      uint32
	GaugeCount byte
	Gauges     []float32
}

// Size returns the encoded chunk size.
func (p Performance) Size() uint32 {
	return uint32(6 + len(p.Gauges)*4)
}

// PerformanceFromQueryState converts sample state to an SQP v2 chunk.
func PerformanceFromQueryState(state common.QueryState) *Performance {
	p := &Performance{
		FlagCount:  byte(len(state.PerformanceFlags)),
		GaugeCount: byte(len(state.PerformanceGauges)),
		Gauges:     append([]float32(nil), state.PerformanceGauges...),
	}
	for i, flag := range state.PerformanceFlags {
		if flag {
			p.Flags |= 1 << i
		}
	}
	return p
}
