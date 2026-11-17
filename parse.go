// This file is the Film & TV filename parser hook (GuessIt-class):
// movie-vs-episode classification, year extraction, and release-name cleanup
// for unidentified Media. It implements the SDK's scan-side FilenameParser
// seam.
package filmtv

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/beamfall/plugin-filmtv/internal/mediaimport"
	sdk "github.com/beamfall/plugin-sdk"
)

// PluginID is the canonical plugin identifier; bundle.go's Bundle().ID
// references this constant — scan resolves the parser through the
// Library's plugin binding.
const PluginID = "filmtv"

// Parser implements scan.FilenameParser with stdlib-only filename heuristics.
type Parser struct{}

// New returns a ready Parser.
func New() *Parser { return &Parser{} }

var (
	// SxxEyy, also written "S01.E01" or "S01 E01". The multi-episode tail
	// (E02, -E02, -S01E02, ...) is read by episodeChain, not this pattern.
	reSeasonEpisode = regexp.MustCompile(`s(\d{1,2})[ ._]?e(\d{1,3})`)
	// One further episode token of a multi-episode file, anchored at the end of
	// the previous one: an optional separator, an optional repeated season, Eyy.
	// Groups: the repeated season (empty when absent), the episode.
	reEpisodeChainLink = regexp.MustCompile(`^[-+._ ]?(?:s(\d{1,2}))?e(\d{1,3})`)
	// "season X episode Y" long form.
	reSeasonEpisodeWords = regexp.MustCompile(`season[ ._]*(\d{1,2})[ ._,-]*episode[ ._]*(\d{1,3})`)
	// XxYY shorthand (e.g. 3x07); guarded so a resolution like 1920x1080 never
	// matches (those numbers are far larger than a season/episode). A range
	// ("1x01-1x02-1x03") is read by episodeChain with reSeasonXChainLink.
	reSeasonX = regexp.MustCompile(`\b(\d{1,2})x(\d{1,3})\b`)
	// One further XxYY token of a multi-episode shorthand range. Groups: the
	// season, the episode.
	reSeasonXChainLink = regexp.MustCompile(`^-(\d{1,2})x(\d{1,3})`)
	// A season-by-episode code with a bare "e" ("1e01-Yesterday's Jam"), at
	// the start of the name or after a separator and not followed by a letter
	// or digit, so a checksum tag ("[4E01A2B3]") never matches. Read only with
	// episode context (bareSeasonEpisode). Groups: the season, the episode.
	reSeasonE = regexp.MustCompile(`(?:^|[ ._-])(\d{1,2})e(\d{2,3})(?:$|[^a-z0-9])`)
	// A leading "season episode" number pair ("3 01 From Hell"). Only read in
	// a season folder (leadingEpisode): a film name can start with two numbers
	// too ("3 10 to Yuma"). Groups: season, episode.
	reLeadingNumbers = regexp.MustCompile(`^(\d{1,2})[ ._-]+(\d{2,3})(?:$|[ ._-])`)
	// An absolute episode number after a series name: zero-padded to four
	// digits ("Dealbreakers Talk Show 0001"), or to three after " - "
	// ("Naruto - 001"). It ends the title, or an episode title follows it after
	// " - " ("Show - 001 - Title"). The title cut may leave the separator or
	// bracket that opened the release noise. Read only with episode context
	// (episodeContext). Groups: the four-digit form, the three-digit form.
	reAbsoluteEpisode = regexp.MustCompile(`(?:[ ._-](0\d{3})|[ ._]-[ ._](0\d{2}))(?:[ ._\-\[(]*$|[ ._]-[ ._])`)
	// A CRC32 checksum bracket an anime release carries ("[ABCD1234]"):
	// release noise, and a sign the file is an episode.
	reChecksum = regexp.MustCompile(`\[[0-9a-f]{8}\]`)
	// An air date naming a dated (daily) episode: YYYY-MM-DD, also with ".",
	// "_" or " " separators, not inside a longer run of letters or digits
	// (an underscore still separates it, unlike a \b boundary). Calendar-checked
	// in code. Groups: year, month, day.
	reAirDate = regexp.MustCompile(`(?:^|[^a-z0-9])((?:19|20)\d{2})[-._ ](\d{2})[-._ ](\d{2})(?:$|[^a-z0-9])`)
	// A folder that holds one season of a series: the series name is then the
	// folder above it.
	reSeasonFolder = regexp.MustCompile(`^(?:(?:season|series)[ ._-]*\d{1,4}|s\d{1,2}|specials)$`)
	// Release-group brackets in front of a series name ("[Group] Show - S01E01").
	reLeadingGroups = regexp.MustCompile(`^(?:\s*\[[^\]]*\])+`)
	// A video resolution token: 480p, 1080i, 2160p.
	reResolution = regexp.MustCompile(`^\d{3,4}[pi]$`)
	// A 4-digit year, optionally parenthesized; range-guarded to 1900–2099 in
	// code (the regexp only narrows the century).
	reYear         = regexp.MustCompile(`\(?\b(19\d{2}|20\d{2})\b\)?`)
	providerIDTags = []providerIDTag{
		{provider: "tmdb", pattern: regexp.MustCompile(`(?i)(?:\{tmdb(?:id)?-(\d+)\}|\[tmdbid-(\d+)\])`)},
		{provider: "imdb", pattern: regexp.MustCompile(`(?i)(?:\{imdb-(tt\d+)\}|\[imdbid-(tt\d+)\])`)},
		{provider: "tvdb", pattern: regexp.MustCompile(`(?i)(?:\{tvdb-(\d+)\}|\[tvdbid-(\d+)\])`)},
		{provider: "anidb", pattern: regexp.MustCompile(`(?i)\{anidb-(\d+)\}`)},
	}
)

