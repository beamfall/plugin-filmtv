package mediaimport

import (
	"os"
	"path/filepath"
	"testing"
)

func TestArtworkCandidatesEnumeratesExistingSidecars(t *testing.T) {
	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "Example.Item.mkv")
	touch(t, mediaPath)
	touch(t, filepath.Join(dir, "poster.jpg"))
	touch(t, filepath.Join(dir, "fanart.png"))
	touch(t, filepath.Join(dir, "folder.jpeg"))
	touch(t, filepath.Join(dir, "Example.Item-poster.webp"))
	touch(t, filepath.Join(dir, "Example.Item-fanart.jpg"))
	touch(t, filepath.Join(dir, "Example.Item-thumb.jpg"))
	touch(t, filepath.Join(dir, "ignored.txt"))

	got, err := ArtworkCandidates(mediaPath)
	if err != nil {
		t.Fatalf("ArtworkCandidates() error = %v", err)
	}

	want := []ArtworkCandidate{
		{Path: filepath.Join(dir, "poster.jpg"), Slot: ArtworkSlotPoster},
		{Path: filepath.Join(dir, "folder.jpeg"), Slot: ArtworkSlotPoster},
		{Path: filepath.Join(dir, "fanart.png"), Slot: ArtworkSlotBackdrop},
		{Path: filepath.Join(dir, "Example.Item-poster.webp"), Slot: ArtworkSlotPoster},
		{Path: filepath.Join(dir, "Example.Item-fanart.jpg"), Slot: ArtworkSlotBackdrop},
		{Path: filepath.Join(dir, "Example.Item-thumb.jpg"), Slot: ArtworkSlotThumb},
	}
	if len(got) != len(want) {
		t.Fatalf("candidate length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("candidate[%d] = %#v, want %#v; full result %#v", i, got[i], want[i], got)
		}
	}
}

func TestArtworkCandidatesIgnoresMissingAndDirectories(t *testing.T) {
	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "Example.Item.mkv")
	touch(t, mediaPath)
	if err := os.Mkdir(filepath.Join(dir, "poster.jpg"), 0o755); err != nil {
		t.Fatalf("mkdir poster.jpg: %v", err)
	}

	got, err := ArtworkCandidates(mediaPath)
	if err != nil {
		t.Fatalf("ArtworkCandidates() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("candidates = %#v, want none", got)
	}
}

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
