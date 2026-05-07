package platform

import (
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresAccountStore struct {
	mu sync.Mutex
	db *sql.DB
}

func NewPostgresAccountStore(dsn string) (*PostgresAccountStore, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	return newPostgresAccountStoreWithDB(db)
}

func newPostgresAccountStoreWithDB(db *sql.DB) (*PostgresAccountStore, error) {
	store := &PostgresAccountStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *PostgresAccountStore) Backend() string {
	return "postgres"
}

func (s *PostgresAccountStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *PostgresAccountStore) Stats() AccountStoreStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	var stats AccountStoreStats
	_ = s.db.QueryRow(`select count(*) from accounts`).Scan(&stats.AccountCount)
	_ = s.db.QueryRow(`select count(*) from account_guest_links`).Scan(&stats.LinkedGuestCount)
	_ = s.db.QueryRow(`select count(*) from accounts where session_expires_at is not null and session_expires_at > $1`, time.Now().UTC()).Scan(&stats.ActiveSessionCount)
	return stats
}

func (s *PostgresAccountStore) ClaimGuest(guest GuestProfile, handle string) (AccountSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	tx, err := s.db.Begin()
	if err != nil {
		return AccountSession{}, err
	}
	defer func() { _ = tx.Rollback() }()

	if strings.TrimSpace(guest.GuestID) != "" {
		session, ok, err := lookupPostgresAccountSessionByGuestTx(tx, guest.GuestID)
		if err != nil {
			return AccountSession{}, err
		}
		if ok {
			normalizedHandle, err := normalizeAccountHandle(handle)
			if err == nil && normalizedHandle != "" && normalizedHandle != session.Account.Handle {
				return AccountSession{}, ErrAccountHandleTaken
			}
			session.Account.LastSeenAt = now
			session.SessionToken = firstNonEmpty(strings.TrimSpace(session.SessionToken), "accttok_"+randomToken(18))
			session.ExpiresAt = now.Add(defaultAccountSessionTTL)
			if err := updatePostgresAccountSessionTx(tx, session); err != nil {
				return AccountSession{}, err
			}
			if err := tx.Commit(); err != nil {
				return AccountSession{}, err
			}
			return session, nil
		}
	}

	normalizedHandle, err := normalizeAccountHandle(handle)
	if err != nil || normalizedHandle == "" {
		if err == nil {
			err = ErrInvalidAccountHandle
		}
		return AccountSession{}, err
	}
	existingAccountID, exists, err := lookupPostgresAccountIDByHandleTx(tx, normalizedHandle)
	if err != nil {
		return AccountSession{}, err
	}
	if exists && existingAccountID != "" {
		return AccountSession{}, ErrAccountHandleTaken
	}

	accountID := "acct_" + randomToken(8)
	account := AccountProfile{
		AccountID:      accountID,
		Handle:         normalizedHandle,
		PrimaryGuestID: guest.GuestID,
		LinkedGuestIDs: []string{guest.GuestID},
		CreatedAt:      now,
		LastSeenAt:     now,
	}
	session := AccountSession{
		Account:      account,
		SessionToken: "accttok_" + randomToken(18),
		ExpiresAt:    now.Add(defaultAccountSessionTTL),
	}
	if err := insertPostgresAccountTx(tx, session); err != nil {
		return AccountSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return AccountSession{}, err
	}
	return session, nil
}

func (s *PostgresAccountStore) ResumeAccount(accountID, sessionToken string) (AccountSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return AccountSession{}, os.ErrInvalid
	}

	session, ok, err := lookupPostgresAccountSessionByID(s.db, resolvedAccountID)
	if err != nil {
		return AccountSession{}, err
	}
	if !ok {
		return AccountSession{}, os.ErrNotExist
	}
	if !accountSessionTokenValid(AccountPrivateState{SessionToken: session.SessionToken, ExpiresAt: session.ExpiresAt}, sessionToken, time.Now().UTC()) {
		return AccountSession{}, ErrUnauthorizedAccountSession
	}

	now := time.Now().UTC()
	session.Account.LastSeenAt = now
	session.ExpiresAt = now.Add(defaultAccountSessionTTL)
	if _, err := s.db.Exec(`update accounts set last_seen_at = $1, session_token = $2, session_expires_at = $3 where account_id = $4`, now, session.SessionToken, session.ExpiresAt, resolvedAccountID); err != nil {
		return AccountSession{}, err
	}
	return session, nil
}

