package filmtv_test

import (
	"reflect"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"

	filmtv "github.com/beamfall/plugin-filmtv"
)

// This file is the plugin's parser regression corpus:
// an adversarial filename corpus that replaces the prose enumeration of parser
// behavior with executable, pinned expectations, plus a scored auto-match-rate
// metric with a documented target.
//
// Two things are asserted, against exactly the same fixtures:
//
//   - TestParseCorpus is the per-case *regression* gate. Every case pins the
//     parser's full ParsedMedia output (or its documented refusal) for one
//     hostile-input class the Film & TV parser must classify consistently
//     across releases. A change in any field reds the case — the corpus, not a
//     prose paragraph, is the contract for "what does this filename parse to".
//
//   - TestParseCorpusAutoMatchRate is the aggregate *auto-match-rate* metric
//     (the parser-side analog of matchcorpus_test.go's precision gate). It
//     measures the fraction of the reference corpus the parser resolves cleanly
//     enough to feed unattended identification, and fails below a documented
//     target. Cases the parser is known not to handle yet are pinned as
//     unresolved so the rate reports honest coverage, not an inflated number.
//
// The corpus is deliberately adversarial: it covers the hard filename classes
// incumbent servers handle — daily/date-stamped shows, absolute-numbered anime,
// contiguous and non-contiguous multi-episode ranges, movies nested in show
// folders, and scene-noise token stacks — including the cases the current
// stdlib-only parser (parse.go) does NOT yet resolve, so a future parser
// improvement shows up as a rising auto-match-rate, and a regression as a
// falling one.

// parseCase is one adversarial fixture: an input path plus the parser output it
// must produce. want is the expected ParsedMedia when wantOK is true; when
// wantOK is false the parser is expected to refuse (return ok==false), which is
// a *documented limitation*, not a passing resolution — it counts against the
// auto-match-rate.
type parseCase struct {
	name   string
	path   string
	wantOK bool
	want   sdk.ParsedMedia
	// resolvable marks whether this case counts as an auto-match for the rate
	// metric. A case is a clean auto-match only when the parser produced the
	// exact expected fields (title present, class right, ordinals right). Cases
	// pinned as documented parser gaps (wrong/empty title, dropped date) set
	// resolvable=false even if ok==true, so the rate reflects real usability.
	resolvable bool
}

