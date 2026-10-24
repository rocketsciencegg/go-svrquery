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

func TestQuery(t *testing.T) {
	setup := func(t *testing.T, proto string) string {
		t.Helper()
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
		t.Cleanup(func() { require.NoError(t, conn.Close()) })
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
		t.Cleanup(func() { require.NoError(t, <-done) })
		return conn.LocalAddr().String()
	}

	t.Run("sqp-v1", func(t *testing.T) {
		address := setup(t, "sqp")
		data := captureQueryOutput(t, "sqp", address)
		require.JSONEq(t, `{"version":1,"address":"`+address+`","server_info":{"current_players":1,"max_players":2,"server_name":"performance sample","game_type":"","build_id":"","map":"","port":0}}`, string(data))
	})

	t.Run("sqp-v2", func(t *testing.T) {
		address := setup(t, "sqp-v2")
		data := captureQueryOutput(t, "sqp-v2", address)
		require.JSONEq(t, `{"version":2,"address":"`+address+`","server_info":{"current_players":1,"max_players":2,"server_name":"performance sample","game_type":"","build_id":"","map":"","port":0},"performance_info":{"flag_0":true,"flag_1":false,"flag_2":true,"gauge_0":60,"gauge_1":-1.5}}`, string(data))
	})

	t.Run("unsupported-protocol", func(t *testing.T) {
		err := query("unsupported", "127.0.0.1:0", "")
		require.Error(t, err)
	})
}

func captureQueryOutput(t *testing.T, proto, address string) []byte {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	writerClosed := false
	t.Cleanup(func() {
		os.Stdout = old
		require.NoError(t, r.Close())
		if !writerClosed {
			require.NoError(t, w.Close())
		}
	})
	os.Stdout = w
	queryErr := query(proto, address, "")
	require.NoError(t, queryErr)
	os.Stdout = old
	require.NoError(t, w.Close())
	writerClosed = true
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	require.True(t, json.Valid(data), "stdout must contain only JSON")
	return data
}

func TestServer(t *testing.T) {
	err := server(log.New(io.Discard, "", 0), "unsupported", "127.0.0.1:0")
	require.ErrorIs(t, err, svrsample.ErrProtoNotSupported)
}
