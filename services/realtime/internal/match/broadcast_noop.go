package match

type NoopBroadcaster struct{}

func (NoopBroadcaster) Publish(matchID string, data []byte) error  { return nil }
func (NoopBroadcaster) Subscribe(matchID string) <-chan []byte     { return nil }
func (NoopBroadcaster) Unsubscribe(matchID string)                {}
func (NoopBroadcaster) Ping() error                               { return nil }
func (NoopBroadcaster) Close() error                              { return nil }