// parseCorpus is the full parser regression corpus. It is
// shared by the per-case regression gate and the auto-match-rate metric so both
// grade against identical fixtures.
func parseCorpus() []parseCase {
	return []parseCase{
		// --- Scene-noise token stacks: title/year survive a deep noise tail. ---
		{
			name:       "scene-noise-movie-year",
			path:       "Interstellar.2014.1080p.BluRay.x264.YIFY.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Interstellar", Class: "movie", Year: 2014},
			resolvable: true,
		},
		{
			name:       "scene-noise-stacked-tokens",
			path:       "Dune.2021.2160p.BluRay.x265.HDR.DTS-HD.MA.7.1.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Dune", Class: "movie", Year: 2021},
			resolvable: true,
		},
		{
			name:       "scene-noise-episode-title-and-noise",
			path:       "Sherlock.S02E01.A.Scandal.in.Belgravia.1080p.HDTV.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Sherlock", Class: "episode", Sequence: "2", Index: 1},
			resolvable: true,
		},
		{
			name: "noise-word-inside-title-with-year",
			path: "Le.Cercle.Internal.2020.x.mkv",
			// "Internal" is a release-noise token, but a year already bounds the
			// title, so the noise cut must not apply and truncate a real title
			// word -- the noise cut only fires when no year/episode cut was found.
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Le Cercle Internal", Class: "movie", Year: 2020},
			resolvable: true,
		},
		{
			name: "noise-word-starts-title-with-year",
			path: "Extended.Family.2020.mkv",
			// "Extended" is a release-noise token AND the first word of the real
			// title; with a year present the noise cut must not fire (it would
			// otherwise empty the title and fail the parse entirely).
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Extended Family", Class: "movie", Year: 2020},
			resolvable: true,
		},
		{
			name: "noise-cut-applies-with-no-year",
			path: "Cool.Movie.PROPER.mkv",
			// No year/episode token bounds the title, so the noise cut still
			// applies as before: "PROPER" truncates the title.
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Cool Movie", Class: "movie"},
			resolvable: true,
		},
		{
			name: "noise-cut-before-release-group",
			path: "Cool.Movie.PROPER-GRP.mkv",
			// A release group hyphenated onto the last tag is not a title
			// word: "PROPER" still ends the title.
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Cool Movie", Class: "movie"},
			resolvable: true,
		},
		{
			name:       "strong-release-word-before-resolution",
			path:       "Cool.Movie.PROPER.1080p.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Cool Movie", Class: "movie"},
			resolvable: true,
		},

		// --- Movie nested in a show/library folder: only the basename is read. ---
		{
			name:       "movie-in-titled-folder",
			path:       "Movies/The Matrix (1999)/The.Matrix.1999.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "The Matrix", Class: "movie", Year: 1999},
			resolvable: true,
		},
		{
			name:       "episode-in-show-and-season-folder",
			path:       "Breaking.Bad/Season 01/Breaking.Bad.S01E01.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Breaking Bad", Class: "episode", Sequence: "1", Index: 1},
			resolvable: true,
		},

		// --- Multi-episode ranges: contiguous and non-contiguous. ---
		{
			name:   "contiguous-multi-episode-range",
			path:   "Show.Name.S01E01-E02-E03.mkv",
			wantOK: true,
			// The range is the first and the LAST E-token of the chain, however
			// many tokens it holds: E01-E02-E03 is episodes 1 to 3.
			want:       sdk.ParsedMedia{Title: "Show Name", Class: "episode", Sequence: "1", Index: 1, IndexEnd: 3},
			resolvable: true,
		},
		{
			name:   "non-contiguous-multi-episode",
			path:   "Show.Name.S02E05E07.mkv",
			wantOK: true,
			// Adjacent E-tokens with no separator: Index=5, IndexEnd=7.
			want:       sdk.ParsedMedia{Title: "Show Name", Class: "episode", Sequence: "2", Index: 5, IndexEnd: 7},
			resolvable: true,
		},

		// --- Long-form and shorthand episode tokens. ---
		{
			name:       "long-form-season-episode-words",
			path:       "Some.Show.Season 2 Episode 5.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Some Show", Class: "episode", Sequence: "2", Index: 5},
			resolvable: true,
		},
		{
			name:       "xXYY-shorthand-title-before",
			path:       "Some.Show.2x05.Great.Episode.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Some Show", Class: "episode", Sequence: "2", Index: 5},
			resolvable: true,
		},
		{
			name:       "adjacent-two-episode-file",
			path:       "Pioneer One - S01E01E02 - Earthfall.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Pioneer One", Class: "episode", Sequence: "1", Index: 1, IndexEnd: 2},
			resolvable: true,
		},
		{
			name:       "multi-episode-repeated-season",
			path:       "Show Name - S01E01-S01E02 - Pilot.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Show Name", Class: "episode", Sequence: "1", Index: 1, IndexEnd: 2},
			resolvable: true,
		},
		{
			name:       "multi-episode-xXYY-range",
			path:       "Show Name - 1x01-1x02 - Pilot.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Show Name", Class: "episode", Sequence: "1", Index: 1, IndexEnd: 2},
			resolvable: true,
		},
		{
			name:       "multi-episode-xXYY-three-token-range",
			path:       "Show.Name.1x01-1x02-1x03.Pilot.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Show Name", Class: "episode", Sequence: "1", Index: 1, IndexEnd: 3},
			resolvable: true,
		},
		{
			name: "cross-season-chain-is-not-a-range",
			path: "Show.S01E01-S02E03.Pilot.mkv",
			// A repeated season that differs makes the chain span seasons:
			// season 2's episode is not part of a season 1 range.
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Show", Class: "episode", Sequence: "1", Index: 1},
			resolvable: true,
		},
		{
			name:       "cross-season-xXYY-chain-is-not-a-range",
			path:       "Show.1x01-2x02-2x03.Pilot.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Show", Class: "episode", Sequence: "1", Index: 1},
			resolvable: true,
		},
		{
			name:       "dash-prefixed-xXYY-range",
			path:       "Pioneer One - 1x01-1x02 - Earthfall.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Pioneer One", Class: "episode", Sequence: "1", Index: 1, IndexEnd: 2},
			resolvable: true,
		},
		{
			name:       "dash-prefixed-multi-episode-chain",
			path:       "Pioneer One - S01E01-E02-E03 - Earthfall.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Pioneer One", Class: "episode", Sequence: "1", Index: 1, IndexEnd: 3},
			resolvable: true,
		},

		// --- Separators around the episode token never leak into the series
		// title: it is the card title, the provider search query and the
		// scorer's input (the "Pioneer One -" defect). ---
		{
			name:       "dash-separated-episode-title",
			path:       "Pioneer One - S01E01 - Earthfall.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Pioneer One", Class: "episode", Sequence: "1", Index: 1},
			resolvable: true,
		},
		{
			name:       "dotted-scene-episode-title-and-group",
			path:       "Pioneer.One.S01E01.Earthfall.1080p.x264-GRP.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Pioneer One", Class: "episode", Sequence: "1", Index: 1},
			resolvable: true,
		},
		{
			name:       "dash-separated-xXYY",
			path:       "Pioneer One - 1x01 - Earthfall.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Pioneer One", Class: "episode", Sequence: "1", Index: 1},
			resolvable: true,
		},
		{
			name:       "bare-series-and-token",
			path:       "Pioneer One S01E01.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Pioneer One", Class: "episode", Sequence: "1", Index: 1},
			resolvable: true,
		},
		{
			name:       "underscore-and-dash-separators",
			path:       "Show_Name_-_S01E01_-_Episode_Title.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Show Name", Class: "episode", Sequence: "1", Index: 1},
			resolvable: true,
		},
		{
			name:       "spaced-season-episode-token",
			path:       "Show.S01.E02.Title.Here.2160p.WEB.h265.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Show", Class: "episode", Sequence: "1", Index: 2},
			resolvable: true,
		},
		{
			name:       "release-group-bracket-before-series",
			path:       "[Group] Pioneer One - S01E03 - Alone in the Night [1080p].mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Pioneer One", Class: "episode", Sequence: "1", Index: 3},
			resolvable: true,
		},
		{
			name:       "year-before-episode-token",
			path:       "Doctor.Who.2005.S01E01.Rose.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Doctor Who", Class: "episode", Year: 2005, Sequence: "1", Index: 1},
			resolvable: true,
		},
		{
			name:       "movie-dash-before-year",
			path:       "Movie Title - 1999.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Movie Title", Class: "movie", Year: 1999},
			resolvable: true,
		},

		// --- A file with no series name of its own takes the folder above its
		// season folder; nothing else is trusted as a series name. ---
		{
			name:       "series-name-from-folder-above-season",
			path:       "Pioneer One (2010)/Season 01/S01E01 - Earthfall.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Pioneer One", Class: "episode", Year: 2010, Sequence: "1", Index: 1},
			resolvable: true,
		},
		{
			name:       "xXYY-first-in-season-folder",
			path:       "Pioneer One/Season 1/1x02 - The Man From Mars.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Pioneer One", Class: "episode", Sequence: "1", Index: 2},
			resolvable: true,
		},
		// The parent of a loose file is as likely a library root ("TV") as a
		// series, so with no season folder the parser still refuses.
		{
			name:       "no-series-name-outside-season-folder",
			path:       "TV/S01E01 - Earthfall.mkv",
			wantOK:     false,
			resolvable: false,
		},

		// --- A release tag that is also a word ends a title only in front of
		// another release tag, or (for the unambiguous ones) as the last word. ---
		{
			name:       "release-word-is-the-title-no-year",
			path:       "Extended.Family.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Extended Family", Class: "movie"},
			resolvable: true,
		},
		{
			name:       "release-word-ends-title-no-year",
			path:       "Charlotte's Web.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Charlotte's Web", Class: "movie"},
			resolvable: true,
		},
		{
			name: "release-word-before-resolution-stays-in-title",
			path: "Charlotte's.Web.1080p.mkv",
			// A resolution alone after "Web" does not make it a tag: titles
			// run straight into a resolution.
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Charlotte's Web", Class: "movie"},
			resolvable: true,
		},
		{
			name:       "release-word-before-resolution-and-group",
			path:       "Charlotte's.Web.1080p-GRP.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Charlotte's Web", Class: "movie"},
			resolvable: true,
		},
		{
			name: "release-word-before-resolution-and-codec-stays-in-title",
			path: "Charlotte's.Web.1080p.x264.mkv",
			// The look-ahead stops at the resolution: the codec after it does
			// not make "Web" a tag.
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Charlotte's Web", Class: "movie"},
			resolvable: true,
		},
		{
			name: "two-hyphenated-words-are-the-title",
			path: "Uncut-Gems.mkv",
			// "Uncut" is a release word, but the hyphenated second word is
			// not a release group: dropping it would empty the title.
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Uncut-Gems", Class: "movie"},
			resolvable: true,
		},
		{
			name:       "hyphenated-resolution-is-still-a-tag",
			path:       "Movie-1080p.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Movie", Class: "movie"},
			resolvable: true,
		},
		{
			name:       "hyphenated-tag-ends-title-no-year",
			path:       "Cool.Movie.WEB-DL.DDP5.1.H.264-GRP.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Cool Movie", Class: "movie"},
			resolvable: true,
		},
		{
			name:       "roman-numeral-and-parenthetical-year",
			path:       "Frozen II (2019).mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Frozen II", Class: "movie", Year: 2019},
			resolvable: true,
		},

		// --- Documented parser gaps: pinned so a fix RAISES the auto-match-rate. ---

		// Daily/date-stamped show: the AIR DATE (2024.01.15) is the real episode
		// key, but the stdlib parser has no date-episode rule, so it reads the
		// leading 2024 as a movie year and drops the day. Pinned as a movie parse
		// with the date lost — a documented limitation, NOT an auto-match.
		{
			name:       "daily-show-date-stamped",
			path:       "The.Daily.Show.2024.01.15.Guest.1080p.WEB.mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "The Daily Show", Class: "movie", Year: 2024},
			resolvable: false,
		},
		// Absolute-numbered anime with fansub group + resolution brackets: there
		// is no SxxEyy/xXYY/year token, so only the bracketed resolution is cut
		// as release noise and the group + absolute number leak into the title.
		// Pinned as the current (unusable) parse so a future absolute-numbering
		// rule shows up as a rising rate.
		{
			name:       "absolute-anime-fansub-brackets",
			path:       "[HorribleSubs] One Piece - 1015 [720p].mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "[HorribleSubs] One Piece - 1015", Class: "movie"},
			resolvable: false,
		},
		{
			name:       "absolute-anime-dash-number",
			path:       "Naruto - 001 [1080p].mkv",
			wantOK:     true,
			want:       sdk.ParsedMedia{Title: "Naruto - 001", Class: "movie"},
			resolvable: false,
		},
		// Episode token BEFORE the title, outside a season folder: the xXYY
		// match sits at index 0, so the title cut is empty and the parser
		// refuses. Documented limitation.
		{
			name:       "episode-token-before-title",
			path:       "3x07.Some.Show.mkv",
			wantOK:     false,
			resolvable: false,
		},
		// Pure release noise, no title at all: correct refusal (fail-closed).
		// A refusal here is the RIGHT answer, but it still is not an auto-match,
		// so it counts against the rate — the corpus intentionally mixes in
		// genuinely unresolvable inputs so the target is not gamed to 1.0.
		{
			name:       "noise-only-no-title",
			path:       "1080p.x264.mkv",
			wantOK:     false,
			resolvable: false,
		},
	}
}

