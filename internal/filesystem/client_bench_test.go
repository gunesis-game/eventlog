package filesystem

import "testing"

// Smoke benchmarks for the file-system boundary (docs/project/benchmarks.md):
// every request goes through client, server, and OS backend. No numeric
// targets are approved (D-040).

func benchmarkClient(b *testing.B) *Client {
	b.Helper()
	backend, err := NewOSBackend(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	server, err := Serve(backend)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		_ = server.Close()
	})
	c := NewClient(server.URL(), server.Token())
	b.Cleanup(c.Close)
	if err := c.CreateExclusive("f"); err != nil {
		b.Fatal(err)
	}
	return c
}

// B-D02: one ranged read of 200 bytes.
func BenchmarkReadRoundTrip(b *testing.B) {
	c := benchmarkClient(b)
	if _, err := c.Append("f", make([]byte, 4096)); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.SetBytes(200)
	for b.Loop() {
		data, err := c.Read("f", 100, 200)
		if err != nil {
			b.Fatal(err)
		}
		if len(data) != 200 {
			b.Fatalf("read %d bytes, want 200", len(data))
		}
	}
}

// B-D03: small (128-byte) append operations to one file.
func BenchmarkSmallAppend(b *testing.B) {
	const size = 128
	c := benchmarkClient(b)
	record := make([]byte, size)
	b.ReportAllocs()
	b.SetBytes(size)
	var want int64
	for b.Loop() {
		got, err := c.Append("f", record)
		if err != nil {
			b.Fatal(err)
		}
		want += size
		if got != want {
			b.Fatalf("size after append = %d, want %d", got, want)
		}
	}
}
