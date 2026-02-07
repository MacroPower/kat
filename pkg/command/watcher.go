package command

import (
	"fmt"

	"github.com/fsnotify/fsnotify"
)

// Watcher abstracts filesystem event notification so that consumers can
// optionally inject alternate implementations instead of relying on OS-level
// notifications.
type Watcher interface {
	Add(path string) error
	Remove(path string) error
	Close() error
	Events() <-chan fsnotify.Event
	Errors() <-chan error
}

// fsnotifyWatcher adapts [fsnotify.Watcher] to the [Watcher] interface.
type fsnotifyWatcher struct {
	w *fsnotify.Watcher
}

func newFSNotifyWatcher() (*fsnotifyWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create watcher: %w", err)
	}

	return &fsnotifyWatcher{w: w}, nil
}

func (f *fsnotifyWatcher) Add(path string) error {
	err := f.w.Add(path)
	if err != nil {
		return fmt.Errorf("add watch: %w", err)
	}

	return nil
}

func (f *fsnotifyWatcher) Remove(path string) error {
	err := f.w.Remove(path)
	if err != nil {
		return fmt.Errorf("remove watch: %w", err)
	}

	return nil
}

func (f *fsnotifyWatcher) Close() error {
	err := f.w.Close()
	if err != nil {
		return fmt.Errorf("close watcher: %w", err)
	}

	return nil
}

func (f *fsnotifyWatcher) Events() <-chan fsnotify.Event { return f.w.Events }
func (f *fsnotifyWatcher) Errors() <-chan error          { return f.w.Errors }
