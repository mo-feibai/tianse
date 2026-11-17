// Package logging writes a small rotating log next to the config file.
package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

const (
	maxBytes = 1 << 20 // 1 MiB
	keep     = 3
)

var (
	mu  sync.Mutex
	out *os.File
)

// Setup opens tianse.log in dir, rotating a file that already exceeds the size
// limit. A failure to open the log is not fatal: the app keeps running with its
// log discarded, because a permission problem in %APPDATA% must not stop the
// screen from being adjusted.
func Setup(dir string) error {
	mu.Lock()
	defer mu.Unlock()

	path := filepath.Join(dir, "tianse.log")
	rotate(path)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.SetOutput(os.Stderr)
		return err
	}
	out = f
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	return nil
}

// Close flushes and closes the log file.
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if out != nil {
		_ = out.Close()
		out = nil
	}
}

// rotate shifts tianse.log to .1, .1 to .2 and so on, then drops the oldest.
func rotate(path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() < maxBytes {
		return
	}
	for i := keep - 1; i >= 1; i-- {
		from := fmt.Sprintf("%s.%d", path, i)
		to := fmt.Sprintf("%s.%d", path, i+1)
		_ = os.Rename(from, to)
	}
	_ = os.Rename(path, path+".1")
}
