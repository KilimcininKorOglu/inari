// Package core provides file-system watching for incremental re-indexing.
//
// Uses fsnotify with manual debounce to watch for file changes and emit
// batched events for re-indexing.
package core

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches the file system for changes to supported source files.
//
// Respects ignore patterns and filters events to only supported file
// extensions. Debounces rapid changes into batched events.
type Watcher struct {
	// Project root directory being watched.
	projectRoot string
	// Patterns to ignore (from config + .gitignore).
	ignorePatterns []string
	// Supported file extensions (without the dot, e.g. "ts", "tsx", "cs").
	supportedExtensions []string
	// Debounce duration for file events.
	debounceDuration time.Duration
}

// NewWatcher creates a new file watcher.
//
// Parameters:
//   - projectRoot: the root directory to watch recursively
//   - ignorePatterns: patterns to exclude (e.g. "node_modules", "dist")
//   - supportedExtensions: file extensions to include (without dot)
//   - debounceDuration: how long to wait before emitting a batch
func NewWatcher(
	projectRoot string,
	ignorePatterns []string,
	supportedExtensions []string,
	debounceDuration time.Duration,
) *Watcher {
	return &Watcher{
		projectRoot:         projectRoot,
		ignorePatterns:      ignorePatterns,
		supportedExtensions: supportedExtensions,
		debounceDuration:    debounceDuration,
	}
}

// Start begins watching and returns a channel of batched file change paths,
// a stop function, and any initialization error.
//
// The returned channel emits []string batches whenever files matching the
// supported extensions change. Only Write and Create events are processed.
// The stop function should be called to clean up resources.
func (w *Watcher) Start() (<-chan []string, func(), error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Add the project root and all subdirectories recursively.
	err = filepath.Walk(w.projectRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			// Skip ignored directories to avoid unnecessary watches.
			if IsIgnored(path, w.projectRoot, w.ignorePatterns) {
				return filepath.SkipDir
			}
			return watcher.Add(path)
		}
		return nil
	})
	if err != nil {
		watcher.Close()
		return nil, nil, fmt.Errorf("failed to watch directory %s: %w", w.projectRoot, err)
	}

	out := make(chan []string, 16)
	var stopOnce sync.Once
	done := make(chan struct{})

	stop := func() {
		stopOnce.Do(func() {
			close(done)
			watcher.Close()
		})
	}

	go w.eventLoop(watcher, out, done)

	return out, stop, nil
}

// eventLoop is the internal goroutine that receives fsnotify events,
// deduplicates them, applies filters, debounces, and sends batches.
func (w *Watcher) eventLoop(watcher *fsnotify.Watcher, out chan<- []string, done <-chan struct{}) {
	defer close(out)

	pending := make(map[string]struct{})
	var mu sync.Mutex
	timer := time.NewTimer(0)
	if !timer.Stop() {
		<-timer.C
	}
	timerActive := false

	for {
		select {
		case <-done:
			// Drain any remaining pending events before exiting.
			mu.Lock()
			if len(pending) > 0 {
				batch := drainPending(pending)
				mu.Unlock()
				select {
				case out <- batch:
				default:
				}
			} else {
				mu.Unlock()
			}
			timer.Stop()
			return

		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			// Only process Write and Create events (analogous to DebouncedEventKind::Any).
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}

			path := event.Name
			if !IsSupportedFile(path, w.supportedExtensions) {
				continue
			}
			if IsIgnored(path, w.projectRoot, w.ignorePatterns) {
				continue
			}

			mu.Lock()
			pending[path] = struct{}{}
			mu.Unlock()

			// Reset debounce timer on each qualifying event.
			if timerActive {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
			}
			timer.Reset(w.debounceDuration)
			timerActive = true

		case <-timer.C:
			timerActive = false
			mu.Lock()
			if len(pending) == 0 {
				mu.Unlock()
				continue
			}
			batch := drainPending(pending)
			mu.Unlock()

			select {
			case out <- batch:
			case <-done:
				return
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("File watcher error: %v", err)
		}
	}
}

// drainPending extracts all keys from the pending map, clears it,
// and returns the keys as a slice.
func drainPending(pending map[string]struct{}) []string {
	batch := make([]string, 0, len(pending))
	for p := range pending {
		batch = append(batch, p)
	}
	for p := range pending {
		delete(pending, p)
	}
	return batch
}

