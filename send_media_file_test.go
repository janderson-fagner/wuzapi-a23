package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestReadUserMediaFile verifies that "file:" references are sandboxed to the
// caller's own files directory and that path-traversal attempts are rejected.
func TestReadUserMediaFile(t *testing.T) {
	tmp := t.TempDir()
	userDir := filepath.Join(tmp, "files", "user_42")
	if err := os.MkdirAll(filepath.Join(userDir, "sub"), 0o751); err != nil {
		t.Fatal(err)
	}
	want := []byte("hello world")
	if err := os.WriteFile(filepath.Join(userDir, "ok.jpg"), want, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "sub", "nested.png"), want, 0o600); err != nil {
		t.Fatal(err)
	}
	// A secret outside the user directory that must never be readable.
	if err := os.WriteFile(filepath.Join(tmp, "secret.txt"), []byte("TOPSECRET"), 0o600); err != nil {
		t.Fatal(err)
	}

	s := &server{exPath: tmp}

	t.Run("reads own file", func(t *testing.T) {
		got, err := s.readUserMediaFile("42", "file:ok.jpg")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(got) != string(want) {
			t.Fatalf("got %q want %q", got, want)
		}
	})

	t.Run("reads nested file", func(t *testing.T) {
		if _, err := s.readUserMediaFile("42", "file:sub/nested.png"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	rejects := []struct {
		name string
		ref  string
	}{
		{"parent traversal", "file:../secret.txt"},
		{"deep traversal", "file:../../secret.txt"},
		{"absolute path", "file:/etc/passwd"},
		{"other user dir", "file:../user_99/x.jpg"},
		{"empty", "file:"},
	}
	for _, tc := range rejects {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			if _, err := s.readUserMediaFile("42", tc.ref); err == nil {
				t.Fatalf("expected error for %q, got nil", tc.ref)
			}
		})
	}
}
