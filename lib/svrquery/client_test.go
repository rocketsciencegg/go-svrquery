package svrquery

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	addr := "localhost:8000"

	t.Run("tf2e", func(t *testing.T) {
		c, err := NewClient("tf2e", addr, WithKey("test"), WithTimeout(time.Second))
		require.NoError(t, err)
		require.NotNil(t, c)
	})

	t.Run("invalid-protocol", func(t *testing.T) {
		c, err := NewClient("my-protocol", addr, WithKey("test"), WithTimeout(time.Second))
		require.Error(t, err)
		require.Nil(t, c)
	})
}

func TestQuery(t *testing.T) {
	addr := os.Getenv("TEST_QUERY_ADDR")
	if addr == "" {
		t.Skip("env TEST_QUERY_ADDR not set")
	}

	proto := os.Getenv("TEST_QUERY_PROTO")
	if proto == "" {
		t.Skip("env TEST_QUERY_PROTO not set")
	}

	c, err := NewClient(proto, addr)
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		r, err := c.Query()
		require.NoError(t, err)
		fmt.Printf("%#v\n", r)
	}
}