type providerIDTag struct {
	provider string
	pattern  *regexp.Regexp
}

// EpisodeTitleField is the plugin-declared Media field that carries an
// episode's own name ("Earthfall"), parsed from the text after the episode
// token. ParsedMedia.Title stays the series name: it is the provider search
// query and the scorer's input.
const EpisodeTitleField = "episode-title"

// AirDateField is the declared date Media field that carries an episode's air
// date. The parser fills it for a dated episode ("Show 2023-03-01.mkv");
// identification fills it from the provider.
const AirDateField = "air-date"

// releaseNoise is the lower-cased set of release-name tokens that always mark
// the end of a real title: sources, codecs and audio formats that are not
// ordinary words. Resolutions are matched by reResolution. A hyphenated or
// dotted tag is listed joined ("web-dl" and "h.264" tokenize as two tokens and
// are looked up as "webdl" and "h264").
var releaseNoise = map[string]bool{
	"4k": true, "uhd": true,
	"bluray": true, "brrip": true, "bdrip": true, "bdremux": true,
	"webrip": true, "webdl": true, "hdrip": true, "dvdrip": true,
	"hdtv": true, "pdtv": true, "sdtv": true, "remux": true,
	"x264": true, "x265": true, "h264": true, "h265": true, "hevc": true,
	"xvid": true, "divx": true, "av1": true, "10bit": true,
	"aac": true, "aac2": true, "ac3": true, "eac3": true, "dts": true,
	"dd5": true, "ddp5": true, "truehd": true, "hdr10": true,
}

// releaseWords are release tags that are also ordinary words, so one can sit
// inside a real title ("Internal Affairs", "Extended Family", "Charlotte's
// Web"). Such a word ends the title only when a release tag follows it; the
// ones mapped to true also end it as the last word of the name
// ("Cool.Movie.PROPER").
var releaseWords = map[string]bool{
	"proper": true, "repack": true, "rerip": true, "extended": true,
	"unrated": true, "internal": true, "uncut": true, "remastered": true,
	"web": false, "multi": false, "dubbed": false, "subbed": false,
}

// Parse derives a best-guess ParsedMedia from a path and its local NFO evidence.
// probe is accepted to satisfy the seam shape but not read. ok is
// true only when a non-empty Title was extracted.
func (p *Parser) Parse(path string, probe sdk.ParseProbe) (sdk.ParsedMedia, bool) {
	_ = probe // probe rides the seam for future parsers.

	name, ok := parseName(path)
	if !ok {
		return sdk.ParsedMedia{}, false
	}
	parsed := name.parsed
	parsed.ProviderIDs, parsed.HintSource = providerIDHints(path)
	return parsed, true
}

// ParseWithContext implements sdk.ContextFilenameParser. Film and TV provider
// hints are file-local, so bounded directory-count context does not alter them.
func (p *Parser) ParseWithContext(path string, probe sdk.ParseProbe, _ sdk.ParseContext) (sdk.ParsedMedia, bool) {
	return p.Parse(path, probe)
}

// ParseMediaFields implements sdk.MediaFieldParser: an episode file whose name
// carries the episode's own title yields it as the EpisodeTitleField Media
// field. It reads the path only, so it agrees with Parse on what is an episode.
func (p *Parser) ParseMediaFields(path string, _ sdk.ParseProbe) []sdk.ParsedTextField {
	name, ok := parseName(path)
	if !ok || name.parsed.Class != "episode" || name.episodeTitle == "" {
		return nil
	}
	return []sdk.ParsedTextField{{Name: EpisodeTitleField, Text: name.episodeTitle}}
}

