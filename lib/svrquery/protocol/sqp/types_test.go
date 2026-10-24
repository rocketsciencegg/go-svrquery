package sqp

import (
	"encoding/json"
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPerformanceInfoChunk_MarshalJSON(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		data, err := json.Marshal(PerformanceInfoChunk{})
		require.NoError(t, err)
		require.JSONEq(t, `{}`, string(data))
	})

	t.Run("flags-and-gauges", func(t *testing.T) {
		data, err := json.Marshal(PerformanceInfoChunk{NumFlags: 3, Flags: 5, Gauges: []float32{60, -1.5}})
		require.NoError(t, err)
		require.JSONEq(t, `{"flag_0":true,"flag_1":false,"flag_2":true,"gauge_0":60,"gauge_1":-1.5}`, string(data))
	})

	t.Run("high-bit", func(t *testing.T) {
		data, err := json.Marshal(PerformanceInfoChunk{NumFlags: 32, Flags: 1 << 31})
		require.NoError(t, err)
		var values map[string]bool
		err = json.Unmarshal(data, &values)
		require.NoError(t, err)
		expected := make(map[string]bool, 32)
		for i := 0; i < 32; i++ {
			expected["flag_"+strconv.Itoa(i)] = i == 31
		}
		require.Equal(t, expected, values)
	})

	t.Run("invalid-flags", func(t *testing.T) {
		_, err := json.Marshal(PerformanceInfoChunk{NumFlags: 33})
		require.ErrorContains(t, err, "performance flag count exceeds 32")
	})

	t.Run("nan", func(t *testing.T) {
		_, err := json.Marshal(PerformanceInfoChunk{Gauges: []float32{float32(math.NaN())}})
		require.ErrorContains(t, err, "unsupported value: NaN")
	})

	t.Run("infinity", func(t *testing.T) {
		_, err := json.Marshal(PerformanceInfoChunk{Gauges: []float32{float32(math.Inf(1))}})
		require.ErrorContains(t, err, "unsupported value: +Inf")
	})
}
