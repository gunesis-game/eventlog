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
	storageDirectory string
	storageSet       bool
}

// Storage selects where the log is stored. Local is the only storage
// implemented.
type Storage interface {
	resolve() (string, error)
}

// Local stores the log in a directory on this device. A relative Directory is
// resolved against the working directory and lexically cleaned when Config
// runs. Config does not create the directory; PrepareLog does.
type Local struct {
	Directory string
}

func (l Local) resolve() (string, error) {
	if strings.TrimSpace(l.Directory) == "" {
		return "", errors.New("eventlog: storage directory is empty")
	}
	if strings.ContainsRune(l.Directory, 0) {
		return "", errors.New("eventlog: storage directory contains a NUL byte")
	}
	absolute, err := filepath.Abs(l.Directory)
	if err != nil {
		return "", fmt.Errorf("eventlog: resolve storage directory: %w", err)
	}
	return filepath.Clean(absolute), nil
}

type storageOption struct {
	storage Storage
}

// WithStorage sets the required storage.
func WithStorage(storage Storage) Option {
	return storageOption{storage: storage}
}

func (o storageOption) applyTo(settings *configSettings) error {
	if settings.storageSet {
		return errors.New("eventlog: storage set more than once")
	}
	if o.storage == nil {
		return errors.New("eventlog: storage is nil")
	}
	directory, err := o.storage.resolve()
	if err != nil {
		return err
	}
	settings.storageDirectory = directory
	settings.storageSet = true
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
	if !settings.storageSet {
		return nil, errors.New("eventlog: WithStorage is required")
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
