package filmtv

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/beamfall/plugin-filmtv/internal/mediaimport"
)

// maxNFOFileBytes bounds a single NFO sidecar read so a hostile/huge sidecar
// cannot stall identification. The read streams into the
// XML decoder (never buffered whole), so this is a bound, not an allocation;
// 4 MiB is far beyond any real Kodi-family NFO (large tvshow.nfo files run
// tens of KB), while a truncated oversize file degrades to no hint.
const maxNFOFileBytes = 4 << 20

// videoExts is the media-file extension set the NFO tree walk recognises. NFO
// sidecar libraries (the Kodi/Plex/Emby/Jellyfin convention family) place one .nfo next to each video file, plus folder-level conventions.
// The walk keys on the video file, not the NFO, so the path matches what
// Beamfall scanned.
var videoExts = map[string]bool{
	".mkv": true, ".mp4": true, ".m4v": true, ".avi": true,
	".mov": true, ".wmv": true, ".ts": true, ".m2ts": true,
	".webm": true, ".mpg": true, ".mpeg": true, ".flv": true,
}

// walkErrOverride optionally replaces the error filepath.WalkDir reports for
// a given path during walkNFOTree, so tests can exercise the unreadable
// directory/file skip path deterministically instead of depending on
// platform-specific chmod behavior (permission bits are ignored when tests
// run as root and don't exist on Windows). Nil in production — never
// overridden outside tests.
var walkErrOverride func(path string) error

// walkNFOTree walks an NFO sidecar library rooted at dir and yields one
// mediaimport.NFOItem per video file, keyed by the video file's absolute path.
// For each video file it collects local artwork candidates beside the file
// (mediaimport.ArtworkCandidates). The parser independently reads NFO provider
// IDs during identification. The walk never writes NFO.
//
// Items are returned sorted by path for a deterministic run. An unreadable
// non-root directory or file is skipped and traversal continues. Only a
// failure to open/stat the root itself is fatal (nothing to walk).
func walkNFOTree(dir string) ([]mediaimport.NFOItem, error) {
	return walkNFOTreeContext(context.Background(), dir)
}

func walkNFOTreeContext(ctx context.Context, dir string) ([]mediaimport.NFOItem, error) {
	var items []mediaimport.NFOItem
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if walkErrOverride != nil {
			if injected := walkErrOverride(path); injected != nil {
				err = injected
			}
		}
		if err != nil {
			if path == dir {
				// The root itself could not be opened/stat'd: there is
				// nothing to walk, so this is not recoverable.
				return err
			}
			// A non-root directory or file was unreadable; skip it and keep
			// walking the rest of the tree.
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !videoExts[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("nfo tree: resolve %q: %w", path, err)
		}
		item := mediaimport.NFOItem{Path: abs}
		// ArtworkCandidates only stats sibling images; ignore its (always-nil)
		// error and keep whatever it found.
		if art, _ := mediaimport.ArtworkCandidates(path); len(art) > 0 {
			item.Artwork = art
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		items = append(items, item)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("nfo tree walk %q: %w", dir, err)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
	return items, nil
}

// nfoSidecarFor returns the nearest NFO describing mediaPath. A per-file
// sidecar wins, then the film-level folder NFO, then the nearest tvshow.nfo in
// the media folder or an ancestor show folder.
func nfoSidecarFor(mediaPath string) (string, bool) {
	dir := filepath.Dir(mediaPath)
	base := strings.TrimSuffix(filepath.Base(mediaPath), filepath.Ext(mediaPath))
	candidates := []string{
		filepath.Join(dir, base+".nfo"),
		filepath.Join(dir, "mo"+"vie.nfo"),
	}
	for current := dir; ; current = filepath.Dir(current) {
		candidates = append(candidates, filepath.Join(current, "tvshow.nfo"))
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			return candidate, true
		}
	}
	return "", false
}

// parseNFOFile opens and parses one NFO file through mediaimport.ParseNFO. A
// read or parse failure returns ok=false, making malformed NFO evidence absent.
func parseNFOFile(path string) (mediaimport.ParseHint, bool) {
	f, err := os.Open(path)
	if err != nil {
		return mediaimport.ParseHint{}, false
	}
	defer func() { _ = f.Close() }()
	hint, err := mediaimport.ParseNFO(io.LimitReader(f, maxNFOFileBytes))
	if err != nil {
		return mediaimport.ParseHint{}, false
	}
	return hint, true
}
