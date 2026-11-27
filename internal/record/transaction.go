package record

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

// Checksum returns the lowercase SHA-256 checksum of the exact
// serialized transaction-record frames. Each frame must include its terminating
// newline. Commit records are not included.
func Checksum(records [][]byte) string {
	hash := sha256.New()
	for _, record := range records {
		_, _ = hash.Write(record)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// ErrInvalidTransaction marks a complete record group that violates the v1
// transaction contract.
var ErrInvalidTransaction = errors.New("invalid transaction")

// ValidateTransaction validates a complete transaction using decoded records and
// their exact newline-terminated serialized frames. A transaction belongs to one
// Channel and holds one or more Event records, each optionally preceded by a
// Channel record for that same Channel. The Commit record is decoded separately
// and is not included in frames.
func ValidateTransaction(records []interface{}, frames [][]byte, commit Commit) error {
	if len(records) != len(frames) {
		return fmt.Errorf("%w: %d records but %d frames", ErrInvalidTransaction, len(records), len(frames))
	}
	if len(records) == 0 {
		return fmt.Errorf("%w: transaction has no records", ErrInvalidTransaction)
	}
	var firstSequence uint64
	var channelID string
	for i, record := range records {
		var sequence uint64
		switch r := record.(type) {
		case Channel:
			next, ok := nextEvent(records, i)
			if !ok || next.ChannelID != r.ID {
				return fmt.Errorf("%w: Channel record at %d is not followed by an Event for that Channel", ErrInvalidTransaction, i)
			}
			sequence = r.Sequence
		case Event:
			if i == 0 {
				channelID = r.ChannelID
			} else if r.ChannelID != channelID {
				return fmt.Errorf("%w: transaction spans more than one Channel", ErrInvalidTransaction)
			}
			sequence = r.Sequence
		default:
			return fmt.Errorf("%w: unexpected transaction record type %T", ErrInvalidTransaction, record)
		}
		if i == 0 {
			firstSequence = sequence
			if channel, ok := record.(Channel); ok {
				channelID = channel.ID
			}
		} else if sequence != firstSequence+uint64(i) {
			return fmt.Errorf("%w: nonconsecutive transaction sequence at record %d", ErrInvalidTransaction, i)
		}
	}
	if _, ok := records[len(records)-1].(Event); !ok {
		return fmt.Errorf("%w: transaction must end with an Event record", ErrInvalidTransaction)
	}
	if commit.TransactionID != firstSequence {
		return fmt.Errorf("%w: commit transaction ID %d does not match first sequence %d", ErrInvalidTransaction, commit.TransactionID, firstSequence)
	}
	if commit.RecordCount != uint64(len(records)) {
		return fmt.Errorf("%w: commit record count %d does not match transaction size %d", ErrInvalidTransaction, commit.RecordCount, len(records))
	}
	if commit.Sequence != firstSequence+uint64(len(records)) {
		return fmt.Errorf("%w: commit sequence %d is not consecutive", ErrInvalidTransaction, commit.Sequence)
	}
	if commit.Checksum != Checksum(frames) {
		return fmt.Errorf("%w: checksum mismatch", ErrInvalidTransaction)
	}
	return nil
}

func nextEvent(records []interface{}, i int) (Event, bool) {
	if i+1 >= len(records) {
		return Event{}, false
	}
	next, ok := records[i+1].(Event)
	return next, ok
}
