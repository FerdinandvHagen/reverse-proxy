package main

import "testing"

func TestCleanupPath(t *testing.T) {
	testStrings := []string{
		"/api/rest/1",
		"/api/rest/1/3",
		"/api/rest/1/locations/1234/rooms/5678",
	}

	expected := []string{
		"/api/rest/1",
		"/api/rest/1/_",
		"/api/rest/1/locations/_/rooms/_",
	}

	for i, s := range testStrings {
		if got := cleanupPath(s); got != expected[i] {
			t.Errorf("Expected %s, got %s", expected[i], got)
		}
	}
}
