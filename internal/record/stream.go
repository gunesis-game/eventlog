package record

import (
	"bytes"
	"fmt"
)

// StreamValidation describes the verified prefix of a segment. IncompleteStart is
// set when the final transaction or frame is incomplete and identifies the byte
// offset at which recovery must truncate.
type StreamValidation struct {
	CommittedSequence uint64
	CommittedOffset   int64
	IncompleteStart   *int64
}

// ValidateStream validates a segment whose first global sequence is
// expectedSequence. Segment sequences are global and therefore do not restart at 1.
func ValidateStream(data []byte, validator *Validator, expectedSequence uint64) (StreamValidation, error) {
	if expectedSequence == 0 {
		return StreamValidation{}, fmt.Errorf("%w: stream starting sequence must be positive", ErrInvalidTransaction)
	}
	result := StreamValidation{CommittedSequence: expectedSequence - 1}
	var pendingRecords []interface{}
	var pendingFrames [][]byte
	var pendingStart int64
	offset := int64(0)

	for offset < int64(len(data)) {
		remaining := data[offset:]
		newline := bytes.IndexByte(remaining, '\n')
		if newline < 0 {
			start := offset
			if len(pendingRecords) > 0 {
				start = pendingStart
			}
			result.IncompleteStart = &start
			return result, nil
		}
		raw := remaining[:newline]
		frame := append(append([]byte(nil), raw...), '\n')
		frameStart := offset
		frameEnd := offset + int64(newline) + 1
		offset = frameEnd

		record, err := Decode(raw, validator)
		if err != nil {
			return result, fmt.Errorf("decode record at byte offset %d: %w", frameStart, err)
		}
		sequence, kind, err := SequenceAndKind(record)
		if err != nil {
			return result, fmt.Errorf("record at byte offset %d: %w", frameStart, err)
		}
		if sequence != expectedSequence {
			return result, fmt.Errorf("record sequence %d at byte offset %d, want %d", sequence, frameStart, expectedSequence)
		}
		expectedSequence++

		if kind != KindCommit {
			if len(pendingRecords) == 0 {
				pendingStart = frameStart
			}
			pendingRecords = append(pendingRecords, record)
			pendingFrames = append(pendingFrames, frame)
			continue
		}

		commit, ok := record.(Commit)
		if !ok {
			return result, fmt.Errorf("unexpected Commit record type %T", record)
		}
		if len(pendingRecords) == 0 {
			return result, fmt.Errorf("%w: Commit without transaction records at byte offset %d", ErrInvalidTransaction, frameStart)
		}
		if err := ValidateTransaction(pendingRecords, pendingFrames, commit); err != nil {
			return result, fmt.Errorf("validate transaction starting at byte offset %d: %w", pendingStart, err)
		}
		result.CommittedSequence = commit.Sequence
		result.CommittedOffset = frameEnd
		pendingRecords = nil
		pendingFrames = nil
	}

	if len(pendingRecords) > 0 {
		start := pendingStart
		result.IncompleteStart = &start
	}
	return result, nil
}

func SequenceAndKind(record interface{}) (uint64, string, error) {
	switch r := record.(type) {
	case Channel:
		return r.Sequence, r.Kind, nil
	case Event:
		return r.Sequence, r.Kind, nil
	case Commit:
		return r.Sequence, r.Kind, nil
	default:
		return 0, "", fmt.Errorf("unsupported decoded record type %T", record)
	}
}
