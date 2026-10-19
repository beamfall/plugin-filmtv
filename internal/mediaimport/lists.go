package mediaimport

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// maxOrderedListsBytes bounds a single collection/playlist export read so a
// hostile/huge export cannot exhaust memory during an operator-triggered
// import, while comfortably covering a real-world collection/playlist
// JSON/M3U export (which can run well past 64 KB for a large library).
const maxOrderedListsBytes = 64 << 20

// errOrderedListsImportTooLarge is returned when a collection/playlist export
// exceeds maxOrderedListsBytes.
var errOrderedListsImportTooLarge = errors.New("import source exceeds 64 MB size limit")

// ParseOrderedLists parses simple collection and playlist export files into
// ordered lists keyed by external IDs.
func ParseOrderedLists(r io.Reader) ([]OrderedList, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxOrderedListsBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read ordered lists: %w", err)
	}
	if len(data) > maxOrderedListsBytes {
		return nil, errOrderedListsImportTooLarge
	}
	trimmed := bytes.TrimSpace(data)
	if bytes.HasPrefix(trimmed, []byte("#EXTM3U")) {
		return parseM3UOrderedList(trimmed), nil
	}
	return parseJSONOrderedLists(trimmed)
}

type orderedListExport struct {
	Collections []jsonOrderedList `json:"Collections"`
	Playlists   []jsonOrderedList `json:"Playlists"`
}

type jsonOrderedList struct {
	Name  string           `json:"Name"`
	Items []jsonListMember `json:"Items"`
}

type jsonListMember struct {
	ProviderIDs map[string]string `json:"ProviderIds"`
}

func parseJSONOrderedLists(data []byte) ([]OrderedList, error) {
	var export orderedListExport
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, fmt.Errorf("parse ordered lists: %w", err)
	}
	var lists []OrderedList
	for _, list := range append(export.Collections, export.Playlists...) {
		ordered := OrderedList{Name: strings.TrimSpace(list.Name)}
		for _, item := range list.Items {
			ids := externalIDsFromProviders(item.ProviderIDs)
			if len(ids) > 0 {
				ordered.Items = append(ordered.Items, ids[0])
			}
		}
		if ordered.Name != "" || len(ordered.Items) > 0 {
			lists = append(lists, ordered)
		}
	}
	return lists, nil
}

func parseM3UOrderedList(data []byte) []OrderedList {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	// Raise the scanner's per-line cap (64 KB by default) to the whole
	// already-bounded input: a single line longer than the default cap would
	// otherwise stop the scan with a silently discarded bufio.ErrTooLong,
	// truncating every entry after it. data is capped by maxOrderedListsBytes
	// upstream, so this stays bounded.
	scanner.Buffer(nil, len(data)+1)
	var list OrderedList
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#PLAYLIST:") && list.Name == "" {
			list.Name = strings.TrimSpace(strings.TrimPrefix(line, "#PLAYLIST:"))
			continue
		}
		for _, prefix := range []string{"#EXT-X-BEAMFALL-ID:", "#EXT-X-EXTERNAL-ID:"} {
			if strings.HasPrefix(line, prefix) {
				parts := strings.SplitN(strings.TrimPrefix(line, prefix), ":", 2)
				if len(parts) == 2 {
					list.Items = appendExternalID(list.Items, parts[0], parts[1])
				}
			}
		}
	}
	if list.Name == "" && len(list.Items) == 0 {
		return nil
	}
	return []OrderedList{list}
}
