package svrquery

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/multiplay/go-svrquery/lib/svrquery/protocol"
	"github.com/stretchr/testify/require"
)

const bulkProbeProtocol = "bulk-client-test"

var errBulkProbe = errors.New("query failed")

func init() {
	protocol.MustRegister(bulkProbeProtocol, func(c protocol.Client) protocol.Queryer {
		return &bulkProbe{client: c.(*Client), key: c.Key(), address: c.Address()}
	})
}

type bulkProbe struct {
	client       *Client
	key, address string
}

type bulkProbeResponse struct {
	key, address, protocol string
	timeout                time.Duration
	conn                   *net.UDPConn
}

func (bulkProbeResponse) NumClients() int64 { return 1 }
func (bulkProbeResponse) MaxClients() int64 { return 2 }

func (q *bulkProbe) Query() (protocol.Responser, error) {
	r := bulkProbeResponse{
		key: q.key, address: q.address, protocol: q.client.Protocol(),
		timeout: q.client.timeout, conn: q.client.c,
	}
	if q.key == "query-error" {
		return r, errBulkProbe
	}
	return r, nil
}

func TestBulkClientQuery(t *testing.T) {
	listener, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })
	address := listener.LocalAddr().String()

	for _, defaults := range []struct {
		name string
		key  string
	}{
		{name: "empty defaults"},
		{name: "constructor defaults", key: "default-key"},
	} {
		t.Run(defaults.name, func(t *testing.T) {
			const timeout = 2 * time.Second
			b, err := NewBulkClient(WithKey(defaults.key), WithTimeout(timeout))
			require.NoError(t, err)

			for _, tc := range []struct {
				name    string
				options []Option
				key     string
				timeout time.Duration
				err     error
			}{
				{"override", []Option{WithKey("entry-key"), WithTimeout(time.Millisecond)}, "entry-key", time.Millisecond, nil},
				{"reset after success", nil, defaults.key, timeout, nil},
				{"query error", []Option{WithKey("query-error")}, "query-error", timeout, errBulkProbe},
				{"reset after query error", nil, defaults.key, timeout, nil},
			} {
				t.Run(tc.name, func(t *testing.T) {
					response, err := b.Query(bulkProbeProtocol, address, tc.options...)
					if tc.err != nil {
						require.ErrorIs(t, err, tc.err)
					} else {
						require.NoError(t, err)
					}
					r := response.(bulkProbeResponse)
					require.Equal(t, tc.key, r.key)
					require.Equal(t, address, r.address)
					require.Equal(t, bulkProbeProtocol, r.protocol)
					require.Equal(t, tc.timeout, r.timeout)
					_, err = r.conn.Write([]byte("closed"))
					require.ErrorIs(t, err, net.ErrClosed, "every query must release its socket")
				})
			}

			_, err = b.Query(bulkProbeProtocol, address, func(c *Client) error {
				c.key = "partial-option"
				return errBulkProbe
			})
			require.ErrorIs(t, err, errBulkProbe)
			_, err = b.Query("unsupported", address)
			require.Error(t, err)
			_, err = b.Query(bulkProbeProtocol, "127.0.0.1:invalid")
			require.Error(t, err)
			response, err := b.Query(bulkProbeProtocol, address)
			require.NoError(t, err)
			require.Equal(t, defaults.key, response.(bulkProbeResponse).key)
		})
	}
}

func TestNewBulkClientOptionError(t *testing.T) {
	b, err := NewBulkClient(func(*Client) error { return errBulkProbe })
	require.ErrorIs(t, err, errBulkProbe)
	require.Nil(t, b)
}
