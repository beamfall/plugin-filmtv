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
	// A compact season-and-episode code with no "e" ("S0101": season 01,
	// episode 01), after a separator and not followed by a letter or digit.
	// A film can carry the same shape ("Agent S0101"), so it is read only with
	// episode context (compactSeasonEpisode, CORE-2554). Groups: the season,
	// the episode.
	reCompactSeason = regexp.MustCompile(`(?:^|[ ._-])s(\d{2})(\d{2})(?:$|[^a-z0-9])`)
	// A leading "season episode" number pair ("3 01 From Hell"). Only read in
	// a season folder (leadingEpisode): a film name can start with two numbers
	// too ("3 10 to Yuma"). Groups: season, episode.
	reLeadingNumbers = regexp.MustCompile(`^(\d{1,2})[ ._-]+(\d{2,3})(?:$|[ ._-])`)
	// An absolute episode number after a series name: zero-padded to four
	// digits ("Dealbreakers Talk Show 0001"), or two to four digits after
	// " - " ("Naruto - 001", "Mob Psycho 100 - 09", "One Piece - 1071"), where
	// a release revision may follow ("Show - 12v2"). A single digit after
	// " - " is no episode: it numbers a film sequel ("Movie - 2") as often.
	// It ends the title, or an episode title follows it after " - " ("Show -
	// 001 - Title"). The title cut may leave the separator or bracket that
	// opened the release noise. Read only with episode context
	// (episodeContext). Groups: the separator and digits of the four-digit
	// form, the digits of the dashed form.
	reAbsoluteEpisode = regexp.MustCompile(`(?:([ ._-])(0\d{3})|[ ._]-[ ._](\d{2,4})(?:v\d{1,2})?)(?:[ ._\-\[(]*$|[ ._]-[ ._])`)
	// A CRC32 checksum bracket an anime release carries ("[ABCD1234]"):
	// release noise, and a sign the file is an episode.
	reChecksum = regexp.MustCompile(`\[[0-9a-f]{8}\]`)
	// An air date naming a dated (daily) episode: YYYY-MM-DD, also with ".",
	// "_" or " " separators, not inside a longer run of letters or digits
	// (an underscore still separates it, unlike a \b boundary). Calendar-checked
	// in code. Groups: year, month, day.
	reAirDate = regexp.MustCompile(`(?:^|[^a-z0-9])((?:19|20)\d{2})[-._ ](\d{2})[-._ ](\d{2})(?:$|[^a-z0-9])`)
	// A folder that holds one season of a series: the series name is then the
	// folder above it. The season word may follow the series name ("The IT
	// Crowd  Series 3"). Groups: the series prefix, the season number of the
	// worded form, the season number of the "s1" form.
	reSeasonFolder = regexp.MustCompile(`^(?:(?:(.*?)[ ._-]+)?(?:season|series)[ ._-]*(\d{1,4})|s(\d{1,2})|specials)$`)
	// An extras label numbered by season but not by episode ("3.x Out Takes",
	// "3x Set Tour"): never a film. Groups: the season, the label.
	reSeasonExtra = regexp.MustCompile(`^(\d{1,2})[ ._-]*x[ ._-]+([a-z].*)$`)
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
	// Dynamic-range, audio and streaming-source tags that are not words
	// (CORE-2470: "Ted.Lasso.S03E08.DV.2160p" must not be titled "DV"). A
	// "+" is not a token separator, so "HDR10+" is looked up whole.
	"dovi": true, "hdr10+": true, "hdr10plus": true, "hlg": true, "sdr": true,
	"ddp": true, "avc": true,
	"atvp": true, "amzn": true, "dsnp": true, "hmax": true,
}

