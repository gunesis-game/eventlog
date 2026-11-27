package storage

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"sync"

	"github.com/gunesis-game/eventlog/internal/filesystem"
	"github.com/gunesis-game/eventlog/internal/identity"
	"github.com/gunesis-game/eventlog/internal/record"
)

// remoteLog is another device's log, read but never written (D-073). Its
// state is a tolerant recovery of the log, extended by later refreshes.
type remoteLog struct {
	device    identity.UUID
	files     *filesystem.Client
	validator *record.Validator

	mu sync.Mutex
	// state is the log as last read; nil until a read succeeds.
	state *recovery
	// err is why the last refresh could not read the log at all, such as an
	// I/O error; the previous state is dropped, so nothing stale is served.
	err error
}

// Refresh opens the logs of devices that have appeared under the log folder
// and reads what every other device has appended since the last refresh.
// Damage in another device's log is recorded, never returned (D-073); the
// error reports only that the devices could not be listed.
func (s *Store) Refresh() error {
	entries, err := s.log.List(devicesDirName)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("list devices: %w", err)
	}
	for _, entry := range entries {
		device, err := identity.ParseUUID(entry.Name)
		if !entry.Dir || err != nil || device == s.device {
			continue
		}
		s.mu.Lock()
		remote := s.remotes[device]
		if remote == nil {
			remote = &remoteLog{device: device, files: s.log.Sub(deviceLogDir(device)), validator: s.validator}
			s.remotes[device] = remote
		}
		s.mu.Unlock()
		remote.refresh()
	}
	return nil
}

// Devices returns the other devices whose logs this store reads.
func (s *Store) Devices() []identity.UUID {
	s.mu.Lock()
	defer s.mu.Unlock()
	devices := make([]identity.UUID, 0, len(s.remotes))
	for device := range s.remotes {
		devices = append(devices, device)
	}
	slices.SortFunc(devices, func(a, b identity.UUID) int {
		return slices.Compare(a[:], b[:])
	})
	return devices
}

// RemoteHistory reports how much of another device's log is readable: nil when
// all of it is; otherwise why not, and the first sequence of its history that is
// missing (D-070, D-073).
func (s *Store) RemoteHistory(device identity.UUID) (missingFrom uint64, cause error) {
	remote, err := s.remote(device)
	if err != nil {
		return 0, err
	}
	remote.mu.Lock()
	defer remote.mu.Unlock()
	switch {
	case remote.err != nil:
		return 1, remote.err
	case remote.state.damage != nil:
		return remote.state.damagedFrom, remote.state.damage
	default:
		return 0, nil
	}
}

// RemoteOccurrences returns up to limit consecutive occurrences of a Channel
// in another device's log, with their record bytes (without newlines),
// starting after that device's first position occurrences of the Channel. The
// position is that device's count of its occurrences of the Channel, its entry
// in the version vector, so it stays valid when that device compacts its log
// (D-073). A file gone since the last read, as after compaction, is found by
// reading the device's log again once. An empty result means there are no more
// readable occurrences yet.
func (s *Store) RemoteOccurrences(device, channelID identity.UUID, position, limit int) ([]Occurrence, [][]byte, error) {
	remote, err := s.remote(device)
	if err != nil {
		return nil, nil, err
	}
	batch, frames, err := remote.read(channelID, position, limit)
	if errors.Is(err, fs.ErrNotExist) {
		remote.refresh()
		batch, frames, err = remote.read(channelID, position, limit)
	}
	return batch, frames, err
}

func (s *Store) remote(device identity.UUID) (*remoteLog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	remote := s.remotes[device]
	if remote == nil {
		return nil, fmt.Errorf("no log of device %s", device)
	}
	return remote, nil
}

func (l *remoteLog) read(channelID identity.UUID, position, limit int) ([]Occurrence, [][]byte, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.err != nil {
		return nil, nil, l.err
	}
	batch := collectBatch(l.state.index, channelID, position, limit)
	if len(batch) == 0 {
		return nil, nil, nil
	}
	frames, err := readFrames(l.files, batch)
	if err != nil {
		return nil, nil, err
	}
	return batch, frames, nil
}

// refresh reads what the device appended since the last read. When the
// device's manifest only adds to what was read, it reads just the new bytes;
// otherwise, as after the device compacted or after damage, it reads the
// whole log again.
func (l *remoteLog) refresh() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.state != nil && l.state.damage == nil {
		if m, err := readManifest(l.files); err == nil && l.state.extends(m) && l.state.extend(m) == nil {
			return
		}
	}
	state, err := newRemoteRecovery(l.files, l.validator, l.device).run()
	l.state, l.err = state, err
}

// extends reports whether manifest m only adds segments to the manifest r
// read, in the same generation, so reading can continue where it stopped.
func (r *recovery) extends(m manifest) bool {
	return m.Generation == r.manifest.Generation && len(m.Segments) >= len(r.manifest.Segments) &&
		slices.Equal(m.Segments[:len(r.manifest.Segments)], r.manifest.Segments) && r.active.path != ""
}

// extend continues a tolerant recovery after the bytes it has read: the rest
// of the segment it read last, then each segment m adds. Any damage or
// unexpected state returns an error, after which the caller reads the whole
// log again.
func (r *recovery) extend(m manifest) error {
	last := len(r.manifest.Segments) - 1
	r.current = last
	data, err := r.files.Read(r.active.path, r.active.size, -1)
	if err != nil {
		return err
	}
	result, err := record.ValidateStream(data, r.validator, r.nextSequence)
	if err != nil {
		return err
	}
	if result.IncompleteStart != nil && len(m.Segments) > len(r.manifest.Segments) {
		return errors.New("a finalized segment ends with an incomplete transaction")
	}
	committed := data[:result.CommittedOffset]
	if err := r.indexSegment(r.active, committed, r.active.size); err != nil {
		return err
	}
	r.active.size += int64(len(committed))
	r.nextSequence = result.CommittedSequence + 1
	for i := len(r.manifest.Segments); i < len(m.Segments); i++ {
		r.finalized = append(r.finalized, r.active)
		r.active = nil
		if err := r.recoverSegment(i, m.Segments[i], i == len(m.Segments)-1); err != nil {
			return err
		}
	}
	r.manifest = m
	return nil
}
