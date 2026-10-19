package mediaimport

import (
	"sort"
	"strings"
)

var preferredIDSourceOrder = []string{"tmdb", "imdb", "tvdb"}

func normalizeExternalID(source, id string) (ExternalID, bool) {
	source = strings.ToLower(strings.TrimSpace(source))
	id = strings.TrimSpace(id)
	if source == "" || id == "" {
		return ExternalID{}, false
	}
	switch source {
	case "themoviedb":
		source = "tmdb"
	case "internetmoviedatabase":
		source = "imdb"
	case "thetvdb":
		source = "tvdb"
	}
	return ExternalID{Source: source, ID: id}, true
}

func appendExternalID(ids []ExternalID, source, id string) []ExternalID {
	next, ok := normalizeExternalID(source, id)
	if !ok {
		return ids
	}
	for _, existing := range ids {
		if existing == next {
			return ids
		}
	}
	return append(ids, next)
}

func externalIDsFromProviders(providers map[string]string) []ExternalID {
	var ids []ExternalID
	used := make(map[string]bool, len(providers))
	for _, source := range preferredIDSourceOrder {
		for key, id := range providers {
			normalizedKey := strings.ToLower(strings.TrimSpace(key))
			if normalizedKey == source {
				ids = appendExternalID(ids, source, id)
				used[key] = true
			}
		}
	}
	var rest []string
	for key := range providers {
		if !used[key] {
			rest = append(rest, key)
		}
	}
	sort.Strings(rest)
	for _, key := range rest {
		ids = appendExternalID(ids, key, providers[key])
	}
	return ids
}
