package matchmaking

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chess404/realtime/internal/contracts"
	_ "modernc.org/sqlite"
)

type sqliteTicketStore struct {
	db *sql.DB
}

func newSQLiteTicketStore(path string) (*sqliteTicketStore, error) {
	if path != "" && path != ":memory:" && !strings.HasPrefix(path, "file:") {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	store := &sqliteTicketStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *sqliteTicketStore) backend() string {
	return "sqlite"
}

func (s *sqliteTicketStore) load() (map[string]Ticket, error) {
	rows, err := s.db.Query(`
		select ticket_id, guest_id, account_id, display_name, queue, mode_id, status, rating, created_at, updated_at, matched_at, matched_with, seat_color, opponent_name, assigned_room
		from tickets
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tickets := make(map[string]Ticket)
	for rows.Next() {
		var (
			ticket       Ticket
			accountID    sql.NullString
			displayName  sql.NullString
			queue        string
			modeID       sql.NullString
			status       string
			createdAt    string
			updatedAt    string
			matchedAt    sql.NullString
			matchedWith  sql.NullString
			seatColor    sql.NullString
			opponentName sql.NullString
			assignedRoom sql.NullString
		)
		if err := rows.Scan(
			&ticket.TicketID,
			&ticket.GuestID,
			&accountID,
			&displayName,
			&queue,
			&modeID,
			&status,
			&ticket.Rating,
			&createdAt,
			&updatedAt,
			&matchedAt,
			&matchedWith,
			&seatColor,
			&opponentName,
			&assignedRoom,
		); err != nil {
			return nil, err
		}
		if displayName.Valid {
			ticket.DisplayName = displayName.String
		}
		if accountID.Valid {
			ticket.AccountID = accountID.String
		}
		ticket.Queue = QueueName(queue)
		if modeID.Valid {
			ticket.ModeID = normalizeModeID(contracts.MatchModeID(modeID.String))
		} else {
			ticket.ModeID = normalizeModeID(ticket.ModeID)
		}
		ticket.Status = TicketStatus(status)
		parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, err
		}
		parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
		if err != nil {
			return nil, err
		}
		ticket.CreatedAt = parsedCreatedAt
		ticket.UpdatedAt = parsedUpdatedAt
		if matchedAt.Valid {
			parsedMatchedAt, err := time.Parse(time.RFC3339Nano, matchedAt.String)
			if err != nil {
				return nil, err
			}
			ticket.MatchedAt = &parsedMatchedAt
		}
		if matchedWith.Valid {
			ticket.MatchedWith = matchedWith.String
		}
		if seatColor.Valid {
			ticket.SeatColor = seatColor.String
		}
		if opponentName.Valid {
			ticket.OpponentName = opponentName.String
		}
		if assignedRoom.Valid {
			ticket.AssignedRoom = assignedRoom.String
		}
		tickets[ticket.TicketID] = ticket
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tickets, nil
}

func (s *sqliteTicketStore) persist(tickets map[string]Ticket) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.Exec(`delete from tickets`); err != nil {
		return err
	}

	for _, ticket := range tickets {
		var matchedAt any
		if ticket.MatchedAt != nil {
			matchedAt = ticket.MatchedAt.UTC().Format(time.RFC3339Nano)
		}
		var matchedWith any
		if ticket.MatchedWith != "" {
			matchedWith = ticket.MatchedWith
		}
		var accountID any
		if ticket.AccountID != "" {
			accountID = ticket.AccountID
		}
		var modeID any
		if ticket.ModeID != "" {
			modeID = string(ticket.ModeID)
		}
		var seatColor any
		if ticket.SeatColor != "" {
			seatColor = ticket.SeatColor
		}
		var opponentName any
		if ticket.OpponentName != "" {
			opponentName = ticket.OpponentName
		}
		var assignedRoom any
		if ticket.AssignedRoom != "" {
			assignedRoom = ticket.AssignedRoom
		}
		if _, err := tx.Exec(`
			insert into tickets(ticket_id, guest_id, account_id, display_name, queue, mode_id, status, rating, created_at, updated_at, matched_at, matched_with, seat_color, opponent_name, assigned_room)
			values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			ticket.TicketID,
			ticket.GuestID,
			accountID,
			ticket.DisplayName,
			string(ticket.Queue),
			modeID,
			string(ticket.Status),
			ticket.Rating,
			ticket.CreatedAt.UTC().Format(time.RFC3339Nano),
			ticket.UpdatedAt.UTC().Format(time.RFC3339Nano),
			matchedAt,
			matchedWith,
			seatColor,
			opponentName,
			assignedRoom,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *sqliteTicketStore) close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *sqliteTicketStore) init() error {
	_, _ = s.db.Exec(`PRAGMA journal_mode=WAL`)
	_, _ = s.db.Exec(`PRAGMA busy_timeout=5000`)
	_, err := s.db.Exec(`
		create table if not exists tickets (
			ticket_id text primary key,
			guest_id text not null,
			account_id text,
			display_name text,
			queue text not null,
			mode_id text,
			status text not null,
			rating integer not null,
			created_at text not null,
			updated_at text not null,
			matched_at text,
			matched_with text,
			seat_color text,
			opponent_name text,
			assigned_room text
		)
	`)
	if err != nil {
		return err
	}
	migrations := []string{
		`alter table tickets add column account_id text`,
		`alter table tickets add column display_name text`,
		`alter table tickets add column mode_id text`,
		`alter table tickets add column seat_color text`,
		`alter table tickets add column opponent_name text`,
	}
	for _, stmt := range migrations {
		if _, err := s.db.Exec(stmt); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
			return err
		}
	}
	return nil
}
