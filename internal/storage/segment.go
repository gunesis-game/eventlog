package storage

// segment is one authoritative segment file of the current generation. path
// is storage-relative. refs counts reads in progress; a retired segment is
// deleted once its last read finishes. The store's mutex guards every field.
type segment struct {
	id      uint64
	path    string
	size    int64
	refs    int
	retired bool
}