// ParseAutoMatchTarget is the documented auto-match-rate floor for the parser
// against this reference corpus: at least this fraction of the corpus must parse
// into a clean, auto-appliable ParsedMedia (correct title, class, and ordinals).
//
// The corpus holds 36 cases; 30 currently resolve cleanly, for an observed rate
// of 0.83. The target sits at 0.80 — below the current rate so a single new
// parser regression (dropping to 29/36 = 0.81) still passes but two (28/36 =
// 0.78) reds this test, and ABOVE it enough that improving the documented gaps
// (date-stamped shows, absolute-numbered anime) only ever moves the rate up.
// It is deliberately not 1.0: six corpus cases are known parser limitations or
// genuinely unresolvable noise, so a 1.0 target would be dishonest. Raising this
// floor is the acceptance signal for a future parser-coverage improvement.
const ParseAutoMatchTarget = 0.80

// TestParseCorpus is the per-case parser regression gate: every adversarial
// fixture pins the parser's exact output (or documented refusal), so any change
// to parse.go that shifts a field is caught here.
func TestParseCorpus(t *testing.T) {
	parser := filmtv.New()
	for _, c := range parseCorpus() {
		t.Run(c.name, func(t *testing.T) {
			got, ok := parser.Parse(c.path, sdk.ParseProbe{})
			if ok != c.wantOK {
				t.Fatalf("Parse(%q) ok = %v, want %v (got %+v)", c.path, ok, c.wantOK, got)
			}
			if !c.wantOK {
				return // a documented refusal has no fields to compare.
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("Parse(%q) =\n  %+v\nwant\n  %+v", c.path, got, c.want)
			}
		})
	}
}

