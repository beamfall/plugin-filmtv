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
	// SxxEyy with optional -Eyy / Eyy range tail (multi-episode files).
	reSeasonEpisode = regexp.MustCompile(`s(\d{1,2})e(\d{1,3})(?:[-]?e(\d{1,3}))?`)
	// "season X episode Y" long form.
	reSeasonEpisodeWords = regexp.MustCompile(`season[ ._]*(\d{1,2})[ ._]*episode[ ._]*(\d{1,3})`)
	// XxYY shorthand (e.g. 3x07); guarded so a resolution like 1920x1080 never
	// matches (those numbers are far larger than a season/episode).
	reSeasonX = regexp.MustCompile(`\b(\d{1,2})x(\d{1,3})\b`)
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

// releaseNoise is the lower-cased set of release-name tokens that mark the end
// of a real title. The cut is conservative: only well-known noise words.
var releaseNoise = map[string]bool{
	"1080p": true, "720p": true, "480p": true, "2160p": true, "4k": true,
	"bluray": true, "blu-ray": true, "brrip": true, "bdrip": true,
	"webrip": true, "web-dl": true, "webdl": true, "hdrip": true,
	"dvdrip": true, "hdtv": true, "x264": true, "x265": true,
	"h264": true, "h265": true, "hevc": true, "xvid": true, "divx": true,
	"aac": true, "ac3": true, "dts": true, "remux": true, "proper": true,
	"repack": true, "extended": true, "unrated": true, "internal": true,
}

// Parse derives a best-guess ParsedMedia from a path and its local NFO evidence.
// probe is accepted to satisfy the seam shape but not read. ok is
// true only when a non-empty Title was extracted.
func (p *Parser) Parse(path string, probe sdk.ParseProbe) (sdk.ParsedMedia, bool) {
	_ = probe // probe rides the seam for future parsers.

	base := filepath.Base(path)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	name = removeProviderIDTags(name)
	lower := asciiLower(name)

	var parsed sdk.ParsedMedia

	// 1. Episode detection. cut marks where the title ends on the original-cased
	// string (the earliest of any episode/year/noise token start).
	cut := len(name)
	episodeCut := -1
	if m := reSeasonEpisode.FindStringSubmatchIndex(lower); m != nil {
		parsed.Sequence = numericText(lower[m[2]:m[3]])
		parsed.Index = atoi(lower[m[4]:m[5]])
		if m[6] >= 0 {
			parsed.IndexEnd = atoi(lower[m[6]:m[7]])
		}
		parsed.Class = "episode"
		episodeCut = m[0]
	} else if m := reSeasonEpisodeWords.FindStringSubmatchIndex(lower); m != nil {
		parsed.Sequence = numericText(lower[m[2]:m[3]])
		parsed.Index = atoi(lower[m[4]:m[5]])
		parsed.Class = "episode"
		episodeCut = m[0]
	} else if m := reSeasonX.FindStringSubmatchIndex(lower); m != nil {
		parsed.Sequence = numericText(lower[m[2]:m[3]])
		parsed.Index = atoi(lower[m[4]:m[5]])
		parsed.Class = "episode"
		episodeCut = m[0]
	}
	if episodeCut >= 0 && episodeCut < cut {
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
		if noiseStart := firstNoiseToken(lower); noiseStart >= 0 && noiseStart < cut {
			cut = noiseStart
		}
	}

	// 4. Title cleanup: take everything before the cut, normalise separators.
	parsed.Title = cleanTitle(name[:cut])

	// 5. Class fallback: episode signals already set "episode"; otherwise any
	// useful parse (a year or a clean title) is a movie.
	if parsed.Class == "" && (parsed.Year > 0 || parsed.Title != "") {
		parsed.Class = "movie"
	}

	// Return ok only when a non-empty Title survived cleanup.
	if parsed.Title == "" {
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

// firstNoiseToken returns the start index in lower of the first release-noise
// token, or -1. Tokens are split on the same separators cleanTitle collapses.
func firstNoiseToken(lower string) int {
	idx := 0
	for _, token := range splitTokens(lower) {
		start := strings.Index(lower[idx:], token)
		if start < 0 {
			continue
		}
		start += idx
		idx = start + len(token)
		if releaseNoise[token] {
			return start
		}
	}
	return -1
}

// splitTokens splits a lower-cased name into word tokens on the separators
// cleanTitle normalises (`.`, `_`, space, `-`).
func splitTokens(lower string) []string {
	return strings.FieldsFunc(lower, func(r rune) bool {
		return r == '.' || r == '_' || r == ' ' || r == '-'
	})
}

// cleanTitle turns a raw title fragment into a display title: `.`/`_` → spaces,
// whitespace collapsed, trailing separators trimmed.
func cleanTitle(raw string) string {
	replaced := strings.NewReplacer(".", " ", "_", " ").Replace(raw)
	return strings.Join(strings.Fields(replaced), " ")
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
