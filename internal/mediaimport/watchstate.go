package mediaimport

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

// maxWatchStatesBytes bounds a single watch-state export read so a
// hostile/huge export cannot exhaust memory during an operator-triggered
// import. Sized the same order of magnitude as maxOrderedListsBytes (this
// package's largest sibling import cap, lists.go) — a full-library
// Jellyfin/Emby watch-state export (one entry per item, every provider id)
// can comfortably run past a few MB for a large library, so 64 MB is a
// generous ceiling rather than a tight one.
const maxWatchStatesBytes = 64 << 20

// errWatchStatesTooLarge is returned when a watch-state export exceeds
// maxWatchStatesBytes.
var errWatchStatesTooLarge = errors.New("import source exceeds 64 MB size limit")

// ParseJellyfinWatchStates parses Jellyfin user-data export JSON.
func ParseJellyfinWatchStates(r io.Reader) ([]WatchState, error) {
	return parseEmbyFamilyWatchStates(r)
}

// ParseEmbyWatchStates parses Emby user-data export JSON.
func ParseEmbyWatchStates(r io.Reader) ([]WatchState, error) {
	return parseEmbyFamilyWatchStates(r)
}

// ParsePlexWatchStates intentionally refuses direct Plex database parsing:
// Plex has no clean watch-history export shape for this parser package.
func ParsePlexWatchStates(io.Reader) ([]WatchState, error) {
	return nil, errors.New("mediaimport: Plex watch-state database import is unsupported")
}

type embyFamilyExport struct {
	Items []embyFamilyItem `json:"Items"`
}

type embyFamilyItem struct {
	ProviderIDs map[string]string  `json:"ProviderIds"`
	UserData    embyFamilyUserData `json:"UserData"`
}

type embyFamilyUserData struct {
	Played                bool   `json:"Played"`
	PlaybackPositionTicks int64  `json:"PlaybackPositionTicks"`
	LastPlayedDate        string `json:"LastPlayedDate"`
}

func parseEmbyFamilyWatchStates(r io.Reader) ([]WatchState, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxWatchStatesBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read watch states: %w", err)
	}
	if len(data) > maxWatchStatesBytes {
		return nil, errWatchStatesTooLarge
	}
	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse watch states: %w", err)
	}
	var items []embyFamilyItem
	if len(raw) > 0 && raw[0] == '[' {
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, fmt.Errorf("parse watch states: %w", err)
		}
	} else {
		var export embyFamilyExport
		if err := json.Unmarshal(raw, &export); err != nil {
			return nil, fmt.Errorf("parse watch states: %w", err)
		}
		items = export.Items
	}
	var states []WatchState
	for _, item := range items {
		// A malformed LastPlayedDate degrades this one item to a zero
		// LastPlayed rather than aborting the whole import, matching the
		// SQLite-reader path's graceful handling of the same field
		// (foreignimport.readEmbyFamily) — a single corrupt row must not
		// lose every other item's watch-state.
		lastPlayed, _ := parseExportTime(item.UserData.LastPlayedDate)
		for _, id := range externalIDsFromProviders(item.ProviderIDs) {
			states = append(states, WatchState{
				ExternalID:      id,
				Watched:         item.UserData.Played,
				PositionSeconds: item.UserData.PlaybackPositionTicks / 10_000_000,
				LastPlayed:      lastPlayed,
			})
		}
	}
	return states, nil
}

func parseExportTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse export time %q: %w", value, err)
	}
	return parsed, nil
}
