package qdrant

import "testing"

func TestParseGrpcAddr(t *testing.T) {
	cases := []struct {
		in     string
		host   string
		port   int
		useTLS bool
	}{
		{"http://localhost:6333", "localhost", 6334, false},
		{"http://localhost:6334", "localhost", 6334, false},
		{"http://localhost", "localhost", 6334, false},
		{"https://xyz.qdrant.io:6333", "xyz.qdrant.io", 6334, true},
		{"https://xyz.qdrant.io:6334", "xyz.qdrant.io", 6334, true},
		{"http://q.cloud:443", "q.cloud", 443, false},
		{"localhost:6334", "localhost", 6334, false},
	}
	for _, c := range cases {
		host, port, useTLS, err := ParseGrpcAddr(c.in)
		if err != nil {
			t.Fatalf("ParseGrpcAddr(%q) error: %v", c.in, err)
		}
		if host != c.host || port != c.port || useTLS != c.useTLS {
			t.Fatalf("ParseGrpcAddr(%q) = (%s,%d,%v), want (%s,%d,%v)",
				c.in, host, port, useTLS, c.host, c.port, c.useTLS)
		}
	}
}
