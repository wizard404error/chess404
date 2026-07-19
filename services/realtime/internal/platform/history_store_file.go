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

// query* methods for the file backend are not implemented — the store falls
// back to the in-memory map populated by load().

func (s *fileArchiveStore) queryGet(_ string) (MatchArchiveEntry, bool, error) {
	return MatchArchiveEntry{}, false, nil
}

func (s *fileArchiveStore) queryPrivate(_ string) (MatchArchivePrivateEntry, bool, error) {
	return MatchArchivePrivateEntry{}, false, nil
}

func (s *fileArchiveStore) queryList(_, _ int) ([]MatchArchiveEntry, error) {
	return nil, nil
}

func (s *fileArchiveStore) queryUnfinishedIDs(_ int) ([]string, error) {
	return nil, nil
}

func (s *fileArchiveStore) queryFinishedIDs(_ int) ([]string, error) {
	return nil, nil
}

func (s *fileArchiveStore) queryByGuest(_ string, _, _ int) ([]MatchArchiveEntry, error) {
	return nil, nil
}

func (s *fileArchiveStore) queryByAccount(_ string, _ []string, _, _ int) ([]MatchArchiveEntry, error) {
	return nil, nil
}

func (s *fileArchiveStore) queryStats() (MatchArchiveStats, error) {
	return MatchArchiveStats{}, nil
}
