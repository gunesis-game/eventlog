package eventlog

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

const helperScenarioEnv = "EVENTLOG_HELPER_SCENARIO"

// runHelper re-executes the test binary as a consumer process. Each child is a
// fresh process with its own one-time Config and process-lifetime runtime.
func runHelper(t *testing.T, scenario, dir string, args ...string) []string {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$", "-test.v")
	cmd.Env = append(os.Environ(), helperScenarioEnv+"="+scenario, "EVENTLOG_HELPER_DIR="+dir, "EVENTLOG_HELPER_ARGS="+strings.Join(args, ","))
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	var lines []string
	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		if line, ok := strings.CutPrefix(scanner.Text(), "RESULT "); ok {
			lines = append(lines, line)
		}
	}
	var exitErr *exec.ExitError
	if err != nil && !(errors.As(err, &exitErr) && exitErr.ExitCode() == 3) {
		t.Fatalf("helper %s failed: %v\nstdout:\n%s\nstderr:\n%s", scenario, err, stdout.String(), stderr.String())
	}
	return lines
}

// TestHelperProcess is the child-process body; it is a no-op in normal runs.
func TestHelperProcess(t *testing.T) {
	scenario := os.Getenv(helperScenarioEnv)
	if scenario == "" {
		return
	}
	dir := os.Getenv("EVENTLOG_HELPER_DIR")
	var args []int64
	for _, field := range strings.Split(os.Getenv("EVENTLOG_HELPER_ARGS"), ",") {
		if field == "" {
			continue
		}
		n, _ := strconv.ParseInt(field, 10, 64)
		args = append(args, n)
	}
	if err := runScenario(scenario, dir, args); err != nil {
		fmt.Println("RESULT error", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func runScenario(scenario, dir string, args []int64) error {
	switch scenario {
	case "config":
		return scenarioConfig(dir)
	case "append":
		return scenarioAppend(dir, args)
	case "replay":
		return scenarioReplay(dir, args)
	default:
		return fmt.Errorf("unknown scenario %q", scenario)
	}
}

func helperPrepare(dir string) (*OrderPlaced, *Orders, error) {
	if err := Config(WithStorageDirectory(dir)); err != nil {
		return nil, nil, err
	}
	return PrepareLog[*OrderPlaced, *Orders](context.Background(), placedFactory(testUUID(2)), ordersFactory(testUUID(1)))
}

func helperReceive(channel *Orders) (*OrderPlaced, error) {
	select {
	case event, ok := <-channel.Notifications():
		if !ok {
			return nil, errors.New("notifications closed")
		}
		return event.(*OrderPlaced), nil
	case err := <-channel.Errors():
		return nil, err
	case <-time.After(testWait):
		return nil, errors.New("timed out waiting for notification")
	}
}

// scenarioConfig checks the one-time configuration contract (RT-02, RT-03,
// API-02) in a fresh process.
func scenarioConfig(dir string) error {
	if _, _, err := PrepareLog[*OrderPlaced, *Orders](context.Background(), placedFactory(testUUID(2)), ordersFactory(testUUID(1))); !errors.Is(err, ErrConfigNotInitialized) {
		return fmt.Errorf("PrepareLog before Config: %v", err)
	}
	invalid := [][]Option{
		nil,
		{nil},
		{WithStorageDirectory("")},
		{WithStorageDirectory("   ")},
		{WithStorageDirectory(dir), WithStorageDirectory(dir)},
	}
	for i, options := range invalid {
		if err := Config(options...); err == nil {
			return fmt.Errorf("invalid config %d accepted", i)
		}
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		return errors.New("Config must not create the storage directory")
	}
	if err := Config(WithStorageDirectory(dir)); err != nil {
		return fmt.Errorf("valid config after failures: %v", err)
	}
	if err := Config(WithStorageDirectory(dir)); !errors.Is(err, ErrConfigAlreadySet) {
		return fmt.Errorf("second Config: %v", err)
	}
	if _, _, err := PrepareLog[*OrderPlaced, *Orders](context.Background(), placedFactory(testUUID(2)), ordersFactory(testUUID(1))); err != nil {
		return fmt.Errorf("PrepareLog: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err != nil {
		return fmt.Errorf("PrepareLog must create storage: %v", err)
	}
	fmt.Println("RESULT ok")
	return nil
}

// scenarioAppend commits each amount as one occurrence. With a
// negative final argument it terminates abruptly right after the last commit,
// before any notification is consumed (RT-09, N-11, N-13).
func scenarioAppend(dir string, amounts []int64) error {
	event, _, err := helperPrepare(dir)
	if err != nil {
		return err
	}
	abrupt := len(amounts) > 0 && amounts[len(amounts)-1] < 0
	if abrupt {
		amounts = amounts[:len(amounts)-1]
	}
	base := committedOccurrences(dir)
	for i, amount := range amounts {
		setAmount(event, amount)
		deadline := time.Now().Add(testWait)
		for committedOccurrences(dir) < base+i+1 {
			if time.Now().After(deadline) {
				return errors.New("timed out waiting for commit")
			}
			time.Sleep(time.Millisecond)
		}
		fmt.Println("RESULT", amount)
	}
	if abrupt {
		os.Exit(3)
	}
	return nil
}

func committedOccurrences(dir string) int {
	return countRecords(dir, "event")
}

// scenarioReplay reads expected history, then appends one live occurrence.
func scenarioReplay(dir string, args []int64) error {
	event, channel, err := helperPrepare(dir)
	if err != nil {
		return err
	}
	for i := int64(0); i < args[0]; i++ {
		got, err := helperReceive(channel)
		if err != nil {
			return err
		}
		fmt.Println("RESULT", got.Amount)
	}
	setAmount(event, args[1])
	got, err := helperReceive(channel)
	if err != nil {
		return err
	}
	fmt.Println("RESULT", got.Amount)
	return nil
}

func TestProcessConfigContract(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "log")
	if got := runHelper(t, "config", dir); strings.Join(got, ",") != "ok" {
		t.Fatalf("config scenario = %v", got)
	}
}

// RT-08..RT-13, API-08, API-09, API-11, N-11, N-13: committed history survives
// process termination and is replayed before newer occurrences; an interrupted
// trailing transaction is discarded; repeated lifetimes stay consistent; and
// re-delivery after a restart demonstrates possible duplicates.
func TestProcessTerminationAndRecovery(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "log")
	if got := strings.Join(runHelper(t, "append", dir, "1", "2"), ","); got != "1,2" {
		t.Fatalf("first lifetime committed %s", got)
	}
	if got := strings.Join(runHelper(t, "append", dir, "3", "4", "-1"), ","); got != "3,4" {
		t.Fatalf("second lifetime committed %s", got)
	}

	segments, _ := filepath.Glob(filepath.Join(dir, "segments", "segment-*.ndjson"))
	if len(segments) == 0 {
		t.Fatal("no segments")
	}
	sort.Strings(segments)
	f, err := os.OpenFile(segments[len(segments)-1], os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(`{"schema_version":1,"sequence":99,"kind":"event","id":"`)
	_ = f.Close()

	if got := strings.Join(runHelper(t, "replay", dir, "4", "50"), ","); got != "1,2,3,4,50" {
		t.Fatalf("third lifetime delivered %s", got)
	}
	if got := strings.Join(runHelper(t, "replay", dir, "5", "60"), ","); got != "1,2,3,4,50,60" {
		t.Fatalf("fourth lifetime delivered %s", got)
	}
}
