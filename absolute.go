package filmtv

import (
	"encoding/json"
	"fmt"
	"sort"
)

// EpisodeRef is one regular episode's place in a series: its season and its
// episode number within that season.
type EpisodeRef struct {
	Season int
	Number int
}

// AbsoluteEpisode maps an absolute episode number (the 13 in an anime release
// named "Show - 13") to a season and episode. It counts the series' regular
// episodes in season, then episode, order; season 0 and entries without a
// positive number (specials) are not counted. It reports false when absolute
// is not positive or exceeds the number of regular episodes, rather than
// guessing.
func AbsoluteEpisode(episodes []EpisodeRef, absolute int) (EpisodeRef, bool) {
	if absolute < 1 {
		return EpisodeRef{}, false
	}
	regular := make([]EpisodeRef, 0, len(episodes))
	for _, episode := range episodes {
		if episode.Season > 0 && episode.Number > 0 {
			regular = append(regular, episode)
		}
	}
	if absolute > len(regular) {
		return EpisodeRef{}, false
	}
	sort.Slice(regular, func(i, j int) bool {
		if regular[i].Season != regular[j].Season {
			return regular[i].Season < regular[j].Season
		}
		return regular[i].Number < regular[j].Number
	})
	return regular[absolute-1], true
}

// tvmazeShowEpisodes is the slice of a TVmaze /shows/{id}?embed=episodes
// document the mapper needs.
type tvmazeShowEpisodes struct {
	Embedded struct {
		Episodes []struct {
			Season int    `json:"season"`
			Number *int   `json:"number"`
			Type   string `json:"type"`
		} `json:"episodes"`
	} `json:"_embedded"`
}

// DecodeTVmazeEpisodes reads the regular episodes from a TVmaze show document
// fetched with embed=episodes. TVmaze gives specials a null number and a
// "significant_special" or "insignificant_special" type; both are dropped.
func DecodeTVmazeEpisodes(data []byte) ([]EpisodeRef, error) {
	var doc tvmazeShowEpisodes
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("filmtv: decode TVmaze episodes: %w", err)
	}
	episodes := make([]EpisodeRef, 0, len(doc.Embedded.Episodes))
	for _, episode := range doc.Embedded.Episodes {
		if episode.Number == nil || (episode.Type != "" && episode.Type != "regular") {
			continue
		}
		episodes = append(episodes, EpisodeRef{Season: episode.Season, Number: *episode.Number})
	}
	return episodes, nil
}
