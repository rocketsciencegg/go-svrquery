package svrquery

import (
	"net"

	"github.com/multiplay/go-svrquery/lib/svrquery/protocol"
)

// BulkClient is a client which can be reused with multiple requests.
type BulkClient struct {
	client *Client
}

// NewBulkClient creates a new client with no protocol or
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

	return &BulkClient{client: c}, nil
}

// Query runs a query against addr with proto and options.
func (b *BulkClient) Query(proto, addr string, options ...Option) (protocol.Responser, error) {
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

	return b.client.Query()
}
