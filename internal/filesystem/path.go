package filesystem

import (
	"fmt"
	"path"
	"strings"
)

// cleanPath validates a storage-relative, slash-separated path. The empty
// path is the storage root.
func cleanPath(p string) (string, error) {
	if p == "" {
		return "", nil
	}
	if strings.ContainsAny(p, "\\\x00") || strings.HasPrefix(p, "/") || strings.Contains(p, ":") {
		return "", fmt.Errorf("invalid storage path %q", p)
	}
	cleaned := path.Clean(p)
	if cleaned != p || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("invalid storage path %q", p)
	}
	return cleaned, nil
}
