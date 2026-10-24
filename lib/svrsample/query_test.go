package svrsample

import (
	"net"
	"testing"
	"time"

	"github.com/multiplay/go-svrquery/lib/svrquery"
	"github.com/multiplay/go-svrquery/lib/svrquery/protocol"
	"github.com/multiplay/go-svrquery/lib/svrquery/protocol/titanfall"
	"github.com/multiplay/go-svrquery/lib/svrsample/common"
	"github.com/stretchr/testify/require"
)

func TestResponderUDP(t *testing.T) {
	for _, proto := range []string{"sqp", "tf2e", "tf2e-v7", "tf2e-v8"} {
		t.Run(proto, func(t *testing.T) {
			state := common.QueryState{
				CurrentPlayers: 1, MaxPlayers: 2, ServerName: "sample",
				GameType: "mode", Map: "map", Port: 1000,
			}
			responder, err := GetResponder(proto, state)
			require.NoError(t, err)
			conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
			require.NoError(t, err)
			require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))
			done := make(chan error, 1)
			go func() {
				requests := 1
				if proto == "sqp" {
					requests = 2
				}
				for i := 0; i < requests; i++ {
					buf := make([]byte, 2048)
					n, addr, err := conn.ReadFromUDP(buf)
					if err != nil {
						done <- err
						return
					}
					data, err := responder.Respond(addr.String(), buf[:n])
					if err != nil {
						done <- err
						return
					}
					if _, err = conn.WriteToUDP(data, addr); err != nil {
						done <- err
						return
					}
				}
				done <- nil
			}()
			t.Cleanup(func() {
				require.NoError(t, conn.Close())
				require.NoError(t, <-done)
			})
			client, err := svrquery.NewClient(proto, conn.LocalAddr().String())
			require.NoError(t, err)
			defer client.Close()
			response, err := client.Query()
			require.NoError(t, err)
			require.EqualValues(t, state.CurrentPlayers, response.NumClients())
			require.EqualValues(t, state.MaxPlayers, response.MaxClients())
			mapper, ok := response.(protocol.Mapper)
			require.True(t, ok)
			require.Equal(t, state.Map, mapper.Map())
			if info, ok := response.(*titanfall.Info); ok {
				require.Equal(t, state.ServerName, info.BuildName)
				require.Equal(t, state.GameType, info.GameMode)
				require.Zero(t, info.TimeLimit)
				require.Empty(t, info.Teams)
				require.Empty(t, info.Clients)
			}
		})
	}
}
