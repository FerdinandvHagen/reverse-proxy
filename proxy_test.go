package main

import "testing"

func TestCleanupPath(t *testing.T) {
	testStrings := []string{
		"/rest/api/1",
		"/rest/api/1/3",
		"/rest/api/1/locations/1234/rooms/5678",
	}

	expected := []string{
		"/rest/api/1",
		"/rest/api/1/_",
		"/rest/api/1/locations/_/rooms/_",
	}

	for i, s := range testStrings {
		if got := cleanupPath(s); got != expected[i] {
			t.Errorf("Expected %s, got %s", expected[i], got)
		}
	}
}