func (s *PostgresAccountStore) SyncGuestStats(guest GuestProfile) (AccountProfile, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok, err := lookupPostgresAccountSessionByGuest(s.db, strings.TrimSpace(guest.GuestID))
	if err != nil || !ok {
		return AccountProfile{}, ok, err
	}
	if accountHasDirectStats(session.Account) {
		return session.Account, true, nil
	}
	session.Account = seedAccountStatsFromGuestIfNeeded(session.Account, guest)
	if err := updatePostgresAccountSession(s.db, session); err != nil {
		return AccountProfile{}, false, err
	}
	return session.Account, true, nil
}

func (s *PostgresAccountStore) FinalizeMatch(matchID, whiteAccountID, blackAccountID, winner string) (AccountProfile, AccountProfile, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedMatchID := strings.TrimSpace(matchID)
	if resolvedMatchID == "" {
		return AccountProfile{}, AccountProfile{}, false, os.ErrInvalid
	}

	whiteSession, ok, err := lookupPostgresAccountSessionByID(s.db, strings.TrimSpace(whiteAccountID))
	if err != nil {
		return AccountProfile{}, AccountProfile{}, false, err
	}
	if !ok {
		return AccountProfile{}, AccountProfile{}, false, os.ErrNotExist
	}
	blackSession, ok, err := lookupPostgresAccountSessionByID(s.db, strings.TrimSpace(blackAccountID))
	if err != nil {
		return AccountProfile{}, AccountProfile{}, false, err
	}
	if !ok {
		return AccountProfile{}, AccountProfile{}, false, os.ErrNotExist
	}

	tx, err := s.db.Begin()
	if err != nil {
		return AccountProfile{}, AccountProfile{}, false, err
	}
	defer func() { _ = tx.Rollback() }()

	var existingWinner string
	err = tx.QueryRow(`select winner from account_finalized_matches where match_id = $1`, resolvedMatchID).Scan(&existingWinner)
	if err == nil {
		return whiteSession.Account, blackSession.Account, false, nil
	}
	if err != nil && err != sql.ErrNoRows {
		return AccountProfile{}, AccountProfile{}, false, err
	}

	if whiteSession.Account.Rating <= 0 {
		whiteSession.Account.Rating = 1200
	}
	if blackSession.Account.Rating <= 0 {
		blackSession.Account.Rating = 1200
	}
	whiteBefore := whiteSession.Account.Rating
	blackBefore := blackSession.Account.Rating

	now := time.Now().UTC()
	switch winner {
	case "white":
		whiteSession.Account.Rating += 16
		blackSession.Account.Rating = maxInt(100, blackSession.Account.Rating-16)
		whiteSession.Account.Wins++
		blackSession.Account.Losses++
	case "black":
		blackSession.Account.Rating += 16
		whiteSession.Account.Rating = maxInt(100, whiteSession.Account.Rating-16)
		blackSession.Account.Wins++
		whiteSession.Account.Losses++
	case "draw":
		whiteSession.Account.Draws++
		blackSession.Account.Draws++
	default:
		return AccountProfile{}, AccountProfile{}, false, os.ErrInvalid
	}
	whiteSession.Account.MatchesPlayed++
	blackSession.Account.MatchesPlayed++
	whiteSession.Account.LastSeenAt = now
	blackSession.Account.LastSeenAt = now
	whiteSession.Account.RatingHistory = appendAccountRatingHistory(
		whiteSession.Account.RatingHistory,
		buildAccountRatingHistoryEntry(resolvedMatchID, blackSession.Account.AccountID, winner, whiteBefore, whiteSession.Account.Rating, whiteSession.Account.MatchesPlayed, "white", now),
	)
	blackSession.Account.RatingHistory = appendAccountRatingHistory(
		blackSession.Account.RatingHistory,
		buildAccountRatingHistoryEntry(resolvedMatchID, whiteSession.Account.AccountID, winner, blackBefore, blackSession.Account.Rating, blackSession.Account.MatchesPlayed, "black", now),
	)
	whiteSession.ExpiresAt = now.Add(defaultAccountSessionTTL)
	blackSession.ExpiresAt = now.Add(defaultAccountSessionTTL)

	if err := updatePostgresAccountSessionTx(tx, whiteSession); err != nil {
		return AccountProfile{}, AccountProfile{}, false, err
	}
	if err := updatePostgresAccountSessionTx(tx, blackSession); err != nil {
		return AccountProfile{}, AccountProfile{}, false, err
	}
	if _, err := tx.Exec(`insert into account_finalized_matches(match_id, winner) values($1, $2)`, resolvedMatchID, winner); err != nil {
		return AccountProfile{}, AccountProfile{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return AccountProfile{}, AccountProfile{}, false, err
	}
	return whiteSession.Account, blackSession.Account, true, nil
}

func (s *PostgresAccountStore) GetAccount(accountID string) (AccountProfile, bool) {
	session, ok, err := lookupPostgresAccountSessionByID(s.db, strings.TrimSpace(accountID))
	if err != nil || !ok {
		return AccountProfile{}, false
	}
	return session.Account, true
}

func (s *PostgresAccountStore) GetAccountByGuest(guestID string) (AccountProfile, bool) {
	session, ok, err := lookupPostgresAccountSessionByGuest(s.db, strings.TrimSpace(guestID))
	if err != nil || !ok {
		return AccountProfile{}, false
	}
	return session.Account, true
}

func (s *PostgresAccountStore) ListAccounts(limit int) []AccountProfile {
	query := `select account_id, handle, primary_guest_id, linked_guest_ids, rating, matches_played, wins, losses, draws, rating_history, created_at, last_seen_at, session_token, session_expires_at from accounts order by last_seen_at desc, created_at desc, account_id asc`
	var rows *sql.Rows
	var err error
	if limit > 0 {
		rows, err = s.db.Query(query+` limit $1`, limit)
	} else {
		rows, err = s.db.Query(query)
	}
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]AccountProfile, 0)
	for rows.Next() {
		session, ok, err := scanPostgresAccountSession(rows)
		if err != nil || !ok {
			return items
		}
		items = append(items, session.Account)
	}
	return items
}

