package matchmaking

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
)

const defaultRedisTicketKey = "chess404:matchmaking:tickets"

type redisTicketStore struct {
	client *redis.Client
	key    string
}

func newRedisTicketStore(redisURL, key string) (*redisTicketStore, error) {
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(options)
	if err := client.Ping(context.Background()).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	if key == "" {
		key = defaultRedisTicketKey
	}
	return &redisTicketStore{
		client: client,
		key:    key,
	}, nil
}

func (s *redisTicketStore) backend() string {
	return "redis"
}

func (s *redisTicketStore) load() (map[string]Ticket, error) {
	values, err := s.client.HGetAll(context.Background(), s.key).Result()
	if err != nil {
		return nil, err
	}

	tickets := make(map[string]Ticket, len(values))
	for ticketID, raw := range values {
		var ticket Ticket
		if err := json.Unmarshal([]byte(raw), &ticket); err != nil {
			return nil, err
		}
		tickets[ticketID] = ticket
	}
	return tickets, nil
}

func (s *redisTicketStore) persist(tickets map[string]Ticket) error {
	ctx := context.Background()

	existing, err := s.client.HKeys(ctx, s.key).Result()
	if err != nil {
		return err
	}

	pipe := s.client.Pipeline()

	stale := make(map[string]struct{}, len(existing))
	for _, k := range existing {
		stale[k] = struct{}{}
	}

	for ticketID, ticket := range tickets {
		delete(stale, ticketID)
		encoded, err := json.Marshal(ticket)
		if err != nil {
			return err
		}
		pipe.HSet(ctx, s.key, ticketID, string(encoded))
	}

	for id := range stale {
		pipe.HDel(ctx, s.key, id)
	}

	_, err = pipe.Exec(ctx)
	return err
}

func (s *redisTicketStore) close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}
