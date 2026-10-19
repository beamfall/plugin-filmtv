package filmtv

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestWalkNFOTreeSurfacesPathsAndArtwork asserts the migration walk yields one
// path-keyed item per video file and local artwork. Provider IDs are read by
// Parser during identification, not carried through the import source.
func TestWalkNFOTreeSurfacesPathsAndArtwork(t *testing.T) {
	root := t.TempDir()

	// Per-file sidecar + poster.
	write(t, filepath.Join(root, "A", "A.mkv"), "x")
	write(t, filepath.Join(root, "A", "A.nfo"), `<movie><title>A</title><year>2024</year><uniqueid type="tmdb">12345</uniqueid></movie>`)
	write(t, filepath.Join(root, "A", "poster.jpg"), "img")
	// Folder-level NFO is irrelevant to the migration walk.
	write(t, filepath.Join(root, "B", "video.mkv"), "y")
	write(t, filepath.Join(root, "B", "movie.nfo"), `<movie><title>B</title><uniqueid type="imdb">tt9999999</uniqueid></movie>`)
	// No sidecar at all → path-only item.
	write(t, filepath.Join(root, "C", "c.mkv"), "z")
	// A non-video file is ignored.
	write(t, filepath.Join(root, "C", "readme.txt"), "ignore")

	items, err := walkNFOTree(root)
	if err != nil {
		t.Fatalf("walkNFOTree: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3: %#v", len(items), items)
	}

	byBase := map[string]int{}
	for i, it := range items {
		byBase[filepath.Base(it.Path)] = i
	}

	a := items[byBase["A.mkv"]]
	if len(a.Artwork) == 0 {
		t.Fatalf("A artwork = %#v, want the poster candidate", a.Artwork)
	}
}

// TestParseNFOFileBoundsReadSize asserts an oversized NFO sidecar is read
// through the maxNFOFileBytes cap rather than fully into memory: the fixture
// exceeds the cap, so parseNFOFile sees a truncated document (cut mid-tag)
// and returns ok=false instead of allocating unbounded memory for a
// hostile/huge sidecar.
func TestParseNFOFileBoundsReadSize(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "big.nfo")

	padding := strings.Repeat("x", maxNFOFileBytes)
	write(t, path, `<movie><title>`+padding+`</title><uniqueid type="tmdb">12345</uniqueid></movie>`)

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat fixture: %v", err)
	}
	if info.Size() <= maxNFOFileBytes {
		t.Fatalf("fixture size = %d, want > %d", info.Size(), maxNFOFileBytes)
	}

	if _, ok := parseNFOFile(path); ok {
		t.Fatalf("parseNFOFile() ok = true, want false for an over-cap file (truncated mid-document)")
	}
}

// TestWalkNFOTreeSkipsUnreadableNonRootFile asserts that a non-root file the
// walk cannot read (permission denied, a vanished entry, ...) is skipped —
// dropped from the result, not fatal to the rest of the walk — matching the
// same "one bad entry must not lose the rest of the library" posture as a
// corrupt NFO sidecar. Uses walkErrOverride (not chmod) so the failure is
// deterministic regardless of platform or whether the test runs as root.
func TestWalkNFOTreeSkipsUnreadableNonRootFile(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "A", "A.mkv"), "x")
	write(t, filepath.Join(root, "B", "bad.mkv"), "y")

	badPath := filepath.Join(root, "B", "bad.mkv")
	injected := errors.New("injected: permission denied")

	prev := walkErrOverride
	walkErrOverride = func(path string) error {
		if path == badPath {
			return injected
		}
		return nil
	}
	defer func() { walkErrOverride = prev }()

	items, err := walkNFOTree(root)
	if err != nil {
		t.Fatalf("walkNFOTree: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1 (bad.mkv skipped): %#v", len(items), items)
	}
	if filepath.Base(items[0].Path) != "A.mkv" {
		t.Fatalf("items[0] = %#v, want A.mkv", items[0])
	}
}

// TestWalkNFOTreeSkipsUnreadableNonRootDirectory asserts that a non-root
// directory the walk cannot open/read is skipped and traversal continues to
// its siblings, rather than aborting the whole walk. Uses walkErrOverride so
// the failure is deterministic instead of depending on platform-specific
// chmod behavior.
func TestWalkNFOTreeSkipsUnreadableNonRootDirectory(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "A", "A.mkv"), "x")
	badDir := filepath.Join(root, "bad")
	if err := os.MkdirAll(badDir, 0o755); err != nil {
		t.Fatal(err)
	}

	injected := errors.New("injected: permission denied")
	prev := walkErrOverride
	walkErrOverride = func(path string) error {
		if path == badDir {
			return injected
		}
		return nil
	}
	defer func() { walkErrOverride = prev }()

	items, err := walkNFOTree(root)
	if err != nil {
		t.Fatalf("walkNFOTree: %v", err)
	}
	if len(items) != 1 || filepath.Base(items[0].Path) != "A.mkv" {
		t.Fatalf("items = %#v, want just A.mkv", items)
	}
}

// TestWalkNFOTreeReturnsRootOpenFailure asserts that a root that cannot be
// opened/stat'd at all (here: doesn't exist) is still fatal — there is
// nothing to walk — unlike a non-root directory/file failure, which is
// skipped.
func TestWalkNFOTreeReturnsRootOpenFailure(t *testing.T) {
	root := filepath.Join(t.TempDir(), "does-not-exist")

	if _, err := walkNFOTree(root); err == nil {
		t.Fatal("walkNFOTree() err = nil, want non-nil for an unopenable root")
	}
}