func (s *PostgresAccountStore) init() error {
	_, err := s.db.Exec(`
		create table if not exists accounts (
			account_id text primary key,
			handle text not null unique,
			primary_guest_id text not null,
			linked_guest_ids jsonb not null,
			rating integer not null default 1200,
			matches_played integer not null default 0,
			wins integer not null default 0,
			losses integer not null default 0,
			draws integer not null default 0,
			rating_history jsonb not null default '[]'::jsonb,
			created_at timestamptz not null,
			last_seen_at timestamptz not null,
			session_token text,
			session_expires_at timestamptz
		);
		create table if not exists account_guest_links (
			guest_id text primary key,
			account_id text not null references accounts(account_id) on delete cascade
		);
		create table if not exists account_finalized_matches (
			match_id text primary key,
			winner text not null
		);
		alter table accounts add column if not exists rating integer not null default 1200;
		alter table accounts add column if not exists matches_played integer not null default 0;
		alter table accounts add column if not exists wins integer not null default 0;
		alter table accounts add column if not exists losses integer not null default 0;
		alter table accounts add column if not exists draws integer not null default 0;
		alter table accounts add column if not exists rating_history jsonb not null default '[]'::jsonb;
		create index if not exists accounts_last_seen_order_idx on accounts (last_seen_at desc, created_at desc, account_id asc);
	`)
	return err
}

func lookupPostgresAccountSessionByID(queryable interface {
	QueryRow(query string, args ...any) *sql.Row
}, accountID string) (AccountSession, bool, error) {
	return scanPostgresAccountSession(queryable.QueryRow(`select account_id, handle, primary_guest_id, linked_guest_ids, rating, matches_played, wins, losses, draws, rating_history, created_at, last_seen_at, session_token, session_expires_at from accounts where account_id = $1`, accountID))
}

func lookupPostgresAccountSessionByGuest(queryable interface {
	QueryRow(query string, args ...any) *sql.Row
}, guestID string) (AccountSession, bool, error) {
	return scanPostgresAccountSession(queryable.QueryRow(`select a.account_id, a.handle, a.primary_guest_id, a.linked_guest_ids, a.rating, a.matches_played, a.wins, a.losses, a.draws, a.rating_history, a.created_at, a.last_seen_at, a.session_token, a.session_expires_at from accounts a join account_guest_links l on l.account_id = a.account_id where l.guest_id = $1`, guestID))
}

func lookupPostgresAccountSessionByGuestTx(tx *sql.Tx, guestID string) (AccountSession, bool, error) {
	return lookupPostgresAccountSessionByGuest(tx, guestID)
}

func lookupPostgresAccountIDByHandleTx(tx *sql.Tx, handle string) (string, bool, error) {
	var accountID string
	err := tx.QueryRow(`select account_id from accounts where handle = $1`, handle).Scan(&accountID)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return accountID, true, nil
}