// TestParseCorpusAutoMatchRate scores the aggregate auto-match-rate — the
// fraction of the corpus the parser resolves into a clean, auto-appliable parse
// — and fails below ParseAutoMatchTarget. A resolvable case counts only when the
// parser produced exactly the expected fields; documented gaps and unresolvable
// noise count against the rate, so the metric tracks real parser coverage.
func TestParseCorpusAutoMatchRate(t *testing.T) {
	parser := filmtv.New()
	corpus := parseCorpus()
	resolved := 0
	for _, c := range corpus {
		got, ok := parser.Parse(c.path, sdk.ParseProbe{})
		autoMatched := ok == c.wantOK && (!c.wantOK || reflect.DeepEqual(got, c.want))
		if c.resolvable && autoMatched {
			resolved++
		}
	}
	total := len(corpus)
	if total == 0 {
		t.Fatal("parser corpus is empty; auto-match-rate is undefined")
	}
	rate := float64(resolved) / float64(total)
	t.Logf("parser auto-match-rate: %.4f (%d/%d clean parses) — target %.4f",
		rate, resolved, total, ParseAutoMatchTarget)
	if rate < ParseAutoMatchTarget {
		t.Errorf("parser auto-match-rate %.4f (%d/%d) is below the %.4f target",
			rate, resolved, total, ParseAutoMatchTarget)
	}
}

