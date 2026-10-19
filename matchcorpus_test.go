package filmtv_test

import (
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"
	"github.com/beamfall/plugin-sdk/sdktest"

	filmtv "github.com/beamfall/plugin-filmtv"
)

// TestMatchCorpus seeds the Film & TV plugin's matching-regression corpus via
// the SDK's sdktest.RunMatchCorpus harness: one MatchFamily per hostile-input
// class this plugin must not silently mis-resolve. Matching is fail-closed, so
// the regression a family guards is a *wrong match* — a WantID=="" case routing to the unmatched queue is the intended,
// passing outcome, not a failure.
func TestMatchCorpus(t *testing.T) {
	sdktest.RunMatchCorpus(t, filmtv.New(), filmtv.NewScorer(), corpusFamilies(t))
}

// corpusFamilies is the plugin's full matching-regression corpus,
// shared by the per-case harness (TestMatchCorpus) and the corpus-wide precision
// gate (TestMatchCorpusPrecision) so both grade against exactly the same fixtures.
func corpusFamilies(t *testing.T) []sdktest.MatchFamily {
	return []sdktest.MatchFamily{
		providerIDHintFamily(t),
		sceneNamingFamily(),
		multiEpisodeFamily(),
		splitMovieFamily(),
		animeAbsoluteNumberingFamily(),
		nonLatinTitleFamily(),
		yearDisambiguationFamily(),
		falseFriendTitleFamily(),
		popularityTieBreakFamily(),
	}
}

// MatchPrecisionTarget is the hard precision floor the whole corpus must clear:
// of every case where the fail-closed pipeline commits to a candidate (emits a
// non-empty match), at least this fraction must be the *correct* candidate.
// Precision here is the wrong-match regression expressed as an
// aggregate rate rather than a per-case assertion — a false positive is either a
// wrong ID on a should-match file or any match at all on a should-be-unmatched
// file (WantID==""). The corpus is verified to resolve every positive correctly
// (precision 1.0); the target sits just below that so a *single* new mis-resolve
// among the 18 positive cases (which would drop precision to ~0.94) reds this
// test. It is deliberately not 1.0, so the metric — not an incidental exact
// count — is what the gate asserts.
const MatchPrecisionTarget = 0.95

// TestMatchCorpusPrecision states a corpus-wide precision target and fails below
// it. It reuses the same fixtures TestMatchCorpus grades per case,
// but instead of asserting each case in isolation it measures aggregate match
// precision — correct positive matches over all positive matches emitted — and
// requires it to clear MatchPrecisionTarget. This guards against a scoring change
// that trades a broad accuracy regression for still passing every individual
// pinned case: the rate catches drift the per-case corpus could miss as it grows.
func TestMatchCorpusPrecision(t *testing.T) {
	parser, scorer := filmtv.New(), filmtv.NewScorer()
	truePositives, positives := 0, 0
	for _, fam := range corpusFamilies(t) {
		for _, c := range fam.Cases {
			gotID := resolveCorpusMatch(t, parser, scorer, c)
			if gotID == "" {
				continue // fail-closed to unmatched — not a positive match
			}
			positives++
			if gotID == c.WantID {
				truePositives++
			}
		}
	}
	if positives == 0 {
		t.Fatal("corpus emitted no positive matches; precision is undefined — check the fixtures")
	}
	precision := float64(truePositives) / float64(positives)
	if precision < MatchPrecisionTarget {
		t.Errorf("corpus match precision %.4f (%d/%d correct) is below the %.4f target",
			precision, truePositives, positives, MatchPrecisionTarget)
	}
}