func insertPostgresAccountTx(tx *sql.Tx, session AccountSession) error {
	linkedGuestIDs, err := json.Marshal(session.Account.LinkedGuestIDs)
	if err != nil {
		return err
	}
	ratingHistory, err := json.Marshal(session.Account.RatingHistory)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(
		`insert into accounts(account_id, handle, primary_guest_id, linked_guest_ids, rating, matches_played, wins, losses, draws, rating_history, created_at, last_seen_at, session_token, session_expires_at) values($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		session.Account.AccountID,
		session.Account.Handle,
		session.Account.PrimaryGuestID,
		linkedGuestIDs,
		session.Account.Rating,
		session.Account.MatchesPlayed,
		session.Account.Wins,
		session.Account.Losses,
		session.Account.Draws,
		ratingHistory,
		session.Account.CreatedAt,
		session.Account.LastSeenAt,
		strings.TrimSpace(session.SessionToken),
		session.ExpiresAt,
	); err != nil {
		return err
	}
	for _, guestID := range session.Account.LinkedGuestIDs {
		if _, err := tx.Exec(`insert into account_guest_links(guest_id, account_id) values($1, $2)`, strings.TrimSpace(guestID), session.Account.AccountID); err != nil {
			return err
		}
	}
	return nil
}

func updatePostgresAccountSessionTx(tx *sql.Tx, session AccountSession) error {
	linkedGuestIDs, err := json.Marshal(session.Account.LinkedGuestIDs)
	if err != nil {
		return err
	}
	ratingHistory, err := json.Marshal(session.Account.RatingHistory)
	if err != nil {
		return err
	}
	_, err = tx.Exec(
		`update accounts set handle = $1, primary_guest_id = $2, linked_guest_ids = $3, rating = $4, matches_played = $5, wins = $6, losses = $7, draws = $8, rating_history = $9, created_at = $10, last_seen_at = $11, session_token = $12, session_expires_at = $13 where account_id = $14`,
		session.Account.Handle,
		session.Account.PrimaryGuestID,
		linkedGuestIDs,
		session.Account.Rating,
		session.Account.MatchesPlayed,
		session.Account.Wins,
		session.Account.Losses,
		session.Account.Draws,
		ratingHistory,
		session.Account.CreatedAt,
		session.Account.LastSeenAt,
		strings.TrimSpace(session.SessionToken),
		session.ExpiresAt,
		session.Account.AccountID,
	)
	return err
}

func updatePostgresAccountSession(execable interface {
	Exec(query string, args ...any) (sql.Result, error)
}, session AccountSession) error {
	linkedGuestIDs, err := json.Marshal(session.Account.LinkedGuestIDs)
	if err != nil {
		return err
	}
	ratingHistory, err := json.Marshal(session.Account.RatingHistory)
	if err != nil {
		return err
	}
	_, err = execable.Exec(
		`update accounts set handle = $1, primary_guest_id = $2, linked_guest_ids = $3, rating = $4, matches_played = $5, wins = $6, losses = $7, draws = $8, rating_history = $9, created_at = $10, last_seen_at = $11, session_token = $12, session_expires_at = $13 where account_id = $14`,
		session.Account.Handle,
		session.Account.PrimaryGuestID,
		linkedGuestIDs,
		session.Account.Rating,
		session.Account.MatchesPlayed,
		session.Account.Wins,
		session.Account.Losses,
		session.Account.Draws,
		ratingHistory,
		session.Account.CreatedAt,
		session.Account.LastSeenAt,
		strings.TrimSpace(session.SessionToken),
		session.ExpiresAt,
		session.Account.AccountID,
	)
	return err
}

type postgresAccountScanner interface {
	Scan(dest ...any) error
}

func scanPostgresAccountSession(scanner postgresAccountScanner) (AccountSession, bool, error) {
	var (
		session        AccountSession
		linkedGuestIDs []byte
		ratingHistory  []byte
		sessionToken   sql.NullString
		sessionExpires sql.NullTime
	)
	err := scanner.Scan(
		&session.Account.AccountID,
		&session.Account.Handle,
		&session.Account.PrimaryGuestID,
		&linkedGuestIDs,
		&session.Account.Rating,
		&session.Account.MatchesPlayed,
		&session.Account.Wins,
		&session.Account.Losses,
		&session.Account.Draws,
		&ratingHistory,
		&session.Account.CreatedAt,
		&session.Account.LastSeenAt,
		&sessionToken,
		&sessionExpires,
	)
	if err == sql.ErrNoRows {
		return AccountSession{}, false, nil
	}
	if err != nil {
		return AccountSession{}, false, err
	}
	if len(linkedGuestIDs) > 0 {
		if err := json.Unmarshal(linkedGuestIDs, &session.Account.LinkedGuestIDs); err != nil {
			return AccountSession{}, false, err
		}
	}
	if len(ratingHistory) > 0 {
		if err := json.Unmarshal(ratingHistory, &session.Account.RatingHistory); err != nil {
			return AccountSession{}, false, err
		}
	}
	session.SessionToken = strings.TrimSpace(sessionToken.String)
	if sessionExpires.Valid {
		session.ExpiresAt = sessionExpires.Time.UTC()
	}
	return session, true, nil
}