// ParseTypedMediaFields implements sdk.TypedMediaFieldParser, which Core
// prefers over ParseMediaFields: the same EpisodeTitleField text plus, for a
// dated episode, its air date as the AirDateField date.
func (p *Parser) ParseTypedMediaFields(path string, _ sdk.ParseProbe) []sdk.ParsedField {
	name, ok := parseName(path)
	if !ok || name.parsed.Class != "episode" {
		return nil
	}
	var fields []sdk.ParsedField
	if name.episodeTitle != "" {
		fields = append(fields, sdk.ParsedField{Name: EpisodeTitleField, Text: name.episodeTitle})
	}
	if !name.airDate.IsZero() {
		fields = append(fields, sdk.ParsedField{Name: AirDateField, Date: name.airDate})
	}
	return fields
}

// nameParse is the path-only parse of one file name: the parsed fields
// (without provider hints) and, for an episode, its own title when the name
// carries one and its air date when it is a dated episode.
type nameParse struct {
	parsed       sdk.ParsedMedia
	episodeTitle string
	airDate      time.Time
}

// imageExtensions are the artwork files a library keeps beside its media
// ("season01-banner.jpg", "poster.png"): never a film or an episode.
var imageExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".bmp": true, ".tbn": true,
}

// parseName is the path-only parse shared by Parse, the Media field parsers,
// MediaTitle and LocalHydration.
func parseName(path string) (nameParse, bool) {
	base := filepath.Base(path)
	if imageExtensions[asciiLower(filepath.Ext(base))] {
		return nameParse{}, false
	}
	name := strings.TrimSuffix(base, filepath.Ext(base))
	name = removeProviderIDTags(name)
	lower := asciiLower(name)

	var parsed sdk.ParsedMedia

	// 1. Episode detection. cut marks where the title ends on the original-cased
	// string (the earliest of any episode/year/noise token start).
	cut := len(name)
	episode, isEpisode := findEpisode(lower)
	if !isEpisode {
		episode, isEpisode = bareSeasonEpisode(path, lower)
	}
	if !isEpisode {
		episode, isEpisode = leadingEpisode(path, lower)
	}
	episodeCut := -1
	if isEpisode {
		parsed.Class = "episode"
		if episode.airDate.IsZero() {
			parsed.Sequence = episode.sequence
			parsed.Index = episode.index
			parsed.IndexEnd = episode.indexEnd
		}
		episodeCut = episode.start
		cut = episodeCut
	}

	// 2. Year. Prefer a parenthesized year; otherwise the first standalone year
	// token. Skip tokens part of an SxxEyy match and tokens that are really a
	// resolution (followed by p/i, e.g. 2160p). Range-guard to 1900–2099.
	if year, yearStart := findYear(name, lower, episodeCut); year > 0 {
		parsed.Year = year
		if yearStart >= 0 && yearStart < cut {
			cut = yearStart
		}
		if !isEpisode {
			parsed.VersionLabel = versionLabel(name, yearStart)
		}
	}

	// 3. Title-ending release noise. Only applied when neither an episode nor a
	// year cut was already found (cut still == len(name)): a noise WORD can
	// legitimately appear inside a real title ("Le Cercle Internal", "Extended
	// Family"), so once a year/episode token has already bounded the title, a
	// noise token elsewhere in the string must not truncate it further.
	if cut == len(name) {
		if noiseStart := firstNoiseToken(lower); noiseStart >= 0 {
			cut = noiseStart
		}
		if checksum := reChecksum.FindStringIndex(lower[:cut]); checksum != nil {
			cut = checksum[0]
		}
	}

	// 3b. Absolute episode number. A name with no other episode token and no
	// year whose series name is followed by a zero-padded number is an
	// absolute-numbered episode when the path says it is TV: it carries an
	// index and an absolute ordinal but no season.
	if !isEpisode && parsed.Year == 0 {
		if token, ok := findAbsoluteEpisode(path, name[:cut], lower[:cut]); ok {
			episode, isEpisode = token, true
			parsed.Class = "episode"
			parsed.Index = token.index
			parsed.AbsoluteOrdinal = token.index
			cut = token.start
		}
	}

	// 4. Title cleanup: take everything before the cut, normalise separators.
	// For an episode this is the series name: a release-group bracket in front
	// of it is dropped, and a file that carries no series name of its own
	// ("S01E01 - Earthfall") takes the one of the folder above its season folder.
	rawTitle := name[:cut]
	if isEpisode {
		rawTitle = rawTitle[len(reLeadingGroups.FindString(rawTitle)):]
	}
	parsed.Title = cleanTitle(rawTitle)
	if isEpisode && parsed.Title == "" {
		var folderYear int
		if parsed.Title, folderYear = seriesFromSeasonFolder(path); folderYear > 0 {
			parsed.Year = folderYear
		}
	}

	// 5. Class fallback: episode signals already set "episode"; otherwise any
	// useful parse (a year or a clean title) is a movie.
	if parsed.Class == "" && (parsed.Year > 0 || parsed.Title != "") {
		parsed.Class = "movie"
	}

	// Return ok only when a non-empty Title survived cleanup.
	if parsed.Title == "" {
		return nameParse{}, false
	}
	if !isEpisode {
		return nameParse{parsed: parsed}, true
	}
	return nameParse{
		parsed:       parsed,
		episodeTitle: episodeTitle(name[episode.end:], lower[episode.end:]),
		airDate:      episode.airDate,
	}, true
}

