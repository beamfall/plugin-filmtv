package filmtv

import (
	"fmt"
	"strconv"
	"time"

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
// episode; the other fields are then empty. A dated episode
// ("Show 2023-03-01.mkv") sits in the season of its air year, its EpisodeIndex
// is its month and day (MMDD) so one year's episodes order by date, and AirDate
// is its ISO air date ("2023-03-01"); AirDate is empty for any other episode.
type LocalGrouping struct {
	Episode         bool
	SeriesTitle     string
	SeasonNumber    int
	SeasonTitle     string
	EpisodeIndex    int
	EpisodeIndexEnd int
	EpisodeTitle    string
	AirDate         string
}

// LocalHydration reports the local show and season grouping of path. It reads
// the path only, through the same parse as Parse, so the series title is the
// one Core already stores as the parsed title.
func LocalHydration(path string) LocalGrouping {
	name, ok := parseName(path)
	if !ok || name.parsed.Class != "episode" {
		return LocalGrouping{}
	}
	parsed := name.parsed
	if !name.airDate.IsZero() {
		year := name.airDate.Year()
		return LocalGrouping{
			Episode:      true,
			SeriesTitle:  parsed.Title,
			SeasonNumber: year,
			SeasonTitle:  seasonTitle(year),
			EpisodeIndex: int(name.airDate.Month())*100 + name.airDate.Day(),
			EpisodeTitle: name.episodeTitle,
			AirDate:      name.airDate.Format(time.DateOnly),
		}
	}
	// The parser writes Sequence as plain decimal digits (numericText). An
	// absolute-numbered episode has no season of its own; it groups under the
	// season folder holding it, or season 1, until a provider places it.
	season := atoi(parsed.Sequence)
	if parsed.AbsoluteOrdinal > 0 {
		season = 1
		if number, ok := seasonFolderNumber(path); ok {
			season = number
		}
	}
	return LocalGrouping{
		Episode:         true,
		SeriesTitle:     parsed.Title,
		SeasonNumber:    season,
		SeasonTitle:     seasonTitle(season),
		EpisodeIndex:    parsed.Index,
		EpisodeIndexEnd: parsed.IndexEnd,
		EpisodeTitle:    name.episodeTitle,
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
// for a file (on every card and in the library landing) when it differs from
// the parsed title. An episode shows its code, then " · " and its own title
// when the name carries one: "S01E03 · Earthfall", "S01E03-04" for a range,
// "E01" for an absolute-numbered episode, "2023-03-01" for a dated episode. The code is zero-padded and the date ISO,
// so a title sort orders one series' episodes. Anything else returns "" and
// keeps the parsed title. The parsed title stays the series name because it
// is the provider search query and the scorer's input.
func (p *Parser) MediaTitle(path string, _ sdk.ParseProbe) string {
	name, ok := parseName(path)
	if !ok || name.parsed.Class != "episode" {
		return ""
	}
	code := episodeCode(name)
	if name.episodeTitle == "" {
		return code
	}
	return code + " · " + name.episodeTitle
}

// episodeCode is an episode's short display code: its ISO air date when it is
// a dated episode, Eyy for an absolute-numbered one, otherwise SxxEyy with a
// "-zz" range end. Numbers are padded
// to two digits, the common naming, so a season past episode 99 ("E100") sorts
// ahead of "E11" by title; ordered season lists use the season relation's
// numeric sort order instead.
func episodeCode(name nameParse) string {
	if !name.airDate.IsZero() {
		return name.airDate.Format(time.DateOnly)
	}
	if name.parsed.AbsoluteOrdinal > 0 {
		return fmt.Sprintf("E%02d", name.parsed.AbsoluteOrdinal)
	}
	code := fmt.Sprintf("S%02dE%02d", atoi(name.parsed.Sequence), name.parsed.Index)
	if name.parsed.IndexEnd > 0 {
		code += fmt.Sprintf("-%02d", name.parsed.IndexEnd)
	}
	return code
}
