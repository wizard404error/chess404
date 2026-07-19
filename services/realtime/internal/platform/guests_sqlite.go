package platform

import (
	"crypto/subtle"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteGuestStore struct {
	mu sync.Mutex
	db *sql.DB
}

func (s *SQLiteGuestStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func NewSQLiteGuestStore(path string) (*SQLiteGuestStore, error) {
	if path != "" && path != ":memory:" && !strings.HasPrefix(path, "file:") {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	store := &SQLiteGuestStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteGuestStore) Backend() string {
	return "sqlite"
}

func (s *SQLiteGuestStore) EnsureGuest(guestID, sessionSecret string) (GuestSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	tx, err := s.db.Begin()
	if err != nil {
		return GuestSession{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if guestID != "" {
		session, ok, err := lookupGuestSessionTx(tx, guestID)
		if err != nil {
			return GuestSession{}, err
		}
		if ok {
			resolvedSecret := session.SessionSecret
			if resolvedSecret == "" {
				resolvedSecret = firstNonEmpty(sessionSecret, "guestsess_"+randomToken(12))
			} else if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(sessionSecret)), []byte(resolvedSecret)) != 1 {
				return GuestSession{}, ErrUnauthorizedGuestSession
			}
			session.Guest.LastSeenAt = now
			session.SessionSecret = resolvedSecret
			session.SessionToken = firstNonEmpty(strings.TrimSpace(session.SessionToken), "guesttok_"+randomToken(18))
			session.ExpiresAt = now.Add(defaultGuestSessionTTL)
			if _, err := tx.Exec(`update guests set last_seen_at = ?, session_secret = ?, session_token = ?, session_expires_at = ? where guest_id = ?`, timeString(now), resolvedSecret, session.SessionToken, timeString(session.ExpiresAt), guestID); err != nil {
				return GuestSession{}, err
			}
			if err := tx.Commit(); err != nil {
				return GuestSession{}, err
			}
			return session, nil
		}
	}

	if guestID == "" {
		guestID = "guest_" + randomToken(6)
	}
	if strings.TrimSpace(sessionSecret) == "" {
		sessionSecret = "guestsess_" + randomToken(12)
	}

	count, err := countGuestsTx(tx)
	if err != nil {
		return GuestSession{}, err
	}

	entry := GuestProfile{
		GuestID:     guestID,
		DisplayName: generateGuestName(count + 1),
		Rating:      1200,
		CreatedAt:   now,
		LastSeenAt:  now,
	}
	session := GuestSession{
		Guest:         entry,
		SessionSecret: strings.TrimSpace(sessionSecret),
		SessionToken:  "guesttok_" + randomToken(18),
		ExpiresAt:     now.Add(defaultGuestSessionTTL),
	}
	if err := insertGuestTx(tx, session); err != nil {
		return GuestSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return GuestSession{}, err
	}
	return session, nil
}

func (s *SQLiteGuestStore) IssueGuestSession(guestID string) (GuestSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	guestID = strings.TrimSpace(guestID)
	if guestID == "" {
		return GuestSession{}, os.ErrInvalid
	}

	session, ok, err := lookupGuestSessionDB(s.db, guestID)
	if err != nil {
		return GuestSession{}, err
	}
	if !ok {
		return GuestSession{}, os.ErrNotExist
	}

	now := time.Now().UTC()
	session.Guest.LastSeenAt = now
	session.SessionSecret = firstNonEmpty(strings.TrimSpace(session.SessionSecret), "guestsess_"+randomToken(12))
	session.SessionToken = firstNonEmpty(strings.TrimSpace(session.SessionToken), "guesttok_"+randomToken(18))
	session.ExpiresAt = now.Add(defaultGuestSessionTTL)
	if _, err := s.db.Exec(`update guests set last_seen_at = ?, session_secret = ?, session_token = ?, session_expires_at = ? where guest_id = ?`, timeString(now), session.SessionSecret, session.SessionToken, timeString(session.ExpiresAt), guestID); err != nil {
		return GuestSession{}, err
	}

	return session, nil
}

func (s *SQLiteGuestStore) ResumeGuest(guestID, sessionSecret string) (GuestSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	guestID = strings.TrimSpace(guestID)
	if guestID == "" {
		return GuestSession{}, os.ErrInvalid
	}

	session, ok, err := lookupGuestSessionDB(s.db, guestID)
	if err != nil {
		return GuestSession{}, err
	}
	if !ok {
		return GuestSession{}, os.ErrNotExist
	}
	if session.SessionSecret == "" {
		return GuestSession{}, ErrUnauthorizedGuestSession
	}
	if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(sessionSecret)), []byte(session.SessionSecret)) != 1 {
		return GuestSession{}, ErrUnauthorizedGuestSession
	}

	now := time.Now().UTC()
	session.Guest.LastSeenAt = now
	session.SessionToken = firstNonEmpty(strings.TrimSpace(session.SessionToken), "guesttok_"+randomToken(18))
	session.ExpiresAt = now.Add(defaultGuestSessionTTL)
	if _, err := s.db.Exec(`update guests set last_seen_at = ?, session_token = ?, session_expires_at = ? where guest_id = ?`, timeString(now), session.SessionToken, timeString(session.ExpiresAt), guestID); err != nil {
		return GuestSession{}, err
	}

	return session, nil
}

func (s *SQLiteGuestStore) ResumeGuestByToken(guestID, sessionToken string) (GuestSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	guestID = strings.TrimSpace(guestID)
	if guestID == "" {
		return GuestSession{}, os.ErrInvalid
	}

	session, ok, err := lookupGuestSessionDB(s.db, guestID)
	if err != nil {
		return GuestSession{}, err
	}
	if !ok {
		return GuestSession{}, os.ErrNotExist
	}
	now := time.Now().UTC()
	if !guestSessionTokenMatches(session, sessionToken, now) {
		return GuestSession{}, ErrUnauthorizedGuestSession
	}

	session.Guest.LastSeenAt = now
	session.ExpiresAt = now.Add(defaultGuestSessionTTL)
	if _, err := s.db.Exec(`update guests set last_seen_at = ?, session_token = ?, session_expires_at = ? where guest_id = ?`, timeString(now), session.SessionToken, timeString(session.ExpiresAt), guestID); err != nil {
		return GuestSession{}, err
	}

	return session, nil
}

func (s *SQLiteGuestStore) FinalizeMatch(matchID, whiteGuestID, blackGuestID, winner string) (GuestProfile, GuestProfile, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return GuestProfile{}, GuestProfile{}, false, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	white, ok, err := lookupGuestTx(tx, whiteGuestID)
	if err != nil {
		return GuestProfile{}, GuestProfile{}, false, err
	}
	if !ok {
		return GuestProfile{}, GuestProfile{}, false, os.ErrNotExist
	}

	black, ok, err := lookupGuestTx(tx, blackGuestID)
	if err != nil {
		return GuestProfile{}, GuestProfile{}, false, err
	}
	if !ok {
		return GuestProfile{}, GuestProfile{}, false, os.ErrNotExist
	}

	if matchID == "" {
		return GuestProfile{}, GuestProfile{}, false, os.ErrInvalid
	}

	var existingWinner string
	switch err := tx.QueryRow(`select winner from finalized_matches where match_id = ?`, matchID).Scan(&existingWinner); err {
	case nil:
		return white, black, false, nil
	case sql.ErrNoRows:
	default:
		return GuestProfile{}, GuestProfile{}, false, err
	}

	now := time.Now().UTC()
	newWhite, newBlack := ApplyEloMatchResult(white.Rating, black.Rating, winner)
	switch winner {
	case "white":
		white.Rating = newWhite
		black.Rating = newBlack
		white.Wins++
		black.Losses++
	case "black":
		white.Rating = newWhite
		black.Rating = newBlack
		black.Wins++
		white.Losses++
	case "draw":
		white.Rating = newWhite
		black.Rating = newBlack
		white.Draws++
		black.Draws++
	default:
		return GuestProfile{}, GuestProfile{}, false, os.ErrInvalid
	}

	white.MatchesPlayed++
	black.MatchesPlayed++
	white.LastSeenAt = now
	black.LastSeenAt = now

	if err := updateGuestTx(tx, white); err != nil {
		return GuestProfile{}, GuestProfile{}, false, err
	}
	if err := updateGuestTx(tx, black); err != nil {
		return GuestProfile{}, GuestProfile{}, false, err
	}
	if _, err := tx.Exec(`insert into finalized_matches(match_id, winner, finalized_at) values(?, ?, ?)`, matchID, winner, timeString(now)); err != nil {
		return GuestProfile{}, GuestProfile{}, false, err
	}

	if err := tx.Commit(); err != nil {
		return GuestProfile{}, GuestProfile{}, false, err
	}
	return white, black, true, nil
}

func (s *SQLiteGuestStore) ListGuests(limit int) []GuestProfile {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query(`select guest_id, display_name, rating, matches_played, wins, losses, draws, created_at, last_seen_at from guests order by rating desc, created_at asc, guest_id asc`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	items, err := scanGuestRows(rows)
	if err != nil {
		return nil
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func (s *SQLiteGuestStore) GetGuest(guestID string) (GuestProfile, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok, err := lookupGuestDB(s.db, guestID)
	if err != nil {
		return GuestProfile{}, false
	}
	return entry, ok
}

func (s *SQLiteGuestStore) ListRecentGuests(limit int) []GuestProfile {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query(`select guest_id, display_name, rating, matches_played, wins, losses, draws, created_at, last_seen_at from guests order by last_seen_at desc, guest_id asc`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	items, err := scanGuestRows(rows)
	if err != nil {
		return nil
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func (s *SQLiteGuestStore) Stats() GuestStoreStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	stats := GuestStoreStats{}
	_ = s.db.QueryRow(`select count(*) from guests`).Scan(&stats.GuestCount)
	_ = s.db.QueryRow(`select count(*) from finalized_matches`).Scan(&stats.FinalizedMatchCount)
	_ = s.db.QueryRow(`select count(*) from guests where matches_played > 0`).Scan(&stats.RankedPlayers)
	return stats
}

func (s *SQLiteGuestStore) init() error {
	_, _ = s.db.Exec(`PRAGMA journal_mode=WAL`)
	_, _ = s.db.Exec(`PRAGMA busy_timeout=5000`)
	_, err := s.db.Exec(`
		create table if not exists guests (
			guest_id text primary key,
			display_name text not null,
			rating integer not null,
			matches_played integer not null,
			wins integer not null,
			losses integer not null,
			draws integer not null,
			created_at text not null,
			last_seen_at text not null,
			session_secret text,
			session_token text,
			session_expires_at text
		);
		create table if not exists finalized_matches (
			match_id text primary key,
			winner text not null,
			finalized_at text not null
		);
	`)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`alter table guests add column session_secret text`); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
		return err
	}
	if _, err := s.db.Exec(`alter table guests add column session_token text`); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
		return err
	}
	if _, err := s.db.Exec(`alter table guests add column session_expires_at text`); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
		return err
	}
	return nil
}

func lookupGuestDB(db *sql.DB, guestID string) (GuestProfile, bool, error) {
	session, ok, err := lookupGuestSessionDB(db, guestID)
	return session.Guest, ok, err
}

func lookupGuestSessionDB(db *sql.DB, guestID string) (GuestSession, bool, error) {
	return lookupGuestSessionScanner(db.QueryRow(`select guest_id, display_name, rating, matches_played, wins, losses, draws, created_at, last_seen_at, session_secret, session_token, session_expires_at from guests where guest_id = ?`, guestID))
}

func lookupGuestTx(tx *sql.Tx, guestID string) (GuestProfile, bool, error) {
	session, ok, err := lookupGuestSessionTx(tx, guestID)
	return session.Guest, ok, err
}

func lookupGuestSessionTx(tx *sql.Tx, guestID string) (GuestSession, bool, error) {
	return lookupGuestSessionScanner(tx.QueryRow(`select guest_id, display_name, rating, matches_played, wins, losses, draws, created_at, last_seen_at, session_secret, session_token, session_expires_at from guests where guest_id = ?`, guestID))
}

type guestScanner interface {
	Scan(dest ...any) error
}

func lookupGuestSessionScanner(scanner guestScanner) (GuestSession, bool, error) {
	var (
		entry         GuestProfile
		createdAt     string
		lastSeenAt    string
		sessionSecret sql.NullString
		sessionToken  sql.NullString
		expiresAt     sql.NullString
	)
	err := scanner.Scan(
		&entry.GuestID,
		&entry.DisplayName,
		&entry.Rating,
		&entry.MatchesPlayed,
		&entry.Wins,
		&entry.Losses,
		&entry.Draws,
		&createdAt,
		&lastSeenAt,
		&sessionSecret,
		&sessionToken,
		&expiresAt,
	)
	if err == sql.ErrNoRows {
		return GuestSession{}, false, nil
	}
	if err != nil {
		return GuestSession{}, false, err
	}
	entry.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return GuestSession{}, false, err
	}
	entry.LastSeenAt, err = time.Parse(time.RFC3339Nano, lastSeenAt)
	if err != nil {
		return GuestSession{}, false, err
	}
	session := GuestSession{
		Guest:         entry,
		SessionSecret: strings.TrimSpace(sessionSecret.String),
		SessionToken:  strings.TrimSpace(sessionToken.String),
	}
	if strings.TrimSpace(expiresAt.String) != "" {
		session.ExpiresAt, err = time.Parse(time.RFC3339Nano, expiresAt.String)
		if err != nil {
			return GuestSession{}, false, err
		}
	}
	return session, true, nil
}

func countGuestsTx(tx *sql.Tx) (int, error) {
	var count int
	if err := tx.QueryRow(`select count(*) from guests`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func insertGuestTx(tx *sql.Tx, session GuestSession) error {
	_, err := tx.Exec(
		`insert into guests(guest_id, display_name, rating, matches_played, wins, losses, draws, created_at, last_seen_at, session_secret, session_token, session_expires_at) values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.Guest.GuestID,
		session.Guest.DisplayName,
		session.Guest.Rating,
		session.Guest.MatchesPlayed,
		session.Guest.Wins,
		session.Guest.Losses,
		session.Guest.Draws,
		timeString(session.Guest.CreatedAt),
		timeString(session.Guest.LastSeenAt),
		strings.TrimSpace(session.SessionSecret),
		strings.TrimSpace(session.SessionToken),
		timeString(session.ExpiresAt),
	)
	return err
}

func updateGuestTx(tx *sql.Tx, entry GuestProfile) error {
	_, err := tx.Exec(
		`update guests set display_name = ?, rating = ?, matches_played = ?, wins = ?, losses = ?, draws = ?, created_at = ?, last_seen_at = ? where guest_id = ?`,
		entry.DisplayName,
		entry.Rating,
		entry.MatchesPlayed,
		entry.Wins,
		entry.Losses,
		entry.Draws,
		timeString(entry.CreatedAt),
		timeString(entry.LastSeenAt),
		entry.GuestID,
	)
	return err
}

func scanGuestRows(rows *sql.Rows) ([]GuestProfile, error) {
	items := make([]GuestProfile, 0)
	for rows.Next() {
		var (
			entry      GuestProfile
			createdAt  string
			lastSeenAt string
		)
		if err := rows.Scan(
			&entry.GuestID,
			&entry.DisplayName,
			&entry.Rating,
			&entry.MatchesPlayed,
			&entry.Wins,
			&entry.Losses,
			&entry.Draws,
			&createdAt,
			&lastSeenAt,
		); err != nil {
			return nil, err
		}
		parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, err
		}
		parsedLastSeenAt, err := time.Parse(time.RFC3339Nano, lastSeenAt)
		if err != nil {
			return nil, err
		}
		entry.CreatedAt = parsedCreatedAt
		entry.LastSeenAt = parsedLastSeenAt
		items = append(items, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func timeString(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func guestSessionTokenMatches(session GuestSession, sessionToken string, now time.Time) bool {
	if strings.TrimSpace(session.SessionToken) == "" || strings.TrimSpace(sessionToken) == "" {
		return false
	}
	if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(sessionToken)), []byte(strings.TrimSpace(session.SessionToken))) != 1 {
		return false
	}
	if session.ExpiresAt.IsZero() || !session.ExpiresAt.After(now) {
		return false
	}
	return true
}
