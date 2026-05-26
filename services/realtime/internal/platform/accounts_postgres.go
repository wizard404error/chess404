package platform

import (
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chess404/realtime/internal/contracts"
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
	_ = s.db.QueryRow(`select count(*) from account_sessions where expires_at > $1`, time.Now().UTC()).Scan(&stats.ActiveSessionCount)
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
			touchAccountPresence(&session.Account, now)
			record := newAccountSessionRecord(now)
			session.SessionToken = record.SessionToken
			session.ExpiresAt = record.ExpiresAt
			if err := updatePostgresAccountSessionTx(tx, session); err != nil {
				return AccountSession{}, err
			}
			if err := insertPostgresAccountSessionRecordTx(tx, session.Account.AccountID, record); err != nil {
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
	if err := insertPostgresAccountTx(tx, session); err != nil {
		return AccountSession{}, err
	}
	if err := insertPostgresAccountSessionRecordTx(tx, accountID, record); err != nil {
		return AccountSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return AccountSession{}, err
	}
	return session, nil
}

func (s *PostgresAccountStore) RegisterGuestAccount(guest GuestProfile, handle, email, password string) (AccountSession, error) {
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

	if session, ok, err := lookupPostgresAccountSessionByGuestTx(tx, resolvedGuestID); err != nil {
		return AccountSession{}, err
	} else if ok {
		if normalizedHandle != session.Account.Handle {
			return AccountSession{}, ErrAccountHandleTaken
		}
		if existingAccountID, exists, err := lookupPostgresCredentialOwnerByEmailTx(tx, normalizedEmail); err != nil {
			return AccountSession{}, err
		} else if exists && existingAccountID != session.Account.AccountID {
			return AccountSession{}, ErrAccountEmailTaken
		}

		touchAccountPresence(&session.Account, now)
		record := newAccountSessionRecord(now)
		session.SessionToken = record.SessionToken
		session.ExpiresAt = record.ExpiresAt
		if err := updatePostgresAccountSessionTx(tx, session); err != nil {
			return AccountSession{}, err
		}
		if err := insertPostgresAccountSessionRecordTx(tx, session.Account.AccountID, record); err != nil {
			return AccountSession{}, err
		}
		existingState, err := lookupPostgresAccountAuthStateByAccountIDTx(tx, session.Account.AccountID)
		if err != nil {
			return AccountSession{}, err
		}
		emailChanged := !strings.EqualFold(strings.TrimSpace(existingState.Email), normalizedEmail)
		if _, err := tx.Exec(
			`insert into account_credentials(account_id, email, password_hash) values($1, $2, $3) on conflict(account_id) do update set email = excluded.email, password_hash = excluded.password_hash`,
			session.Account.AccountID,
			normalizedEmail,
			passwordHash,
		); err != nil {
			return AccountSession{}, err
		}
		if emailChanged {
			if _, err := tx.Exec(`update account_credentials set email_verified_at = null where account_id = $1`, session.Account.AccountID); err != nil {
				return AccountSession{}, err
			}
			if _, err := tx.Exec(`delete from account_email_verifications where account_id = $1`, session.Account.AccountID); err != nil {
				return AccountSession{}, err
			}
			if _, err := tx.Exec(`delete from account_password_resets where account_id = $1`, session.Account.AccountID); err != nil {
				return AccountSession{}, err
			}
		}
		if err := tx.Commit(); err != nil {
			return AccountSession{}, err
		}
		return session, nil
	}

	existingAccountID, exists, err := lookupPostgresAccountIDByHandleTx(tx, normalizedHandle)
	if err != nil {
		return AccountSession{}, err
	}
	if exists && existingAccountID != "" {
		return AccountSession{}, ErrAccountHandleTaken
	}
	if existingAccountID, exists, err := lookupPostgresCredentialOwnerByEmailTx(tx, normalizedEmail); err != nil {
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
	if err := insertPostgresAccountTx(tx, session); err != nil {
		return AccountSession{}, err
	}
	if err := insertPostgresAccountSessionRecordTx(tx, accountID, record); err != nil {
		return AccountSession{}, err
	}
	if _, err := tx.Exec(
		`insert into account_credentials(account_id, email, password_hash) values($1, $2, $3)`,
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

func (s *PostgresAccountStore) ResumeAccount(accountID, sessionToken string) (AccountSession, error) {
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

	session, ok, err := lookupPostgresAccountSessionByID(tx, resolvedAccountID)
	if err != nil {
		return AccountSession{}, err
	}
	if !ok {
		return AccountSession{}, os.ErrNotExist
	}
	record, ok, err := lookupPostgresActiveAccountSessionRecord(tx, resolvedAccountID, sessionToken, time.Now().UTC())
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
	if err := upsertPostgresAccountSessionRecord(tx, resolvedAccountID, record); err != nil {
		return AccountSession{}, err
	}
	if _, err := tx.Exec(`update accounts set last_seen_at = $1, last_active_at = $2, session_token = $3, session_expires_at = $4 where account_id = $5`, session.Account.LastSeenAt, nullableTime(session.Account.LastActiveAt), session.SessionToken, session.ExpiresAt, resolvedAccountID); err != nil {
		return AccountSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return AccountSession{}, err
	}
	return session, nil
}

func (s *PostgresAccountStore) TouchPresence(accountID, sessionToken string) (AccountSession, error) {
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

	session, ok, err := lookupPostgresAccountSessionByID(tx, resolvedAccountID)
	if err != nil {
		return AccountSession{}, err
	}
	if !ok {
		return AccountSession{}, os.ErrNotExist
	}
	record, ok, err := lookupPostgresActiveAccountSessionRecord(tx, resolvedAccountID, sessionToken, time.Now().UTC())
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
	if err := upsertPostgresAccountSessionRecord(tx, resolvedAccountID, record); err != nil {
		return AccountSession{}, err
	}
	if _, err := tx.Exec(`update accounts set last_seen_at = $1, last_active_at = $2, session_token = $3, session_expires_at = $4 where account_id = $5`, session.Account.LastSeenAt, nullableTime(session.Account.LastActiveAt), session.SessionToken, session.ExpiresAt, resolvedAccountID); err != nil {
		return AccountSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return AccountSession{}, err
	}
	return session, nil
}

func (s *PostgresAccountStore) GetAccountAuthOverview(accountID, sessionToken string) (AccountAuthOverview, error) {
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

	session, ok, err := lookupPostgresAccountSessionByID(tx, resolvedAccountID)
	if err != nil {
		return AccountAuthOverview{}, err
	}
	if !ok {
		return AccountAuthOverview{}, os.ErrNotExist
	}
	record, ok, err := lookupPostgresActiveAccountSessionRecord(tx, resolvedAccountID, sessionToken, time.Now().UTC())
	if err != nil {
		return AccountAuthOverview{}, err
	}
	if !ok {
		return AccountAuthOverview{}, ErrUnauthorizedAccountSession
	}
	record.LastSeenAt = time.Now().UTC()
	record.ExpiresAt = time.Now().UTC().Add(defaultAccountSessionTTL)
	if err := upsertPostgresAccountSessionRecord(tx, resolvedAccountID, record); err != nil {
		return AccountAuthOverview{}, err
	}
	authState, err := lookupPostgresAccountAuthStateByAccountIDTx(tx, resolvedAccountID)
	if err != nil {
		return AccountAuthOverview{}, err
	}
	if pending, ok, err := lookupPostgresPendingEmailVerificationTx(tx, resolvedAccountID, time.Now().UTC()); err != nil {
		return AccountAuthOverview{}, err
	} else if ok {
		authState.EmailVerifications = []AccountEmailVerificationRecord{pending}
	}
	if err := tx.Commit(); err != nil {
		return AccountAuthOverview{}, err
	}
	return buildAccountAuthOverview(session.Account, authState, time.Now().UTC()), nil
}

func (s *PostgresAccountStore) ListAccountSessions(accountID, sessionToken string) (AccountSessionOverview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return AccountSessionOverview{}, os.ErrInvalid
	}

	session, ok, err := lookupPostgresAccountSessionByID(s.db, resolvedAccountID)
	if err != nil {
		return AccountSessionOverview{}, err
	}
	if !ok {
		return AccountSessionOverview{}, os.ErrNotExist
	}
	if _, ok, err := lookupPostgresActiveAccountSessionRecord(s.db, resolvedAccountID, sessionToken, time.Now().UTC()); err != nil {
		return AccountSessionOverview{}, err
	} else if !ok {
		return AccountSessionOverview{}, ErrUnauthorizedAccountSession
	}
	records, err := listPostgresActiveAccountSessions(s.db, resolvedAccountID, time.Now().UTC())
	if err != nil {
		return AccountSessionOverview{}, err
	}
	return buildAccountSessionOverview(session.Account, records), nil
}

func (s *PostgresAccountStore) RevokeAccountSession(accountID, sessionToken, revokeToken string) error {
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

	if _, ok, err := lookupPostgresAccountSessionByID(tx, resolvedAccountID); err != nil {
		return err
	} else if !ok {
		return os.ErrNotExist
	}
	if _, ok, err := lookupPostgresActiveAccountSessionRecord(tx, resolvedAccountID, sessionToken, time.Now().UTC()); err != nil {
		return err
	} else if !ok {
		return ErrUnauthorizedAccountSession
	}
	if err := deletePostgresAccountSessionRecord(tx, resolvedAccountID, revokeToken); err != nil {
		return err
	}
	if err := syncPostgresLegacyAccountSession(tx, tx, resolvedAccountID, time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PostgresAccountStore) RevokeOtherAccountSessions(accountID, sessionToken string) error {
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

	if _, ok, err := lookupPostgresAccountSessionByID(tx, resolvedAccountID); err != nil {
		return err
	} else if !ok {
		return os.ErrNotExist
	}
	if _, ok, err := lookupPostgresActiveAccountSessionRecord(tx, resolvedAccountID, sessionToken, time.Now().UTC()); err != nil {
		return err
	} else if !ok {
		return ErrUnauthorizedAccountSession
	}
	if err := deleteOtherPostgresAccountSessions(tx, resolvedAccountID, sessionToken); err != nil {
		return err
	}
	if err := syncPostgresLegacyAccountSession(tx, tx, resolvedAccountID, time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PostgresAccountStore) EnablePasswordLogin(accountID, sessionToken, email, password string) (AccountSession, error) {
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

	session, ok, err := lookupPostgresAccountSessionByID(tx, resolvedAccountID)
	if err != nil {
		return AccountSession{}, err
	}
	if !ok {
		return AccountSession{}, os.ErrNotExist
	}
	record, ok, err := lookupPostgresActiveAccountSessionRecord(tx, resolvedAccountID, sessionToken, time.Now().UTC())
	if err != nil {
		return AccountSession{}, err
	}
	if !ok {
		return AccountSession{}, ErrUnauthorizedAccountSession
	}
	if existingAccountID, exists, err := lookupPostgresCredentialOwnerByEmailTx(tx, normalizedEmail); err != nil {
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
	if err := updatePostgresAccountSessionTx(tx, session); err != nil {
		return AccountSession{}, err
	}
	if err := upsertPostgresAccountSessionRecord(tx, resolvedAccountID, record); err != nil {
		return AccountSession{}, err
	}
	existingState, err := lookupPostgresAccountAuthStateByAccountIDTx(tx, resolvedAccountID)
	if err != nil {
		return AccountSession{}, err
	}
	emailChanged := !strings.EqualFold(strings.TrimSpace(existingState.Email), normalizedEmail)
	if _, err := tx.Exec(
		`insert into account_credentials(account_id, email, password_hash) values($1, $2, $3) on conflict(account_id) do update set email = excluded.email, password_hash = excluded.password_hash`,
		resolvedAccountID,
		normalizedEmail,
		passwordHash,
	); err != nil {
		return AccountSession{}, err
	}
	if emailChanged {
		if _, err := tx.Exec(`update account_credentials set email_verified_at = null where account_id = $1`, resolvedAccountID); err != nil {
			return AccountSession{}, err
		}
		if _, err := tx.Exec(`delete from account_email_verifications where account_id = $1`, resolvedAccountID); err != nil {
			return AccountSession{}, err
		}
		if _, err := tx.Exec(`delete from account_password_resets where account_id = $1`, resolvedAccountID); err != nil {
			return AccountSession{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return AccountSession{}, err
	}
	return session, nil
}

func (s *PostgresAccountStore) StartEmailVerification(accountID, sessionToken string) (AccountEmailVerificationChallenge, error) {
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

	if _, ok, err := lookupPostgresAccountSessionByID(tx, resolvedAccountID); err != nil {
		return AccountEmailVerificationChallenge{}, err
	} else if !ok {
		return AccountEmailVerificationChallenge{}, os.ErrNotExist
	}
	if _, ok, err := lookupPostgresActiveAccountSessionRecord(tx, resolvedAccountID, sessionToken, time.Now().UTC()); err != nil {
		return AccountEmailVerificationChallenge{}, err
	} else if !ok {
		return AccountEmailVerificationChallenge{}, ErrUnauthorizedAccountSession
	}
	authState, err := lookupPostgresAccountAuthStateByAccountIDTx(tx, resolvedAccountID)
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
	if _, err := tx.Exec(`delete from account_email_verifications where account_id = $1`, resolvedAccountID); err != nil {
		return AccountEmailVerificationChallenge{}, err
	}
	for _, item := range authState.EmailVerifications {
		if err := insertPostgresAccountEmailVerificationTx(tx, resolvedAccountID, item); err != nil {
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

func (s *PostgresAccountStore) VerifyEmail(accountID, token string) (AccountAuthOverview, error) {
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

	session, ok, err := lookupPostgresAccountSessionByID(tx, resolvedAccountID)
	if err != nil {
		return AccountAuthOverview{}, err
	}
	if !ok {
		return AccountAuthOverview{}, os.ErrNotExist
	}
	record, ok, err := lookupPostgresAccountEmailVerificationByTokenTx(tx, resolvedAccountID, token, time.Now().UTC())
	if err != nil {
		return AccountAuthOverview{}, err
	}
	if !ok {
		return AccountAuthOverview{}, ErrUnauthorizedAccountEmailVerification
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(`update account_email_verifications set used_at = $1, updated_at = $2 where account_id = $3 and token = $4`, now, now, resolvedAccountID, strings.TrimSpace(token)); err != nil {
		return AccountAuthOverview{}, err
	}
	if _, err := tx.Exec(`update account_credentials set email = $1, email_verified_at = $2 where account_id = $3`, record.Email, now, resolvedAccountID); err != nil {
		return AccountAuthOverview{}, err
	}
	if _, err := tx.Exec(`delete from account_password_resets where account_id = $1`, resolvedAccountID); err != nil {
		return AccountAuthOverview{}, err
	}
	authState, err := lookupPostgresAccountAuthStateByAccountIDTx(tx, resolvedAccountID)
	if err != nil {
		return AccountAuthOverview{}, err
	}
	if err := tx.Commit(); err != nil {
		return AccountAuthOverview{}, err
	}
	return buildAccountAuthOverview(session.Account, authState, now), nil
}

func (s *PostgresAccountStore) LoginWithPassword(identifier, password string) (AccountSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	accountID, passwordHash, ok, err := lookupPostgresAccountCredentialsByIdentifier(s.db, identifier)
	if err != nil {
		return AccountSession{}, err
	}
	if !ok || !verifyAccountPassword(password, passwordHash) {
		return AccountSession{}, ErrUnauthorizedAccountCredentials
	}

	session, ok, err := lookupPostgresAccountSessionByID(s.db, accountID)
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
	if err := insertPostgresAccountSessionRecordTx(tx, accountID, record); err != nil {
		return AccountSession{}, err
	}
	if _, err := tx.Exec(`update accounts set last_seen_at = $1, last_active_at = $2, session_token = $3, session_expires_at = $4 where account_id = $5`, session.Account.LastSeenAt, nullableTime(session.Account.LastActiveAt), session.SessionToken, session.ExpiresAt, accountID); err != nil {
		return AccountSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return AccountSession{}, err
	}
	return session, nil
}

func (s *PostgresAccountStore) StartPasswordReset(identifier string) (AccountPasswordResetChallenge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	accountID, email, passwordHash, verifiedAt, ok, err := lookupPostgresAccountCredentialStateByIdentifier(s.db, identifier)
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

	authState, err := lookupPostgresAccountAuthStateByAccountIDTx(tx, accountID)
	if err != nil {
		return AccountPasswordResetChallenge{}, err
	}
	authState, record := issueAccountPasswordReset(authState, time.Now().UTC())
	if _, err := tx.Exec(`delete from account_password_resets where account_id = $1`, accountID); err != nil {
		return AccountPasswordResetChallenge{}, err
	}
	for _, item := range authState.PasswordResets {
		if err := insertPostgresAccountPasswordResetTx(tx, accountID, item); err != nil {
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

func (s *PostgresAccountStore) ResetPassword(accountID, token, password string) (AccountSession, error) {
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

	session, ok, err := lookupPostgresAccountSessionByID(tx, resolvedAccountID)
	if err != nil {
		return AccountSession{}, err
	}
	if !ok {
		return AccountSession{}, os.ErrNotExist
	}
	authState, err := lookupPostgresAccountAuthStateByAccountIDTx(tx, resolvedAccountID)
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
	if err := insertPostgresAccountSessionRecordTx(tx, resolvedAccountID, record); err != nil {
		return AccountSession{}, err
	}
	if _, err := tx.Exec(`update accounts set last_seen_at = $1, last_active_at = $2, session_token = $3, session_expires_at = $4 where account_id = $5`, session.Account.LastSeenAt, nullableTime(session.Account.LastActiveAt), session.SessionToken, session.ExpiresAt, resolvedAccountID); err != nil {
		return AccountSession{}, err
	}
	if _, err := tx.Exec(`update account_credentials set password_hash = $1 where account_id = $2`, passwordHash, resolvedAccountID); err != nil {
		return AccountSession{}, err
	}
	if _, err := tx.Exec(`delete from account_password_resets where account_id = $1`, resolvedAccountID); err != nil {
		return AccountSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return AccountSession{}, err
	}
	return session, nil
}

func (s *PostgresAccountStore) LogoutAccount(accountID, sessionToken string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedAccountID := strings.TrimSpace(accountID)
	if resolvedAccountID == "" {
		return os.ErrInvalid
	}
	if _, ok, err := lookupPostgresAccountSessionByID(s.db, resolvedAccountID); err != nil {
		return err
	} else if !ok {
		return os.ErrNotExist
	}
	if _, ok, err := lookupPostgresActiveAccountSessionRecord(s.db, resolvedAccountID, sessionToken, time.Now().UTC()); err != nil {
		return err
	} else if !ok {
		return ErrUnauthorizedAccountSession
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := deletePostgresAccountSessionRecord(tx, resolvedAccountID, sessionToken); err != nil {
		return err
	}
	if err := syncPostgresLegacyAccountSession(tx, tx, resolvedAccountID, time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit()
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

func (s *PostgresAccountStore) FinalizeMatch(matchID, whiteAccountID, blackAccountID, winner, queue string, modeID contracts.MatchModeID) (AccountProfile, AccountProfile, bool, error) {
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
	if err := applyAccountMatchResult(&whiteSession.Account, &blackSession.Account, winner); err != nil {
		return AccountProfile{}, AccountProfile{}, false, err
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
	query := `select account_id, handle, primary_guest_id, linked_guest_ids, rating, matches_played, wins, losses, draws, rating_history, created_at, last_seen_at, last_active_at, session_token, session_expires_at from accounts order by last_seen_at desc, created_at desc, account_id asc`
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
			last_active_at timestamptz,
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
		create table if not exists account_credentials (
			account_id text primary key references accounts(account_id) on delete cascade,
			email text not null unique,
			password_hash text not null,
			email_verified_at timestamptz
		);
		create table if not exists account_email_verifications (
			account_id text not null references accounts(account_id) on delete cascade,
			token text primary key,
			email text not null,
			expires_at timestamptz not null,
			created_at timestamptz not null,
			used_at timestamptz,
			updated_at timestamptz not null
		);
		create table if not exists account_password_resets (
			account_id text not null references accounts(account_id) on delete cascade,
			token text primary key,
			expires_at timestamptz not null,
			created_at timestamptz not null,
			used_at timestamptz,
			updated_at timestamptz not null
		);
		create table if not exists account_sessions (
			account_id text not null references accounts(account_id) on delete cascade,
			session_token text primary key,
			expires_at timestamptz not null,
			created_at timestamptz not null,
			last_seen_at timestamptz not null
		);
		alter table accounts add column if not exists rating integer not null default 1200;
		alter table accounts add column if not exists matches_played integer not null default 0;
		alter table accounts add column if not exists wins integer not null default 0;
		alter table accounts add column if not exists losses integer not null default 0;
		alter table accounts add column if not exists draws integer not null default 0;
		alter table accounts add column if not exists rating_history jsonb not null default '[]'::jsonb;
		alter table accounts add column if not exists last_active_at timestamptz;
		alter table account_credentials add column if not exists email_verified_at timestamptz;
		create index if not exists accounts_last_seen_order_idx on accounts (last_seen_at desc, created_at desc, account_id asc);
		create index if not exists account_sessions_account_idx on account_sessions (account_id, last_seen_at desc, created_at desc, session_token asc);
		create index if not exists account_sessions_expires_idx on account_sessions (expires_at);
		create index if not exists account_email_verifications_account_idx on account_email_verifications (account_id, created_at desc);
		create index if not exists account_password_resets_account_idx on account_password_resets (account_id, created_at desc);
		insert into account_sessions(account_id, session_token, expires_at, created_at, last_seen_at)
		select account_id, session_token, session_expires_at, coalesce(last_active_at, last_seen_at, created_at), coalesce(last_active_at, last_seen_at, created_at)
		from accounts
		where session_token is not null
			and session_expires_at is not null
			and not exists (
				select 1 from account_sessions s where s.session_token = accounts.session_token
			)
		on conflict (session_token) do nothing;
	`)
	return err
}

func lookupPostgresAccountSessionByID(queryable interface {
	QueryRow(query string, args ...any) *sql.Row
}, accountID string) (AccountSession, bool, error) {
	return scanPostgresAccountSession(queryable.QueryRow(`select account_id, handle, primary_guest_id, linked_guest_ids, rating, matches_played, wins, losses, draws, rating_history, created_at, last_seen_at, last_active_at, session_token, session_expires_at from accounts where account_id = $1`, accountID))
}

func lookupPostgresAccountSessionByGuest(queryable interface {
	QueryRow(query string, args ...any) *sql.Row
}, guestID string) (AccountSession, bool, error) {
	return scanPostgresAccountSession(queryable.QueryRow(`select a.account_id, a.handle, a.primary_guest_id, a.linked_guest_ids, a.rating, a.matches_played, a.wins, a.losses, a.draws, a.rating_history, a.created_at, a.last_seen_at, a.last_active_at, a.session_token, a.session_expires_at from accounts a join account_guest_links l on l.account_id = a.account_id where l.guest_id = $1`, guestID))
}

func lookupPostgresAccountSessionByGuestTx(tx *sql.Tx, guestID string) (AccountSession, bool, error) {
	return lookupPostgresAccountSessionByGuest(tx, guestID)
}

func lookupPostgresActiveAccountSessionRecord(queryable postgresAccountQueryable, accountID, sessionToken string, now time.Time) (AccountSessionRecord, bool, error) {
	var record AccountSessionRecord
	err := queryable.QueryRow(
		`select session_token, expires_at, created_at, last_seen_at from account_sessions where account_id = $1 and session_token = $2 and expires_at > $3`,
		accountID,
		strings.TrimSpace(sessionToken),
		now.UTC(),
	).Scan(&record.SessionToken, &record.ExpiresAt, &record.CreatedAt, &record.LastSeenAt)
	if err == sql.ErrNoRows {
		return AccountSessionRecord{}, false, nil
	}
	if err != nil {
		return AccountSessionRecord{}, false, err
	}
	record.ExpiresAt = record.ExpiresAt.UTC()
	record.CreatedAt = record.CreatedAt.UTC()
	record.LastSeenAt = record.LastSeenAt.UTC()
	return record, true, nil
}

func listPostgresActiveAccountSessions(queryable postgresAccountQueryable, accountID string, now time.Time) ([]AccountSessionRecord, error) {
	rows, err := queryable.Query(
		`select session_token, expires_at, created_at, last_seen_at from account_sessions where account_id = $1 and expires_at > $2 order by last_seen_at desc, created_at desc, session_token asc`,
		accountID,
		now.UTC(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]AccountSessionRecord, 0)
	for rows.Next() {
		var record AccountSessionRecord
		if err := rows.Scan(&record.SessionToken, &record.ExpiresAt, &record.CreatedAt, &record.LastSeenAt); err != nil {
			return nil, err
		}
		record.ExpiresAt = record.ExpiresAt.UTC()
		record.CreatedAt = record.CreatedAt.UTC()
		record.LastSeenAt = record.LastSeenAt.UTC()
		records = append(records, record)
	}
	return records, rows.Err()
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

func lookupPostgresCredentialOwnerByEmailTx(tx *sql.Tx, email string) (string, bool, error) {
	var accountID string
	err := tx.QueryRow(`select account_id from account_credentials where email = $1`, email).Scan(&accountID)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return accountID, true, nil
}

func lookupPostgresAccountAuthStateByAccountIDTx(queryable interface {
	QueryRow(query string, args ...any) *sql.Row
}, accountID string) (AccountPrivateState, error) {
	var (
		email         sql.NullString
		passwordHash  sql.NullString
		emailVerified sql.NullTime
		state         AccountPrivateState
	)
	if err := queryable.QueryRow(`select email, password_hash, email_verified_at from account_credentials where account_id = $1`, accountID).Scan(&email, &passwordHash, &emailVerified); err != nil {
		if err == sql.ErrNoRows {
			return AccountPrivateState{}, nil
		}
		return AccountPrivateState{}, err
	}
	state.Email = strings.TrimSpace(email.String)
	state.PasswordHash = strings.TrimSpace(passwordHash.String)
	if emailVerified.Valid {
		state.EmailVerifiedAt = emailVerified.Time.UTC()
	}
	return normalizeAccountPrivateState(state), nil
}

func lookupPostgresPendingEmailVerificationTx(queryable postgresAccountQueryable, accountID string, now time.Time) (AccountEmailVerificationRecord, bool, error) {
	var (
		record AccountEmailVerificationRecord
		usedAt sql.NullTime
	)
	err := queryable.QueryRow(
		`select token, email, expires_at, created_at, used_at from account_email_verifications where account_id = $1 and used_at is null and expires_at > $2 order by created_at desc limit 1`,
		accountID,
		now.UTC(),
	).Scan(&record.Token, &record.Email, &record.ExpiresAt, &record.CreatedAt, &usedAt)
	if err == sql.ErrNoRows {
		return AccountEmailVerificationRecord{}, false, nil
	}
	if err != nil {
		return AccountEmailVerificationRecord{}, false, err
	}
	record.ExpiresAt = record.ExpiresAt.UTC()
	record.CreatedAt = record.CreatedAt.UTC()
	if usedAt.Valid {
		parsed := usedAt.Time.UTC()
		record.UsedAt = &parsed
	}
	return record, true, nil
}

func lookupPostgresAccountEmailVerificationByTokenTx(queryable postgresAccountQueryable, accountID, token string, now time.Time) (AccountEmailVerificationRecord, bool, error) {
	var (
		record AccountEmailVerificationRecord
		usedAt sql.NullTime
	)
	err := queryable.QueryRow(
		`select token, email, expires_at, created_at, used_at from account_email_verifications where account_id = $1 and token = $2 and used_at is null and expires_at > $3`,
		accountID,
		strings.TrimSpace(token),
		now.UTC(),
	).Scan(&record.Token, &record.Email, &record.ExpiresAt, &record.CreatedAt, &usedAt)
	if err == sql.ErrNoRows {
		return AccountEmailVerificationRecord{}, false, nil
	}
	if err != nil {
		return AccountEmailVerificationRecord{}, false, err
	}
	record.ExpiresAt = record.ExpiresAt.UTC()
	record.CreatedAt = record.CreatedAt.UTC()
	if usedAt.Valid {
		parsed := usedAt.Time.UTC()
		record.UsedAt = &parsed
	}
	return record, true, nil
}

func insertPostgresAccountEmailVerificationTx(tx *sql.Tx, accountID string, record AccountEmailVerificationRecord) error {
	_, err := tx.Exec(
		`insert into account_email_verifications(account_id, token, email, expires_at, created_at, used_at, updated_at) values($1, $2, $3, $4, $5, $6, $7)`,
		accountID,
		strings.TrimSpace(record.Token),
		strings.TrimSpace(record.Email),
		record.ExpiresAt.UTC(),
		record.CreatedAt.UTC(),
		postgresNullableTimePointer(record.UsedAt),
		record.CreatedAt.UTC(),
	)
	return err
}

func insertPostgresAccountPasswordResetTx(tx *sql.Tx, accountID string, record AccountPasswordResetRecord) error {
	_, err := tx.Exec(
		`insert into account_password_resets(account_id, token, expires_at, created_at, used_at, updated_at) values($1, $2, $3, $4, $5, $6)`,
		accountID,
		strings.TrimSpace(record.Token),
		record.ExpiresAt.UTC(),
		record.CreatedAt.UTC(),
		postgresNullableTimePointer(record.UsedAt),
		record.CreatedAt.UTC(),
	)
	return err
}

func lookupPostgresAccountCredentialStateByIdentifier(queryable interface {
	QueryRow(query string, args ...any) *sql.Row
}, identifier string) (string, string, string, time.Time, bool, error) {
	resolved := strings.ToLower(strings.TrimSpace(identifier))
	if resolved == "" {
		return "", "", "", time.Time{}, false, nil
	}

	var (
		accountID     string
		email         string
		passwordHash  string
		emailVerified sql.NullTime
	)
	err := queryable.QueryRow(`select account_id, email, password_hash, email_verified_at from account_credentials where email = $1`, resolved).Scan(&accountID, &email, &passwordHash, &emailVerified)
	switch err {
	case nil:
	case sql.ErrNoRows:
		normalizedHandle, err := normalizeAccountHandle(resolved)
		if err != nil || normalizedHandle == "" {
			return "", "", "", time.Time{}, false, nil
		}
		err = queryable.QueryRow(`select c.account_id, c.email, c.password_hash, c.email_verified_at from accounts a join account_credentials c on c.account_id = a.account_id where a.handle = $1`, normalizedHandle).Scan(&accountID, &email, &passwordHash, &emailVerified)
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
	if emailVerified.Valid {
		verifiedAt = emailVerified.Time.UTC()
	}
	return accountID, email, passwordHash, verifiedAt, true, nil
}

func lookupPostgresAccountCredentialsByIdentifier(queryable interface {
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
	err := queryable.QueryRow(`select account_id, password_hash from account_credentials where email = $1`, resolved).Scan(&accountID, &passwordHash)
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
	err = queryable.QueryRow(`select a.account_id, c.password_hash from accounts a join account_credentials c on c.account_id = a.account_id where a.handle = $1`, normalizedHandle).Scan(&accountID, &passwordHash)
	if err == sql.ErrNoRows {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	return accountID, passwordHash, true, nil
}

func postgresNullableTimePointer(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.UTC()
}

func insertPostgresAccountSessionRecordTx(tx *sql.Tx, accountID string, record AccountSessionRecord) error {
	_, err := tx.Exec(
		`insert into account_sessions(account_id, session_token, expires_at, created_at, last_seen_at) values($1, $2, $3, $4, $5)`,
		accountID,
		strings.TrimSpace(record.SessionToken),
		record.ExpiresAt.UTC(),
		record.CreatedAt.UTC(),
		record.LastSeenAt.UTC(),
	)
	return err
}

func upsertPostgresAccountSessionRecord(execable postgresAccountExecable, accountID string, record AccountSessionRecord) error {
	_, err := execable.Exec(
		`insert into account_sessions(account_id, session_token, expires_at, created_at, last_seen_at) values($1, $2, $3, $4, $5) on conflict (session_token) do update set account_id = excluded.account_id, expires_at = excluded.expires_at, created_at = excluded.created_at, last_seen_at = excluded.last_seen_at`,
		accountID,
		strings.TrimSpace(record.SessionToken),
		record.ExpiresAt.UTC(),
		record.CreatedAt.UTC(),
		record.LastSeenAt.UTC(),
	)
	return err
}

func deletePostgresAccountSessionRecord(execable postgresAccountExecable, accountID, sessionToken string) error {
	result, err := execable.Exec(`delete from account_sessions where account_id = $1 and session_token = $2`, accountID, strings.TrimSpace(sessionToken))
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

func deleteOtherPostgresAccountSessions(execable postgresAccountExecable, accountID, sessionToken string) error {
	_, err := execable.Exec(`delete from account_sessions where account_id = $1 and session_token <> $2`, accountID, strings.TrimSpace(sessionToken))
	return err
}

func syncPostgresLegacyAccountSession(execable postgresAccountExecable, queryable postgresAccountQueryable, accountID string, now time.Time) error {
	records, err := listPostgresActiveAccountSessions(queryable, accountID, now)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		_, err = execable.Exec(`update accounts set session_token = null, session_expires_at = null where account_id = $1`, accountID)
		return err
	}
	current := records[0]
	_, err = execable.Exec(`update accounts set session_token = $1, session_expires_at = $2 where account_id = $3`, current.SessionToken, current.ExpiresAt.UTC(), accountID)
	return err
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
		`insert into accounts(account_id, handle, primary_guest_id, linked_guest_ids, rating, matches_played, wins, losses, draws, rating_history, created_at, last_seen_at, last_active_at, session_token, session_expires_at) values($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
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
		nullableTime(session.Account.LastActiveAt),
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
		`update accounts set handle = $1, primary_guest_id = $2, linked_guest_ids = $3, rating = $4, matches_played = $5, wins = $6, losses = $7, draws = $8, rating_history = $9, created_at = $10, last_seen_at = $11, last_active_at = $12, session_token = $13, session_expires_at = $14 where account_id = $15`,
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
		nullableTime(session.Account.LastActiveAt),
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
		`update accounts set handle = $1, primary_guest_id = $2, linked_guest_ids = $3, rating = $4, matches_played = $5, wins = $6, losses = $7, draws = $8, rating_history = $9, created_at = $10, last_seen_at = $11, last_active_at = $12, session_token = $13, session_expires_at = $14 where account_id = $15`,
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
		nullableTime(session.Account.LastActiveAt),
		strings.TrimSpace(session.SessionToken),
		session.ExpiresAt,
		session.Account.AccountID,
	)
	return err
}

type postgresAccountScanner interface {
	Scan(dest ...any) error
}

type postgresAccountQueryable interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

type postgresAccountExecable interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func scanPostgresAccountSession(scanner postgresAccountScanner) (AccountSession, bool, error) {
	var (
		session        AccountSession
		linkedGuestIDs []byte
		ratingHistory  []byte
		lastActiveAt   sql.NullTime
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
	if lastActiveAt.Valid {
		session.Account.LastActiveAt = lastActiveAt.Time.UTC()
	}
	session.SessionToken = strings.TrimSpace(sessionToken.String)
	if sessionExpires.Valid {
		session.ExpiresAt = sessionExpires.Time.UTC()
	}
	return session, true, nil
}
