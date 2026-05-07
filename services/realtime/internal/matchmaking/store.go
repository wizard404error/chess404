package matchmaking

type ticketStore interface {
	backend() string
	load() (map[string]Ticket, error)
	persist(map[string]Ticket) error
	close() error
}
