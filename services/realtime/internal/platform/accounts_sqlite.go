package platform

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteAccountStore struct {
	mu sync.Mutex
	db *sql.DB
}

func NewSQLiteAccountStore(path string) (*SQLiteAccountStore, error) {
	if path != "" && path != ":memory:" && !strings.HasPrefix(path, "file:") {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	store := &SQLiteAccountStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteAccountStore) Backend() string {
	return "sqlite"
}

func (s *SQLiteAccountStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteAccountStore) Stats() AccountStoreStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	var stats AccountStoreStats
	_ = s.db.QueryRow(`select count(*) from accounts`).Scan(&stats.AccountCount)
	_ = s.db.QueryRow(`select count(*) from account_guest_links`).Scan(&stats.LinkedGuestCount)
	_ = s.db.QueryRow(`select count(*) from accounts where session_expires_at is not null and session_expires_at > ?`, timeString(time.Now().UTC())).Scan(&stats.ActiveSessionCount)
	return stats
}

func (s *SQLiteAccountStore) ClaimGuest(guest GuestProfile, handle string) (AccountSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	tx, err := s.db.Begin()
	if err != nil {
		return AccountSession{}, err
	}
	defer func() { _ = tx.Rollback() }()

	if strings.TrimSpace(guest.GuestID) != "" {
		session, ok, err := lookupSQLiteAccountSessionByGuestTx(tx, guest.GuestID)
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
			if err := updateSQLiteAccountSessionTx(tx, session); err != nil {
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
	existingAccountID, exists, err := lookupSQLiteAccountIDByHandleTx(tx, normalizedHandle)
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
	if err := insertSQLiteAccountTx(tx, session); err != nil {
		return AccountSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return AccountSession{}, err
	}
	return session, nil
}

func (s *SQLiteAccountStore) ResumeAccount(accountID, sessionToken string) (AccountSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return AccountSession{}, os.ErrInvalid
	}

	session, ok, err := lookupSQLiteAccountSessionByID(s.db, resolvedAccountID)
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
	if _, err := s.db.Exec(`update accounts set last_seen_at = ?, session_token = ?, session_expires_at = ? where account_id = ?`, timeString(now), session.SessionToken, timeString(session.ExpiresAt), resolvedAccountID); err != nil {
		return AccountSession{}, err
	}
	return session, nil
}

func (s *SQLiteAccountStore) SyncGuestStats(guest GuestProfile) (AccountProfile, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok, err := lookupSQLiteAccountSessionByGuest(s.db, strings.TrimSpace(guest.GuestID))
	if err != nil || !ok {
		return AccountProfile{}, ok, err
	}
	if accountHasDirectStats(session.Account) {
		return session.Account, true, nil
	}
	session.Account = seedAccountStatsFromGuestIfNeeded(session.Account, guest)
	if err := updateSQLiteAccountSession(s.db, session); err != nil {
		return AccountProfile{}, false, err
	}
	return session.Account, true, nil
}

func (s *SQLiteAccountStore) FinalizeMatch(matchID, whiteAccountID, blackAccountID, winner string) (AccountProfile, AccountProfile, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedMatchID := strings.TrimSpace(matchID)
	if resolvedMatchID == "" {
		return AccountProfile{}, AccountProfile{}, false, os.ErrInvalid
	}

	whiteSession, ok, err := lookupSQLiteAccountSessionByID(s.db, strings.TrimSpace(whiteAccountID))
	if err != nil {
		return AccountProfile{}, AccountProfile{}, false, err
	}
	if !ok {
		return AccountProfile{}, AccountProfile{}, false, os.ErrNotExist
	}
	blackSession, ok, err := lookupSQLiteAccountSessionByID(s.db, strings.TrimSpace(blackAccountID))
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
	err = tx.QueryRow(`select winner from account_finalized_matches where match_id = ?`, resolvedMatchID).Scan(&existingWinner)
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

	if err := updateSQLiteAccountSessionTx(tx, whiteSession); err != nil {
		return AccountProfile{}, AccountProfile{}, false, err
	}
	if err := updateSQLiteAccountSessionTx(tx, blackSession); err != nil {
		return AccountProfile{}, AccountProfile{}, false, err
	}
	if _, err := tx.Exec(`insert into account_finalized_matches(match_id, winner) values(?, ?)`, resolvedMatchID, winner); err != nil {
		return AccountProfile{}, AccountProfile{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return AccountProfile{}, AccountProfile{}, false, err
	}
	return whiteSession.Account, blackSession.Account, true, nil
}

func (s *SQLiteAccountStore) GetAccount(accountID string) (AccountProfile, bool) {
	session, ok, err := lookupSQLiteAccountSessionByID(s.db, strings.TrimSpace(accountID))
	if err != nil || !ok {
		return AccountProfile{}, false
	}
	return session.Account, true
}

func (s *SQLiteAccountStore) GetAccountByGuest(guestID string) (AccountProfile, bool) {
	session, ok, err := lookupSQLiteAccountSessionByGuest(s.db, strings.TrimSpace(guestID))
	if err != nil || !ok {
		return AccountProfile{}, false
	}
	return session.Account, true
}

func (s *SQLiteAccountStore) ListAccounts(limit int) []AccountProfile {
	query := `select account_id, handle, primary_guest_id, linked_guest_ids, rating, matches_played, wins, losses, draws, rating_history, created_at, last_seen_at, session_token, session_expires_at from accounts order by last_seen_at desc, created_at desc, account_id asc`
	var rows *sql.Rows
	var err error
	if limit > 0 {
		rows, err = s.db.Query(query+` limit ?`, limit)
	} else {
		rows, err = s.db.Query(query)
	}
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]AccountProfile, 0)
	for rows.Next() {
		session, ok, err := scanSQLiteAccountSession(rows)
		if err != nil || !ok {
			return items
		}
		items = append(items, session.Account)
	}
	return items
}

