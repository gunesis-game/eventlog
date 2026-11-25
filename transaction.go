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

// ErrInvalidTransaction marks a complete record group that violates the v1
// transaction contract.
var ErrInvalidTransaction = errors.New("invalid transaction")

// validateTransaction validates a complete transaction using decoded records and
// their exact newline-terminated serialized frames. It accepts either Event-only
// transactions (for an existing Channel) or Channel+Event transactions (for a new
// Channel). The Commit record is decoded separately and is not included in frames.
func validateTransaction(records []interface{}, frames [][]byte, commit commitRecord) error {
	if len(records) != len(frames) {
		return fmt.Errorf("%w: %d records but %d frames", ErrInvalidTransaction, len(records), len(frames))
	}
	if len(records) < 1 || len(records) > 2 {
		return fmt.Errorf("%w: transaction must contain one Event and optional Channel record", ErrInvalidTransaction)
	}

	var firstSequence uint64
	var eventCount, channelCount int
	for i, record := range records {
		var sequence uint64
		switch r := record.(type) {
		case channelRecord:
			channelCount++
			if i != 0 || len(records) != 2 {
				return fmt.Errorf("%w: Channel record must be first and paired with an Event", ErrInvalidTransaction)
			}
			sequence = r.Sequence
		case eventRecord:
			eventCount++
			if i != len(records)-1 {
				return fmt.Errorf("%w: Event record must be last", ErrInvalidTransaction)
			}
			sequence = r.Sequence
		default:
			return fmt.Errorf("%w: unexpected transaction record type %T", ErrInvalidTransaction, record)
		}
		if i == 0 {
			firstSequence = sequence
		} else if sequence != firstSequence+uint64(i) {
			return fmt.Errorf("%w: nonconsecutive transaction sequence at record %d", ErrInvalidTransaction, i)
		}
	}
	if channelCount > 1 || eventCount != 1 {
		return fmt.Errorf("%w: requires exactly one Event and at most one Channel", ErrInvalidTransaction)
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
	if commit.Checksum != transactionChecksum(frames) {
		return fmt.Errorf("%w: checksum mismatch", ErrInvalidTransaction)
	}
	return nil
}