// episodeToken is one recognised season/episode marker: its byte span in the
// name and the ordinals it carries, or the air date of a dated episode (which
// carries no ordinals).
type episodeToken struct {
	start, end      int
	sequence        string
	index, indexEnd int
	airDate         time.Time
}

// findEpisode locates the season/episode marker in the ASCII-folded name. The
// forms are tried in order of how unambiguous they are: SxxEyy, "season X
// episode Y", XxYY, then an air date.
func findEpisode(lower string) (episodeToken, bool) {
	if m := reSeasonEpisode.FindStringSubmatchIndex(lower); m != nil {
		token := episodeToken{start: m[0], end: m[1], sequence: numericText(lower[m[2]:m[3]]), index: atoi(lower[m[4]:m[5]])}
		last, end := episodeChain(lower, token.end, reEpisodeChainLink, token.sequence)
		token.end = end
		token.indexEnd = rangeEnd(token.index, last)
		return token, true
	}
	if m := reSeasonEpisodeWords.FindStringSubmatchIndex(lower); m != nil {
		return episodeToken{start: m[0], end: m[1], sequence: numericText(lower[m[2]:m[3]]), index: atoi(lower[m[4]:m[5]])}, true
	}
	if m := reSeasonX.FindStringSubmatchIndex(lower); m != nil {
		token := episodeToken{start: m[0], end: m[1], sequence: numericText(lower[m[2]:m[3]]), index: atoi(lower[m[4]:m[5]])}
		last, end := episodeChain(lower, token.end, reSeasonXChainLink, token.sequence)
		token.end = end
		token.indexEnd = rangeEnd(token.index, last)
		return token, true
	}
	return findAirDate(lower)
}

// bareSeasonEpisode reads a season-by-episode code with a bare "e" ("1e01")
// when the path gives episode context. A code that follows a number and a
// dot is a channel layout ("DD5.1e01"), not a season.
func bareSeasonEpisode(path, lower string) (episodeToken, bool) {
	for _, m := range reSeasonE.FindAllStringSubmatchIndex(lower, -1) {
		if start := m[2]; start >= 2 && lower[start-1] == '.' && lower[start-2] >= '0' && lower[start-2] <= '9' {
			continue
		}
		sequence, ok := folderSeason(path, numericText(lower[m[2]:m[3]]))
		if !ok || !episodeContext(path, lower, cleanTitle(lower[:m[2]])) {
			return episodeToken{}, false
		}
		return episodeToken{start: m[2], end: m[5], sequence: sequence, index: atoi(lower[m[4]:m[5]])}, true
	}
	return episodeToken{}, false
}

// folderSeason checks a season read from a file name against the season
// folder holding the file. In a numbered season folder the two must agree; in
// "Specials" the season is 0 whatever the name says. Outside a season folder
// the name's season stands.
func folderSeason(path, sequence string) (string, bool) {
	number, inSeasonFolder := seasonFolderNumber(path)
	switch {
	case !inSeasonFolder:
		return sequence, true
	case isSpecialsFolder(path):
		return "0", true
	default:
		return sequence, number == atoi(sequence)
	}
}

// leadingEpisode reads a leading "season episode" number pair ("3 01 From
// Hell") when path sits in a season folder of that season, or in "Specials"
// (season 0). Outside a season folder two leading numbers are as likely a
// film title ("3 10 to Yuma"), so they are not read.
func leadingEpisode(path, lower string) (episodeToken, bool) {
	m := reLeadingNumbers.FindStringSubmatchIndex(lower)
	if m == nil {
		return episodeToken{}, false
	}
	if _, inSeasonFolder := seasonFolderNumber(path); !inSeasonFolder {
		return episodeToken{}, false
	}
	sequence, ok := folderSeason(path, numericText(lower[m[2]:m[3]]))
	if !ok {
		return episodeToken{}, false
	}
	return episodeToken{start: 0, end: m[5], sequence: sequence, index: atoi(lower[m[4]:m[5]])}, true
}