func (s *SQLiteAccountStore) init() error {
	_, err := s.db.Exec(`
		create table if not exists accounts (
			account_id text primary key,
			handle text not null unique,
			primary_guest_id text not null,
			linked_guest_ids text not null,
			rating integer not null default 1200,
			matches_played integer not null default 0,
			wins integer not null default 0,
			losses integer not null default 0,
			draws integer not null default 0,
			rating_history text not null default '[]',
			created_at text not null,
			last_seen_at text not null,
			session_token text,
			session_expires_at text
		);
		create table if not exists account_guest_links (
			guest_id text primary key,
			account_id text not null references accounts(account_id) on delete cascade
		);
		create table if not exists account_finalized_matches (
			match_id text primary key,
			winner text not null
		);
		create index if not exists accounts_last_seen_order_idx on accounts (last_seen_at desc, created_at desc, account_id asc);
	`)
	if err != nil {
		return err
	}
	for _, column := range []struct {
		name string
		def  string
	}{
		{name: "rating", def: "integer not null default 1200"},
		{name: "matches_played", def: "integer not null default 0"},
		{name: "wins", def: "integer not null default 0"},
		{name: "losses", def: "integer not null default 0"},
		{name: "draws", def: "integer not null default 0"},
		{name: "rating_history", def: "text not null default '[]'"},
	} {
		if err := ensureSQLiteTableColumn(s.db, "accounts", column.name, column.def); err != nil {
			return err
		}
	}
	return nil
}

type sqliteAccountScanner interface {
	Scan(dest ...any) error
}

func lookupSQLiteAccountSessionByID(queryable interface {
	QueryRow(query string, args ...any) *sql.Row
}, accountID string) (AccountSession, bool, error) {
	return scanSQLiteAccountSession(queryable.QueryRow(`select account_id, handle, primary_guest_id, linked_guest_ids, rating, matches_played, wins, losses, draws, rating_history, created_at, last_seen_at, session_token, session_expires_at from accounts where account_id = ?`, accountID))
}

func lookupSQLiteAccountSessionByGuest(queryable interface {
	QueryRow(query string, args ...any) *sql.Row
}, guestID string) (AccountSession, bool, error) {
	return scanSQLiteAccountSession(queryable.QueryRow(`select a.account_id, a.handle, a.primary_guest_id, a.linked_guest_ids, a.rating, a.matches_played, a.wins, a.losses, a.draws, a.rating_history, a.created_at, a.last_seen_at, a.session_token, a.session_expires_at from accounts a join account_guest_links l on l.account_id = a.account_id where l.guest_id = ?`, guestID))
}

func lookupSQLiteAccountSessionByGuestTx(tx *sql.Tx, guestID string) (AccountSession, bool, error) {
	return lookupSQLiteAccountSessionByGuest(tx, guestID)
}

func lookupSQLiteAccountIDByHandleTx(tx *sql.Tx, handle string) (string, bool, error) {
	var accountID string
	err := tx.QueryRow(`select account_id from accounts where handle = ?`, handle).Scan(&accountID)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return accountID, true, nil
}

