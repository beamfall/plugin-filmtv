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

	parsed, _, ok := parseName(path)
	if !ok {
		return sdk.ParsedMedia{}, false
	}
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
	parsed, episodeTitle, ok := parseName(path)
	if !ok || parsed.Class != "episode" || episodeTitle == "" {
		return nil
	}
	return []sdk.ParsedTextField{{Name: EpisodeTitleField, Text: episodeTitle}}
}

// parseName is the path-only parse shared by Parse and ParseMediaFields. It
// returns the parsed fields (without provider hints) and, for an episode, the
// episode's own title when the name carries one.
func parseName(path string) (sdk.ParsedMedia, string, bool) {
	base := filepath.Base(path)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	name = removeProviderIDTags(name)
	lower := asciiLower(name)

	var parsed sdk.ParsedMedia

	// 1. Episode detection. cut marks where the title ends on the original-cased
	// string (the earliest of any episode/year/noise token start).
	cut := len(name)
	episode, isEpisode := findEpisode(lower)
	episodeCut := -1
	if isEpisode {
		parsed.Class = "episode"
		parsed.Sequence = episode.sequence
		parsed.Index = episode.index
		parsed.IndexEnd = episode.indexEnd
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
		return sdk.ParsedMedia{}, "", false
	}
	if !isEpisode {
		return parsed, "", true
	}
	return parsed, episodeTitle(name[episode.end:], lower[episode.end:]), true
}

// episodeToken is one recognised season/episode marker: its byte span in the
// name and the ordinals it carries.
type episodeToken struct {
	start, end      int
	sequence        string
	index, indexEnd int
}

// findEpisode locates the season/episode marker in the ASCII-folded name. The
// forms are tried in order of how unambiguous they are: SxxEyy, "season X
// episode Y", then XxYY.
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
		start := pos
		pos += m[1]
		if pos == len(lower) || !isASCIIAlnum(lower[pos]) {
			if m[2] >= 0 && numericText(lower[start+m[2]:start+m[3]]) != sequence {
				crossesSeason = true
			}
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
	series := removeProviderIDTags(filepath.Base(filepath.Dir(seasonDir)))
	if series == "." || strings.ContainsRune(series, filepath.Separator) {
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
