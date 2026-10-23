package titanfall

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHealthFlags(t *testing.T) {
	testCases := []struct {
		input               uint32
		expNone             bool
		expPacketLossIn     bool
		expPacketLossOut    bool
		expPacketChokedIn   bool
		expPacketChokedOut  bool
		expSlowServerFrames bool
		expHitching         bool
		expDOS              bool
		expRelay            bool
	}{
		{
			input:   0,
			expNone: true,
		},
		{
			input:           1 << 0,
			expPacketLossIn: true,
		},
		{
			input:            1 << 1,
			expPacketLossOut: true,
		},
		{
			input:             1 << 2,
			expPacketChokedIn: true,
		},
		{
			input:              1 << 3,
			expPacketChokedOut: true,
		},
		{
			input:               1 << 4,
			expSlowServerFrames: true,
		},
		{
			input:       1 << 5,
			expHitching: true,
		},
		{
			input:  1 << 6,
			expDOS: true,
		},
		{
			input:    1 << 7,
			expRelay: true,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("verify 0b%.32b", tc.input), func(t *testing.T) {
			hf := HealthFlags(tc.input)
			require.Equal(t, tc.expNone, hf.None())
			require.Equal(t, tc.expPacketLossIn, hf.PacketLossIn())
			require.Equal(t, tc.expPacketLossOut, hf.PacketLossOut())
			require.Equal(t, tc.expPacketChokedIn, hf.PacketChokedIn())
			require.Equal(t, tc.expPacketChokedOut, hf.PacketChokedOut())
			require.Equal(t, tc.expSlowServerFrames, hf.SlowServerFrames())
			require.Equal(t, tc.expHitching, hf.Hitching())
			require.Equal(t, tc.expDOS, hf.DOS())
			require.Equal(t, tc.expRelay, hf.Relay())
		})
	}
}
