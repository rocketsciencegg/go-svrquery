package main

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"os"
	"testing"
	"time"

	"github.com/multiplay/go-svrquery/lib/svrsample"
	"github.com/multiplay/go-svrquery/lib/svrsample/common"
	"github.com/stretchr/testify/require"
)

func TestQuerySampleServer(t *testing.T) {
	for _, proto := range []string{"sqp", "sqp-v2"} {
		t.Run(proto, func(t *testing.T) {
			responder, err := svrsample.GetResponder(proto, common.QueryState{
				ServerName:        "performance sample",
				CurrentPlayers:    1,
				MaxPlayers:        2,
				PerformanceFlags:  []bool{true, false, true},
				PerformanceGauges: []float32{60, -1.5},
			})
			require.NoError(t, err)
			conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
			require.NoError(t, err)
			defer conn.Close()
			done := make(chan error, 1)
			go func() {
				for i := 0; i < 2; i++ {
					if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
						done <- err
						return
					}
					buf := make([]byte, 128)
					n, addr, err := conn.ReadFromUDP(buf)
					if err != nil {
						done <- err
						return
					}
					resp, err := responder.Respond(addr.String(), buf[:n])
					if err != nil {
						done <- err
						return
					}
					if _, err := conn.WriteToUDP(resp, addr); err != nil {
						done <- err
						return
					}
				}
				done <- nil
			}()
			old := os.Stdout
			r, w, err := os.Pipe()
			require.NoError(t, err)
			t.Cleanup(func() {
				os.Stdout = old
				r.Close()
				w.Close()
			})
			os.Stdout = w
			queryErr := query(proto, conn.LocalAddr().String(), "")
			os.Stdout = old
			require.NoError(t, w.Close())
			data, err := io.ReadAll(r)
			require.NoError(t, err)
			require.NoError(t, queryErr)
			require.NoError(t, <-done)
			var result struct {
				Version    uint16 `json:"version"`
				ServerInfo struct {
					Name string `json:"server_name"`
				} `json:"server_info"`
				Performance map[string]interface{} `json:"performance_info"`
			}
			require.NoError(t, json.Unmarshal(data, &result), "stdout must contain only JSON")
			require.Equal(t, "performance sample", result.ServerInfo.Name)
			if proto == "sqp-v2" {
				require.Equal(t, uint16(2), result.Version)
				require.Equal(t, map[string]interface{}{
					"flag_0": true, "flag_1": false, "flag_2": true,
					"gauge_0": float64(60), "gauge_1": -1.5,
				}, result.Performance)
			} else {
				require.Equal(t, uint16(1), result.Version)
				require.Nil(t, result.Performance)
			}
		})
	}
}

func TestServerUnsupportedProtocol(t *testing.T) {
	err := server(log.New(io.Discard, "", 0), "unsupported", "127.0.0.1:0")
	require.ErrorIs(t, err, svrsample.ErrProtoNotSupported)
}