// seasonFolderNumber reports whether path sits in a season folder and that
// folder's season number, 0 for "Specials". Every other season folder name
// reSeasonFolder accepts ends in its number.
func seasonFolderNumber(path string) (int, bool) {
	folder := asciiLower(filepath.Base(filepath.Dir(path)))
	if !reSeasonFolder.MatchString(folder) {
		return 0, false
	}
	return atoi(strings.TrimLeftFunc(folder, func(r rune) bool { return r < '0' || r > '9' })), true
}

// isSpecialsFolder reports whether path sits in a "Specials" season folder.
func isSpecialsFolder(path string) bool {
	return asciiLower(filepath.Base(filepath.Dir(path))) == "specials"
}

// cameraPrefixes are the stems cameras and phones number their clips with
// ("VID_0001", "DSC_0012"): never a series name.
var cameraPrefixes = map[string]bool{
	"vid": true, "img": true, "dsc": true, "dscf": true, "dscn": true, "mov": true,
	"mvi": true, "gopr": true, "gp": true, "pxl": true,
}

// tvFolders name a folder that holds series: a file anywhere under one is TV.
var tvFolders = map[string]bool{
	"tv": true, "tv shows": true, "television": true, "shows": true, "series": true, "anime": true,
}

// episodeContext reports whether path carries evidence beyond the bare
// numbers in its name that it is an episode of series: a season folder, a
// show folder named for series, a TV folder above it, or an anime release's
// group or checksum bracket. A camera clip stem is never a series. The weak
// episode forms (a bare-e code, an absolute number) are read only with this
// context, because a film or a home video can end on the same digits
// ("Agent 0011", "Wedding_0002").
func episodeContext(path, lower, series string) bool {
	if cameraPrefixes[asciiLower(series)] {
		return false
	}
	if reLeadingGroups.MatchString(lower) || reChecksum.MatchString(lower) {
		return true
	}
	if _, inSeasonFolder := seasonFolderNumber(path); inSeasonFolder {
		return true
	}
	dir := filepath.Dir(path)
	if folder, _ := seriesFromFolder(filepath.Base(dir)); series != "" && strings.EqualFold(folder, series) {
		return true
	}
	for _, folder := range strings.Split(filepath.ToSlash(dir), "/") {
		if tvFolders[asciiLower(folder)] {
			return true
		}
	}
	return false
}

// findAbsoluteEpisode reads an absolute episode number in title, the part of
// path's name before any year or release noise, and lowerTitle, its
// ASCII-folded copy. The number must follow a series name of its own, and
// path must give episode context.
func findAbsoluteEpisode(path, title, lowerTitle string) (episodeToken, bool) {
	m := reAbsoluteEpisode.FindStringSubmatchIndex(lowerTitle)
	if m == nil {
		return episodeToken{}, false
	}
	series := title[:m[0]]
	series = cleanTitle(series[len(reLeadingGroups.FindString(series)):])
	if series == "" || !episodeContext(path, lowerTitle, series) {
		return episodeToken{}, false
	}
	digits := m[4:6]
	if m[2] >= 0 {
		digits = m[2:4]
	}
	index := atoi(lowerTitle[digits[0]:digits[1]])
	if index == 0 {
		return episodeToken{}, false
	}
	return episodeToken{start: m[0], end: m[1], index: index}, true
}

// findAirDate locates the first calendar-valid air date in the ASCII-folded
// name. A date-shaped token that is no real day ("2023-02-30") is skipped.
func findAirDate(lower string) (episodeToken, bool) {
	for _, m := range reAirDate.FindAllStringSubmatchIndex(lower, -1) {
		year, month, day := atoi(lower[m[2]:m[3]]), atoi(lower[m[4]:m[5]]), atoi(lower[m[6]:m[7]])
		date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
		if date.Year() == year && int(date.Month()) == month && date.Day() == day {
			return episodeToken{start: m[2], end: m[7], airDate: date}, true
		}
	}
	return episodeToken{}, false
}