func providerIDHintFamily(t *testing.T) sdktest.MatchFamily {
	t.Helper()
	root := t.TempDir()
	nfoDir := filepath.Join(root, "The Matrix (1999) {tmdb-999}")
	nfoMedia := filepath.Join(nfoDir, "The.Matrix.1999.mkv")
	writeCorpusFile(t, nfoMedia, "")
	writeCorpusFile(t, filepath.Join(nfoDir, "movie.nfo"), `<movie><uniqueid type="imdb">tt0133093</uniqueid></movie>`)
	urlDir := filepath.Join(root, "URL only")
	urlMedia := filepath.Join(urlDir, "The.Matrix.1999.mkv")
	writeCorpusFile(t, urlMedia, "")
	writeCorpusFile(t, filepath.Join(urlDir, "movie.nfo"), "https://www.themoviedb.org/movie/603")

	return sdktest.MatchFamily{
		Name: "provider-id-hints",
		Cases: []sdktest.MatchCase{
			{Name: "tmdb-folder-tag-zero-search", Path: "The Matrix (1999) {tmdb-603}/The.Matrix.1999.mkv", Candidates: []sdk.Candidate{{ID: "603", Title: "The Matrix"}}, WantID: "603", HintProvider: "tmdb"},
			{Name: "nfo-beats-conflicting-folder-tag", Path: nfoMedia, Candidates: []sdk.Candidate{{ID: "tt0133093", Title: "The Matrix"}, {ID: "999", Title: "The Matrix"}}, WantID: "tt0133093", HintProvider: "imdb"},
			{Name: "url-only-nfo-zero-search", Path: urlMedia, Candidates: []sdk.Candidate{{ID: "603", Title: "The Matrix"}}, WantID: "603", HintProvider: "tmdb"},
		},
	}
}

func writeCorpusFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// corpusAmbiguityMargin mirrors sdktest's unexported ambiguityMargin: the band
// below the top score inside which a second candidate makes the pick ambiguous.
const corpusAmbiguityMargin = 0.05

// resolveCorpusMatch runs the same parse → score → fail-closed pick the SDK's
// RunMatchCorpus grades against, returning the resolved candidate ID or "" for
// unmatched: the single candidate strictly above the floor wins; nothing above
// the floor, an ambiguous top band (a second candidate within
// corpusAmbiguityMargin of the best), or an exact-score tie without a unique
// popularity winner all route to unmatched. Popularity only breaks true
// equal-score ties at the top — it never promotes a lower score. Kept in
// lockstep with sdktest.pickMatch's documented rule.
func resolveCorpusMatch(t *testing.T, p sdk.FilenameParser, s sdk.MatchScorer, c sdktest.MatchCase) string {
	t.Helper()
	parsed, ok := p.Parse(c.Path, c.Probe)
	if !ok {
		return ""
	}
	if err := sdk.ValidateParseHints(parsed); err != nil {
		t.Fatalf("case %q: parser hint contract: %v", c.Name, err)
	}
	providers := make([]string, 0, len(parsed.ProviderIDs))
	for provider := range parsed.ProviderIDs {
		providers = append(providers, provider)
	}
	sort.Strings(providers)
	for _, provider := range providers {
		for _, candidate := range c.Candidates {
			if candidate.ID == parsed.ProviderIDs[provider] {
				return candidate.ID
			}
		}
	}
	floor := c.MinScore
	if floor == 0 {
		floor = sdktest.DefaultMatchThreshold
	}
	type scoredCandidate struct {
		candidate sdk.Candidate
		score     float64
	}
	var scored []scoredCandidate
	best := scoredCandidate{score: floor}
	for _, cand := range c.Candidates {
		score := s.Score(cand, parsed)
		if math.IsNaN(score) || score < 0 || score > 1 {
			t.Fatalf("case %q: scorer returned %v for candidate %q, out of contract range [0,1]", c.Name, score, cand.ID)
		}
		entry := scoredCandidate{candidate: cand, score: score}
		scored = append(scored, entry)
		if score > best.score {
			best = entry
		}
	}
	if best.candidate.ID == "" {
		return ""
	}
	var band []scoredCandidate
	for _, entry := range scored {
		if best.score-entry.score < corpusAmbiguityMargin {
			band = append(band, entry)
		}
	}
	if len(band) < 2 {
		return best.candidate.ID
	}
	var exactTop []scoredCandidate
	for _, entry := range band {
		if entry.score == best.score {
			exactTop = append(exactTop, entry)
		}
	}
	if len(exactTop) < 2 {
		return ""
	}
	popular := exactTop[0]
	popularityTied := false
	popularityEligible := popular.candidate.Popularity > 0
	for _, entry := range exactTop[1:] {
		eligible := entry.candidate.Popularity > 0
		switch {
		case eligible && (!popularityEligible || entry.candidate.Popularity > popular.candidate.Popularity):
			popular, popularityTied, popularityEligible = entry, false, true
		case eligible && popularityEligible && entry.candidate.Popularity == popular.candidate.Popularity:
			popularityTied = true
		}
	}
	if !popularityEligible || popularityTied {
		return ""
	}
	return popular.candidate.ID
}

