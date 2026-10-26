package svrquery

import (
	"errors"
	"net"

	"github.com/multiplay/go-svrquery/lib/svrquery/protocol"
)

// BulkClient is a client which can be reused for sequential requests.
// It must not be used concurrently.
type BulkClient struct {
	client   *Client
	defaults Client
}

// NewBulkClient creates a reusable client with default options for each query.
func NewBulkClient(options ...Option) (*BulkClient, error) {
	c := &Client{
		network: DefaultNetwork,
		timeout: DefaultTimeout,
	}

	for _, o := range options {
		if err := o(c); err != nil {
			return nil, err
		}
	}

	return &BulkClient{client: c, defaults: *c}, nil
}

// Query runs a query against addr with proto and options, then closes its socket.
// Per-query options override constructor options only for the current query.
func (b *BulkClient) Query(proto, addr string, options ...Option) (resp protocol.Responser, err error) {
	*b.client = b.defaults
	b.client.protocol = proto
	b.client.addr = addr

	f, err := protocol.Get(proto)
	if err != nil {
		return nil, err
	}

	for _, o := range options {
		if err := o(b.client); err != nil {
			return nil, err
		}
	}

	b.client.Queryer = f(b.client)

	if b.client.ua, err = net.ResolveUDPAddr(b.client.network, addr); err != nil {
		return nil, err
	}

	if b.client.c, err = net.DialUDP(b.client.network, nil, b.client.ua); err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, b.client.c.Close())
		b.client.c = nil
	}()

	return b.client.Query()
}