// TestParseCaseFoldOffsetSafety pins that the parser folds case with an
// offset-preserving ASCII fold (parse.go asciiLower), never strings.ToLower:
// some Unicode case mappings change byte length (U+023A "Ⱥ" grows a byte when
// folded, U+212A "K" shrinks by two), so match indices found on a
// strings.ToLower copy drift off the original name — the grow side indexed
// past len(name) and panicked the title slice, the shrink side silently cut
// the title at the wrong byte.
func TestParseCaseFoldOffsetSafety(t *testing.T) {
	parser := filmtv.New()
	cases := []struct {
		name string
		path string
		want sdk.ParsedMedia
	}{
		{
			// Seven U+023A runes fold 2→3 bytes each under strings.ToLower,
			// pushing the parenthesized-year match index past len(name):
			// previously an index-out-of-range panic in findYear.
			name: "grow-fold-parenthesized-year",
			path: "ȺȺȺȺȺȺȺ (2020).mkv",
			want: sdk.ParsedMedia{Title: "ȺȺȺȺȺȺȺ", Class: "movie", Year: 2020},
		},
		{
			// Two U+212A Kelvin signs fold 3→1 byte each under
			// strings.ToLower, dragging the year cut two bytes left per rune:
			// previously the title was sliced mid-string.
			name: "shrink-fold-year-offset",
			path: "KK Movie 2003.mkv",
			want: sdk.ParsedMedia{Title: "KK Movie", Class: "movie", Year: 2003},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parser.Parse(tc.path, sdk.ParseProbe{})
			if !ok {
				t.Fatalf("Parse(%q) ok = false, want true", tc.path)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Parse(%q) =\n  %+v\nwant\n  %+v", tc.path, got, tc.want)
			}
		})
	}
}

