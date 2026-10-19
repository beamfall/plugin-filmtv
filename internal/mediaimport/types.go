// Package mediaimport parses predecessor export sidecars into neutral import
// structs. It does not create jobs, write stores, or depend on plugin code.
package mediaimport

import "time"

// ExternalID is a source-qualified identifier from a predecessor export.
type ExternalID struct {
	Source string
	ID     string
}

// ParseHint is metadata the identification pipeline can use as an input hint.
type ParseHint struct {
	Title       string
	Year        int
	ExternalIDs []ExternalID
}

// Artwork slots are the fixed core artwork vocabulary used by candidates.
const (
	ArtworkSlotPoster   = "poster"
	ArtworkSlotBackdrop = "backdrop"
	ArtworkSlotThumb    = "thumb"
	ArtworkSlotLogo     = "logo"
	ArtworkSlotPortrait = "portrait"
)

// ArtworkCandidate is a local sidecar image associated with a Media file.
type ArtworkCandidate struct {
	Path string
	Slot string
}

// WatchState is a neutral playback-history row keyed by an external ID.
type WatchState struct {
	ExternalID      ExternalID
	Watched         bool
	PositionSeconds int64
	LastPlayed      time.Time
}

// OrderedList is an imported ordered collection keyed by external IDs.
type OrderedList struct {
	Name  string
	Items []ExternalID
}

// NFOItem is one media file discovered in an NFO sidecar tree: the media file path and local artwork candidates beside it.
type NFOItem struct {
	// Path is the absolute path of the media file the NFO describes.
	Path string
	// Artwork is the local image sidecars found beside the media file.
	Artwork []ArtworkCandidate
}
