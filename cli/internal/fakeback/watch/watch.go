// Package watch drives fsnotify-based hot-reload of db.json and the
// scenarios directory. On Write / Create / Rename events under the
// watched paths, the callback fires after a short debounce so a
// burst of editor saves doesn't trigger N reloads.
package watch

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Debounce is the window the watcher waits after the last event
// before firing the callback. 200ms matches the dev-experience the
// `tavora dev` source watcher already uses.
const Debounce = 200 * time.Millisecond

// Run starts an fsnotify loop and calls onChange when any path under
// watchPaths receives a write/create/rename/remove event. Blocks
// until ctx is cancelled. Returns the first fsnotify error
// encountered (or nil on clean shutdown).
//
// Errors during onChange are logged but don't terminate the loop —
// a malformed save shouldn't kill the watcher.
func Run(ctx context.Context, log *slog.Logger, watchPaths []string, onChange func() error) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()

	added := 0
	for _, p := range watchPaths {
		if p == "" {
			continue
		}
		// fsnotify watches directories; watch the parent of files we
		// care about (writing usually creates a temp+rename, which the
		// file-level watch loses).
		target := p
		if !isDir(p) {
			target = filepath.Dir(p)
		}
		if err := w.Add(target); err != nil {
			log.Warn("watch add failed", "path", target, "err", err)
			continue
		}
		added++
	}
	if added == 0 {
		log.Warn("watcher started with zero paths; hot-reload disabled")
		<-ctx.Done()
		return nil
	}

	var timer *time.Timer
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-w.Errors:
			log.Warn("fsnotify error", "err", err)
		case ev, ok := <-w.Events:
			if !ok {
				return nil
			}
			if !isReloadEvent(ev.Op) {
				continue
			}
			log.Debug("fs change", "name", ev.Name, "op", ev.Op.String())
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(Debounce, func() {
				if err := onChange(); err != nil {
					log.Warn("reload failed", "err", err)
				}
			})
		}
	}
}

func isReloadEvent(op fsnotify.Op) bool {
	return op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) != 0
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	if err != nil {
		return false
	}
	return info.IsDir()
}
