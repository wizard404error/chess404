package platform

import (
	"database/sql"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chess404/realtime/internal/contracts"
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
	_ = s.db.QueryRow(`select count(*) from account_sessions where expires_at > ?`, timeString(time.Now().UTC())).Scan(&stats.ActiveSessionCount)
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
			touchAccountPresence(&session.Account, now)
			record := newAccountSessionRecord(now)
			session.SessionToken = record.SessionToken
			session.ExpiresAt = record.ExpiresAt
			if err := updateSQLiteAccountSessionTx(tx, session); err != nil {
				return AccountSession{}, err
			}
			if err := insertSQLiteAccountSessionRecordTx(tx, session.Account.AccountID, record); err != nil {
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
		LastActiveAt:   now,
	}
	session := AccountSession{
		Account:      account,
		SessionToken: "",
		ExpiresAt:    time.Time{},
	}
	record := newAccountSessionRecord(now)
	session.SessionToken = record.SessionToken
	session.ExpiresAt = record.ExpiresAt
	if err := insertSQLiteAccountTx(tx, session); err != nil {
		return AccountSession{}, err
	}
	if err := insertSQLiteAccountSessionRecordTx(tx, accountID, record); err != nil {
		return AccountSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return AccountSession{}, err
	}
	return session, nil
}

func (s *SQLiteAccountStore) RegisterGuestAccount(guest GuestProfile, handle, email, password string) (AccountSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedGuestID := strings.TrimSpace(guest.GuestID)
	if resolvedGuestID == "" {
		return AccountSession{}, os.ErrInvalid
	}
	normalizedHandle, err := normalizeAccountHandle(handle)
	if err != nil || normalizedHandle == "" {
		if err == nil {
			err = ErrInvalidAccountHandle
		}
		return AccountSession{}, err
	}
	normalizedEmail, err := normalizeAccountEmail(email)
	if err != nil {
		return AccountSession{}, err
	}
	passwordHash, err := hashAccountPassword(password)
	if err != nil {
		return AccountSession{}, err
	}

	now := time.Now().UTC()
	tx, err := s.db.Begin()
	if err != nil {
		return AccountSession{}, err
	}
	defer func() { _ = tx.Rollback() }()

	if session, ok, err := lookupSQLiteAccountSessionByGuestTx(tx, resolvedGuestID); err != nil {
		return AccountSession{}, err
	} else if ok {
		if normalizedHandle != session.Account.Handle {
			return AccountSession{}, ErrAccountHandleTaken
		}
		if existingAccountID, exists, err := lookupSQLiteCredentialOwnerByEmailTx(tx, normalizedEmail); err != nil {
			return AccountSession{}, err
		} else if exists && existingAccountID != session.Account.AccountID {
			return AccountSession{}, ErrAccountEmailTaken
		}

		touchAccountPresence(&session.Account, now)
		record := newAccountSessionRecord(now)
		session.SessionToken = record.SessionToken
		session.ExpiresAt = record.ExpiresAt
		if err := updateSQLiteAccountSessionTx(tx, session); err != nil {
			return AccountSession{}, err
		}
		if err := insertSQLiteAccountSessionRecordTx(tx, session.Account.AccountID, record); err != nil {
			return AccountSession{}, err
		}
		existingState, err := lookupSQLiteAccountAuthStateByAccountIDTx(tx, session.Account.AccountID)
		if err != nil {
			return AccountSession{}, err
		}
		emailChanged := !strings.EqualFold(strings.TrimSpace(existingState.Email), normalizedEmail)
		if _, err := tx.Exec(
			`insert into account_credentials(account_id, email, password_hash) values(?, ?, ?) on conflict(account_id) do update set email = excluded.email, password_hash = excluded.password_hash`,
			session.Account.AccountID,
			normalizedEmail,
			passwordHash,
		); err != nil {
			return AccountSession{}, err
		}
		if emailChanged {
			if _, err := tx.Exec(`update account_credentials set email_verified_at = null where account_id = ?`, session.Account.AccountID); err != nil {
				return AccountSession{}, err
			}
			if _, err := tx.Exec(`delete from account_email_verifications where account_id = ?`, session.Account.AccountID); err != nil {
				return AccountSession{}, err
			}
			if _, err := tx.Exec(`delete from account_password_resets where account_id = ?`, session.Account.AccountID); err != nil {
				return AccountSession{}, err
			}
		}
		if err := tx.Commit(); err != nil {
			return AccountSession{}, err
		}
		return session, nil
	}

	existingAccountID, exists, err := lookupSQLiteAccountIDByHandleTx(tx, normalizedHandle)
	if err != nil {
		return AccountSession{}, err
	}
	if exists && existingAccountID != "" {
		return AccountSession{}, ErrAccountHandleTaken
	}
	if existingAccountID, exists, err := lookupSQLiteCredentialOwnerByEmailTx(tx, normalizedEmail); err != nil {
		return AccountSession{}, err
	} else if exists && existingAccountID != "" {
		return AccountSession{}, ErrAccountEmailTaken
	}

	accountID := "acct_" + randomToken(8)
	account := AccountProfile{
		AccountID:      accountID,
		Handle:         normalizedHandle,
		PrimaryGuestID: resolvedGuestID,
		LinkedGuestIDs: []string{resolvedGuestID},
		CreatedAt:      now,
		LastSeenAt:     now,
		LastActiveAt:   now,
	}
	session := AccountSession{
		Account:      account,
		SessionToken: "",
		ExpiresAt:    time.Time{},
	}
	record := newAccountSessionRecord(now)
	session.SessionToken = record.SessionToken
	session.ExpiresAt = record.ExpiresAt
	if err := insertSQLiteAccountTx(tx, session); err != nil {
		return AccountSession{}, err
	}
	if err := insertSQLiteAccountSessionRecordTx(tx, accountID, record); err != nil {
		return AccountSession{}, err
	}
	if _, err := tx.Exec(
		`insert into account_credentials(account_id, email, password_hash) values(?, ?, ?)`,
		accountID,
		normalizedEmail,
		passwordHash,
	); err != nil {
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

	tx, err := s.db.Begin()
	if err != nil {
		return AccountSession{}, err
	}
	defer func() { _ = tx.Rollback() }()

	session, ok, err := lookupSQLiteAccountSessionByID(tx, resolvedAccountID)
	if err != nil {
		return AccountSession{}, err
	}
	if !ok {
		return AccountSession{}, os.ErrNotExist
	}
	record, ok, err := lookupSQLiteActiveAccountSessionRecordTx(tx, resolvedAccountID, sessionToken, time.Now().UTC())
	if err != nil {
		return AccountSession{}, err
	}
	if !ok {
		return AccountSession{}, ErrUnauthorizedAccountSession
	}

	now := time.Now().UTC()
	touchAccountPresence(&session.Account, now)
	record.LastSeenAt = now
	record.ExpiresAt = now.Add(defaultAccountSessionTTL)
	session.SessionToken = record.SessionToken
	session.ExpiresAt = record.ExpiresAt
	if err := upsertSQLiteAccountSessionRecordTx(tx, resolvedAccountID, record); err != nil {
		return AccountSession{}, err
	}
	if _, err := tx.Exec(`update accounts set last_seen_at = ?, last_active_at = ?, session_token = ?, session_expires_at = ? where account_id = ?`, timeString(session.Account.LastSeenAt), nullTimeString(session.Account.LastActiveAt), session.SessionToken, timeString(session.ExpiresAt), resolvedAccountID); err != nil {
		return AccountSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return AccountSession{}, err
	}
	return session, nil
}

func (s *SQLiteAccountStore) TouchPresence(accountID, sessionToken string) (AccountSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return AccountSession{}, os.ErrInvalid
	}

	tx, err := s.db.Begin()
	if err != nil {
		return AccountSession{}, err
	}
	defer func() { _ = tx.Rollback() }()

	session, ok, err := lookupSQLiteAccountSessionByID(tx, resolvedAccountID)
	if err != nil {
		return AccountSession{}, err
	}
	if !ok {
		return AccountSession{}, os.ErrNotExist
	}
	record, ok, err := lookupSQLiteActiveAccountSessionRecordTx(tx, resolvedAccountID, sessionToken, time.Now().UTC())
	if err != nil {
		return AccountSession{}, err
	}
	if !ok {
		return AccountSession{}, ErrUnauthorizedAccountSession
	}

	now := time.Now().UTC()
	touchAccountPresence(&session.Account, now)
	record.LastSeenAt = now
	record.ExpiresAt = now.Add(defaultAccountSessionTTL)
	session.SessionToken = record.SessionToken
	session.ExpiresAt = record.ExpiresAt
	if err := upsertSQLiteAccountSessionRecordTx(tx, resolvedAccountID, record); err != nil {
		return AccountSession{}, err
	}
	if _, err := tx.Exec(`update accounts set last_seen_at = ?, last_active_at = ?, session_token = ?, session_expires_at = ? where account_id = ?`, timeString(session.Account.LastSeenAt), nullTimeString(session.Account.LastActiveAt), session.SessionToken, timeString(session.ExpiresAt), resolvedAccountID); err != nil {
		return AccountSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return AccountSession{}, err
	}
	return session, nil
}

