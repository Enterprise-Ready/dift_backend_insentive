package nats

import (
	"time"

	"github.com/nats-io/nats.go"
)

type Client struct {
	Conn *nats.Conn
	Js   nats.JetStreamContext
}

func NewClient(url string) (*Client, error) {

	nc, err := nats.Connect(
		url,
		nats.ReconnectWait(2*time.Second),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		return nil, err
	}

	js, err := nc.JetStream()
	if err != nil {
		return nil, err
	}

	return &Client{
		Conn: nc,
		Js:   js,
	}, nil
}

func (c *Client) Close() {
	if c.Conn != nil {
		c.Conn.Drain()
		c.Conn.Close()
	}
}
