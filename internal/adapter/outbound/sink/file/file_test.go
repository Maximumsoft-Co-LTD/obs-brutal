package file

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
)

func entry(msg string) *domain.LogEntry {
	return &domain.LogEntry{Level: domain.InfoLevel, Msg: msg, Timestamp: time.Now()}
}

// TestOptimal_RotationRenameFailureDoesNotTruncate: when a rotation
// rename fails (directory not writable), the live log file must NOT be
// truncated — its accumulated entries must survive.
func TestOptimal_RotationRenameFailureDoesNotTruncate(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("runs as root; directory permissions are not enforced")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	s := NewOptimalFileSink().(*OptimalFileSink)
	if err := s.Configure(map[string]interface{}{"filename": path, "rotate_size_bytes": int64(64), "max_backups": 3}); err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	for i := 0; i < 5; i++ {
		if err := s.Write(entry("filler entry to grow the file")); err != nil {
			t.Fatal(err)
		}
	}
	before, _ := os.ReadFile(path)
	if len(before) < 64 {
		t.Fatalf("setup: file only %d bytes, expected >= rotate threshold", len(before))
	}

	// Make the directory read-only so os.Rename fails, then trigger
	// rotation with one more write.
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o755)
	_ = s.Write(entry("entry that triggers rotation"))

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("log file gone after failed rotation: %v", err)
	}
	if len(after) < len(before) {
		t.Fatalf("DATA LOSS: file shrank from %d to %d bytes on a failed rotation rename", len(before), len(after))
	}
	if !strings.Contains(string(after), "filler entry") {
		t.Fatal("DATA LOSS: original entries were truncated on failed rotation")
	}
}

// TestOptimal_WedgedAfterFailedReopen: a transient reopen failure during
// rotation must not permanently wedge the sink — once the condition
// clears, Write must recover.
func TestOptimal_WedgedAfterFailedReopen(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "logs2")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sub, "app.log")
	s := NewOptimalFileSink().(*OptimalFileSink)
	_ = s.Configure(map[string]interface{}{"filename": path, "rotate_size_bytes": int64(1), "max_backups": 2})
	defer s.Close()

	if err := s.Write(entry("first")); err != nil {
		t.Fatal(err)
	}
	// Remove the directory so the rotation reopen fails.
	_ = os.RemoveAll(sub)
	_ = s.Write(entry("during outage")) // expected to error

	// Restore the directory; the sink must recover rather than stay wedged.
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := s.Write(entry("after recovery")); err != nil {
		t.Fatalf("sink stayed wedged after the transient condition cleared: %v", err)
	}
}

// TestOptimal_FilenameChangeHonored: changing filename via Configure
// after a Write must route subsequent entries to the new path.
func TestOptimal_FilenameChangeHonored(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old.log")
	newp := filepath.Join(dir, "new.log")
	s := NewOptimalFileSink().(*OptimalFileSink)
	_ = s.Configure(map[string]interface{}{"filename": old})
	defer s.Close()

	_ = s.Write(entry("first"))
	_ = s.Configure(map[string]interface{}{"filename": newp})
	_ = s.Write(entry("second"))

	newContent, err := os.ReadFile(newp)
	if err != nil || !strings.Contains(string(newContent), "second") {
		t.Fatalf("filename change ignored: 'second' did not land in new.log (err=%v)", err)
	}
}

// TestLumberjack_WriteConfigureNoRace: concurrent Write/Configure must be
// race-free and never nil-panic.
func TestLumberjack_WriteConfigureNoRace(t *testing.T) {
	dir := t.TempDir()
	s := NewLumberjackSink().(*LumberjackSink)
	_ = s.Configure(map[string]interface{}{"filename": filepath.Join(dir, "a.log")})
	defer s.Close()

	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = s.Write(entry("x"))
			}
		}
	}()
	go func() {
		defer wg.Done()
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
				i++
				name := filepath.Join(dir, "b.log")
				if i%2 == 0 {
					name = filepath.Join(dir, "c.log")
				}
				_ = s.Configure(map[string]interface{}{"filename": name})
			}
		}
	}()
	time.Sleep(100 * time.Millisecond)
	close(stop)
	wg.Wait()
}