// releaseWords are release tags that are also ordinary words, so one can sit
// inside a real title ("Internal Affairs", "Extended Family", "Charlotte's
// Web"). Such a word ends the title only when a release tag follows it; the
// ones mapped to true also end it as the last word of the name
// ("Cool.Movie.PROPER").
var releaseWords = map[string]bool{
	"proper": true, "repack": true, "rerip": true, "extended": true,
	"unrated": true, "internal": true, "uncut": true, "remastered": true,
	// Short tags that can still be words or initials in a real title
	// ("The HDR Story"): noise only before more release tags or at the end.
	"dv": true, "hdr": true, "atmos": true,
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
		episode, isEpisode = compactSeasonEpisode(path, lower)
	}
	if !isEpisode {
		episode, isEpisode = leadingEpisode(path, lower)
	}
	if !isEpisode {
		episode, isEpisode = seriesCodeEpisode(path, lower)
	}
	if !isEpisode {
		episode, isEpisode = absoluteEpisode(path, name, lower)
	}
	episodeCut := -1
	if isEpisode {
		parsed.Class = "episode"
		if episode.airDate.IsZero() {
			parsed.Sequence = episode.sequence
			parsed.Index = episode.index
			parsed.IndexEnd = episode.indexEnd
			parsed.AbsoluteOrdinal = episode.absolute
		}
		episodeCut = episode.start
		cut = episodeCut
	}

	// 2. Year (see findYear). The title ends at the year, or starts after a
	// leading parenthesized one ("(2000) Title").
	titleStart := 0
	year := findYear(name, lower, episodeCut)
	if year.found() {
		parsed.Year = year.year
		switch {
		case year.leading:
			titleStart = year.end
		case year.start < cut:
			cut = year.start
		}
		if !isEpisode && !year.leading {
			parsed.VersionLabel = versionLabel(name, year.start)
		}
	}

	// 3. Title-ending release noise. Only applied when neither an episode nor a
	// year cut was already found (cut still == len(name)): a noise WORD can
	// legitimately appear inside a real title ("Le Cercle Internal", "Extended
	// Family"), so once a year/episode token has already bounded the title, a
	// noise token elsewhere in the string must not truncate it further.
	if cut == len(name) {
		if noiseStart := firstNoiseToken(lower[titleStart:], true); noiseStart >= 0 {
			cut = titleStart + noiseStart
		}
		if checksum := reChecksum.FindStringIndex(lower[titleStart:cut]); checksum != nil {
			cut = titleStart + checksum[0]
		}
	}
	titleStart = min(titleStart, cut)
	if year.leading && onlyNoiseAfter(name, lower, year.end, cut) && cleanTitle(name[:year.start]) != "" {
		// Nothing but noise follows a leading year ("[REC] (2007) 1080p"), so the
		// bracket before it is the title and the year ends it.
		titleStart, cut = 0, year.start
		if !isEpisode {
			parsed.VersionLabel = versionLabel(name, year.start)
		}
	}

	// 4. Title cleanup: take everything before the cut, normalise separators.
	// For an episode this is the series name: a release-group bracket in front
	// of it is dropped, and a file that carries no series name of its own
	// ("S01E01 - Earthfall") takes the one of the folder above its season folder.
	rawTitle := name[titleStart:cut]
	if isEpisode {
		rawTitle = rawTitle[len(reLeadingGroups.FindString(rawTitle)):]
	}
	parsed.Title = cleanTitle(rawTitle)
	if isEpisode && parsed.Year == 0 && isYearNumber(parsed.Title) {
		// "Doctor Who (2005)/Season 01/2005.S01E01": a bare year as the whole
		// series name is the show folder's year when that folder carries it.
		if series, folderYear := seriesFromFolders(path); folderYear == atoi(parsed.Title) {
			parsed.Title, parsed.Year = series, folderYear
		}
	}
	if isEpisode && parsed.Title == "" {
		var folderYear int
		if parsed.Title, folderYear = seriesFromFolders(path); folderYear > 0 {
			parsed.Year = folderYear
		}
	}

	// 5. Class fallback: episode signals already set "episode"; otherwise any
	// useful parse (a year or a clean title) is a movie, unless the file sits
	// under a TV library folder. A file there is never a film: it keeps no
	// class, and a season-numbered extras label ("3.x Out Takes") keeps only
	// its label as the title.
	if parsed.Class == "" && (parsed.Year > 0 || parsed.Title != "") {
		if nearestLibraryFolder(filepath.Dir(path)) != "tv" {
			parsed.Class = "movie"
		} else if m := reSeasonExtra.FindStringSubmatchIndex(lower[:cut]); m != nil {
			parsed.Title = cleanTitle(name[m[4]:cut])
		}
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
	// absolute is the absolute episode number of an absolute-numbered
	// episode, which then has no sequence; 0 otherwise.
	absolute int
	airDate  time.Time
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
// dot is a channel layout ("DD5.1e01"), not a season. The code carries its own
// season, so it stands in a season folder that names another one; only
// "Specials" overrides it (season 0).
func bareSeasonEpisode(path, lower string) (episodeToken, bool) {
	for _, m := range reSeasonE.FindAllStringSubmatchIndex(lower, -1) {
		if start := m[2]; start >= 2 && lower[start-1] == '.' && lower[start-2] >= '0' && lower[start-2] <= '9' {
			continue
		}
		if !episodeContext(path, lower, cleanTitle(lower[:m[2]]), formBareCode) {
			return episodeToken{}, false
		}
		sequence := numericText(lower[m[2]:m[3]])
		if isSpecialsFolder(path) {
			sequence = "0"
		}
		return episodeToken{start: m[2], end: m[5], sequence: sequence, index: atoi(lower[m[4]:m[5]])}, true
	}
	return episodeToken{}, false
}

// compactSeasonEpisode reads a compact "S0101" code (season 1, episode 1)
// only with folder context: a season folder, a TV library folder, or a show
// folder named for the series in front of the code. A checksum bracket alone
// is not enough ("Agent S0101 [ABCD1234]" stays a film). Like the bare-e
// code it carries its own season; only "Specials" overrides it (season 0).
func compactSeasonEpisode(path, lower string) (episodeToken, bool) {
	m := reCompactSeason.FindStringSubmatchIndex(lower)
	if m == nil {
		return episodeToken{}, false
	}
	start := m[2] - 1 // the "s"
	// The series in front of the code is normalized as a show folder's name
	// is, so a year in both ("Show (2007)/Show.(2007).S0101") still matches,
	// and a leading release-group bracket is not part of it.
	prefix := lower[:start]
	series, _ := seriesFromFolder(prefix[len(reLeadingGroups.FindString(prefix)):])
	if !episodeContext(path, lower, series, formCompactCode) {
		return episodeToken{}, false
	}
	sequence := numericText(lower[m[2]:m[3]])
	if isSpecialsFolder(path) {
		sequence = "0"
	}
	index := atoi(lower[m[4]:m[5]])
	if index == 0 {
		return episodeToken{}, false
	}
	return episodeToken{start: start, end: m[5], sequence: sequence, index: index}, true
}

// leadingEpisode reads a leading "season episode" number pair ("3 01 From
// Hell") when path sits in a season folder of that season, in "Specials"
// (season 0), or in a show folder under a TV library folder. Elsewhere two
// leading numbers are as likely a film title ("3 10 to Yuma"), so they are not
// read.
func leadingEpisode(path, lower string) (episodeToken, bool) {
	m := reLeadingNumbers.FindStringSubmatchIndex(lower)
	if m == nil {
		return episodeToken{}, false
	}
	number, inSeasonFolder := seasonFolderNumber(path)
	sequence := numericText(lower[m[2]:m[3]])
	switch {
	case !inSeasonFolder && showFolder(path) != "":
		// A show folder under a TV library folder holds no films.
	case !inSeasonFolder:
		return episodeToken{}, false
	case isSpecialsFolder(path):
		sequence = "0"
	case number != atoi(sequence):
		return episodeToken{}, false
	}
	return episodeToken{start: 0, end: m[5], sequence: sequence, index: atoi(lower[m[4]:m[5]])}, true
}

// seasonFolderNumber reports whether path sits in a season folder and that
// folder's season number, 0 for "Specials".
func seasonFolderNumber(path string) (int, bool) {
	dir := filepath.Dir(path)
	m := reSeasonFolder.FindStringSubmatch(asciiLower(filepath.Base(dir)))
	if m == nil || (m[1] != "" && nearestLibraryFolder(dir) != "tv") {
		return 0, false
	}
	return atoi(m[2] + m[3]), true
}

// showFolder returns the name of the folder path sits in when that folder is
// a show folder: it is not a season folder or a library folder itself, and
// the nearest library folder above it names a TV library ("/tv/The Simpsons/
// …"). It returns "" otherwise.
func showFolder(path string) string {
	dir := filepath.Dir(path)
	folder := filepath.Base(dir)
	lower := asciiLower(folder)
	if isSeasonFolder(dir) || tvFolders[lower] || filmFolders[lower] {
		return ""
	}
	if nearestLibraryFolder(filepath.Dir(dir)) != "tv" {
		return ""
	}
	return folder
}

// reSeriesCodeDigits is a three- or four-digit season-and-episode code
// ("2001": season 20, episode 01) that ends the name or a token, so a
// resolution glued to the name ("sherlock1080p") is not one.
var reSeriesCodeDigits = regexp.MustCompile(`^(\d{1,2})(\d{2})(?:$|[^a-z0-9])`)

// resolutionDigits are the line counts a bare resolution names: never a
// season-and-episode code glued to a series name.
var resolutionDigits = map[string]bool{"480": true, "576": true, "720": true, "1080": true, "2160": true}

// seriesCodeEpisode reads a season-and-episode code glued to the series name
// ("aaf-simpsons2001" in "The Simpsons"): the show folder's name, folded to
// letters and digits with a leading "The" optional, followed by the code. It
// is read only in a show folder under a TV library folder, whose name must be
// in the file's; the folder's year glued to the name ("doctorwho2005") is not
// a code. The token starts at the beginning of the name, so a release
// group in front of the series name stays out of the title, which then comes
// from the folder.
func seriesCodeEpisode(path, lower string) (episodeToken, bool) {
	series, year := seriesFromFolder(showFolder(path))
	if series == "" {
		return episodeToken{}, false
	}
	key := foldAlnum(asciiLower(series))
	var keys []string
	if len(key) >= 3 {
		keys = append(keys, key)
	}
	if trimmed := strings.TrimPrefix(key, "the"); trimmed != key && len(trimmed) >= 3 {
		keys = append(keys, trimmed)
	}
	for _, key := range keys {
		for from := 0; ; {
			at := strings.Index(lower[from:], key)
			if at < 0 {
				break
			}
			end := from + at + len(key)
			if m := reSeriesCodeDigits.FindStringSubmatchIndex(lower[end:]); m != nil && !resolutionDigits[lower[end:end+m[5]]] &&
				(year == 0 || lower[end:end+m[5]] != strconv.Itoa(year)) {
				index := atoi(lower[end+m[4] : end+m[5]])
				if index > 0 {
					return episodeToken{start: 0, end: end + m[5], sequence: numericText(lower[end+m[2] : end+m[3]]), index: index}, true
				}
			}
			from = end
		}
	}
	return episodeToken{}, false
}

// foldAlnum keeps only the ASCII letters and digits of s.
func foldAlnum(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 128 && isASCIIAlnum(byte(r)) {
			return r
		}
		return -1
	}, s)
}

