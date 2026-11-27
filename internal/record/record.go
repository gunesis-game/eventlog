// Package record defines eventlog's v1 persisted records: their types, NDJSON
// framing, schema validation against the embedded canonical JSON Schemas,
// checksums, and transaction and stream validation.
package record

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var ErrIncompleteRecord = errors.New("incomplete record")

type Type struct {
	PackagePath string `json:"package_path"`
	Name        string `json:"name"`
}

type Channel struct {
	SchemaVersion int                    `json:"schema_version"`
	Sequence      uint64                 `json:"sequence"`
	Kind          string                 `json:"kind"`
	ID            string                 `json:"id"`
	Type          Type                   `json:"type"`
	Fields        map[string]interface{} `json:"fields"`
}

type Event struct {
	SchemaVersion int                    `json:"schema_version"`
	Sequence      uint64                 `json:"sequence"`
	Kind          string                 `json:"kind"`
	ID            string                 `json:"id"`
	ChannelID     string                 `json:"channel_id"`
	Type          Type                   `json:"type"`
	Fields        map[string]interface{} `json:"fields"`
}

type Commit struct {
	SchemaVersion int    `json:"schema_version"`
	Sequence      uint64 `json:"sequence"`
	Kind          string `json:"kind"`
	TransactionID uint64 `json:"transaction_id"`
	RecordCount   uint64 `json:"record_count"`
	Checksum      string `json:"checksum"`
}

type Framer struct {
	reader *bufio.Reader
}

func NewFramer(r io.Reader) *Framer {
	return &Framer{reader: bufio.NewReader(r)}
}

// next returns one newline-delimited record without its newline. A partial final
// line is reported as ErrIncompleteRecord and is not treated as a complete record.
func (f *Framer) Next() ([]byte, error) {
	line, err := f.reader.ReadBytes('\n')
	if err != nil {
		if errors.Is(err, io.EOF) {
			if len(line) == 0 {
				return nil, io.EOF
			}
			return line, fmt.Errorf("%w: trailing %d bytes", ErrIncompleteRecord, len(line))
		}
		return nil, fmt.Errorf("read record frame: %w", err)
	}
	return bytes.TrimSuffix(line, []byte{'\n'}), nil
}

func Encode(kind string, record interface{}, validator *Validator) ([]byte, error) {
	if validator == nil {
		return nil, errors.New("record schema validator is nil")
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return nil, fmt.Errorf("marshal %s record: %w", kind, err)
	}
	if err := validator.Validate(kind, raw); err != nil {
		return nil, fmt.Errorf("encode %s record: %w", kind, err)
	}
	return append(raw, '\n'), nil
}

// Decode validates a complete record and returns its concrete internal
// record type. The input excludes the framing newline.
func Decode(raw []byte, validator *Validator) (interface{}, error) {
	if validator == nil {
		return nil, errors.New("record schema validator is nil")
	}
	var header struct {
		SchemaVersion int    `json:"schema_version"`
		Kind          string `json:"kind"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return nil, fmt.Errorf("decode record envelope: %w", err)
	}
	if header.SchemaVersion != 1 {
		return nil, fmt.Errorf("unsupported record schema version %d", header.SchemaVersion)
	}

	switch header.Kind {
	case KindChannel:
		var record Channel
		if err := validator.Validate(KindChannel, raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, fmt.Errorf("decode channel record: %w", err)
		}
		return record, nil
	case KindEvent:
		var record Event
		if err := validator.Validate(KindEvent, raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, fmt.Errorf("decode event record: %w", err)
		}
		return record, nil
	case KindCommit:
		var record Commit
		if err := validator.Validate(KindCommit, raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, fmt.Errorf("decode commit record: %w", err)
		}
		return record, nil
	default:
		return nil, fmt.Errorf("unsupported record kind %q", header.Kind)
	}
}
