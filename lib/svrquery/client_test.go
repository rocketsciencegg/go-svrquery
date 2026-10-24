package svrquery

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/multiplay/go-svrquery/lib/svrquery/protocol/sqp"
	"github.com/multiplay/go-svrquery/lib/svrsample"
	"github.com/multiplay/go-svrquery/lib/svrsample/common"
	"github.com/stretchr/testify/require"
)

func TestSQPQuery(t *testing.T) {
	for _, tc := range []struct {
		proto     string
		responder string
		version   uint16
		chunks    byte
	}{
		{"sqp", "sqp", sqp.Version, sqp.ServerInfo},
		{"sqp-v2", "sqp", sqp.Version2, sqp.ServerInfo | sqp.PerformanceInfo},
		{"sqp-v2", "sqp-v2", sqp.Version2, sqp.ServerInfo | sqp.PerformanceInfo},
	} {
		t.Run(tc.proto+"/"+tc.responder, func(t *testing.T) {
			state := common.QueryState{
				CurrentPlayers:    1,
				MaxPlayers:        2,
				ServerName:        "sample",
				Map:               "map",
				Metrics:           []float32{-1, 0, 1, 3.14159},
				PerformanceFlags:  []bool{true, false, true},
				PerformanceGauges: []float32{-1, 0, 1, 3.14159},
			}
			responder, err := svrsample.GetResponder(tc.responder, state)
			require.NoError(t, err)
			conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, conn.Close()) })
			require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))

			done := make(chan error, 1)
			requests := make(chan []byte, 2)
			go func() {
				done <- func() error {
					for i := 0; i < 2; i++ {
						buf := make([]byte, 1472)
						n, addr, err := conn.ReadFromUDP(buf)
						if err != nil {
							return err
						}
						requests <- buf[:n]
						response, err := responder.Respond(addr.String(), buf[:n])
						if err != nil {
							return err
						}
						if _, err := conn.WriteToUDP(response, addr); err != nil {
							return err
						}
					}
					return nil
				}()
			}()

			c, err := NewClient(tc.proto, conn.LocalAddr().String(), WithTimeout(time.Second))
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, c.Close()) })
			response, queryErr := c.Query()
			require.NoError(t, <-done, "sample responder failed")
			require.NoError(t, queryErr)

			require.Equal(t, []byte{0, 0, 0, 0, 0}, <-requests)
			request := <-requests
			require.Len(t, request, 8)
			require.Equal(t, byte(1), request[0])
			require.Equal(t, tc.version, binary.BigEndian.Uint16(request[5:7]))
			require.Equal(t, tc.chunks, request[7])

			qr, ok := response.(*sqp.QueryResponse)
			require.True(t, ok)
			require.Equal(t, tc.version, qr.Version)
			require.NotNil(t, qr.ServerInfo)
			require.Equal(t, state.ServerName, qr.ServerInfo.ServerName)
			require.Equal(t, int64(state.CurrentPlayers), qr.NumClients())
			require.Equal(t, int64(state.MaxPlayers), qr.MaxClients())
			require.Equal(t, state.Map, qr.Map())
			require.Nil(t, qr.Metrics)
			if tc.proto == "sqp-v2" {
				require.NotNil(t, qr.PerformanceInfo)
				require.Equal(t, byte(len(state.PerformanceFlags)), qr.PerformanceInfo.NumFlags)
				require.Equal(t, uint32(5), qr.PerformanceInfo.Flags)
				require.Equal(t, state.PerformanceGauges, qr.PerformanceInfo.Gauges)
			} else {
				require.Nil(t, qr.PerformanceInfo)
			}
		})
	}
}

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