func (s *SQLiteAccountStore) GetAccountAuthOverview(accountID, sessionToken string) (AccountAuthOverview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return AccountAuthOverview{}, os.ErrInvalid
	}

	tx, err := s.db.Begin()
	if err != nil {
		return AccountAuthOverview{}, err
	}
	defer func() { _ = tx.Rollback() }()

	session, ok, err := lookupSQLiteAccountSessionByID(tx, resolvedAccountID)
	if err != nil {
		return AccountAuthOverview{}, err
	}
	if !ok {
		return AccountAuthOverview{}, os.ErrNotExist
	}
	record, ok, err := lookupSQLiteActiveAccountSessionRecordTx(tx, resolvedAccountID, sessionToken, time.Now().UTC())
	if err != nil {
		return AccountAuthOverview{}, err
	}
	if !ok {
		return AccountAuthOverview{}, ErrUnauthorizedAccountSession
	}
	record.LastSeenAt = time.Now().UTC()
	record.ExpiresAt = time.Now().UTC().Add(defaultAccountSessionTTL)
	if err := upsertSQLiteAccountSessionRecordTx(tx, resolvedAccountID, record); err != nil {
		return AccountAuthOverview{}, err
	}
	authState, err := lookupSQLiteAccountAuthStateByAccountIDTx(tx, resolvedAccountID)
	if err != nil {
		return AccountAuthOverview{}, err
	}
	if pending, ok, err := lookupSQLitePendingEmailVerificationTx(tx, resolvedAccountID, time.Now().UTC()); err != nil {
		return AccountAuthOverview{}, err
	} else if ok {
		authState.EmailVerifications = []AccountEmailVerificationRecord{pending}
	}
	if err := tx.Commit(); err != nil {
		return AccountAuthOverview{}, err
	}
	return buildAccountAuthOverview(session.Account, authState, time.Now().UTC()), nil
}

func (s *SQLiteAccountStore) ListAccountSessions(accountID, sessionToken string) (AccountSessionOverview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return AccountSessionOverview{}, os.ErrInvalid
	}

	session, ok, err := lookupSQLiteAccountSessionByID(s.db, resolvedAccountID)
	if err != nil {
		return AccountSessionOverview{}, err
	}
	if !ok {
		return AccountSessionOverview{}, os.ErrNotExist
	}
	if _, ok, err := lookupSQLiteActiveAccountSessionRecordTx(s.db, resolvedAccountID, sessionToken, time.Now().UTC()); err != nil {
		return AccountSessionOverview{}, err
	} else if !ok {
		return AccountSessionOverview{}, ErrUnauthorizedAccountSession
	}
	records, err := listSQLiteActiveAccountSessionRecords(s.db, resolvedAccountID, time.Now().UTC())
	if err != nil {
		return AccountSessionOverview{}, err
	}
	return buildAccountSessionOverview(session.Account, records), nil
}

