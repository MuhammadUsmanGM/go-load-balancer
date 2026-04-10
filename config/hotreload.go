package config

import (
	"context"
	"fmt"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// HotReloader watches config file for changes and reloads it.
type HotReloader struct {
	path     string
	onChange func(cfg *Config)
	mu       sync.RWMutex
	current  *Config
	watcher  *fsnotify.Watcher
}

// NewHotReloader creates a new config hot-reloader.
func NewHotReloader(path string, onChange func(cfg *Config)) (*HotReloader, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("creating watcher: %w", err)
	}

	return &HotReloader{
		path:     path,
		onChange: onChange,
		watcher:  watcher,
	}, nil
}

// Start begins watching for config changes.
func (hr *HotReloader) Start(ctx context.Context) error {
	// Load initial config
	cfg, err := Load(hr.path)
	if err != nil {
		return fmt.Errorf("loading initial config: %w", err)
	}

	hr.mu.Lock()
	hr.current = cfg
	hr.mu.Unlock()

	// Add file to watcher
	if err := hr.watcher.Add(hr.path); err != nil {
		return fmt.Errorf("watching config file: %w", err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				hr.watcher.Close()
				return
			case event, ok := <-hr.watcher.Events:
				if !ok {
					return
				}
				// Check if it's a write event
				if event.Op&fsnotify.Write == fsnotify.Write {
					hr.reload()
				}
			case err, ok := <-hr.watcher.Errors:
				if !ok {
					return
				}
				// Log error but continue watching
				fmt.Printf("config watch error: %v\n", err)
			}
		}
	}()

	return nil
}

// reload loads the config file and calls the onChange callback.
func (hr *HotReloader) reload() {
	cfg, err := Load(hr.path)
	if err != nil {
		fmt.Printf("failed to reload config: %v\n", err)
		return
	}

	hr.mu.Lock()
	hr.current = cfg
	hr.mu.Unlock()

	// Call onChange callback
	if hr.onChange != nil {
		hr.onChange(cfg)
	}
}

// GetConfig returns the current config.
func (hr *HotReloader) GetConfig() *Config {
	hr.mu.RLock()
	defer hr.mu.RUnlock()
	return hr.current
}

// Close stops the hot-reloader.
func (hr *HotReloader) Close() error {
	return hr.watcher.Close()
}
