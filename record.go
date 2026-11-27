package eventlog

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var ErrIncompleteRecord = errors.New("incomplete record")

type recordType struct {
	PackagePath string `json:"package_path"`
	Name        string `json:"name"`
}

type channelRecord struct {
	SchemaVersion int                    `json:"schema_version"`
	Sequence      uint64                 `json:"sequence"`
	Kind          string                 `json:"kind"`
	ID            string                 `json:"id"`
	Type          recordType             `json:"type"`
	Fields        map[string]interface{} `json:"fields"`
}

type eventRecord struct {
	SchemaVersion int                    `json:"schema_version"`
	Sequence      uint64                 `json:"sequence"`
	Kind          string                 `json:"kind"`
	ID            string                 `json:"id"`
	ChannelID     string                 `json:"channel_id"`
	Type          recordType             `json:"type"`
	Fields        map[string]interface{} `json:"fields"`
}

type commitRecord struct {
	SchemaVersion int    `json:"schema_version"`
	Sequence      uint64 `json:"sequence"`
	Kind          string `json:"kind"`
	TransactionID uint64 `json:"transaction_id"`
	RecordCount   uint64 `json:"record_count"`
	Checksum      string `json:"checksum"`
}

type recordFramer struct {
	reader *bufio.Reader
}

func newRecordFramer(r io.Reader) *recordFramer {
	return &recordFramer{reader: bufio.NewReader(r)}
}

// next returns one newline-delimited record without its newline. A partial final
// line is reported as ErrIncompleteRecord and is not treated as a complete record.
func (f *recordFramer) next() ([]byte, error) {
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

func encodeRecord(kind string, record interface{}, validator *recordSchemaValidator) ([]byte, error) {
	if validator == nil {
		return nil, errors.New("record schema validator is nil")
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return nil, fmt.Errorf("marshal %s record: %w", kind, err)
	}
	if err := validator.validate(kind, raw); err != nil {
		return nil, fmt.Errorf("encode %s record: %w", kind, err)
	}
	return append(raw, '\n'), nil
}

// decodeRecord validates a complete record and returns its concrete internal
// record type. The input excludes the framing newline.
func decodeRecord(raw []byte, validator *recordSchemaValidator) (interface{}, error) {
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
	case recordKindChannel:
		var record channelRecord
		if err := validator.validate(recordKindChannel, raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, fmt.Errorf("decode channel record: %w", err)
		}
		return record, nil
	case recordKindEvent:
		var record eventRecord
		if err := validator.validate(recordKindEvent, raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, fmt.Errorf("decode event record: %w", err)
		}
		return record, nil
	case recordKindCommit:
		var record commitRecord
		if err := validator.validate(recordKindCommit, raw); err != nil {
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
