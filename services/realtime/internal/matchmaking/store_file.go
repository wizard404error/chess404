package matchmaking

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type fileTicketStore struct {
	path string
}

func newFileTicketStore(path string) ticketStore {
	return &fileTicketStore{path: path}
}

func (s *fileTicketStore) backend() string {
	return "file"
}

func (s *fileTicketStore) load() (map[string]Ticket, error) {
	if s.path == "" {
		return map[string]Ticket{}, nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Ticket{}, nil
		}
		return nil, err
	}
	var tickets map[string]Ticket
	if err := json.Unmarshal(data, &tickets); err != nil {
		return nil, err
	}
	if tickets == nil {
		tickets = map[string]Ticket{}
	}
	return tickets, nil
}

func (s *fileTicketStore) persist(tickets map[string]Ticket) error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(tickets, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

func (s *fileTicketStore) close() error {
	return nil
}