// sceneNamingFamily covers scene-release episode tokens (XxYY shorthand,
// SxxEyy) resolving against the right series and not a same-vocabulary sibling.
func sceneNamingFamily() sdktest.MatchFamily {
	candidates := []sdk.Candidate{
		{ID: "some-show", Title: "Some Show"},
		{ID: "another-series", Title: "Another Series"},
	}
	return sdktest.MatchFamily{
		Name: "scene-naming",
		Cases: []sdktest.MatchCase{
			{Name: "xxyy-shorthand", Path: "Some.Show.2x05.Great.Episode.mkv", Candidates: candidates, WantID: "some-show"},
			{Name: "no-candidate-clears-floor", Path: "Some.Show.2x05.Great.Episode.mkv",
				Candidates: []sdk.Candidate{{ID: "unrelated", Title: "Totally Unrelated Title"}}, WantID: ""},
			{Name: "candidate-episode-ordinals-break-same-title-fallback", Path: "Some.Show.S02E05.mkv",
				Candidates: []sdk.Candidate{
					{ID: "series-level", Title: "Some Show"},
					{ID: "episode-level", Title: "Some Show", Sequence: 2, Index: 5},
				}, WantID: "episode-level"},
		},
	}
}

// multiEpisodeFamily covers SxxEyy-Ezz multi-episode range files resolving to
// the right series from the episode-range parse alone.
func multiEpisodeFamily() sdktest.MatchFamily {
	candidates := []sdk.Candidate{
		{ID: "show-name", Title: "Show Name"},
		{ID: "different-series", Title: "Different Series"},
	}
	return sdktest.MatchFamily{
		Name: "multi-episode",
		Cases: []sdktest.MatchCase{
			{Name: "three-episode-range", Path: "Show.Name.S01E01-E02-E03.mkv", Candidates: candidates, WantID: "show-name"},
			{Name: "unparseable-noise-only", Path: "1080p.x264.mkv", Candidates: candidates, WantID: ""},
		},
	}
}

// splitMovieFamily covers a movie released as Part1/Part2 files: the parser
// carries the "Part" suffix into the title (no dedicated PartGroup parse yet),
// so the fixture pins that containment scoring still resolves both parts to
// the same candidate and not an unrelated title.
func splitMovieFamily() sdktest.MatchFamily {
	candidates := []sdk.Candidate{
		{ID: "grand-adventure", Title: "The Grand Adventure"},
		{ID: "unrelated", Title: "City Lights"},
	}
	return sdktest.MatchFamily{
		Name: "split-movie",
		Cases: []sdktest.MatchCase{
			{Name: "part-1", Path: "The.Grand.Adventure.Part1.mkv", Candidates: candidates, WantID: "grand-adventure"},
			{Name: "part-2", Path: "The.Grand.Adventure.Part.2.mkv", Candidates: candidates, WantID: "grand-adventure"},
		},
	}
}