// episodeChain reads the further episode tokens of a multi-episode file
// ("S01E01E02", "S01E01-E02-E03", "S01E01-S01E02", or with reSeasonXChainLink
// "1x01-1x02-1x03") starting at pos, the end of the first one; link matches one
// token and captures its season (empty when not repeated) and episode number.
// It returns the last episode number of the chain (0 when there is none) and
// the offset the chain ends at. A chain with a token naming a season other
// than sequence ("S01E01-S02E03") spans seasons, so it reports no range, but
// its tokens still end at the returned offset and stay out of the episode
// title. A link must end on a word boundary, so a title word that merely
// starts like one ("E20th") is not eaten.
func episodeChain(lower string, pos int, link *regexp.Regexp, sequence string) (int, int) {
	last, end, crossesSeason := 0, pos, false
	for {
		m := link.FindStringSubmatchIndex(lower[pos:])
		if m == nil {
			if crossesSeason {
				return 0, end
			}
			return last, end
		}
		if m[2] >= 0 && numericText(lower[pos+m[2]:pos+m[3]]) != sequence {
			crossesSeason = true
		}
		pos += m[1]
		if pos == len(lower) || !isASCIIAlnum(lower[pos]) {
			last, end = atoi(lower[pos-(m[5]-m[4]):pos]), pos
		}
	}
}

// rangeEnd is the IndexEnd of an episode range first..last: last when it is a
// real range end, 0 (no range) otherwise.
func rangeEnd(first, last int) int {
	if last > first {
		return last
	}
	return 0
}

// episodeTitle extracts the episode's own title from the text after the
// episode token: everything up to the first tag bracket or release-noise
// token, cleaned like a title. tail and lowerTail are the same span of the
// name and of its ASCII-folded copy.
func episodeTitle(tail, lowerTail string) string {
	end := len(tail)
	if bracket := strings.IndexAny(tail, "[{"); bracket >= 0 {
		end = bracket
	}
	if noiseStart := firstNoiseToken(lowerTail[:end]); noiseStart >= 0 {
		end = noiseStart
	}
	return cleanTitle(tail[:end])
}

// seriesFromSeasonFolder returns the series name and year carried by the
// folder above path's season folder ("Pioneer One (2010)/Season 01/…"), or
// ("", 0) when path does not sit in a season folder. Only that layout is
// trusted: the parent of a loose file is as likely a library root as a series.
func seriesFromSeasonFolder(path string) (string, int) {
	seasonDir := filepath.Dir(path)
	if !reSeasonFolder.MatchString(asciiLower(filepath.Base(seasonDir))) {
		return "", 0
	}
	return seriesFromFolder(filepath.Base(filepath.Dir(seasonDir)))
}

// seriesFromFolder returns the series name and year a show folder's name
// carries ("Pioneer One (2010)"), or ("", 0) when it names none.
func seriesFromFolder(folder string) (string, int) {
	series := removeProviderIDTags(folder)
	if series == "." || series == string(filepath.Separator) || strings.ContainsRune(series, filepath.Separator) {
		return "", 0
	}
	year, yearStart := findYear(series, asciiLower(series), -1)
	if yearStart >= 0 {
		series = series[:yearStart]
	}
	title := cleanTitle(series)
	if title == "" {
		return "", 0
	}
	return title, year
}

func providerIDHints(path string) (map[string]string, sdk.ParseHintSource) {
	if nfoPath, ok := nfoSidecarFor(path); ok {
		if hint, ok := parseNFOFile(nfoPath); ok {
			if ids := nfoProviderIDs(hint); len(ids) > 0 {
				return ids, sdk.ParseHintSourceNFO
			}
		}
	}
	if ids := filenameProviderIDs(path); len(ids) > 0 {
		return ids, sdk.ParseHintSourceFilename
	}
	return nil, sdk.ParseHintSourceNone
}

func nfoProviderIDs(hint mediaimport.ParseHint) map[string]string {
	ids := make(map[string]string)
	for _, externalID := range hint.ExternalIDs {
		if !sdk.ValidProviderIDKey(externalID.Source) {
			continue
		}
		if _, exists := ids[externalID.Source]; !exists {
			ids[externalID.Source] = externalID.ID
		}
	}
	return ids
}

func filenameProviderIDs(path string) map[string]string {
	ids := make(map[string]string)
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		component := filepath.Base(current)
		for _, tag := range providerIDTags {
			if _, exists := ids[tag.provider]; exists {
				continue
			}
			if id := providerIDTagValue(tag.pattern, component); id != "" {
				ids[tag.provider] = id
			}
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}
	return ids
}

func providerIDTagValue(pattern *regexp.Regexp, component string) string {
	match := pattern.FindStringSubmatch(component)
	if len(match) < 2 {
		return ""
	}
	for _, value := range match[1:] {
		if value != "" {
			return value
		}
	}
	return ""
}