func insertSQLiteAccountTx(tx *sql.Tx, session AccountSession) error {
	linkedGuestIDs, err := json.Marshal(session.Account.LinkedGuestIDs)
	if err != nil {
		return err
	}
	ratingHistory, err := json.Marshal(session.Account.RatingHistory)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(
		`insert into accounts(account_id, handle, primary_guest_id, linked_guest_ids, rating, matches_played, wins, losses, draws, rating_history, created_at, last_seen_at, session_token, session_expires_at) values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.Account.AccountID,
		session.Account.Handle,
		session.Account.PrimaryGuestID,
		string(linkedGuestIDs),
		session.Account.Rating,
		session.Account.MatchesPlayed,
		session.Account.Wins,
		session.Account.Losses,
		session.Account.Draws,
		string(ratingHistory),
		timeString(session.Account.CreatedAt),
		timeString(session.Account.LastSeenAt),
		strings.TrimSpace(session.SessionToken),
		timeString(session.ExpiresAt),
	); err != nil {
		return err
	}
	for _, guestID := range session.Account.LinkedGuestIDs {
		if _, err := tx.Exec(`insert into account_guest_links(guest_id, account_id) values(?, ?)`, strings.TrimSpace(guestID), session.Account.AccountID); err != nil {
			return err
		}
	}
	return nil
}

func updateSQLiteAccountSessionTx(tx *sql.Tx, session AccountSession) error {
	linkedGuestIDs, err := json.Marshal(session.Account.LinkedGuestIDs)
	if err != nil {
		return err
	}
	ratingHistory, err := json.Marshal(session.Account.RatingHistory)
	if err != nil {
		return err
	}
	_, err = tx.Exec(
		`update accounts set handle = ?, primary_guest_id = ?, linked_guest_ids = ?, rating = ?, matches_played = ?, wins = ?, losses = ?, draws = ?, rating_history = ?, created_at = ?, last_seen_at = ?, session_token = ?, session_expires_at = ? where account_id = ?`,
		session.Account.Handle,
		session.Account.PrimaryGuestID,
		string(linkedGuestIDs),
		session.Account.Rating,
		session.Account.MatchesPlayed,
		session.Account.Wins,
		session.Account.Losses,
		session.Account.Draws,
		string(ratingHistory),
		timeString(session.Account.CreatedAt),
		timeString(session.Account.LastSeenAt),
		strings.TrimSpace(session.SessionToken),
		timeString(session.ExpiresAt),
		session.Account.AccountID,
	)
	return err
}

func updateSQLiteAccountSession(execable interface {
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
		`update accounts set handle = ?, primary_guest_id = ?, linked_guest_ids = ?, rating = ?, matches_played = ?, wins = ?, losses = ?, draws = ?, rating_history = ?, created_at = ?, last_seen_at = ?, session_token = ?, session_expires_at = ? where account_id = ?`,
		session.Account.Handle,
		session.Account.PrimaryGuestID,
		string(linkedGuestIDs),
		session.Account.Rating,
		session.Account.MatchesPlayed,
		session.Account.Wins,
		session.Account.Losses,
		session.Account.Draws,
		string(ratingHistory),
		timeString(session.Account.CreatedAt),
		timeString(session.Account.LastSeenAt),
		strings.TrimSpace(session.SessionToken),
		timeString(session.ExpiresAt),
		session.Account.AccountID,
	)
	return err
}

func scanSQLiteAccountSession(scanner sqliteAccountScanner) (AccountSession, bool, error) {
	var (
		session        AccountSession
		linkedGuestIDs string
		ratingHistory  string
		createdAt      string
		lastSeenAt     string
		sessionToken   sql.NullString
		sessionExpires sql.NullString
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
		&createdAt,
		&lastSeenAt,
		&sessionToken,
		&sessionExpires,
	)
	if err == sql.ErrNoRows {
		return AccountSession{}, false, nil
	}
	if err != nil {
		return AccountSession{}, false, err
	}
	if err := json.Unmarshal([]byte(linkedGuestIDs), &session.Account.LinkedGuestIDs); err != nil {
		return AccountSession{}, false, err
	}
	if strings.TrimSpace(ratingHistory) != "" {
		if err := json.Unmarshal([]byte(ratingHistory), &session.Account.RatingHistory); err != nil {
			return AccountSession{}, false, err
		}
	}
	session.Account.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return AccountSession{}, false, err
	}
	session.Account.LastSeenAt, err = time.Parse(time.RFC3339Nano, lastSeenAt)
	if err != nil {
		return AccountSession{}, false, err
	}
	session.SessionToken = strings.TrimSpace(sessionToken.String)
	if strings.TrimSpace(sessionExpires.String) != "" {
		session.ExpiresAt, err = time.Parse(time.RFC3339Nano, sessionExpires.String)
		if err != nil {
			return AccountSession{}, false, err
		}
	}
	return session, true, nil
}

func ensureSQLiteTableColumn(db *sql.DB, tableName, columnName, columnDef string) error {
	rows, err := db.Query(`pragma table_info(` + tableName + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var (
		cid         int
		name        string
		dataType    string
		notNull     int
		defaultExpr sql.NullString
		primaryKey  int
	)
	for rows.Next() {
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultExpr, &primaryKey); err != nil {
			return err
		}
		if strings.EqualFold(name, columnName) {
			return nil
		}
	}
	_, err = db.Exec(`alter table ` + tableName + ` add column ` + columnName + ` ` + columnDef)
	return err
}
