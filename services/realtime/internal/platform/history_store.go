package platform

type archivePersistence interface {
	backend() string
	load() (map[string]MatchArchiveEntry, map[string]MatchArchivePrivateEntry, error)
	persist(map[string]MatchArchiveEntry, map[string]MatchArchivePrivateEntry) error
	close() error
}