func (s *SQLiteAccountStore) RevokeAccountSession(accountID, sessionToken, revokeToken string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return os.ErrInvalid
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, ok, err := lookupSQLiteAccountSessionByID(tx, resolvedAccountID); err != nil {
		return err
	} else if !ok {
		return os.ErrNotExist
	}
	if _, ok, err := lookupSQLiteActiveAccountSessionRecordTx(tx, resolvedAccountID, sessionToken, time.Now().UTC()); err != nil {
		return err
	} else if !ok {
		return ErrUnauthorizedAccountSession
	}
	if err := deleteSQLiteAccountSessionRecordTx(tx, resolvedAccountID, revokeToken); err != nil {
		return err
	}
	if err := syncSQLiteLegacyAccountSessionTx(tx, tx, resolvedAccountID, time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteAccountStore) RevokeOtherAccountSessions(accountID, sessionToken string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return os.ErrInvalid
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, ok, err := lookupSQLiteAccountSessionByID(tx, resolvedAccountID); err != nil {
		return err
	} else if !ok {
		return os.ErrNotExist
	}
	if _, ok, err := lookupSQLiteActiveAccountSessionRecordTx(tx, resolvedAccountID, sessionToken, time.Now().UTC()); err != nil {
		return err
	} else if !ok {
		return ErrUnauthorizedAccountSession
	}
	if err := deleteOtherSQLiteAccountSessionRecordsTx(tx, resolvedAccountID, sessionToken); err != nil {
		return err
	}
	if err := syncSQLiteLegacyAccountSessionTx(tx, tx, resolvedAccountID, time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteAccountStore) EnablePasswordLogin(accountID, sessionToken, email, password string) (AccountSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return AccountSession{}, os.ErrInvalid
	}
	normalizedEmail, err := normalizeAccountEmail(email)
	if err != nil {
		return AccountSession{}, err
	}
	passwordHash, err := hashAccountPassword(password)
	if err != nil {
		return AccountSession{}, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return AccountSession{}, err
	}
	defer func() { _ = tx.Rollback() }()

	session, ok, err := lookupSQLiteAccountSessionByID(tx, resolvedAccountID)
	if err != nil {
		return AccountSession{}, err
	}
	if !ok {
		return AccountSession{}, os.ErrNotExist
	}
	record, ok, err := lookupSQLiteActiveAccountSessionRecordTx(tx, resolvedAccountID, sessionToken, time.Now().UTC())
	if err != nil {
		return AccountSession{}, err
	}
	if !ok {
		return AccountSession{}, ErrUnauthorizedAccountSession
	}
	if existingAccountID, exists, err := lookupSQLiteCredentialOwnerByEmailTx(tx, normalizedEmail); err != nil {
		return AccountSession{}, err
	} else if exists && existingAccountID != resolvedAccountID {
		return AccountSession{}, ErrAccountEmailTaken
	}

	now := time.Now().UTC()
	touchAccountPresence(&session.Account, now)
	record.LastSeenAt = now
	record.ExpiresAt = now.Add(defaultAccountSessionTTL)
	session.SessionToken = record.SessionToken
	session.ExpiresAt = record.ExpiresAt
	if err := updateSQLiteAccountSessionTx(tx, session); err != nil {
		return AccountSession{}, err
	}
	if err := upsertSQLiteAccountSessionRecordTx(tx, resolvedAccountID, record); err != nil {
		return AccountSession{}, err
	}
	existingState, err := lookupSQLiteAccountAuthStateByAccountIDTx(tx, resolvedAccountID)
	if err != nil {
		return AccountSession{}, err
	}
	emailChanged := !strings.EqualFold(strings.TrimSpace(existingState.Email), normalizedEmail)
	if _, err := tx.Exec(
		`insert into account_credentials(account_id, email, password_hash) values(?, ?, ?) on conflict(account_id) do update set email = excluded.email, password_hash = excluded.password_hash`,
		resolvedAccountID,
		normalizedEmail,
		passwordHash,
	); err != nil {
		return AccountSession{}, err
	}
	if emailChanged {
		if _, err := tx.Exec(`update account_credentials set email_verified_at = null where account_id = ?`, resolvedAccountID); err != nil {
			return AccountSession{}, err
		}
		if _, err := tx.Exec(`delete from account_email_verifications where account_id = ?`, resolvedAccountID); err != nil {
			return AccountSession{}, err
		}
		if _, err := tx.Exec(`delete from account_password_resets where account_id = ?`, resolvedAccountID); err != nil {
			return AccountSession{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return AccountSession{}, err
	}
	return session, nil
}

func (s *SQLiteAccountStore) StartEmailVerification(accountID, sessionToken string) (AccountEmailVerificationChallenge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return AccountEmailVerificationChallenge{}, os.ErrInvalid
	}
	tx, err := s.db.Begin()
	if err != nil {
		return AccountEmailVerificationChallenge{}, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, ok, err := lookupSQLiteAccountSessionByID(tx, resolvedAccountID); err != nil {
		return AccountEmailVerificationChallenge{}, err
	} else if !ok {
		return AccountEmailVerificationChallenge{}, os.ErrNotExist
	}
	if _, ok, err := lookupSQLiteActiveAccountSessionRecordTx(tx, resolvedAccountID, sessionToken, time.Now().UTC()); err != nil {
		return AccountEmailVerificationChallenge{}, err
	} else if !ok {
		return AccountEmailVerificationChallenge{}, ErrUnauthorizedAccountSession
	}
	authState, err := lookupSQLiteAccountAuthStateByAccountIDTx(tx, resolvedAccountID)
	if err != nil {
		return AccountEmailVerificationChallenge{}, err
	}
	if strings.TrimSpace(authState.Email) == "" || strings.TrimSpace(authState.PasswordHash) == "" {
		return AccountEmailVerificationChallenge{}, ErrAccountLoginUnavailable
	}
	if !authState.EmailVerifiedAt.IsZero() {
		return AccountEmailVerificationChallenge{}, ErrAccountEmailAlreadyVerified
	}
	authState, record := issueAccountEmailVerification(authState, authState.Email, time.Now().UTC())
	if _, err := tx.Exec(`delete from account_email_verifications where account_id = ?`, resolvedAccountID); err != nil {
		return AccountEmailVerificationChallenge{}, err
	}
	for _, item := range authState.EmailVerifications {
		if err := insertSQLiteAccountEmailVerificationTx(tx, resolvedAccountID, item); err != nil {
			return AccountEmailVerificationChallenge{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return AccountEmailVerificationChallenge{}, err
	}
	return AccountEmailVerificationChallenge{
		AccountID: resolvedAccountID,
		Email:     record.Email,
		Token:     record.Token,
		ExpiresAt: record.ExpiresAt,
		CreatedAt: record.CreatedAt,
	}, nil
}

func (s *SQLiteAccountStore) VerifyEmail(accountID, token string) (AccountAuthOverview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return AccountAuthOverview{}, os.ErrInvalid
	}
	tx, err := s.db.Begin()
	if err != nil {
		return AccountAuthOverview{}, err
	}
	defer func() { _ = tx.Rollback() }()

	session, ok, err := lookupSQLiteAccountSessionByID(tx, resolvedAccountID)
	if err != nil {
		return AccountAuthOverview{}, err
	}
	if !ok {
		return AccountAuthOverview{}, os.ErrNotExist
	}
	record, ok, err := lookupSQLiteAccountEmailVerificationByTokenTx(tx, resolvedAccountID, token, time.Now().UTC())
	if err != nil {
		return AccountAuthOverview{}, err
	}
	if !ok {
		return AccountAuthOverview{}, ErrUnauthorizedAccountEmailVerification
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(`update account_email_verifications set used_at = ?, updated_at = ? where account_id = ? and token = ?`, timeString(now), timeString(now), resolvedAccountID, strings.TrimSpace(token)); err != nil {
		return AccountAuthOverview{}, err
	}
	if _, err := tx.Exec(`update account_credentials set email = ?, email_verified_at = ? where account_id = ?`, record.Email, timeString(now), resolvedAccountID); err != nil {
		return AccountAuthOverview{}, err
	}
	if _, err := tx.Exec(`delete from account_password_resets where account_id = ?`, resolvedAccountID); err != nil {
		return AccountAuthOverview{}, err
	}
	authState, err := lookupSQLiteAccountAuthStateByAccountIDTx(tx, resolvedAccountID)
	if err != nil {
		return AccountAuthOverview{}, err
	}
	if err := tx.Commit(); err != nil {
		return AccountAuthOverview{}, err
	}
	return buildAccountAuthOverview(session.Account, authState, now), nil
}

func (s *SQLiteAccountStore) LoginWithPassword(identifier, password string) (AccountSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	accountID, passwordHash, ok, err := lookupSQLiteAccountCredentialsByIdentifier(s.db, identifier)
	if err != nil {
		return AccountSession{}, err
	}
	if !ok || !verifyAccountPassword(password, passwordHash) {
		return AccountSession{}, ErrUnauthorizedAccountCredentials
	}

	session, ok, err := lookupSQLiteAccountSessionByID(s.db, accountID)
	if err != nil {
		return AccountSession{}, err
	}
	if !ok {
		return AccountSession{}, os.ErrNotExist
	}
	now := time.Now().UTC()
	touchAccountPresence(&session.Account, now)
	record := newAccountSessionRecord(now)
	session.SessionToken = record.SessionToken
	session.ExpiresAt = record.ExpiresAt
	tx, err := s.db.Begin()
	if err != nil {
		return AccountSession{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := insertSQLiteAccountSessionRecordTx(tx, accountID, record); err != nil {
		return AccountSession{}, err
	}
	if _, err := tx.Exec(`update accounts set last_seen_at = ?, last_active_at = ?, session_token = ?, session_expires_at = ? where account_id = ?`, timeString(session.Account.LastSeenAt), nullTimeString(session.Account.LastActiveAt), session.SessionToken, timeString(session.ExpiresAt), accountID); err != nil {
		return AccountSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return AccountSession{}, err
	}
	return session, nil
}

func (s *SQLiteAccountStore) StartPasswordReset(identifier string) (AccountPasswordResetChallenge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	accountID, email, passwordHash, verifiedAt, ok, err := lookupSQLiteAccountCredentialStateByIdentifier(s.db, identifier)
	if err != nil {
		return AccountPasswordResetChallenge{}, err
	}
	if !ok || strings.TrimSpace(email) == "" || strings.TrimSpace(passwordHash) == "" || verifiedAt.IsZero() {
		return AccountPasswordResetChallenge{Requested: true}, nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return AccountPasswordResetChallenge{}, err
	}
	defer func() { _ = tx.Rollback() }()

	authState, err := lookupSQLiteAccountAuthStateByAccountIDTx(tx, accountID)
	if err != nil {
		return AccountPasswordResetChallenge{}, err
	}
	authState, record := issueAccountPasswordReset(authState, time.Now().UTC())
	if _, err := tx.Exec(`delete from account_password_resets where account_id = ?`, accountID); err != nil {
		return AccountPasswordResetChallenge{}, err
	}
	for _, item := range authState.PasswordResets {
		if err := insertSQLiteAccountPasswordResetTx(tx, accountID, item); err != nil {
			return AccountPasswordResetChallenge{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return AccountPasswordResetChallenge{}, err
	}
	return AccountPasswordResetChallenge{
		Requested: true,
		AccountID: accountID,
		Email:     email,
		Token:     record.Token,
		ExpiresAt: record.ExpiresAt,
		CreatedAt: record.CreatedAt,
	}, nil
}

func (s *SQLiteAccountStore) ResetPassword(accountID, token, password string) (AccountSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return AccountSession{}, os.ErrInvalid
	}
	passwordHash, err := hashAccountPassword(password)
	if err != nil {
		return AccountSession{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return AccountSession{}, err
	}
	defer func() { _ = tx.Rollback() }()

	session, ok, err := lookupSQLiteAccountSessionByID(tx, resolvedAccountID)
	if err != nil {
		return AccountSession{}, err
	}
	if !ok {
		return AccountSession{}, os.ErrNotExist
	}
	authState, err := lookupSQLiteAccountAuthStateByAccountIDTx(tx, resolvedAccountID)
	if err != nil {
		return AccountSession{}, err
	}
	if authState.EmailVerifiedAt.IsZero() {
		return AccountSession{}, ErrAccountEmailNotVerified
	}
	authState, _, ok = consumeAccountPasswordReset(authState, token, time.Now().UTC())
	if !ok {
		return AccountSession{}, ErrUnauthorizedAccountPasswordReset
	}
	now := time.Now().UTC()
	touchAccountPresence(&session.Account, now)
	record := newAccountSessionRecord(now)
	session.SessionToken = record.SessionToken
	session.ExpiresAt = record.ExpiresAt
	if err := insertSQLiteAccountSessionRecordTx(tx, resolvedAccountID, record); err != nil {
		return AccountSession{}, err
	}
	if _, err := tx.Exec(`update accounts set last_seen_at = ?, last_active_at = ?, session_token = ?, session_expires_at = ? where account_id = ?`, timeString(session.Account.LastSeenAt), nullTimeString(session.Account.LastActiveAt), session.SessionToken, timeString(session.ExpiresAt), resolvedAccountID); err != nil {
		return AccountSession{}, err
	}
	if _, err := tx.Exec(`update account_credentials set password_hash = ? where account_id = ?`, passwordHash, resolvedAccountID); err != nil {
		return AccountSession{}, err
	}
	if _, err := tx.Exec(`delete from account_password_resets where account_id = ?`, resolvedAccountID); err != nil {
		return AccountSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return AccountSession{}, err
	}
	return session, nil
}

func (s *SQLiteAccountStore) LogoutAccount(accountID, sessionToken string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return os.ErrInvalid
	}
	if _, ok, err := lookupSQLiteAccountSessionByID(s.db, resolvedAccountID); err != nil {
		return err
	} else if !ok {
		return os.ErrNotExist
	}
	if _, ok, err := lookupSQLiteActiveAccountSessionRecordTx(s.db, resolvedAccountID, sessionToken, time.Now().UTC()); err != nil {
		return err
	} else if !ok {
		return ErrUnauthorizedAccountSession
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := deleteSQLiteAccountSessionRecordTx(tx, resolvedAccountID, sessionToken); err != nil {
		return err
	}
	if err := syncSQLiteLegacyAccountSessionTx(tx, tx, resolvedAccountID, time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit()
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

func (s *SQLiteAccountStore) FinalizeMatch(matchID, whiteAccountID, blackAccountID, winner, queue string, modeID contracts.MatchModeID) (AccountProfile, AccountProfile, bool, error) {
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

	whiteRating := float64(whiteSession.Account.Rating)
	blackRating := float64(blackSession.Account.Rating)
	k := 32.0

	whiteExpected := 1.0 / (1.0 + math.Pow(10, (blackRating-whiteRating)/400.0))
	blackExpected := 1.0 / (1.0 + math.Pow(10, (whiteRating-blackRating)/400.0))

	now := time.Now().UTC()
	switch winner {
	case "white":
		whiteSession.Account.Rating = int(math.Round(whiteRating + k*(1.0-whiteExpected)))
		blackSession.Account.Rating = maxInt(100, int(math.Round(blackRating+k*(0.0-blackExpected))))
		whiteSession.Account.Wins++
		blackSession.Account.Losses++
	case "black":
		blackSession.Account.Rating = int(math.Round(blackRating + k*(1.0-blackExpected)))
		whiteSession.Account.Rating = maxInt(100, int(math.Round(whiteRating+k*(0.0-whiteExpected))))
		blackSession.Account.Wins++
		whiteSession.Account.Losses++
	case "draw":
		whiteSession.Account.Rating = int(math.Round(whiteRating + k*(0.5-whiteExpected)))
		blackSession.Account.Rating = int(math.Round(blackRating + k*(0.5-blackExpected)))
		whiteSession.Account.Draws++
		blackSession.Account.Draws++
	default:
		return AccountProfile{}, AccountProfile{}, false, os.ErrInvalid
	}
	whiteSession.Account.MatchesPlayed++
	blackSession.Account.MatchesPlayed++
	touchAccountPresence(&whiteSession.Account, now)
	touchAccountPresence(&blackSession.Account, now)
	modeID = contracts.NormalizeMatchModeID(string(modeID))
	whiteSession.Account.RatingHistory = appendAccountRatingHistory(
		whiteSession.Account.RatingHistory,
		buildAccountRatingHistoryEntry(resolvedMatchID, blackSession.Account.AccountID, winner, queue, modeID, whiteBefore, whiteSession.Account.Rating, whiteSession.Account.MatchesPlayed, "white", now),
	)
	blackSession.Account.RatingHistory = appendAccountRatingHistory(
		blackSession.Account.RatingHistory,
		buildAccountRatingHistoryEntry(resolvedMatchID, whiteSession.Account.AccountID, winner, queue, modeID, blackBefore, blackSession.Account.Rating, blackSession.Account.MatchesPlayed, "black", now),
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
	query := `select account_id, handle, primary_guest_id, linked_guest_ids, rating, matches_played, wins, losses, draws, rating_history, created_at, last_seen_at, last_active_at, session_token, session_expires_at from accounts order by last_seen_at desc, created_at desc, account_id asc`
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
			last_active_at text,
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
		create table if not exists account_credentials (
			account_id text primary key references accounts(account_id) on delete cascade,
			email text not null unique,
			password_hash text not null,
			email_verified_at text
		);
		create table if not exists account_email_verifications (
			account_id text not null references accounts(account_id) on delete cascade,
			token text primary key,
			email text not null,
			expires_at text not null,
			created_at text not null,
			used_at text,
			updated_at text not null
		);
		create table if not exists account_password_resets (
			account_id text not null references accounts(account_id) on delete cascade,
			token text primary key,
			expires_at text not null,
			created_at text not null,
			used_at text,
			updated_at text not null
		);
		create table if not exists account_sessions (
			account_id text not null references accounts(account_id) on delete cascade,
			session_token text primary key,
			expires_at text not null,
			created_at text not null,
			last_seen_at text not null
		);
		create index if not exists account_sessions_account_idx on account_sessions (account_id, last_seen_at desc, created_at desc, session_token asc);
		create index if not exists account_sessions_expires_idx on account_sessions (expires_at);
		create index if not exists account_email_verifications_account_idx on account_email_verifications (account_id, created_at desc);
		create index if not exists account_password_resets_account_idx on account_password_resets (account_id, created_at desc);
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
		{name: "last_active_at", def: "text"},
	} {
		if err := ensureSQLiteTableColumn(s.db, "accounts", column.name, column.def); err != nil {
			return err
		}
	}
	if err := ensureSQLiteTableColumn(s.db, "account_credentials", "email_verified_at", "text"); err != nil {
		return err
	}
	_, err = s.db.Exec(`
		insert into account_sessions(account_id, session_token, expires_at, created_at, last_seen_at)
		select account_id, session_token, session_expires_at, coalesce(last_active_at, last_seen_at, created_at), coalesce(last_active_at, last_seen_at, created_at)
		from accounts
		where session_token is not null
			and session_expires_at is not null
			and not exists (
				select 1 from account_sessions s where s.session_token = accounts.session_token
			)
	`)
	return err
}

type sqliteAccountScanner interface {
	Scan(dest ...any) error
}

type sqliteAccountQueryable interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

type sqliteAccountExecable interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func lookupSQLiteAccountSessionByID(queryable interface {
	QueryRow(query string, args ...any) *sql.Row
}, accountID string) (AccountSession, bool, error) {
	return scanSQLiteAccountSession(queryable.QueryRow(`select account_id, handle, primary_guest_id, linked_guest_ids, rating, matches_played, wins, losses, draws, rating_history, created_at, last_seen_at, last_active_at, session_token, session_expires_at from accounts where account_id = ?`, accountID))
}

func lookupSQLiteAccountSessionByGuest(queryable interface {
	QueryRow(query string, args ...any) *sql.Row
}, guestID string) (AccountSession, bool, error) {
	return scanSQLiteAccountSession(queryable.QueryRow(`select a.account_id, a.handle, a.primary_guest_id, a.linked_guest_ids, a.rating, a.matches_played, a.wins, a.losses, a.draws, a.rating_history, a.created_at, a.last_seen_at, a.last_active_at, a.session_token, a.session_expires_at from accounts a join account_guest_links l on l.account_id = a.account_id where l.guest_id = ?`, guestID))
}

func lookupSQLiteAccountSessionByGuestTx(tx *sql.Tx, guestID string) (AccountSession, bool, error) {
	return lookupSQLiteAccountSessionByGuest(tx, guestID)
}

func lookupSQLiteActiveAccountSessionRecordTx(queryable sqliteAccountQueryable, accountID, sessionToken string, now time.Time) (AccountSessionRecord, bool, error) {
	var (
		record     AccountSessionRecord
		expiresAt  string
		createdAt  string
		lastSeenAt string
	)
	err := queryable.QueryRow(
		`select session_token, expires_at, created_at, last_seen_at from account_sessions where account_id = ? and session_token = ? and expires_at > ?`,
		accountID,
		strings.TrimSpace(sessionToken),
		timeString(now.UTC()),
	).Scan(&record.SessionToken, &expiresAt, &createdAt, &lastSeenAt)
	if err == sql.ErrNoRows {
		return AccountSessionRecord{}, false, nil
	}
	if err != nil {
		return AccountSessionRecord{}, false, err
	}
	record.ExpiresAt, err = time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil {
		return AccountSessionRecord{}, false, err
	}
	record.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return AccountSessionRecord{}, false, err
	}
	record.LastSeenAt, err = time.Parse(time.RFC3339Nano, lastSeenAt)
	if err != nil {
		return AccountSessionRecord{}, false, err
	}
	return record, true, nil
}

func listSQLiteActiveAccountSessionRecords(queryable sqliteAccountQueryable, accountID string, now time.Time) ([]AccountSessionRecord, error) {
	rows, err := queryable.Query(
		`select session_token, expires_at, created_at, last_seen_at from account_sessions where account_id = ? and expires_at > ? order by last_seen_at desc, created_at desc, session_token asc`,
		accountID,
		timeString(now.UTC()),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]AccountSessionRecord, 0)
	for rows.Next() {
		var (
			record     AccountSessionRecord
			expiresAt  string
			createdAt  string
			lastSeenAt string
		)
		if err := rows.Scan(&record.SessionToken, &expiresAt, &createdAt, &lastSeenAt); err != nil {
			return nil, err
		}
		record.ExpiresAt, err = time.Parse(time.RFC3339Nano, expiresAt)
		if err != nil {
			return nil, err
		}
		record.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, err
		}
		record.LastSeenAt, err = time.Parse(time.RFC3339Nano, lastSeenAt)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
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

func lookupSQLiteCredentialOwnerByEmailTx(tx *sql.Tx, email string) (string, bool, error) {
	var accountID string
	err := tx.QueryRow(`select account_id from account_credentials where email = ?`, email).Scan(&accountID)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return accountID, true, nil
}

func lookupSQLiteAccountAuthStateByAccountIDTx(queryable interface {
	QueryRow(query string, args ...any) *sql.Row
}, accountID string) (AccountPrivateState, error) {
	var (
		email         sql.NullString
		passwordHash  sql.NullString
		emailVerified sql.NullString
		state         AccountPrivateState
		err           error
	)
	if err = queryable.QueryRow(`select email, password_hash, email_verified_at from account_credentials where account_id = ?`, accountID).Scan(&email, &passwordHash, &emailVerified); err != nil {
		if err == sql.ErrNoRows {
			return AccountPrivateState{}, nil
		}
		return AccountPrivateState{}, err
	}
	state.Email = strings.TrimSpace(email.String)
	state.PasswordHash = strings.TrimSpace(passwordHash.String)
	if strings.TrimSpace(emailVerified.String) != "" {
		state.EmailVerifiedAt, err = time.Parse(time.RFC3339Nano, emailVerified.String)
		if err != nil {
			return AccountPrivateState{}, err
		}
	}
	return normalizeAccountPrivateState(state), nil
}

func lookupSQLitePendingEmailVerificationTx(queryable sqliteAccountQueryable, accountID string, now time.Time) (AccountEmailVerificationRecord, bool, error) {
	var (
		record    AccountEmailVerificationRecord
		expiresAt string
		createdAt string
		usedAt    sql.NullString
	)
	err := queryable.QueryRow(
		`select token, email, expires_at, created_at, used_at from account_email_verifications where account_id = ? and used_at is null and expires_at > ? order by created_at desc limit 1`,
		accountID,
		timeString(now.UTC()),
	).Scan(&record.Token, &record.Email, &expiresAt, &createdAt, &usedAt)
	if err == sql.ErrNoRows {
		return AccountEmailVerificationRecord{}, false, nil
	}
	if err != nil {
		return AccountEmailVerificationRecord{}, false, err
	}
	record.ExpiresAt, err = time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil {
		return AccountEmailVerificationRecord{}, false, err
	}
	record.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return AccountEmailVerificationRecord{}, false, err
	}
	if strings.TrimSpace(usedAt.String) != "" {
		parsed, err := time.Parse(time.RFC3339Nano, usedAt.String)
		if err != nil {
			return AccountEmailVerificationRecord{}, false, err
		}
		record.UsedAt = &parsed
	}
	return record, true, nil
}

func lookupSQLiteAccountEmailVerificationByTokenTx(queryable sqliteAccountQueryable, accountID, token string, now time.Time) (AccountEmailVerificationRecord, bool, error) {
	var (
		record    AccountEmailVerificationRecord
		expiresAt string
		createdAt string
		usedAt    sql.NullString
	)
	err := queryable.QueryRow(
		`select token, email, expires_at, created_at, used_at from account_email_verifications where account_id = ? and token = ? and used_at is null and expires_at > ?`,
		accountID,
		strings.TrimSpace(token),
		timeString(now.UTC()),
	).Scan(&record.Token, &record.Email, &expiresAt, &createdAt, &usedAt)
	if err == sql.ErrNoRows {
		return AccountEmailVerificationRecord{}, false, nil
	}
	if err != nil {
		return AccountEmailVerificationRecord{}, false, err
	}
	record.ExpiresAt, err = time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil {
		return AccountEmailVerificationRecord{}, false, err
	}
	record.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return AccountEmailVerificationRecord{}, false, err
	}
	if strings.TrimSpace(usedAt.String) != "" {
		parsed, err := time.Parse(time.RFC3339Nano, usedAt.String)
		if err != nil {
			return AccountEmailVerificationRecord{}, false, err
		}
		record.UsedAt = &parsed
	}
	return record, true, nil
}

func insertSQLiteAccountEmailVerificationTx(tx *sql.Tx, accountID string, record AccountEmailVerificationRecord) error {
	_, err := tx.Exec(
		`insert into account_email_verifications(account_id, token, email, expires_at, created_at, used_at, updated_at) values(?, ?, ?, ?, ?, ?, ?)`,
		accountID,
		strings.TrimSpace(record.Token),
		strings.TrimSpace(record.Email),
		timeString(record.ExpiresAt),
		timeString(record.CreatedAt),
		sqliteNullableTimePointerString(record.UsedAt),
		timeString(record.CreatedAt),
	)
	return err
}

func insertSQLiteAccountPasswordResetTx(tx *sql.Tx, accountID string, record AccountPasswordResetRecord) error {
	_, err := tx.Exec(
		`insert into account_password_resets(account_id, token, expires_at, created_at, used_at, updated_at) values(?, ?, ?, ?, ?, ?)`,
		accountID,
		strings.TrimSpace(record.Token),
		timeString(record.ExpiresAt),
		timeString(record.CreatedAt),
		sqliteNullableTimePointerString(record.UsedAt),
		timeString(record.CreatedAt),
	)
	return err
}

func lookupSQLiteAccountCredentialStateByIdentifier(queryable interface {
	QueryRow(query string, args ...any) *sql.Row
}, identifier string) (string, string, string, time.Time, bool, error) {
	resolved := strings.ToLower(strings.TrimSpace(identifier))
	if resolved == "" {
		return "", "", "", time.Time{}, false, nil
	}

	var (
		accountID        string
		email            string
		passwordHash     string
		emailVerifiedRaw sql.NullString
	)
	err := queryable.QueryRow(`select account_id, email, password_hash, email_verified_at from account_credentials where email = ?`, resolved).Scan(&accountID, &email, &passwordHash, &emailVerifiedRaw)
	switch err {
	case nil:
	case sql.ErrNoRows:
		normalizedHandle, err := normalizeAccountHandle(resolved)
		if err != nil || normalizedHandle == "" {
			return "", "", "", time.Time{}, false, nil
		}
		err = queryable.QueryRow(`select c.account_id, c.email, c.password_hash, c.email_verified_at from accounts a join account_credentials c on c.account_id = a.account_id where a.handle = ?`, normalizedHandle).Scan(&accountID, &email, &passwordHash, &emailVerifiedRaw)
		if err == sql.ErrNoRows {
			return "", "", "", time.Time{}, false, nil
		}
		if err != nil {
			return "", "", "", time.Time{}, false, err
		}
	default:
		return "", "", "", time.Time{}, false, err
	}
	var verifiedAt time.Time
	if strings.TrimSpace(emailVerifiedRaw.String) != "" {
		verifiedAt, err = time.Parse(time.RFC3339Nano, emailVerifiedRaw.String)
		if err != nil {
			return "", "", "", time.Time{}, false, err
		}
	}
	return accountID, email, passwordHash, verifiedAt, true, nil
}

func lookupSQLiteAccountCredentialsByIdentifier(queryable interface {
	QueryRow(query string, args ...any) *sql.Row
}, identifier string) (string, string, bool, error) {
	resolved := strings.ToLower(strings.TrimSpace(identifier))
	if resolved == "" {
		return "", "", false, nil
	}

	var (
		accountID    string
		passwordHash string
	)
	err := queryable.QueryRow(`select account_id, password_hash from account_credentials where email = ?`, resolved).Scan(&accountID, &passwordHash)
	switch err {
	case nil:
		return accountID, passwordHash, true, nil
	case sql.ErrNoRows:
	default:
		return "", "", false, err
	}

	normalizedHandle, err := normalizeAccountHandle(resolved)
	if err != nil || normalizedHandle == "" {
		return "", "", false, nil
	}
	err = queryable.QueryRow(`select a.account_id, c.password_hash from accounts a join account_credentials c on c.account_id = a.account_id where a.handle = ?`, normalizedHandle).Scan(&accountID, &passwordHash)
	if err == sql.ErrNoRows {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	return accountID, passwordHash, true, nil
}

func sqliteNullableTimePointerString(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return timeString(value.UTC())
}

func newAccountSessionRecord(now time.Time) AccountSessionRecord {
	return AccountSessionRecord{
		SessionToken: "accttok_" + randomToken(18),
		ExpiresAt:    now.Add(defaultAccountSessionTTL).UTC(),
		CreatedAt:    now.UTC(),
		LastSeenAt:   now.UTC(),
	}
}

func insertSQLiteAccountSessionRecordTx(tx *sql.Tx, accountID string, record AccountSessionRecord) error {
	_, err := tx.Exec(
		`insert into account_sessions(account_id, session_token, expires_at, created_at, last_seen_at) values(?, ?, ?, ?, ?)`,
		accountID,
		strings.TrimSpace(record.SessionToken),
		timeString(record.ExpiresAt),
		timeString(record.CreatedAt),
		timeString(record.LastSeenAt),
	)
	return err
}

func upsertSQLiteAccountSessionRecordTx(execable sqliteAccountExecable, accountID string, record AccountSessionRecord) error {
	_, err := execable.Exec(
		`insert into account_sessions(account_id, session_token, expires_at, created_at, last_seen_at) values(?, ?, ?, ?, ?) on conflict(session_token) do update set expires_at = excluded.expires_at, created_at = excluded.created_at, last_seen_at = excluded.last_seen_at, account_id = excluded.account_id`,
		accountID,
		strings.TrimSpace(record.SessionToken),
		timeString(record.ExpiresAt),
		timeString(record.CreatedAt),
		timeString(record.LastSeenAt),
	)
	return err
}

func deleteSQLiteAccountSessionRecordTx(execable sqliteAccountExecable, accountID, sessionToken string) error {
	result, err := execable.Exec(`delete from account_sessions where account_id = ? and session_token = ?`, accountID, strings.TrimSpace(sessionToken))
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrUnauthorizedAccountSession
	}
	return nil
}

func deleteOtherSQLiteAccountSessionRecordsTx(execable sqliteAccountExecable, accountID, sessionToken string) error {
	_, err := execable.Exec(`delete from account_sessions where account_id = ? and session_token <> ?`, accountID, strings.TrimSpace(sessionToken))
	return err
}

func syncSQLiteLegacyAccountSessionTx(execable sqliteAccountExecable, queryable sqliteAccountQueryable, accountID string, now time.Time) error {
	records, err := listSQLiteActiveAccountSessionRecords(queryable, accountID, now)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		_, err = execable.Exec(`update accounts set session_token = null, session_expires_at = null where account_id = ?`, accountID)
		return err
	}
	current := records[0]
	_, err = execable.Exec(`update accounts set session_token = ?, session_expires_at = ? where account_id = ?`, current.SessionToken, timeString(current.ExpiresAt), accountID)
	return err
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
		`insert into accounts(account_id, handle, primary_guest_id, linked_guest_ids, rating, matches_played, wins, losses, draws, rating_history, created_at, last_seen_at, last_active_at, session_token, session_expires_at) values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
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
		nullTimeString(session.Account.LastActiveAt),
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
		`update accounts set handle = ?, primary_guest_id = ?, linked_guest_ids = ?, rating = ?, matches_played = ?, wins = ?, losses = ?, draws = ?, rating_history = ?, created_at = ?, last_seen_at = ?, last_active_at = ?, session_token = ?, session_expires_at = ? where account_id = ?`,
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
		nullTimeString(session.Account.LastActiveAt),
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
		`update accounts set handle = ?, primary_guest_id = ?, linked_guest_ids = ?, rating = ?, matches_played = ?, wins = ?, losses = ?, draws = ?, rating_history = ?, created_at = ?, last_seen_at = ?, last_active_at = ?, session_token = ?, session_expires_at = ? where account_id = ?`,
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
		nullTimeString(session.Account.LastActiveAt),
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
		lastActiveAt   sql.NullString
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
		&lastActiveAt,
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
	if strings.TrimSpace(lastActiveAt.String) != "" {
		session.Account.LastActiveAt, err = time.Parse(time.RFC3339Nano, lastActiveAt.String)
		if err != nil {
			return AccountSession{}, false, err
		}
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