// animeAbsoluteNumberingFamily covers bare-number episode filenames (no SxxEyy
// token at all — the parser resolves them as a plain title match). A clean
// series name resolves; a title-only match that's equally contained in two
// franchise-sibling candidates (e.g. "Naruto" vs "Naruto Shippuden") is a
// genuine ambiguity and must fail closed to unmatched, not guess.
func animeAbsoluteNumberingFamily() sdktest.MatchFamily {
	return sdktest.MatchFamily{
		Name: "anime-absolute-numbering",
		Cases: []sdktest.MatchCase{
			{
				Name: "absolute-number-clean-match",
				Path: "Naruto - 001 [1080p].mkv",
				Candidates: []sdk.Candidate{
					{ID: "naruto", Title: "Naruto"},
					{ID: "bleach", Title: "Bleach"},
				},
				WantID: "naruto",
			},
			{
				Name: "franchise-sibling-tie-is-unmatched",
				Path: "Naruto.Shippuden.052.mkv",
				Candidates: []sdk.Candidate{
					{ID: "naruto-shippuden", Title: "Naruto Shippuden"},
					{ID: "naruto", Title: "Naruto"},
				},
				WantID: "",
			},
		},
	}
}

// nonLatinTitleFamily covers CJK and Hangul titles: title normalization is
// Unicode-letter-aware, so exact and near-exact non-Latin titles resolve like
// any other script, and an unrelated non-Latin candidate does not.
func nonLatinTitleFamily() sdktest.MatchFamily {
	return sdktest.MatchFamily{
		Name: "non-latin-titles",
		Cases: []sdktest.MatchCase{
			{
				Name: "japanese-title-no-year",
				Path: "進撃の巨人.01.mkv",
				Candidates: []sdk.Candidate{
					{ID: "aot", Title: "進撃の巨人"},
					{ID: "other", Title: "鬼滅の刃"},
				},
				WantID: "aot",
			},
			{
				Name: "korean-title-with-year",
				Path: "기생충.2019.1080p.BluRay.mkv",
				Candidates: []sdk.Candidate{
					{ID: "parasite", Title: "기생충", Year: 2019},
					{ID: "other", Title: "설국열차", Year: 2013},
				},
				WantID: "parasite",
			},
		},
	}
}

// yearDisambiguationFamily is the corpus's hard "same title, different year"
// class: remakes and franchise entries that share an identical
// title, where the parsed year is the only signal separating them. It exercises
// the scorer's year-corroboration weight (score.go yearScore) as the deciding
// factor, and — critically — pins the fail-closed outcome when the filename
// carries no year at all: two identically-titled candidates then both score a
// perfect title-only 1.0, tie, and must route to unmatched rather than let the
// pipeline silently pick the first (a wrong-match regression; the fail-closed
// baseline routes ties to the unmatched queue).
func yearDisambiguationFamily() sdktest.MatchFamily {
	duneRemakes := []sdk.Candidate{
		{ID: "dune-1984", Title: "Dune", Year: 1984},
		{ID: "dune-2021", Title: "Dune", Year: 2021},
	}
	return sdktest.MatchFamily{
		Name: "year-disambiguation",
		Cases: []sdktest.MatchCase{
			// Filename year picks the right remake: exact-year candidate scores a
			// perfect 1.0, the wrong-year twin only its title weight (0.7).
			{Name: "remake-year-picks-original", Path: "Dune.1984.1080p.BluRay.x264.mkv", Candidates: duneRemakes, WantID: "dune-1984"},
			{Name: "remake-year-picks-reboot", Path: "Dune.2021.2160p.mkv", Candidates: duneRemakes, WantID: "dune-2021"},
			// No year token at all: both same-title candidates tie at title-only
			// 1.0, so the pick is a genuine ambiguity and must fail closed.
			{Name: "no-year-same-title-tie-is-unmatched", Path: "Dune.1080p.BluRay.mkv", Candidates: duneRemakes, WantID: ""},
			// Release-vs-air-year drift (±1): a one-year-off candidate still clears
			// on the half-weight year match and beats a far-off same-title sibling.
			{Name: "air-year-drift-still-resolves", Path: "The.Show.2011.1080p.mkv",
				Candidates: []sdk.Candidate{
					{ID: "the-show-2010", Title: "The Show", Year: 2010},
					{ID: "the-show-1999", Title: "The Show", Year: 1999},
				}, WantID: "the-show-2010"},
		},
	}
}

