package filmtv

import (
	"strconv"

	sdk "github.com/beamfall/plugin-sdk"
)

// HydratedField is the declared flag Media field Core's hydration seam sets
// once a file's local show and season containers are written, so a rescan does
// not write them again. Core's scan backfill reads the same name to find files
// that still need them.
const HydratedField = "filmtv-hydrated"

// Season relations. SeasonEpisodeRelation links a season Entity to each of its
// episode Media (one season per episode); SeasonOfRelation links a season to
// its show. A season title is unique only within its show, so Core scopes
// season Entities by SeasonOfRelation.
const (
	SeasonEpisodeRelation = "season-episode"
	SeasonOfRelation      = "season-of"
)

// LocalGrouping is the show and season a file belongs to, read from its path
// alone. Episode is false for anything the parser does not classify as an
// episode; the other fields are then empty.
type LocalGrouping struct {
	Episode         bool
	SeriesTitle     string
	SeasonNumber    int
	SeasonTitle     string
	EpisodeIndex    int
	EpisodeIndexEnd int
	EpisodeTitle    string
}

// LocalHydration reports the local show and season grouping of path. It reads
// the path only, through the same parse as Parse, so the series title is the
// one Core already stores as the parsed title.
func LocalHydration(path string) LocalGrouping {
	parsed, episodeTitle, ok := parseName(path)
	if !ok || parsed.Class != "episode" {
		return LocalGrouping{}
	}
	// The parser writes Sequence as plain decimal digits (numericText).
	season := atoi(parsed.Sequence)
	return LocalGrouping{
		Episode:         true,
		SeriesTitle:     parsed.Title,
		SeasonNumber:    season,
		SeasonTitle:     seasonTitle(season),
		EpisodeIndex:    parsed.Index,
		EpisodeIndexEnd: parsed.IndexEnd,
		EpisodeTitle:    episodeTitle,
	}
}

// seasonTitle names a season the way the season Kind documents it: season 0 is
// "Specials", every other season is "Season N".
func seasonTitle(number int) string {
	if number == 0 {
		return "Specials"
	}
	return "Season " + strconv.Itoa(number)
}

// MediaTitle implements Core's optional media-title hook: the title Core shows
// for a file when it differs from the parsed title. An episode whose name
// carries its own title shows that title; anything else returns "" and keeps
// the parsed title. The parsed title stays the series name because it is the
// provider search query and the scorer's input.
func (p *Parser) MediaTitle(path string, _ sdk.ParseProbe) string {
	parsed, episodeTitle, ok := parseName(path)
	if !ok || parsed.Class != "episode" {
		return ""
	}
	return episodeTitle
}
