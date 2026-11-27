package record

import (
	"bytes"
	"errors"
	"testing"
)

func streamTestChannel(sequence uint64) Channel {
	return Channel{SchemaVersion: 1, Sequence: sequence, Kind: KindChannel, ID: "12345678-1234-1234-1234-123456789abc", Type: Type{PackagePath: "example.org/test", Name: "Channel"}, Fields: map[string]interface{}{}}
}

func streamTestEvent(sequence uint64) Event {
	return Event{SchemaVersion: 1, Sequence: sequence, Kind: KindEvent, ID: "87654321-4321-4321-4321-cba987654321", ChannelID: "12345678-1234-1234-1234-123456789abc", Type: Type{PackagePath: "example.org/test", Name: "Event"}, Fields: map[string]interface{}{}}
}

func appendStreamRecord(t *testing.T, dst *bytes.Buffer, record interface{}, kind string, validator *Validator) []byte {
	t.Helper()
	frame, err := Encode(kind, record, validator)
	if err != nil {
		t.Fatalf("encode %s: %v", kind, err)
	}
	dst.Write(frame)
	return frame
}

func TestValidateRecordStreamCommittedTransactions(t *testing.T) {
	v := testValidator(t)
	var stream bytes.Buffer

	ch := streamTestChannel(1)
	ev := streamTestEvent(2)
	chFrame := appendStreamRecord(t, &stream, ch, KindChannel, v)
	evFrame := appendStreamRecord(t, &stream, ev, KindEvent, v)
	commit1 := Commit{SchemaVersion: 1, Sequence: 3, Kind: KindCommit, TransactionID: 1, RecordCount: 2, Checksum: Checksum([][]byte{chFrame, evFrame})}
	appendStreamRecord(t, &stream, commit1, KindCommit, v)

	ev2 := streamTestEvent(4)
	ev2Frame := appendStreamRecord(t, &stream, ev2, KindEvent, v)
	commit2 := Commit{SchemaVersion: 1, Sequence: 5, Kind: KindCommit, TransactionID: 4, RecordCount: 1, Checksum: Checksum([][]byte{ev2Frame})}
	appendStreamRecord(t, &stream, commit2, KindCommit, v)

	result, err := ValidateStream(stream.Bytes(), v, 1)
	if err != nil {
		t.Fatalf("validate stream: %v", err)
	}
	if result.CommittedSequence != 5 {
		t.Fatalf("committed sequence %d, want 5", result.CommittedSequence)
	}
	if result.CommittedOffset != int64(stream.Len()) {
		t.Fatalf("committed offset %d, want %d", result.CommittedOffset, stream.Len())
	}
	if result.IncompleteStart != nil {
		t.Fatalf("unexpected incomplete start %d", *result.IncompleteStart)
	}
}

func TestValidateRecordStreamIdentifiesIncompleteTransactionStart(t *testing.T) {
	v := testValidator(t)
	var stream bytes.Buffer

	appendStreamRecord(t, &stream, streamTestChannel(1), KindChannel, v)
	appendStreamRecord(t, &stream, streamTestEvent(2), KindEvent, v)
	commit := Commit{SchemaVersion: 1, Sequence: 3, Kind: KindCommit, TransactionID: 1, RecordCount: 2, Checksum: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	// A partial Commit frame leaves the preceding Channel+Event transaction uncommitted.
	raw, err := Encode(KindCommit, commit, v)
	if err != nil {
		t.Fatalf("encode partial commit: %v", err)
	}
	stream.Write(raw[:len(raw)-4])

	result, err := ValidateStream(stream.Bytes(), v, 1)
	if err != nil {
		t.Fatalf("validate incomplete stream: %v", err)
	}
	wantStart := int64(0)
	if result.IncompleteStart == nil || *result.IncompleteStart != wantStart {
		t.Fatalf("incomplete start = %v, want %d", result.IncompleteStart, wantStart)
	}
	if result.CommittedSequence != 0 || result.CommittedOffset != 0 {
		t.Fatalf("uncommitted data reported as committed: %#v", result)
	}
}

func TestValidateRecordStreamRejectsSequenceGap(t *testing.T) {
	v := testValidator(t)
	var stream bytes.Buffer
	appendStreamRecord(t, &stream, streamTestEvent(2), KindEvent, v)
	if _, err := ValidateStream(stream.Bytes(), v, 1); err == nil {
		t.Fatal("expected sequence gap error")
	}
}

func TestValidateRecordStreamRejectsCommitWithoutRecords(t *testing.T) {
	v := testValidator(t)
	var stream bytes.Buffer
	commit := Commit{SchemaVersion: 1, Sequence: 1, Kind: KindCommit, TransactionID: 1, RecordCount: 1, Checksum: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	appendStreamRecord(t, &stream, commit, KindCommit, v)
	if _, err := ValidateStream(stream.Bytes(), v, 1); err == nil {
		t.Fatal("expected orphan Commit error")
	}
}

func TestValidateStreamRejectsZeroStart(t *testing.T) {
	if _, err := ValidateStream(nil, nil, 0); !errors.Is(err, ErrInvalidTransaction) {
		t.Fatalf("zero start sequence: %v", err)
	}
}
