package platform

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type fileArchiveStore struct {
	path string
}

func newFileArchiveStore(path string) archivePersistence {
	return &fileArchiveStore{path: path}
}

func (s *fileArchiveStore) backend() string {
	return "file"
}

func (s *fileArchiveStore) load() (map[string]MatchArchiveEntry, map[string]MatchArchivePrivateEntry, error) {
	if s.path == "" {
		return map[string]MatchArchiveEntry{}, map[string]MatchArchivePrivateEntry{}, nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]MatchArchiveEntry{}, map[string]MatchArchivePrivateEntry{}, nil
		}
		return nil, nil, err
	}

	var wrapped matchArchiveFile
	if err := json.Unmarshal(data, &wrapped); err == nil && wrapped.Entries != nil {
		if wrapped.Private == nil {
			wrapped.Private = make(map[string]MatchArchivePrivateEntry)
		}
		return wrapped.Entries, wrapped.Private, nil
	}

	var entries map[string]MatchArchiveEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, nil, err
	}
	return entries, map[string]MatchArchivePrivateEntry{}, nil
}

func (s *fileArchiveStore) persist(entries map[string]MatchArchiveEntry, private map[string]MatchArchivePrivateEntry) error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(matchArchiveFile{
		Entries: entries,
		Private: private,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

func (s *fileArchiveStore) close() error {
	return nil
}