// falseFriendTitleFamily is the corpus's substring/containment "false friend"
// class: candidates whose titles overlap by containment or shared
// tokens but are not the same work. It pins that an exact match wins over a
// merely-contained franchise sibling (a sequel title must not steal the base
// film), and that a containment false friend ("Office Space" vs "The Office")
// stays below the fail-closed floor rather than being accepted on the 0.85
// containment bump alone.
func falseFriendTitleFamily() sdktest.MatchFamily {
	return sdktest.MatchFamily{
		Name: "false-friend-titles",
		Cases: []sdktest.MatchCase{
			// Exact title (1.0) beats the contained sequel title (0.85 containment,
			// discounted further by the conflicting year) — the sequel must not
			// capture a file that names only the base title.
			{Name: "sequel-does-not-steal-base-title", Path: "Toy.Story.1995.mkv",
				Candidates: []sdk.Candidate{
					{ID: "toy-story", Title: "Toy Story", Year: 1995},
					{ID: "toy-story-3", Title: "Toy Story 3", Year: 2010},
				}, WantID: "toy-story"},
			// The parsed title carries the sequel tokens ("Dune Part Two"): the
			// exact sequel candidate wins over the contained base-title film.
			{Name: "sequel-title-resolves-to-sequel", Path: "Dune.Part.Two.2024.2160p.mkv",
				Candidates: []sdk.Candidate{
					{ID: "dune-part-two", Title: "Dune Part Two", Year: 2024},
					{ID: "dune-2021", Title: "Dune", Year: 2021},
				}, WantID: "dune-part-two"},
			// Containment false friend: "The Office" is contained in nothing here,
			// and its token overlap with "Office Space" lands well below the floor,
			// so the exact "Office Space" is the only candidate that clears.
			{Name: "containing-title-false-friend-below-floor", Path: "Office.Space.1999.mkv",
				Candidates: []sdk.Candidate{
					{ID: "office-space", Title: "Office Space", Year: 1999},
					{ID: "the-office", Title: "The Office"},
				}, WantID: "office-space"},
		},
	}
}

// popularityTieBreakFamily pins the harness's exact-tie-only popularity
// semantics (sdktest.pickMatch): two candidates with identical title AND year
// score an exact tie, and a unique Candidate.Popularity winner breaks it — but
// equal popularity leaves a genuine tie that must fail closed to unmatched.
// The scorer itself never reads Popularity (TestPopularityDoesNotChangeScore);
// this family exercises the ranking layer the corpus grades against.
func popularityTieBreakFamily() sdktest.MatchFamily {
	return sdktest.MatchFamily{
		Name: "popularity-tiebreak",
		Cases: []sdktest.MatchCase{
			// A duplicate catalog entry (same title, same year) is an exact
			// score tie; the unique popularity signal picks the canonical one.
			{Name: "exact-tie-unique-popularity-wins", Path: "Sintel.2010.1080p.mkv",
				Candidates: []sdk.Candidate{
					{ID: "sintel-dup", Title: "Sintel", Year: 2010, Popularity: 2},
					{ID: "sintel", Title: "Sintel", Year: 2010, Popularity: 80},
				}, WantID: "sintel"},
			// Equal popularity breaks nothing: the exact tie stays a genuine
			// ambiguity and routes to unmatched (popularity never guesses).
			{Name: "exact-tie-equal-popularity-is-unmatched", Path: "Sintel.2010.1080p.mkv",
				Candidates: []sdk.Candidate{
					{ID: "sintel-a", Title: "Sintel", Year: 2010, Popularity: 10},
					{ID: "sintel-b", Title: "Sintel", Year: 2010, Popularity: 10},
				}, WantID: ""},
		},
	}
}
