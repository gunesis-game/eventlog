package storage

import "os"

// segment is one authoritative segment file of the current generation. refs
// counts reads in progress; a retired segment is deleted once its last read
// finishes. reader is a shared read handle opened on first read; ReadAt is safe
// for concurrent use. The store's mutex guards every field.
type segment struct {
	id      uint64
	path    string
	size    int64
	refs    int
	retired bool
	reader  *os.File
}

// readHandle returns the shared read handle, opening it on first use.
func (s *segment) readHandle() (*os.File, error) {
	if s.reader == nil {
		f, err := openSegmentRead(s.path)
		if err != nil {
			return nil, err
		}
		s.reader = f
	}
	return s.reader, nil
}

func (s *segment) closeReader() {
	if s.reader != nil {
		_ = s.reader.Close()
		s.reader = nil
	}
}