// isSeasonFolder reports whether dir is a season folder. The form with a
// series name in front of the season word ("Sherlock Series 3") counts only
// under a TV library folder: elsewhere it is as likely a film collection
// ("Movies/Anthology Series 3").
func isSeasonFolder(dir string) bool {
	_, ok := seasonFolderNumber(filepath.Join(dir, "x"))
	return ok
}

// isSpecialsFolder reports whether path sits in a "Specials" season folder.
func isSpecialsFolder(path string) bool {
	return asciiLower(filepath.Base(filepath.Dir(path))) == "specials"
}

// cameraPrefixes are the stems cameras and phones number their clips with
// ("VID_0001", "DSC_0012", "DJI_0001"): never a series name.
var cameraPrefixes = map[string]bool{
	"vid": true, "img": true, "dsc": true, "dscf": true, "dscn": true, "mov": true,
	"mvi": true, "gopr": true, "gp": true, "pxl": true, "dji": true, "sam": true,
	"cimg": true, "imgp": true,
}

// tvFolders name a folder that holds series, and filmFolders one that holds
// films. The nearest such folder above a file says which it is.
var (
	tvFolders = map[string]bool{
		"tv": true, "tv shows": true, "television": true, "shows": true, "series": true, "anime": true,
	}
	filmFolders = map[string]bool{"movies": true, "movie": true, "films": true, "film": true}
)

