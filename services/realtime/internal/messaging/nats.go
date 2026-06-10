package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

type NATSBus struct {
	conn *nats.Conn
}

func NewNATSBus(url string) (*NATSBus, error) {
	nc, err := nats.Connect(url,
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			fmt.Printf("NATS disconnected: %v\n", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			fmt.Printf("NATS reconnected to %s\n", nc.ConnectedUrl())
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}
	return &NATSBus{conn: nc}, nil
}

func (b *NATSBus) Publish(ctx context.Context, topic string, event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	return b.conn.Publish(topic, data)
}

func (b *NATSBus) Subscribe(ctx context.Context, topic string, handler EventHandler) error {
	_, err := b.conn.Subscribe(topic, func(msg *nats.Msg) {
		var event Event
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			fmt.Printf("NATS unmarshal error: %v\n", err)
			return
		}
		if err := handler(ctx, event); err != nil {
			fmt.Printf("NATS handler error: %v\n", err)
		}
	})
	return err
}

func (b *NATSBus) Close() error {
	if b.conn != nil {
		b.conn.Close()
	}
	return nil
}