// IsSupportedFile checks if a file has one of the supported extensions.
func IsSupportedFile(path string, supportedExtensions []string) bool {
	ext := filepath.Ext(path)
	if ext == "" {
		return false
	}
	// Remove the leading dot.
	ext = ext[1:]
	for _, supported := range supportedExtensions {
		if ext == supported {
			return true
		}
	}
	return false
}

// IsIgnored checks if a file path should be ignored based on ignore patterns.
//
// Uses simple component-level matching: if any path component matches an
// ignore pattern, the file is ignored. The .inari/ directory is always
// ignored regardless of patterns.
func IsIgnored(path string, projectRoot string, ignorePatterns []string) bool {
	relPath, err := filepath.Rel(projectRoot, path)
	if err != nil {
		return false
	}

	// Normalise to forward slashes for consistent component splitting.
	relPath = filepath.ToSlash(relPath)

	// Always ignore the .inari directory.
	if relPath == ".inari" || strings.HasPrefix(relPath, ".inari/") {
		return true
	}

	// Check each path component against ignore patterns.
	components := strings.Split(relPath, "/")
	for _, component := range components {
		for _, pattern := range ignorePatterns {
			if component == pattern {
				return true
			}
		}
	}

	return false
}

// WatchLock provides lock file management for preventing concurrent watchers.
type WatchLock struct {
	lockPath string
}

// NewWatchLock creates a new lock file manager for the given .inari/ directory.
func NewWatchLock(inariDir string) *WatchLock {
	return &WatchLock{
		lockPath: filepath.Join(inariDir, ".watch.lock"),
	}
}

// Acquire acquires the watch lock. Returns an error if another watcher is running.
//
// Uses O_CREATE|O_EXCL for atomic lock file creation to avoid TOCTOU races.
// If the lock file already exists, checks whether the recorded PID is still
// alive. Stale lock files (dead PID or invalid content) are removed and
// creation is retried once.
func (wl *WatchLock) Acquire() error {
	if err := os.MkdirAll(filepath.Dir(wl.lockPath), 0o755); err != nil {
		return fmt.Errorf("failed to create lock directory: %w", err)
	}

	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(wl.lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			// Exclusively created — write our PID.
			_, writeErr := f.WriteString(strconv.Itoa(os.Getpid()))
			f.Close()
			if writeErr != nil {
				os.Remove(wl.lockPath)
				return fmt.Errorf("failed to write lock file: %w", writeErr)
			}
			return nil
		}

		if !os.IsExist(err) {
			return fmt.Errorf("failed to create lock file: %w", err)
		}

		// Lock file exists — check if the owning process is still alive.
		content, readErr := os.ReadFile(wl.lockPath)
		if readErr != nil {
			log.Printf("Cannot read lock file: %v. Treating as stale.", readErr)
		} else {
			pidStr := strings.TrimSpace(string(content))
			if pidStr != "" {
				pid, parseErr := strconv.Atoi(pidStr)
				if parseErr != nil {
					log.Printf("Lock file contains invalid PID '%s': %v. Treating as stale.", pidStr, parseErr)
				} else if isProcessAlive(pid) {
					return fmt.Errorf(
						"another watcher is running (PID %d). "+
							"Stop it first or remove .inari/.watch.lock",
						pid,
					)
				}
			}
		}

		// Stale lock — remove and retry with exclusive create.
		log.Printf("Removing stale watch lock file")
		os.Remove(wl.lockPath)
	}

	return fmt.Errorf("failed to acquire watch lock after retry")
}

// Release removes the lock file.
func (wl *WatchLock) Release() {
	if _, err := os.Stat(wl.lockPath); err == nil {
		if err := os.Remove(wl.lockPath); err != nil {
			fmt.Fprintf(os.Stderr,
				"Warning: failed to remove watch lock file: %v. "+
					"You may need to delete .inari/.watch.lock manually.\n", err)
		}
	}
}

// LockPath returns the path to the lock file.
func (wl *WatchLock) LockPath() string {
	return wl.lockPath
}

// isProcessAlive checks if a process with the given PID is still alive.
//
// On Unix, sends signal 0 via syscall.Kill which checks for existence
// without actually sending a signal.
func isProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 checks whether the process exists without sending a real signal.
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
