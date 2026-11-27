package eventlog

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

// transactionChecksum returns the lowercase SHA-256 checksum of the exact
// serialized transaction-record frames. Each frame must include its terminating
// newline. Commit records are not included.
func transactionChecksum(records [][]byte) string {
	hash := sha256.New()
	for _, record := range records {
		_, _ = hash.Write(record)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// errInvalidTransaction marks a complete record group that violates the v1
// transaction contract.
var errInvalidTransaction = errors.New("invalid transaction")

// validateTransaction validates a complete transaction using decoded records and
// their exact newline-terminated serialized frames. A transaction belongs to one
// Channel and holds one or more Event records, each optionally preceded by a
// Channel record for that same Channel. The Commit record is decoded separately
// and is not included in frames.
func validateTransaction(records []interface{}, frames [][]byte, commit commitRecord) error {
	if len(records) != len(frames) {
		return fmt.Errorf("%w: %d records but %d frames", errInvalidTransaction, len(records), len(frames))
	}
	if len(records) == 0 {
		return fmt.Errorf("%w: transaction has no records", errInvalidTransaction)
	}
	var firstSequence uint64
	var channelID string
	for i, record := range records {
		var sequence uint64
		switch r := record.(type) {
		case channelRecord:
			next, ok := nextEventRecord(records, i)
			if !ok || next.ChannelID != r.ID {
				return fmt.Errorf("%w: Channel record at %d is not followed by an Event for that Channel", errInvalidTransaction, i)
			}
			sequence = r.Sequence
		case eventRecord:
			if i == 0 {
				channelID = r.ChannelID
			} else if r.ChannelID != channelID {
				return fmt.Errorf("%w: transaction spans more than one Channel", errInvalidTransaction)
			}
			sequence = r.Sequence
		default:
			return fmt.Errorf("%w: unexpected transaction record type %T", errInvalidTransaction, record)
		}
		if i == 0 {
			firstSequence = sequence
			if channel, ok := record.(channelRecord); ok {
				channelID = channel.ID
			}
		} else if sequence != firstSequence+uint64(i) {
			return fmt.Errorf("%w: nonconsecutive transaction sequence at record %d", errInvalidTransaction, i)
		}
	}
	if _, ok := records[len(records)-1].(eventRecord); !ok {
		return fmt.Errorf("%w: transaction must end with an Event record", errInvalidTransaction)
	}
	if commit.TransactionID != firstSequence {
		return fmt.Errorf("%w: commit transaction ID %d does not match first sequence %d", errInvalidTransaction, commit.TransactionID, firstSequence)
	}
	if commit.RecordCount != uint64(len(records)) {
		return fmt.Errorf("%w: commit record count %d does not match transaction size %d", errInvalidTransaction, commit.RecordCount, len(records))
	}
	if commit.Sequence != firstSequence+uint64(len(records)) {
		return fmt.Errorf("%w: commit sequence %d is not consecutive", errInvalidTransaction, commit.Sequence)
	}
	if commit.Checksum != transactionChecksum(frames) {
		return fmt.Errorf("%w: checksum mismatch", errInvalidTransaction)
	}
	return nil
}

func nextEventRecord(records []interface{}, i int) (eventRecord, bool) {
	if i+1 >= len(records) {
		return eventRecord{}, false
	}
	next, ok := records[i+1].(eventRecord)
	return next, ok
}