func removeProviderIDTags(name string) string {
	for _, tag := range providerIDTags {
		name = tag.pattern.ReplaceAllString(name, " ")
	}
	return name
}

// findYear returns the chosen year and its start index in name, or (0, -1).
// Parenthesized years win; otherwise the earliest plausible token wins. A token
// inside the SxxEyy match (episodeCut..) or followed by p/i is skipped.
func findYear(name, lower string, episodeCut int) (int, int) {
	matches := reYear.FindAllStringSubmatchIndex(lower, -1)
	bestStart, bestYear := -1, 0
	parenStart, parenYear := -1, 0
	for _, m := range matches {
		digitsStart, digitsEnd := m[2], m[3]
		year := atoi(lower[digitsStart:digitsEnd])
		if year < 1900 || year > 2099 {
			continue
		}
		// Skip a resolution like 2160p/1080i (digit run followed by p or i).
		if digitsEnd < len(lower) {
			if c := lower[digitsEnd]; c == 'p' || c == 'i' {
				continue
			}
		}
		// Skip a year that is part of the episode token (SxxEyy ranges etc.).
		if episodeCut >= 0 && digitsStart >= episodeCut {
			continue
		}
		// Parenthesized? The matched span (m[0]..m[1]) includes the parens; cut
		// the title at the '(' so it does not leak into the title.
		if m[0] < digitsStart && name[m[0]] == '(' {
			if parenStart < 0 {
				parenStart, parenYear = m[0], year
			}
			continue
		}
		if bestStart < 0 {
			bestStart, bestYear = digitsStart, year
		}
	}
	if parenStart >= 0 {
		return parenYear, parenStart
	}
	return bestYear, bestStart
}

// reNotVersion matches a " - <label>" suffix, lowercased with spaces, dots,
// underscores and hyphens removed, that names something other than a version
// of the title: a stacked part of one long file (cd1, part 2, disc1) or an
// extra, singular or plural and optionally numbered (trailer 2, Deleted
// Scenes, Behind the Scenes, Making Of, Bloopers).
var reNotVersion = regexp.MustCompile(`^(?:(?:cd|dvd|part|pt|disc|disk)\d+|(?:trailer|sample|featurette|behindthescene|deleted|deletedscene|interview|scene|short|clip|extra|other|blooper|outtake|gagreel|makingof|teaser|bonus)s?\d*)$`)

// notVersionKey folds a label to the form reNotVersion matches.
func notVersionKey(label string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '.', '_', '-':
			return -1
		}
		return r
	}, asciiLower(label))
}

// versionLabel reads the version label of a movie named in the
// `Title (Year) - <label>` convention ("Heat (1995) - 1080p", "Heat (1995) -
// [Director's Cut]"): the text after a parenthesized year and a spaced hyphen,
// one layer of enclosing brackets dropped, bounded to sdk.MaxVersionLabelLen.
// It returns "" when the name does not follow the convention, when the suffix
// names a stacked part or an extra, or when the label breaks the SDK contract.
// A dotted release name ("Heat.1995.1080p.BluRay") carries no label: its tail
// is release noise, not a version the operator named.
func versionLabel(name string, yearStart int) string {
	const parenYearLen = len("(1995)")
	if yearStart < 0 || yearStart+parenYearLen > len(name) || name[yearStart] != '(' || name[yearStart+parenYearLen-1] != ')' {
		return ""
	}
	rest := name[yearStart+parenYearLen:]
	trimmed := strings.TrimLeft(rest, " ")
	if len(trimmed) == len(rest) || !strings.HasPrefix(trimmed, "- ") {
		return ""
	}
	label := strings.TrimSpace(trimmed[len("- "):])
	if len(label) >= 2 && (label[0] == '[' && label[len(label)-1] == ']' || label[0] == '(' && label[len(label)-1] == ')') {
		label = strings.TrimSpace(label[1 : len(label)-1])
	}
	if len(label) > sdk.MaxVersionLabelLen {
		cut := sdk.MaxVersionLabelLen
		for cut > 0 && !utf8.RuneStart(label[cut]) {
			cut--
		}
		label = strings.TrimSpace(label[:cut])
	}
	if label == "" || reNotVersion.MatchString(notVersionKey(label)) || !sdk.ValidVersionLabel(label) {
		return ""
	}
	return label
}