// TestParseEpisodeTitle pins the episode's own title, which the parser emits as
// the plugin-declared EpisodeTitleField Media field (the series name stays in
// ParsedMedia.Title). want "" means no field is emitted.
func TestParseEpisodeTitle(t *testing.T) {
	var _ sdk.MediaFieldParser = filmtv.New()
	parser := filmtv.New()
	cases := []struct {
		name string
		path string
		want string
	}{
		{"dash-separated", "Pioneer One - S01E01 - Earthfall.mkv", "Earthfall"},
		{"dotted-before-release-tags", "Pioneer.One.S01E01.Earthfall.1080p.x264-GRP.mkv", "Earthfall"},
		{"xXYY", "Pioneer One - 1x01 - Earthfall.mkv", "Earthfall"},
		{"multi-word-dotted", "Sherlock.S02E01.A.Scandal.in.Belgravia.1080p.HDTV.mkv", "A Scandal in Belgravia"},
		{"after-multi-episode-chain", "Pioneer One - S01E01E02 - Earthfall & The Man From Mars.mkv", "Earthfall & The Man From Mars"},
		{"after-xXYY-range", "Show Name - 1x01-1x02 - Pilot.mkv", "Pilot"},
		{"after-three-token-xXYY-range", "Show.Name.1x01-1x02-1x03.Pilot.mkv", "Pilot"},
		{"dash-prefixed-sxxeyy-dotted-title", "Pioneer One - S01E02 - The.Man.From.Mars.mkv", "The Man From Mars"},
		{"dash-prefixed-xXYY-two-digit-season", "Pioneer One - 10x01 - Earthfall.mkv", "Earthfall"},
		{"underscores", "Show_Name_-_S01E01_-_Episode_Title.mkv", "Episode Title"},
		{"parenthesised-part-kept", "Show - S01E01 - The Best of Both Worlds (Part 1).mkv", "The Best of Both Worlds (Part 1)"},
		{"parenthesised-tags-dropped", "Show - S01E01 - Title (1080p WEB-DL).mkv", "Title"},
		{"bracketed-tags-dropped", "[Group] Pioneer One - S01E03 - Alone in the Night [1080p][ABCD1234].mkv", "Alone in the Night"},
		{"series-from-folder", "Pioneer One (2010)/Season 01/S01E01 - Earthfall.mkv", "Earthfall"},
		// A release tag that is also a word stays in the title until a real
		// release tag follows it.
		{"release-word-inside-title", "Show.S01E02.Internal.Affairs.720p.HDTV.x264-GRP.mkv", "Internal Affairs"},
		{"release-word-then-same-tag", "Show.S01E02.The.Web.WEB-DL.DDP5.1.H.264-GRP.mkv", "The Web"},
		{"release-word-then-resolution", "Show.S01E01.The.Web.1080p.mkv", "The Web"},
		{"release-word-then-resolution-and-codec", "Show.S01E01.The.Web.1080p.x264.mkv", "The Web"},
		{"after-cross-season-chain", "Show.S01E01-S02E03.Pilot.mkv", "Pilot"},
		{"after-cross-season-xXYY-chain", "Show.1x01-2x02-2x03.Pilot.mkv", "Pilot"},
		{"strong-release-word-before-group", "Show.S01E01.Pilot.PROPER-GRP.mkv", "Pilot"},
		{"title-word-that-starts-like-an-episode-token", "Show.S01E01.E20th.Century.mkv", "E20th Century"},
		// No episode title in the name: no field.
		{"token-only", "Pioneer One S01E01.mkv", ""},
		{"release-tags-only", "Show.S01E02.PROPER.720p.HDTV.x264-GRP.mkv", ""},
		{"long-form-token", "Some.Show.Season 2 Episode 5.mkv", ""},
		// Not an episode, or refused: no field.
		{"movie", "The.Matrix.1999.1080p.mkv", ""},
		{"refused", "TV/S01E01 - Earthfall.mkv", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parser.ParseMediaFields(tc.path, sdk.ParseProbe{})
			if tc.want == "" {
				if got != nil {
					t.Fatalf("ParseMediaFields(%q) = %+v, want none", tc.path, got)
				}
				return
			}
			want := []sdk.ParsedTextField{{Name: filmtv.EpisodeTitleField, Text: tc.want}}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("ParseMediaFields(%q) = %+v, want %+v", tc.path, got, want)
			}
		})
	}
}

// TestEpisodeTitleFieldDeclared pins that the field the parser emits is one the
// bundle declares: Core persists only declared Media fields.
func TestEpisodeTitleFieldDeclared(t *testing.T) {
	for _, field := range filmtv.Bundle().MediaFields {
		if field.Name == filmtv.EpisodeTitleField {
			if field.Type != "text" || !field.Searchable {
				t.Fatalf("%s is declared as %+v, want searchable text", filmtv.EpisodeTitleField, field)
			}
			return
		}
	}
	t.Fatalf("bundle declares no %s Media field", filmtv.EpisodeTitleField)
}
