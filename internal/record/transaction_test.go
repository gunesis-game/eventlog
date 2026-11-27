package record

import "testing"

func makeTestCommit(records []interface{}, frames [][]byte, first, commitSeq uint64) Commit {
	return Commit{SchemaVersion: 1, Sequence: commitSeq, Kind: KindCommit, TransactionID: first, RecordCount: uint64(len(records)), Checksum: Checksum(frames)}
}

func TestValidateTransactionChannelEvent(t *testing.T) {
	channel := Channel{Sequence: 7, Kind: KindChannel}
	event := Event{Sequence: 8, Kind: KindEvent}
	records := []interface{}{channel, event}
	frames := [][]byte{[]byte("channel\n"), []byte("event\n")}
	if err := ValidateTransaction(records, frames, makeTestCommit(records, frames, 7, 9)); err != nil {
		t.Fatalf("valid Channel+Event transaction: %v", err)
	}
}

func TestValidateTransactionEventOnly(t *testing.T) {
	event := Event{Sequence: 20, Kind: KindEvent}
	records := []interface{}{event}
	frames := [][]byte{[]byte("event\n")}
	if err := ValidateTransaction(records, frames, makeTestCommit(records, frames, 20, 21)); err != nil {
		t.Fatalf("valid Event-only transaction: %v", err)
	}
}

func TestValidateTransactionRejectsContractViolations(t *testing.T) {
	channel := Channel{Sequence: 1, Kind: KindChannel}
	event := Event{Sequence: 2, Kind: KindEvent}
	records := []interface{}{channel, event}
	frames := [][]byte{[]byte("channel\n"), []byte("event\n")}
	valid := makeTestCommit(records, frames, 1, 3)

	tests := []struct {
		name    string
		records []interface{}
		frames  [][]byte
		commit  Commit
	}{
		{name: "wrong transaction ID", records: records, frames: frames, commit: Commit{Sequence: 3, TransactionID: 99, RecordCount: 2, Checksum: valid.Checksum}},
		{name: "wrong count", records: records, frames: frames, commit: Commit{Sequence: 3, TransactionID: 1, RecordCount: 1, Checksum: valid.Checksum}},
		{name: "wrong commit sequence", records: records, frames: frames, commit: Commit{Sequence: 4, TransactionID: 1, RecordCount: 2, Checksum: valid.Checksum}},
		{name: "checksum mismatch", records: records, frames: frames, commit: Commit{Sequence: 3, TransactionID: 1, RecordCount: 2, Checksum: "invalid"}},
		{name: "nonconsecutive sequence", records: []interface{}{Channel{Sequence: 1, Kind: KindChannel}, Event{Sequence: 4, Kind: KindEvent}}, frames: frames, commit: Commit{Sequence: 5, TransactionID: 1, RecordCount: 2, Checksum: valid.Checksum}},
		{name: "event not last", records: []interface{}{event, channel}, frames: frames, commit: valid},
		{name: "missing event", records: []interface{}{channel}, frames: frames[:1], commit: Commit{Sequence: 2, TransactionID: 1, RecordCount: 1, Checksum: Checksum(frames[:1])}},
		{name: "frame mismatch", records: records, frames: frames[:1], commit: valid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateTransaction(tt.records, tt.frames, tt.commit); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestValidateTransactionMultipleOccurrences(t *testing.T) {
	records := []interface{}{
		Channel{Sequence: 1, Kind: KindChannel, ID: "c"},
		Event{Sequence: 2, Kind: KindEvent, ChannelID: "c"},
		Event{Sequence: 3, Kind: KindEvent, ChannelID: "c"},
		Channel{Sequence: 4, Kind: KindChannel, ID: "c"},
		Event{Sequence: 5, Kind: KindEvent, ChannelID: "c"},
	}
	frames := [][]byte{[]byte("1\n"), []byte("2\n"), []byte("3\n"), []byte("4\n"), []byte("5\n")}
	if err := ValidateTransaction(records, frames, makeTestCommit(records, frames, 1, 6)); err != nil {
		t.Fatalf("valid multi-occurrence transaction: %v", err)
	}
}

func TestValidateTransactionRejectsMixedChannels(t *testing.T) {
	frames := [][]byte{[]byte("1\n"), []byte("2\n")}
	cases := map[string][]interface{}{
		"two Channels' Events":                 {Event{Sequence: 1, Kind: KindEvent, ChannelID: "a"}, Event{Sequence: 2, Kind: KindEvent, ChannelID: "b"}},
		"Channel record for another Channel":   {Channel{Sequence: 1, Kind: KindChannel, ID: "a"}, Event{Sequence: 2, Kind: KindEvent, ChannelID: "b"}},
		"Channel record not followed by Event": {Event{Sequence: 1, Kind: KindEvent, ChannelID: "a"}, Channel{Sequence: 2, Kind: KindChannel, ID: "a"}},
	}
	for name, records := range cases {
		if err := ValidateTransaction(records, frames, makeTestCommit(records, frames, 1, 3)); err == nil {
			t.Fatalf("%s: expected rejection", name)
		}
	}
}
