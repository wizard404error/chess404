package matchmaking

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"

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
		select ticket_id, guest_id, queue, status, rating, created_at, updated_at, matched_at, matched_with, assigned_room
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
			queue        string
			status       string
			createdAt    string
			updatedAt    string
			matchedAt    sql.NullString
			matchedWith  sql.NullString
			assignedRoom sql.NullString
		)
		if err := rows.Scan(
			&ticket.TicketID,
			&ticket.GuestID,
			&queue,
			&status,
			&ticket.Rating,
			&createdAt,
			&updatedAt,
			&matchedAt,
			&matchedWith,
			&assignedRoom,
		); err != nil {
			return nil, err
		}
		ticket.Queue = QueueName(queue)
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
		var assignedRoom any
		if ticket.AssignedRoom != "" {
			assignedRoom = ticket.AssignedRoom
		}
		if _, err := tx.Exec(`
			insert into tickets(ticket_id, guest_id, queue, status, rating, created_at, updated_at, matched_at, matched_with, assigned_room)
			values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			ticket.TicketID,
			ticket.GuestID,
			string(ticket.Queue),
			string(ticket.Status),
			ticket.Rating,
			ticket.CreatedAt.UTC().Format(time.RFC3339Nano),
			ticket.UpdatedAt.UTC().Format(time.RFC3339Nano),
			matchedAt,
			matchedWith,
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
	_, err := s.db.Exec(`
		create table if not exists tickets (
			ticket_id text primary key,
			guest_id text not null,
			queue text not null,
			status text not null,
			rating integer not null,
			created_at text not null,
			updated_at text not null,
			matched_at text,
			matched_with text,
			assigned_room text
		)
	`)
	return err
}