// weakForm is the shape of a weak episode marker, which sets how much
// context it needs.
type weakForm int

const (
	// formBareCode is a bare-e season/episode code ("1e01").
	formBareCode weakForm = iota
	// formNumbered is a four-digit number after a space, dot or dash
	// ("Show 0001").
	formNumbered
	// formUnderscored is a four-digit number after an underscore
	// ("Wedding_0002"): the counter a camera or a home video carries as often
	// as an episode number.
	formUnderscored
	// formDashed is a two- to four-digit number after " - " ("Naruto - 001",
	// "One Piece - 1071"), the anime release form.
	formDashed
	// formDashedYear is formDashed after a series name with a year ("Hunter x
	// Hunter (2011) - 12"). It has the shape of a film's version label ("Heat
	// (1995) - 1080"), and a film's own folder bears the same name, so a show
	// folder named like the series is no context for it.
	formDashedYear
	// formCompactCode is a compact season-and-episode code ("S0101"): only a
	// folder gives it context, never a checksum bracket.
	formCompactCode
)

// episodeContext reports whether path carries evidence beyond the bare
// numbers in its name that it is an episode of series. lower is the whole
// ASCII-folded name, release noise included. A film library folder
// ("Movies") as the nearest library folder above the file rules out all
// context, a season folder below it included. The evidence, strongest first:
//
//   - a season folder;
//   - the nearest library folder above the file being a TV folder ("TV",
//     "Anime");
//   - a CRC32 checksum bracket, or a release-group bracket in front of the
//     anime " - 001" form;
//   - a show folder named for series, except for an underscored counter and
//     a dashed number after a series name with a year.
//
// A camera clip stem is never a series. The weak episode forms are read only
// with this context, because a film or a home video can end on the same
// digits ("Agent 0011", "Wedding_0002").
func episodeContext(path, lower, series string, form weakForm) bool {
	if cameraPrefixes[asciiLower(series)] {
		return false
	}
	dir := filepath.Dir(path)
	library := nearestLibraryFolder(dir)
	if library == "film" {
		return false
	}
	if _, inSeasonFolder := seasonFolderNumber(path); inSeasonFolder {
		return true
	}
	switch {
	case library == "tv", form != formCompactCode && reChecksum.MatchString(lower):
		return true
	case (form == formDashed || form == formDashedYear) && reLeadingGroups.MatchString(lower):
		return true
	case form == formUnderscored || form == formDashedYear || series == "":
		return false
	}
	folder, _ := seriesFromFolder(filepath.Base(dir))
	return strings.EqualFold(folder, series)
}

