package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"

	"github.com/gunesis-game/eventlog/internal/filesystem"
	"github.com/gunesis-game/eventlog/internal/identity"
)

// Storage layout (D-060, D-062). The device state folder belongs to this
// device only and is never shared; the eventlog folder is the log, which every
// device of the log shares, each writing only its own folder under devices.
//
//	device/device.json                 this device's identity
//	device/lock                        this device's one-process lock
//	eventlog/log.json                  the log's permanent identity
//	eventlog/devices/{deviceID}/...    one device's manifest, index, segments
const (
	deviceStateDirName    = "device"
	deviceFileName        = "device/device.json"
	lockPath              = "device/lock"
	logFileName           = "eventlog/log.json"
	devicesDirName        = "eventlog/devices"
	identityFormatVersion = 1
)

// logIdentity is the content of log.json: generated once, it identifies the
// log on every backend it is stored in.
type logIdentity struct {
	FormatVersion int    `json:"format_version"`
	LogID         string `json:"log_id"`
}

// deviceIdentity is the content of device.json: generated on a device's
// first start and kept in its device state.
type deviceIdentity struct {
	FormatVersion int    `json:"format_version"`
	DeviceID      string `json:"device_id"`
}

// deviceLogDir returns the folder of a device's log, relative to the storage
// root.
func deviceLogDir(device identity.UUID) string {
	return path.Join(devicesDirName, device.String())
}

// readIdentity reads an identity file, reporting fs.ErrNotExist when it is
// absent and failing closed on anything else unexpected.
func readIdentity(files *filesystem.Client, name string, id func(raw []byte) (int, string, error)) (identity.UUID, error) {
	raw, err := files.Read(name, 0, -1)
	if err != nil {
		return identity.UUID{}, err
	}
	version, text, err := id(raw)
	if err != nil {
		return identity.UUID{}, fmt.Errorf("%w: %s: %v", errCorruptStorage, name, err)
	}
	if version != identityFormatVersion {
		return identity.UUID{}, fmt.Errorf("%w: %s: unsupported format version %d", errCorruptStorage, name, version)
	}
	value, err := identity.ParseUUID(text)
	if err != nil || value.IsZero() {
		return identity.UUID{}, fmt.Errorf("%w: %s: invalid identity %q", errCorruptStorage, name, text)
	}
	return value, nil
}

func readLogID(files *filesystem.Client) (identity.UUID, error) {
	return readIdentity(files, logFileName, func(raw []byte) (int, string, error) {
		var content logIdentity
		err := json.Unmarshal(raw, &content)
		return content.FormatVersion, content.LogID, err
	})
}

func readDeviceID(files *filesystem.Client) (identity.UUID, error) {
	return readIdentity(files, deviceFileName, func(raw []byte) (int, string, error) {
		var content deviceIdentity
		err := json.Unmarshal(raw, &content)
		return content.FormatVersion, content.DeviceID, err
	})
}

// ensureIdentity returns the identity in name, generating and writing a new
// one when the file does not exist yet.
func ensureIdentity(files *filesystem.Client, name string, read func(*filesystem.Client) (identity.UUID, error), content func(identity.UUID) any) (identity.UUID, error) {
	value, err := read(files)
	if !errors.Is(err, fs.ErrNotExist) {
		return value, err
	}
	value = identity.NewRandom()
	raw, err := json.Marshal(content(value))
	if err != nil {
		return identity.UUID{}, err
	}
	if err := files.Replace(name, raw); err != nil {
		return identity.UUID{}, fmt.Errorf("write %s: %w", name, err)
	}
	return value, nil
}

func ensureLogID(files *filesystem.Client) (identity.UUID, error) {
	return ensureIdentity(files, logFileName, readLogID, func(id identity.UUID) any {
		return logIdentity{FormatVersion: identityFormatVersion, LogID: id.String()}
	})
}

func ensureDeviceID(files *filesystem.Client) (identity.UUID, error) {
	return ensureIdentity(files, deviceFileName, readDeviceID, func(id identity.UUID) any {
		return deviceIdentity{FormatVersion: identityFormatVersion, DeviceID: id.String()}
	})
}
