package filesystem

import "testing"

// B-D02 smoke: one ranged read through client, server, and OS backend.

func BenchmarkReadRoundTrip(b *testing.B) {
	backend, _ := NewOSBackend(b.TempDir())
	server, _ := Serve(backend)
	defer server.Close()
	c := NewClient(server.URL(), server.Token())
	_ = c.CreateExclusive("f")
	_, _ = c.Append("f", make([]byte, 4096))
	for b.Loop() {
		if _, err := c.Read("f", 100, 200); err != nil {
			b.Fatal(err)
		}
	}
}
