package messaging

import (
	"fmt"

	"github.com/redis/go-redis/v9"
)

type Config struct {
	Backend  string
	NATSURL  string
	Redis    *redis.Client
	Prefix   string
}

func NewMessageBus(cfg Config) (MessageBus, error) {
	switch cfg.Backend {
	case "nats":
		if cfg.NATSURL == "" {
			return nil, fmt.Errorf("nats url required")
		}
		return NewNATSBus(cfg.NATSURL)
	case "redis", "":
		if cfg.Redis == nil {
			return nil, fmt.Errorf("redis client required for redis stream backend")
		}
		return NewRedisStreamBus(cfg.Redis, cfg.Prefix), nil
	default:
		return nil, fmt.Errorf("unknown message bus backend: %s", cfg.Backend)
	}
}
