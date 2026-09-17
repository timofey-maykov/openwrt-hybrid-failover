package delayhistory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreRecordBatchAndPrune(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "delay-history.json")
	s := NewStore(path)

	if err := s.RecordBatch(map[string]SampleInput{
		"ch-a": {DelayMs: 100, OK: true},
		"ch-b": {OK: false, Error: "HTTP status 503"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}

	data, err := s.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 2 {
		t.Fatalf("channels: %d", len(data))
	}
	for _, channel := range data {
		if channel.Channel == "ch-b" && (len(channel.Samples) != 1 || channel.Samples[0].Error != "HTTP status 503") {
			t.Fatalf("failure reason was not persisted: %+v", channel.Samples)
		}
	}

	if err := s.Prune(map[string]struct{}{"ch-a": {}}); err != nil {
		t.Fatal(err)
	}
	data, err = s.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 1 || data[0].Channel != "ch-a" {
		t.Fatalf("after prune: %+v", data)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 {
		t.Fatal("empty file")
	}
}
