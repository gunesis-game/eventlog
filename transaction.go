package eventlog

import (
	"crypto/sha256"
	"encoding/hex"
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