// firstNoiseToken returns the start index in lower of the first token that
// ends a title as release noise, or -1. A release group hyphenated onto a
// last release word ("Movie.PROPER-GRP") is not a title word, so it is left
// out of the words endsTitle looks ahead over. A name of two hyphenated words
// ("Uncut-Gems") keeps both: the first cannot end a title it would empty.
func firstNoiseToken(lower string) int {
	tokens := tokenSpans(lower)
	if n := len(tokens); n > 2 && trailingGroup(lower, tokens[n-2], tokens[n-1]) {
		tokens = tokens[:n-1]
	}
	words := make([]string, len(tokens))
	for i, span := range tokens {
		words[i] = lower[span[0]:span[1]]
	}
	for i := range words {
		if endsTitle(words[i:]) {
			return tokens[i][0]
		}
	}
	return -1
}

// trailingGroup reports whether last is a release group hyphenated directly
// onto prev, a release word ("PROPER-GRP"); a group that is itself a tag
// ("Movie-1080p") stays a word endsTitle can see.
func trailingGroup(lower string, prev, last [2]int) bool {
	if prev[1] != last[0]-1 || lower[prev[1]] != '-' {
		return false
	}
	group := lower[last[0]:last[1]]
	if reResolution.MatchString(group) || releaseNoise[group] {
		return false
	}
	_, isWord := releaseWords[lower[prev[0]:prev[1]]]
	return isWord
}

// endsTitle reports whether words[0] is release noise, given the words after
// it. A resolution or a releaseNoise tag always is, alone or joined with the
// next word ("web-dl", "h.264"). A releaseWords word mapped to true is noise
// in front of more release noise or as the last word; one mapped to false
// needs a releaseNoise tag after it, because a title often runs straight into
// a resolution ("Charlotte's.Web.1080p"). Said twice in a row
// ("The.Web.WEB-DL") the first is a title word.
func endsTitle(words []string) bool {
	word := words[0]
	if reResolution.MatchString(word) || releaseNoise[word] {
		return true
	}
	if len(words) > 1 && releaseNoise[word+words[1]] {
		return true
	}
	endsName, isWord := releaseWords[word]
	if !isWord {
		return false
	}
	if len(words) == 1 {
		return endsName
	}
	if words[1] == word {
		return false
	}
	if endsName {
		return endsTitle(words[1:])
	}
	return tagFollows(words[1:])
}

// tagFollows reports whether words open with a releaseNoise tag, possibly
// after other release words. A resolution stops the look-ahead: a title runs
// straight into one, and the codec after it ("Charlotte's.Web.1080p.x264")
// says nothing about the word before.
func tagFollows(words []string) bool {
	for i, word := range words {
		if releaseNoise[word] || i+1 < len(words) && releaseNoise[word+words[i+1]] {
			return true
		}
		if _, isWord := releaseWords[word]; !isWord {
			return false
		}
	}
	return false
}

// tokenSpans splits a lower-cased name into word tokens, as [start, end) byte
// spans, on the separators cleanTitle normalises (`.`, `_`, space, `-`) and on
// brackets, so a tag in "[1080p]" or "(WEB-DL)" is seen as a token.
func tokenSpans(lower string) [][2]int {
	var spans [][2]int
	start := -1
	for i := 0; i <= len(lower); i++ {
		if i < len(lower) && !strings.ContainsRune("._ -[](){}", rune(lower[i])) {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			spans = append(spans, [2]int{start, i})
			start = -1
		}
	}
	return spans
}

// cleanTitle turns a raw title fragment into a display title: `.`/`_` → spaces,
// whitespace collapsed, and the separators and unbalanced brackets a cut leaves
// dangling at either end trimmed ("Pioneer One - " → "Pioneer One").
func cleanTitle(raw string) string {
	replaced := strings.NewReplacer(".", " ", "_", " ").Replace(raw)
	title := strings.Join(strings.Fields(replaced), " ")
	title = strings.TrimLeft(title, " -–—,:;)]}")
	return strings.TrimRight(title, " -–—,:;([{")
}

func isASCIIAlnum(c byte) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9'
}

// asciiLower lower-cases only the ASCII letters of s, byte for byte, so every
// byte offset in the result maps to the same offset in s. The episode/year/
// noise patterns matched against the folded copy are all ASCII, so this is
// sufficient — and unlike strings.ToLower it can never change the byte length
// (some Unicode case mappings do: U+023A "Ⱥ" folds to the 3-byte U+2C65 "ⱥ",
// U+212A "K" to the 1-byte "k"), which would let a match index found on the
// folded copy drift off the original string: a grown fold panics the
// name[:cut] title slice, a shrunken one silently truncates the title.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c - 'A' + 'a'
		}
	}
	return string(b)
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func numericText(s string) string {
	return strconv.Itoa(atoi(s))
}
