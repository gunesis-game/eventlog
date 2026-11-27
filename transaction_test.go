package eventlog

import "testing"

func makeTestCommit(records []interface{}, frames [][]byte, first, commitSeq uint64) commitRecord {
	return commitRecord{SchemaVersion: 1, Sequence: commitSeq, Kind: recordKindCommit, TransactionID: first, RecordCount: uint64(len(records)), Checksum: transactionChecksum(frames)}
}

func TestValidateTransactionChannelEvent(t *testing.T) {
	channel := channelRecord{Sequence: 7, Kind: recordKindChannel}
	event := eventRecord{Sequence: 8, Kind: recordKindEvent}
	records := []interface{}{channel, event}
	frames := [][]byte{[]byte("channel\n"), []byte("event\n")}
	if err := validateTransaction(records, frames, makeTestCommit(records, frames, 7, 9)); err != nil {
		t.Fatalf("valid Channel+Event transaction: %v", err)
	}
}

func TestValidateTransactionEventOnly(t *testing.T) {
	event := eventRecord{Sequence: 20, Kind: recordKindEvent}
	records := []interface{}{event}
	frames := [][]byte{[]byte("event\n")}
	if err := validateTransaction(records, frames, makeTestCommit(records, frames, 20, 21)); err != nil {
		t.Fatalf("valid Event-only transaction: %v", err)
	}
}

func TestValidateTransactionRejectsContractViolations(t *testing.T) {
	channel := channelRecord{Sequence: 1, Kind: recordKindChannel}
	event := eventRecord{Sequence: 2, Kind: recordKindEvent}
	records := []interface{}{channel, event}
	frames := [][]byte{[]byte("channel\n"), []byte("event\n")}
	valid := makeTestCommit(records, frames, 1, 3)

	tests := []struct {
		name    string
		records []interface{}
		frames  [][]byte
		commit  commitRecord
	}{
		{name: "wrong transaction ID", records: records, frames: frames, commit: commitRecord{Sequence: 3, TransactionID: 99, RecordCount: 2, Checksum: valid.Checksum}},
		{name: "wrong count", records: records, frames: frames, commit: commitRecord{Sequence: 3, TransactionID: 1, RecordCount: 1, Checksum: valid.Checksum}},
		{name: "wrong commit sequence", records: records, frames: frames, commit: commitRecord{Sequence: 4, TransactionID: 1, RecordCount: 2, Checksum: valid.Checksum}},
		{name: "checksum mismatch", records: records, frames: frames, commit: commitRecord{Sequence: 3, TransactionID: 1, RecordCount: 2, Checksum: "invalid"}},
		{name: "nonconsecutive sequence", records: []interface{}{channelRecord{Sequence: 1, Kind: recordKindChannel}, eventRecord{Sequence: 4, Kind: recordKindEvent}}, frames: frames, commit: commitRecord{Sequence: 5, TransactionID: 1, RecordCount: 2, Checksum: valid.Checksum}},
		{name: "event not last", records: []interface{}{event, channel}, frames: frames, commit: valid},
		{name: "missing event", records: []interface{}{channel}, frames: frames[:1], commit: commitRecord{Sequence: 2, TransactionID: 1, RecordCount: 1, Checksum: transactionChecksum(frames[:1])}},
		{name: "frame mismatch", records: records, frames: frames[:1], commit: valid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateTransaction(tt.records, tt.frames, tt.commit); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
