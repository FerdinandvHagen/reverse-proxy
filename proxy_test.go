package main

import (
	"net/url"
	"testing"
)

func TestCleanupPath(t *testing.T) {
	testStrings := []string{
		"/rest/api/1",
		"/rest/api/1/3",
		"/rest/api/1/locations/1234/rooms/5678",
		"/rest/api/1/f45c21e1-d363-4a02-825e-0d45a0eebc12",
		"/rest/api/1/locations/1234/rooms/a6e1c31d-9c6c-4297-9103-467e9bc3cb51",
	}

	expected := []string{
		"/rest/api/1",
		"/rest/api/1/_",
		"/rest/api/1/locations/_/rooms/_",
		"/rest/api/1/_",
		"/rest/api/1/locations/_/rooms/_",
	}

	for i, s := range testStrings {
		u, _ := url.Parse(s)
		path, ok := cleanupPath(u, "/rest/api", true, nil)
		if !ok || path != expected[i] {
			t.Errorf("Expected %s, got %s", expected[i], path)
		}
	}
}