// nearestLibraryFolder returns "tv" or "film" for the nearest folder above dir
// that names a TV or a film library, or "" when none does.
func nearestLibraryFolder(dir string) string {
	folders := strings.Split(filepath.ToSlash(dir), "/")
	for i := len(folders) - 1; i >= 0; i-- {
		switch name := asciiLower(folders[i]); {
		case tvFolders[name]:
			return "tv"
		case filmFolders[name]:
			return "film"
		}
	}
	return ""
}

// findAbsoluteEpisode reads an absolute episode number in title, the part of
// path's name before any year or release noise, and lowerTitle, its
// ASCII-folded copy. lower is the whole folded name, whose checksum bracket
// counts as context although the title stops before it. The number must
// follow a series name of its own, and path must give episode context.
// absoluteEpisode reads an absolute episode number in name, the file name
// without its extension, and lower, its ASCII-folded copy. It runs before the
// year is read, on the name up to its release noise, so a year in the series
// name ("Show (2011) - 12") or in the episode title ("Show - 12 - Class of
// 1999") leaves the number readable.
func absoluteEpisode(path, name, lower string) (episodeToken, bool) {
	cut := len(name)
	if noiseStart := firstNoiseToken(lower, true); noiseStart >= 0 {
		cut = noiseStart
	}
	if checksum := reChecksum.FindStringIndex(lower[:cut]); checksum != nil {
		cut = checksum[0]
	}
	token, ok := findAbsoluteEpisode(path, name[:cut], lower[:cut], lower)
	token.absolute = token.index
	return token, ok
}

