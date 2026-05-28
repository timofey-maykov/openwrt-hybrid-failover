package audit

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

type Event struct {
	Timestamp string `json:"ts"`
	UserID    int64  `json:"user_id"`
	Action    string `json:"action"`
	Result    string `json:"result"`
	Details   string `json:"details,omitempty"`
}

type Logger struct {
	path string
	mu   sync.Mutex
}

func New(path string) Logger {
	return Logger{path: path}
}

func (l Logger) Write(event Event) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	event.Timestamp = time.Now().UTC().Format(time.RFC3339)
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	return enc.Encode(event)
}
