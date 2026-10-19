package mediaimport

import (
	"os"
	"path/filepath"
	"strings"
)

var artworkExts = []string{".jpg", ".jpeg", ".png", ".webp"}

// ArtworkCandidates returns existing local image sidecars for mediaPath using
// predecessor naming conventions. It only stats candidate paths.
func ArtworkCandidates(mediaPath string) ([]ArtworkCandidate, error) {
	dir := filepath.Dir(mediaPath)
	base := strings.TrimSuffix(filepath.Base(mediaPath), filepath.Ext(mediaPath))
	var candidates []ArtworkCandidate
	for _, pattern := range []struct {
		stem string
		slot string
	}{
		{"poster", ArtworkSlotPoster},
		{"cover", ArtworkSlotPoster},
		{"folder", ArtworkSlotPoster},
		{"fanart", ArtworkSlotBackdrop},
		{"backdrop", ArtworkSlotBackdrop},
		{"thumb", ArtworkSlotThumb},
		{"landscape", ArtworkSlotThumb},
	} {
		candidates = appendExistingArtwork(candidates, dir, pattern.stem, pattern.slot)
	}
	for _, pattern := range []struct {
		suffix string
		slot   string
	}{
		{"-poster", ArtworkSlotPoster},
		{"-cover", ArtworkSlotPoster},
		{"-fanart", ArtworkSlotBackdrop},
		{"-backdrop", ArtworkSlotBackdrop},
		{"-thumb", ArtworkSlotThumb},
		{"-landscape", ArtworkSlotThumb},
		{"-logo", ArtworkSlotLogo},
	} {
		candidates = appendExistingArtwork(candidates, dir, base+pattern.suffix, pattern.slot)
	}
	return candidates, nil
}

func appendExistingArtwork(candidates []ArtworkCandidate, dir, stem, slot string) []ArtworkCandidate {
	for _, ext := range artworkExts {
		path := filepath.Join(dir, stem+ext)
		info, err := os.Stat(path)
		if err == nil && info.Mode().IsRegular() {
			candidates = append(candidates, ArtworkCandidate{Path: path, Slot: slot})
		}
	}
	return candidates
}