func findAbsoluteEpisode(path, title, lowerTitle, lower string) (episodeToken, bool) {
	m := reAbsoluteEpisode.FindStringSubmatchIndex(lowerTitle)
	if m == nil {
		return episodeToken{}, false
	}
	series := title[:m[0]]
	series = series[len(reLeadingGroups.FindString(series)):]
	year := findYear(series, asciiLower(series), -1)
	series = cleanTitle(year.title(series))
	form, digits := formDashed, m[6:8]
	if m[4] >= 0 {
		form, digits = formNumbered, m[4:6]
		if lowerTitle[m[2]] == '_' {
			form = formUnderscored
		}
	}
	if year.found() && !year.leading {
		// After a year only the dashed anime form reads as an episode.
		if form != formDashed {
			return episodeToken{}, false
		}
		form = formDashedYear
	}
	if series == "" || !episodeContext(path, lower, series, form) {
		return episodeToken{}, false
	}
	index := atoi(lowerTitle[digits[0]:digits[1]])
	if index == 0 || (form == formDashed || form == formDashedYear) && digits[1]-digits[0] == 4 && index >= 1900 && index <= 2099 {
		// A dashed four-digit number in the year range is a year ("Show - 2015").
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
	if noiseStart := firstNoiseToken(lowerTail[:end], false); noiseStart >= 0 {
		end = noiseStart
	}
	return cleanTitle(tail[:end])
}

// seriesFromFolders returns the series name and year an episode's folders
// carry: the folder above path's season folder ("Pioneer One (2010)/Season
// 01/…"), or, when that is a library folder, the series name in front of the
// season word ("TV/Sherlock Series 3/…"); else the show folder path sits in
// under a TV library folder ("TV/The Simpsons/…"). It returns ("", 0)
// otherwise: the parent of a loose file elsewhere is as likely a library root
// as a series.
func seriesFromFolders(path string) (string, int) {
	seasonDir := filepath.Dir(path)
	if m := reSeasonFolder.FindStringSubmatchIndex(asciiLower(filepath.Base(seasonDir))); m != nil && (m[2] < 0 || nearestLibraryFolder(seasonDir) == "tv") {
		above := filepath.Base(filepath.Dir(seasonDir))
		if lower := asciiLower(above); m[2] >= 0 && (tvFolders[lower] || filmFolders[lower]) {
			return seriesFromFolder(filepath.Base(seasonDir)[m[2]:m[3]])
		}
		return seriesFromFolder(above)
	}
	return seriesFromFolder(showFolder(path))
}

// seriesFromFolder returns the series name and year a show folder's name
// carries ("Pioneer One (2010)"), or ("", 0) when it names none.
func seriesFromFolder(folder string) (string, int) {
	series := removeProviderIDTags(folder)
	if series == "." || series == string(filepath.Separator) || strings.ContainsRune(series, filepath.Separator) {
		return "", 0
	}
	year := findYear(series, asciiLower(series), -1)
	title := cleanTitle(year.title(series))
	if title == "" {
		return "", 0
	}
	return title, year.year
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

// yearToken is the release year read from a name: the year, the byte span of
// its token (parentheses included), and whether it leads the name. The title
// of a name is the text before its year, or the text after a leading one.
type yearToken struct {
	year, start, end int
	leading          bool
}

// found reports whether the name carries a release year.
func (y yearToken) found() bool { return y.year > 0 }

// title returns the part of s, the name y was read from, that holds the title.
func (y yearToken) title(s string) string {
	switch {
	case !y.found():
		return s
	case y.leading && !onlySeparators(s[y.end:]):
		return s[y.end:]
	default:
		return s[:y.start]
	}
}

// findYear reads the release year of a name. Years from 1900 to 2099 count,
// except a resolution (2160p, 1080i), a token inside the leading release-group
// brackets, and a token at or after episodeCut (part of an SxxEyy match). A
// parenthesized year with title text before it wins ("1917 (2019)");
// otherwise a parenthesized year with none before it leads the name ("(2000)
// Title"). Otherwise the year is the first standalone one with title text
// before it that is not directly followed by another year: "2012.2009" and
// "Blade.Runner.2049.2017" keep their first number as title text, and a lone
// leading year ("2012") is no year at all. A tight rising range ("Fringe
// 2008-2013") is a run of years, not a title number: its first year is the
// year. When no standalone year has title text before it, one after only
// release-group brackets is the year ("[REC].2007").
func findYear(name, lower string, episodeCut int) yearToken {
	groupsEnd := len(reLeadingGroups.FindString(name))
	var candidates []yearToken
	var parens []bool
	for _, m := range reYear.FindAllStringSubmatchIndex(lower, -1) {
		digitsStart, digitsEnd := m[2], m[3]
		year := atoi(lower[digitsStart:digitsEnd])
		if year < 1900 || year > 2099 || digitsStart < groupsEnd {
			continue
		}
		if digitsEnd < len(lower) {
			if c := lower[digitsEnd]; c == 'p' || c == 'i' {
				continue
			}
		}
		if episodeCut >= 0 && digitsStart >= episodeCut {
			continue
		}
		paren := m[0] < digitsStart && name[m[0]] == '('
		start := digitsStart
		if paren {
			start = m[0]
		}
		candidates = append(candidates, yearToken{year: year, start: start, end: m[1]})
		parens = append(parens, paren)
	}
	leading := yearToken{}
	for i, c := range candidates {
		if !parens[i] {
			continue
		}
		if hasTitleText(name[:c.start]) {
			return c
		}
		if !leading.found() {
			leading = c
			leading.leading = true
		}
	}
	if leading.found() {
		return leading
	}
	bracketed := yearToken{}
	rangeEnd := false
	for i, c := range candidates {
		if rangeEnd {
			rangeEnd = false
			continue
		}
		if i+1 < len(candidates) && !parens[i+1] {
			next := candidates[i+1]
			between := name[c.end:next.start]
			if onlySeparators(between) && (between != "-" || next.year <= c.year) {
				continue
			}
			rangeEnd = between == "-" && next.year > c.year
		}
		if hasTitleText(name[:c.start]) {
			return c
		}
		if !bracketed.found() && groupsEnd > 0 && episodeCut < 0 {
			// An episode drops its leading brackets, so they are never its title.
			bracketed = c
		}
	}
	return bracketed
}

// onlyNoiseAfter reports whether name[end:cut], the text after a leading year,
// holds no title: nothing, or release noise from its first word on ("PROPER").
func onlyNoiseAfter(name, lower string, end, cut int) bool {
	if cleanTitle(name[min(end, cut):cut]) == "" {
		return true
	}
	noise := firstNoiseToken(lower[end:], false)
	return noise >= 0 && cleanTitle(name[end:end+noise]) == ""
}

// hasTitleText reports whether s, the text before a year, holds any title:
// anything that survives title cleanup outside the leading release-group
// brackets ("$", not "–").
func hasTitleText(s string) bool {
	return cleanTitle(s[len(reLeadingGroups.FindString(s)):]) != ""
}

// isYearNumber reports whether s is a four-digit number from 1900 to 2099.
func isYearNumber(s string) bool {
	if len(s) != 4 || strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return false
	}
	year := atoi(s)
	return year >= 1900 && year <= 2099
}

// onlySeparators reports whether s, the text between two years, is only
// spaces, dots, underscores and hyphens.
func onlySeparators(s string) bool {
	return strings.Trim(s, " ._-") == ""
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
// With keepFirstWord, a release word that is also an ordinary word never ends
// the title at its first token either, so a film named "Atmos" or "DV" keeps
// its name; an episode title passes false, because the episode token already
// names the file and "S03E08.DV.2160p" has no title of its own.
func firstNoiseToken(lower string, keepFirstWord bool) int {
	tokens := tokenSpans(lower)
	if n := len(tokens); n > 2 && trailingGroup(lower, tokens[n-2], tokens[n-1]) {
		tokens = tokens[:n-1]
	}
	words := make([]string, len(tokens))
	for i, span := range tokens {
		words[i] = lower[span[0]:span[1]]
	}
	for i := range words {
		if i == 0 && keepFirstWord && !isTag(words) {
			continue
		}
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
	if isTag(words) {
		return true
	}
	word := words[0]
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

// isTag reports whether words[0] is a resolution or a releaseNoise tag, alone
// or joined with the next word ("web-dl", "h.264"): never a title word.
func isTag(words []string) bool {
	word := words[0]
	return reResolution.MatchString(word) || releaseNoise[word] ||
		len(words) > 1 && releaseNoise[word+words[1]]
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
