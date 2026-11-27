package eventlog

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

var (
	// ErrConfigAlreadySet is returned by Config after configuration has
	// already succeeded.
	ErrConfigAlreadySet = errors.New("eventlog: configuration already set")
	// ErrConfigNotInitialized is returned by PrepareLog before Config succeeds.
	ErrConfigNotInitialized = errors.New("eventlog: configuration not initialized")
)

// Option is one configuration setting passed to Config.
type Option interface {
	applyTo(settings *configSettings) error
}

type configSettings struct {
	storageDirectory    string
	storageDirectorySet bool
}

type storageDirectoryOption struct {
	path string
}

// WithStorageDirectory sets the required storage directory. A relative path is
// resolved against the working directory and lexically cleaned when Config
// runs. Config does not create the directory; PrepareLog does.
func WithStorageDirectory(path string) Option {
	return storageDirectoryOption{path: path}
}

func (o storageDirectoryOption) applyTo(settings *configSettings) error {
	if settings.storageDirectorySet {
		return errors.New("eventlog: storage directory set more than once")
	}
	if strings.TrimSpace(o.path) == "" {
		return errors.New("eventlog: storage directory is empty")
	}
	if strings.ContainsRune(o.path, 0) {
		return errors.New("eventlog: storage directory contains a NUL byte")
	}
	absolute, err := filepath.Abs(o.path)
	if err != nil {
		return fmt.Errorf("eventlog: resolve storage directory: %w", err)
	}
	settings.storageDirectory = filepath.Clean(absolute)
	settings.storageDirectorySet = true
	return nil
}

var processConfig struct {
	mu       sync.Mutex
	settings *configSettings
}

// Config sets the process-wide eventlog configuration exactly once. It
// validates every option before publishing; a failed Config may be retried.
func Config(options ...Option) error {
	processConfig.mu.Lock()
	defer processConfig.mu.Unlock()
	if processConfig.settings != nil {
		return ErrConfigAlreadySet
	}
	settings, err := buildConfigSettings(options)
	if err != nil {
		return err
	}
	processConfig.settings = settings
	return nil
}

func buildConfigSettings(options []Option) (*configSettings, error) {
	settings := &configSettings{}
	for i, option := range options {
		if option == nil {
			return nil, fmt.Errorf("eventlog: option %d is nil", i)
		}
		if err := option.applyTo(settings); err != nil {
			return nil, err
		}
	}
	if !settings.storageDirectorySet {
		return nil, errors.New("eventlog: WithStorageDirectory is required")
	}
	return settings, nil
}

func currentConfig() (*configSettings, error) {
	processConfig.mu.Lock()
	defer processConfig.mu.Unlock()
	if processConfig.settings == nil {
		return nil, ErrConfigNotInitialized
	}
	return processConfig.settings, nil
}
