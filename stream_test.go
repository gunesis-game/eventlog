package eventlog

import (
	"bytes"
	"testing"
)

func streamTestChannel(sequence uint64) channelRecord {
	return channelRecord{SchemaVersion: 1, Sequence: sequence, Kind: recordKindChannel, ID: "12345678-1234-1234-1234-123456789abc", Type: recordType{PackagePath: "example.org/test", Name: "Channel"}, Fields: map[string]interface{}{}}
}

func streamTestEvent(sequence uint64) eventRecord {
	return eventRecord{SchemaVersion: 1, Sequence: sequence, Kind: recordKindEvent, ID: "87654321-4321-4321-4321-cba987654321", ChannelID: "12345678-1234-1234-1234-123456789abc", Type: recordType{PackagePath: "example.org/test", Name: "Event"}, Fields: map[string]interface{}{}}
}

func appendStreamRecord(t *testing.T, dst *bytes.Buffer, record interface{}, kind string, validator *recordSchemaValidator) []byte {
	t.Helper()
	frame, err := encodeRecord(kind, record, validator)
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
	chFrame := appendStreamRecord(t, &stream, ch, recordKindChannel, v)
	evFrame := appendStreamRecord(t, &stream, ev, recordKindEvent, v)
	commit1 := commitRecord{SchemaVersion: 1, Sequence: 3, Kind: recordKindCommit, TransactionID: 1, RecordCount: 2, Checksum: transactionChecksum([][]byte{chFrame, evFrame})}
	appendStreamRecord(t, &stream, commit1, recordKindCommit, v)

	ev2 := streamTestEvent(4)
	ev2Frame := appendStreamRecord(t, &stream, ev2, recordKindEvent, v)
	commit2 := commitRecord{SchemaVersion: 1, Sequence: 5, Kind: recordKindCommit, TransactionID: 4, RecordCount: 1, Checksum: transactionChecksum([][]byte{ev2Frame})}
	appendStreamRecord(t, &stream, commit2, recordKindCommit, v)

	result, err := validateRecordStream(stream.Bytes(), v)
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

	appendStreamRecord(t, &stream, streamTestChannel(1), recordKindChannel, v)
	appendStreamRecord(t, &stream, streamTestEvent(2), recordKindEvent, v)
	commit := commitRecord{SchemaVersion: 1, Sequence: 3, Kind: recordKindCommit, TransactionID: 1, RecordCount: 2, Checksum: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	// A partial Commit frame leaves the preceding Channel+Event transaction uncommitted.
	raw, err := encodeRecord(recordKindCommit, commit, v)
	if err != nil {
		t.Fatalf("encode partial commit: %v", err)
	}
	stream.Write(raw[:len(raw)-4])

	result, err := validateRecordStream(stream.Bytes(), v)
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
	appendStreamRecord(t, &stream, streamTestEvent(2), recordKindEvent, v)
	if _, err := validateRecordStream(stream.Bytes(), v); err == nil {
		t.Fatal("expected sequence gap error")
	}
}

func TestValidateRecordStreamRejectsCommitWithoutRecords(t *testing.T) {
	v := testValidator(t)
	var stream bytes.Buffer
	commit := commitRecord{SchemaVersion: 1, Sequence: 1, Kind: recordKindCommit, TransactionID: 1, RecordCount: 1, Checksum: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	appendStreamRecord(t, &stream, commit, recordKindCommit, v)
	if _, err := validateRecordStream(stream.Bytes(), v); err == nil {
		t.Fatal("expected orphan Commit error")
	}
}
