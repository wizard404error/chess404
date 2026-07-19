package platform

type archivePersistence interface {
	backend() string

	// load reads every row into memory at startup. Must be called at most once.
	// Implementations that back a DB with denormalised columns may return an
	// empty map and rely on the query* methods instead; load-only callers serve
	// the fallback (file and SQLite) which have few rows.
	load() (map[string]MatchArchiveEntry, map[string]MatchArchivePrivateEntry, error)

	// persist writes a batch of dirty entries to the backend.
	persist(map[string]MatchArchiveEntry, map[string]MatchArchivePrivateEntry) error

	// queryGet returns a single entry. The bool is false when the match does
	// not exist. Implementations that do not support individual queries (file,
	// SQLite) may return false, false — the store will fall back to the
	// in-memory map populated by load().
	queryGet(matchID string) (MatchArchiveEntry, bool, error)
	queryPrivate(matchID string) (MatchArchivePrivateEntry, bool, error)

	// queryList returns entries ordered by updated_at desc, limited and
	// offset for pagination.
	queryList(limit, offset int) ([]MatchArchiveEntry, error)
	queryUnfinishedIDs(limit int) ([]string, error)
	queryFinishedIDs(limit int) ([]string, error)
	queryByGuest(guestID string, limit, offset int) ([]MatchArchiveEntry, error)

	// queryByAccount returns entries matching the accountID or any of the
	// linked guestIDs, ordered by updated_at desc.
	queryByAccount(accountID string, linkedGuestIDs []string, limit, offset int) ([]MatchArchiveEntry, error)

	// queryStats returns aggregate counts. Implementations that cannot compute
	// stats from the backend may return a zero-value — the store falls back to
	// scanning the in-memory map.
	queryStats() (MatchArchiveStats, error)

	close() error
}
