package browser

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublishedCoreArtifactIntegrity(t *testing.T) {
	archive := []byte("offline published archive fixture; never executable")
	digest := sha256.Sum256(archive)
	expected := hex.EncodeToString(digest[:])
	for _, tc := range []struct {
		name    string
		payload []byte
		size    int64
		digest  string
		valid   bool
	}{
		{"platform-issued-bytes", archive, int64(len(archive)), expected, true},
		{"tampered-same-length", bytes.Repeat([]byte("x"), len(archive)), int64(len(archive)), expected, false},
		{"truncated", archive[:len(archive)-1], int64(len(archive)), expected, false},
		{"appended", append(append([]byte{}, archive...), 'x'), int64(len(archive)), expected, false},
		{"empty-size", archive, 0, expected, false},
		{"negative-size", archive, -1, expected, false},
		{"oversized-metadata", archive, MaxPublishedCoreArchiveBytes + 1, expected, false},
		{"missing-digest", archive, int64(len(archive)), "", false},
		{"invalid-digest", archive, int64(len(archive)), strings.Repeat("z", 64), false},
		{"noncanonical-digest", archive, int64(len(archive)), strings.ToUpper(expected), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "archive.zip")
			if err := os.WriteFile(file, tc.payload, 0600); err != nil {
				t.Fatal(err)
			}
			err := VerifyPublishedCoreArchive(file, tc.size, tc.digest)
			if tc.valid && err != nil || !tc.valid && !errors.Is(err, ErrPublishedCoreIntegrity) {
				t.Fatalf("artifact verification error = %v, valid=%t", err, tc.valid)
			}
			after, err := os.ReadFile(file)
			if err != nil || !bytes.Equal(after, tc.payload) {
				t.Fatal("verification altered the artifact")
			}
		})
	}
	for _, file := range []string{t.TempDir(), filepath.Join(t.TempDir(), "missing.zip")} {
		if !errors.Is(VerifyPublishedCoreArchive(file, int64(len(archive)), expected), ErrPublishedCoreIntegrity) {
			t.Fatal("directory or missing file accepted as a published archive")
		}
	}
}
