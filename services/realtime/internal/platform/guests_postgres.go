package platform

import (
	"crypto/subtle"
	"database/sql"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresGuestStore struct {
	mu sync.Mutex
	db *sql.DB
}

func NewPostgresGuestStore(dsn string) (*PostgresGuestStore, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(3 * time.Minute)
	return newPostgresGuestStoreWithDB(db)
}

func newPostgresGuestStoreWithDB(db *sql.DB) (*PostgresGuestStore, error) {
	store := &PostgresGuestStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *PostgresGuestStore) Backend() string {
	return "postgres"
}

func (s *PostgresGuestStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *PostgresGuestStore) EnsureGuest(guestID, sessionSecret string) (GuestSession, error) {
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
		session, ok, err := lookupPostgresGuestSessionTx(tx, guestID)
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
			if _, err := tx.Exec(`update guests set last_seen_at = $1, session_secret = $2, session_token = $3, session_expires_at = $4 where guest_id = $5`, now, resolvedSecret, session.SessionToken, session.ExpiresAt, guestID); err != nil {
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

	count, err := countPostgresGuestsTx(tx)
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
	if err := insertPostgresGuestTx(tx, session); err != nil {
		return GuestSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return GuestSession{}, err
	}
	return session, nil
}

func (s *PostgresGuestStore) IssueGuestSession(guestID string) (GuestSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	guestID = strings.TrimSpace(guestID)
	if guestID == "" {
		return GuestSession{}, os.ErrInvalid
	}

	session, ok, err := lookupPostgresGuestSessionDB(s.db, guestID)
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
	if _, err := s.db.Exec(`update guests set last_seen_at = $1, session_secret = $2, session_token = $3, session_expires_at = $4 where guest_id = $5`, now, session.SessionSecret, session.SessionToken, session.ExpiresAt, guestID); err != nil {
		return GuestSession{}, err
	}

	return session, nil
}

func (s *PostgresGuestStore) ResumeGuest(guestID, sessionSecret string) (GuestSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	guestID = strings.TrimSpace(guestID)
	if guestID == "" {
		return GuestSession{}, os.ErrInvalid
	}

	session, ok, err := lookupPostgresGuestSessionDB(s.db, guestID)
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
	if _, err := s.db.Exec(`update guests set last_seen_at = $1, session_token = $2, session_expires_at = $3 where guest_id = $4`, now, session.SessionToken, session.ExpiresAt, guestID); err != nil {
		return GuestSession{}, err
	}

	return session, nil
}

func (s *PostgresGuestStore) ResumeGuestByToken(guestID, sessionToken string) (GuestSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	guestID = strings.TrimSpace(guestID)
	if guestID == "" {
		return GuestSession{}, os.ErrInvalid
	}

	session, ok, err := lookupPostgresGuestSessionDB(s.db, guestID)
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
	if _, err := s.db.Exec(`update guests set last_seen_at = $1, session_token = $2, session_expires_at = $3 where guest_id = $4`, now, session.SessionToken, session.ExpiresAt, guestID); err != nil {
		return GuestSession{}, err
	}

	return session, nil
}

func (s *PostgresGuestStore) FinalizeMatch(matchID, whiteGuestID, blackGuestID, winner string) (GuestProfile, GuestProfile, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return GuestProfile{}, GuestProfile{}, false, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	white, ok, err := lookupPostgresGuestTx(tx, whiteGuestID)
	if err != nil {
		return GuestProfile{}, GuestProfile{}, false, err
	}
	if !ok {
		return GuestProfile{}, GuestProfile{}, false, os.ErrNotExist
	}

	black, ok, err := lookupPostgresGuestTx(tx, blackGuestID)
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
	switch err := tx.QueryRow(`select winner from finalized_matches where match_id = $1`, matchID).Scan(&existingWinner); err {
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

	if err := updatePostgresGuestTx(tx, white); err != nil {
		return GuestProfile{}, GuestProfile{}, false, err
	}
	if err := updatePostgresGuestTx(tx, black); err != nil {
		return GuestProfile{}, GuestProfile{}, false, err
	}
	if _, err := tx.Exec(`insert into finalized_matches(match_id, winner, finalized_at) values($1, $2, $3)`, matchID, winner, now); err != nil {
		return GuestProfile{}, GuestProfile{}, false, err
	}

	if err := tx.Commit(); err != nil {
		return GuestProfile{}, GuestProfile{}, false, err
	}
	return white, black, true, nil
}

func (s *PostgresGuestStore) ListGuests(limit int) []GuestProfile {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `select guest_id, display_name, rating, matches_played, wins, losses, draws, created_at, last_seen_at from guests order by rating desc, created_at asc, guest_id asc`
	return queryPostgresGuests(s.db, query, limit)
}

func (s *PostgresGuestStore) GetGuest(guestID string) (GuestProfile, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok, err := lookupPostgresGuestDB(s.db, guestID)
	if err != nil {
		return GuestProfile{}, false
	}
	return entry, ok
}

func (s *PostgresGuestStore) ListRecentGuests(limit int) []GuestProfile {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `select guest_id, display_name, rating, matches_played, wins, losses, draws, created_at, last_seen_at from guests order by last_seen_at desc, guest_id asc`
	return queryPostgresGuests(s.db, query, limit)
}

func (s *PostgresGuestStore) Stats() GuestStoreStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	stats := GuestStoreStats{}
	_ = s.db.QueryRow(`select count(*) from guests`).Scan(&stats.GuestCount)
	_ = s.db.QueryRow(`select count(*) from finalized_matches`).Scan(&stats.FinalizedMatchCount)
	_ = s.db.QueryRow(`select count(*) from guests where matches_played > 0`).Scan(&stats.RankedPlayers)
	return stats
}

func (s *PostgresGuestStore) init() error {
	_, err := s.db.Exec(`
		create table if not exists guests (
			guest_id text primary key,
			display_name text not null,
			rating integer not null,
			matches_played integer not null,
			wins integer not null,
			losses integer not null,
			draws integer not null,
			created_at timestamptz not null,
			last_seen_at timestamptz not null,
			session_secret text,
			session_token text,
			session_expires_at timestamptz
		);
		create table if not exists finalized_matches (
			match_id text primary key,
			winner text not null,
			finalized_at timestamptz not null
		);
		create index if not exists guests_rating_order_idx on guests (rating desc, created_at asc, guest_id asc);
		create index if not exists guests_last_seen_order_idx on guests (last_seen_at desc, guest_id asc);
	`)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`alter table guests add column if not exists session_secret text`)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`alter table guests add column if not exists session_token text`)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`alter table guests add column if not exists session_expires_at timestamptz`)
	return err
}

