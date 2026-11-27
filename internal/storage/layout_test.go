package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gunesis-game/eventlog/internal/identity"
)

func readIdentityFromDisk(t *testing.T, path string, field string) identity.UUID {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var content map[string]any
	if err := json.Unmarshal(raw, &content); err != nil {
		t.Fatal(err)
	}
	if content["format_version"] != float64(identityFormatVersion) {
		t.Fatalf("%s format_version = %v", path, content["format_version"])
	}
	text, _ := content[field].(string)
	id, err := identity.ParseUUID(text)
	if err != nil || id.IsZero() || id[6]>>4 != 4 {
		t.Fatalf("%s %s = %q, want a random UUID", path, field, text)
	}
	return id
}

// D-060, D-062: the first start creates the device state folder with the
// device's identity and lock, and the eventlog folder with the log's identity
// and this device's log; later starts keep both identities.
func TestLayoutCreatesIdentitiesOnce(t *testing.T) {
	dir := t.TempDir()
	s := openTestStore(t, dir, fastStoreOptions())
	appendTestOccurrence(t, s, 1, 10, "a")
	logPath := filepath.Join(dir, "eventlog", "log.json")
	devicePath := filepath.Join(dir, "device", "device.json")
	logID := readIdentityFromDisk(t, logPath, "log_id")
	deviceID := readIdentityFromDisk(t, devicePath, "device_id")
	deviceLog := filepath.Join(dir, "eventlog", "devices", deviceID.String())
	for _, path := range []string{
		filepath.Join(dir, "device", "lock"),
		filepath.Join(deviceLog, manifestFileName),
		filepath.Join(deviceLog, filepath.FromSlash(segmentPath(1))),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s: %v", path, err)
		}
	}
	logBytes, _ := os.ReadFile(logPath)
	before := storeSummary(s)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	reopened := openTestStore(t, dir, fastStoreOptions())
	if readIdentityFromDisk(t, logPath, "log_id") != logID || readIdentityFromDisk(t, devicePath, "device_id") != deviceID {
		t.Fatal("identities changed on restart")
	}
	if after, _ := os.ReadFile(logPath); string(after) != string(logBytes) {
		t.Fatal("log.json was rewritten")
	}
	if storeSummary(reopened) != before {
		t.Fatal("the device's log did not recover")
	}
}

// Losing the device state makes the machine a new device with its own log
// folder; the log's identity and the earlier device's log are untouched.
func TestLostDeviceStateMakesANewDevice(t *testing.T) {
	dir := t.TempDir()
	s := openTestStore(t, dir, fastStoreOptions())
	appendTestOccurrence(t, s, 1, 10, "a")
	_ = s.Close()
	logPath := filepath.Join(dir, "eventlog", "log.json")
	logID := readIdentityFromDisk(t, logPath, "log_id")
	first := readIdentityFromDisk(t, filepath.Join(dir, "device", "device.json"), "device_id")
	firstLog := snapshotFiles(t, filepath.Join(dir, "eventlog", "devices", first.String()))
	if err := os.RemoveAll(filepath.Join(dir, "device")); err != nil {
		t.Fatal(err)
	}

	openTestStore(t, dir, fastStoreOptions())
	second := readIdentityFromDisk(t, filepath.Join(dir, "device", "device.json"), "device_id")
	if second == first {
		t.Fatal("a device without state kept the old identity")
	}
	if readIdentityFromDisk(t, logPath, "log_id") != logID {
		t.Fatal("the log's identity changed")
	}
	if _, err := os.Stat(filepath.Join(dir, "eventlog", "devices", second.String(), manifestFileName)); err != nil {
		t.Fatalf("new device log: %v", err)
	}
	after := snapshotFiles(t, filepath.Join(dir, "eventlog", "devices", first.String()))
	for name, data := range firstLog {
		if after[name] != data {
			t.Fatalf("the earlier device's %s changed", name)
		}
	}
}

// An unreadable identity fails closed rather than replacing the identity.
func TestCorruptIdentityFailsClosed(t *testing.T) {
	for _, name := range []string{"device/device.json", "eventlog/log.json"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			s := openTestStore(t, dir, fastStoreOptions())
			_ = s.Close()
			path := filepath.Join(dir, filepath.FromSlash(name))
			for _, content := range []string{"not json", `{"format_version":2,"log_id":"x","device_id":"x"}`, `{"format_version":1}`} {
				if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
				if reopened, err := Open(dir, fastStoreOptions()); !errors.Is(err, errCorruptStorage) {
					if err == nil {
						_ = reopened.Close()
					}
					t.Fatalf("content %q: err = %v, want corrupt storage", content, err)
				}
				if data, _ := os.ReadFile(path); string(data) != content {
					t.Fatalf("content %q was replaced", content)
				}
			}
		})
	}
}