func lookupPostgresGuestDB(db *sql.DB, guestID string) (GuestProfile, bool, error) {
	session, ok, err := lookupPostgresGuestSessionDB(db, guestID)
	return session.Guest, ok, err
}

func lookupPostgresGuestSessionDB(db *sql.DB, guestID string) (GuestSession, bool, error) {
	return scanPostgresGuestSession(db.QueryRow(`select guest_id, display_name, rating, matches_played, wins, losses, draws, created_at, last_seen_at, session_secret, session_token, session_expires_at from guests where guest_id = $1`, guestID))
}

func lookupPostgresGuestTx(tx *sql.Tx, guestID string) (GuestProfile, bool, error) {
	session, ok, err := lookupPostgresGuestSessionTx(tx, guestID)
	return session.Guest, ok, err
}

func lookupPostgresGuestSessionTx(tx *sql.Tx, guestID string) (GuestSession, bool, error) {
	return scanPostgresGuestSession(tx.QueryRow(`select guest_id, display_name, rating, matches_played, wins, losses, draws, created_at, last_seen_at, session_secret, session_token, session_expires_at from guests where guest_id = $1`, guestID))
}

type postgresGuestScanner interface {
	Scan(dest ...any) error
}

func scanPostgresGuestSession(scanner postgresGuestScanner) (GuestSession, bool, error) {
	var (
		entry         GuestProfile
		createdAt     time.Time
		lastSeenAt    time.Time
		sessionSecret sql.NullString
		sessionToken  sql.NullString
		expiresAt     sql.NullTime
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
	entry.CreatedAt = createdAt.UTC()
	entry.LastSeenAt = lastSeenAt.UTC()
	session := GuestSession{
		Guest:         entry,
		SessionSecret: strings.TrimSpace(sessionSecret.String),
		SessionToken:  strings.TrimSpace(sessionToken.String),
	}
	if expiresAt.Valid {
		session.ExpiresAt = expiresAt.Time.UTC()
	}
	return session, true, nil
}

func countPostgresGuestsTx(tx *sql.Tx) (int, error) {
	var count int
	if err := tx.QueryRow(`select count(*) from guests`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func insertPostgresGuestTx(tx *sql.Tx, session GuestSession) error {
	_, err := tx.Exec(
		`insert into guests(guest_id, display_name, rating, matches_played, wins, losses, draws, created_at, last_seen_at, session_secret, session_token, session_expires_at) values($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		session.Guest.GuestID,
		session.Guest.DisplayName,
		session.Guest.Rating,
		session.Guest.MatchesPlayed,
		session.Guest.Wins,
		session.Guest.Losses,
		session.Guest.Draws,
		session.Guest.CreatedAt,
		session.Guest.LastSeenAt,
		strings.TrimSpace(session.SessionSecret),
		strings.TrimSpace(session.SessionToken),
		session.ExpiresAt,
	)
	return err
}

func updatePostgresGuestTx(tx *sql.Tx, entry GuestProfile) error {
	_, err := tx.Exec(
		`update guests set display_name = $1, rating = $2, matches_played = $3, wins = $4, losses = $5, draws = $6, created_at = $7, last_seen_at = $8 where guest_id = $9`,
		entry.DisplayName,
		entry.Rating,
		entry.MatchesPlayed,
		entry.Wins,
		entry.Losses,
		entry.Draws,
		entry.CreatedAt,
		entry.LastSeenAt,
		entry.GuestID,
	)
	return err
}

func queryPostgresGuests(db *sql.DB, baseQuery string, limit int) []GuestProfile {
	query := baseQuery
	args := []any{}
	if limit > 0 {
		query += ` limit $1`
		args = append(args, limit)
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]GuestProfile, 0)
	for rows.Next() {
		var (
			entry      GuestProfile
			createdAt  time.Time
			lastSeenAt time.Time
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
			return nil
		}
		entry.CreatedAt = createdAt.UTC()
		entry.LastSeenAt = lastSeenAt.UTC()
		items = append(items, entry)
	}
	if err := rows.Err(); err != nil {
		return nil
	}
	return items
}
