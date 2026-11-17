package filmtv_test

import (
	"math"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"

	filmtv "github.com/beamfall/plugin-filmtv"
)

// TestAutoApplyFailsClosed pins the fail-closed asymmetry — "wrong
// auto-application is worse than no match" — directly against the auto-apply
// bar (a Score at or above filmtv.AutoApplyThreshold auto-applies; below it the
// file routes to the Unmatched queue). It is the scorer-side complement to the
// pipeline's tie rule: for a *single* candidate there is no tie, so the bias
// toward Unmatched has to live in the score itself. The generic-title-no-year
// cases (Crash / Gladiator) must NOT auto-apply.
func TestAutoApplyFailsClosed(t *testing.T) {
	scorer := filmtv.NewScorer()
	cases := []struct {
		name          string
		candTitle     string
		candYear      int
		parsed        sdk.ParsedMedia
		wantAutoApply bool
	}{
		// Generic single candidate, no year to disambiguate: a mediocre title
		// match must fail closed to Unmatched. "Crash" is a well-known collision
		// (2004 drama vs 1996 Cronenberg); "Gladiator" (2000 vs 1992) likewise.
		// Here the parsed title only partially matches the candidate, so title
		// similarity is non-exact and — with no year — cannot clear the bar.
		{
			name:      "generic contained title, no year, does not auto-apply (Crash)",
			candTitle: "Crash Landing", candYear: 0,
			parsed:        sdk.ParsedMedia{Title: "Crash"},
			wantAutoApply: false,
		},
		{
			name:      "generic contained title, no year, does not auto-apply (Gladiator)",
			candTitle: "Gladiator Games", candYear: 0,
			parsed:        sdk.ParsedMedia{Title: "Gladiator"},
			wantAutoApply: false,
		},
		{
			name:      "partial token-overlap title, no year, does not auto-apply",
			candTitle: "The Great Escape", candYear: 0,
			parsed:        sdk.ParsedMedia{Title: "Great Adventure"},
			wantAutoApply: false,
		},
		// An exact title with no year is still a confident, unique match: it may
		// auto-apply alone. (A genuine same-title collision is caught upstream by
		// the pipeline's fail-closed tie rule, not here.)
		{
			name:      "exact title, no year, still auto-applies",
			candTitle: "Gladiator", candYear: 0,
			parsed:        sdk.ParsedMedia{Title: "Gladiator"},
			wantAutoApply: true,
		},
		// A non-exact title that IS corroborated by an exact year auto-applies —
		// the year is the disambiguation the no-year cases lacked.
		{
			name:      "contained title corroborated by exact year auto-applies",
			candTitle: "Gladiator", candYear: 2000,
			parsed:        sdk.ParsedMedia{Title: "Gladiator", Year: 2000},
			wantAutoApply: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cand := sdk.Candidate{Title: tc.candTitle, Year: tc.candYear}
			score := scorer.Score(cand, tc.parsed)
			if score < 0 || score > 1 {
				t.Fatalf("score %v out of contract range [0,1]", score)
			}
			gotAutoApply := score >= filmtv.AutoApplyThreshold
			if gotAutoApply != tc.wantAutoApply {
				t.Errorf("score=%.4f autoApply=%v, want autoApply=%v (threshold %.2f)",
					score, gotAutoApply, tc.wantAutoApply, filmtv.AutoApplyThreshold)
			}
		})
	}
}

// TestSeasonEpisodeAgreement pins the season/episode factor: when the parse and
// the candidate both carry ordinals, a disagreement must sink an otherwise
// full match below the auto-apply bar (wrong-episode is fail-closed), while a
// show-level candidate that carries no ordinals scores as an agreeing one.
func TestSeasonEpisodeAgreement(t *testing.T) {
	scorer := filmtv.NewScorer()
	// Exact title + exact year: title+year alone is 1.0.
	parsed := sdk.ParsedMedia{Title: "Some Show", Year: 2010, Sequence: "2", Index: 5}
	cases := []struct {
		name          string
		cand          sdk.Candidate
		wantAutoApply bool
	}{
		{
			name:          "matching season+episode still auto-applies",
			cand:          sdk.Candidate{Title: "Some Show", Year: 2010, Sequence: 2, Index: 5},
			wantAutoApply: true,
		},
		{
			name:          "wrong episode sinks below auto-apply",
			cand:          sdk.Candidate{Title: "Some Show", Year: 2010, Sequence: 2, Index: 6},
			wantAutoApply: false,
		},
		{
			name:          "wrong season sinks below auto-apply",
			cand:          sdk.Candidate{Title: "Some Show", Year: 2010, Sequence: 3, Index: 5},
			wantAutoApply: false,
		},
		{
			name:          "candidate carries no ordinals: tiny penalty, title+year carry",
			cand:          sdk.Candidate{Title: "Some Show", Year: 2010},
			wantAutoApply: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			score := scorer.Score(tc.cand, parsed)
			if score < 0 || score > 1 {
				t.Fatalf("score %v out of contract range [0,1]", score)
			}
			if got := score >= filmtv.AutoApplyThreshold; got != tc.wantAutoApply {
				t.Errorf("score=%.4f autoApply=%v, want %v", score, got, tc.wantAutoApply)
			}
		})
	}
	matching := scorer.Score(sdk.Candidate{Title: "Some Show", Year: 2010, Sequence: 2, Index: 5}, parsed)
	missing := scorer.Score(sdk.Candidate{Title: "Some Show", Year: 2010}, parsed)
	wrong := scorer.Score(sdk.Candidate{Title: "Some Show", Year: 2010, Sequence: 2, Index: 6}, parsed)
	if !(matching > missing && missing > wrong) {
		t.Errorf("ordinal ordering matching %.4f > missing %.4f > wrong %.4f not satisfied", matching, missing, wrong)
	}
	// A show-level candidate stays an exact title's near-certain match: it
	// only orders below the candidate whose episode agrees.
	exactNoYear := sdk.ParsedMedia{Title: "Some Show", Class: "episode", Sequence: "2", Index: 5}
	if got := scorer.Score(sdk.Candidate{Title: "Some Show"}, exactNoYear); got != 0.99 {
		t.Errorf("show-level exact title = %.4f, want 0.99", got)
	}
}

// TestPopularityDoesNotChangeScore pins that popularity is ranker-only: this
// scorer handles title/year/ordinal confidence, while core breaks exact score
// ties using Candidate.Popularity because it can see the whole candidate set.
func TestPopularityDoesNotChangeScore(t *testing.T) {
	scorer := filmtv.NewScorer()
	tieParsed := sdk.ParsedMedia{Title: "Matrix", Year: 1999}
	unpopular := scorer.Score(sdk.Candidate{Title: "The Matrix", Year: 1999, Popularity: 1}, tieParsed)
	popular := scorer.Score(sdk.Candidate{Title: "The Matrix", Year: 1999, Popularity: 500}, tieParsed)
	if popular != unpopular {
		t.Errorf("popularity changed scorer confidence: popular=%.6f unpopular=%.6f", popular, unpopular)
	}

	parsed := sdk.ParsedMedia{Title: "The Matrix", Year: 1999}
	right := scorer.Score(sdk.Candidate{Title: "The Matrix", Year: 1999, Popularity: 0}, parsed)
	wrongYearPopular := scorer.Score(sdk.Candidate{Title: "The Matrix", Year: 2010, Popularity: 1e6}, parsed)
	if !(right > wrongYearPopular) {
		t.Errorf("popularity overrode year: right(0 pop)=%.6f must beat wrong-year(1e6 pop)=%.6f", right, wrongYearPopular)
	}

	nearBoundary := scorer.Score(
		sdk.Candidate{Title: "The Matrix Revolutions", Year: 2000, Popularity: 1e9},
		sdk.ParsedMedia{Title: "Matrix", Year: 1999},
	)
	if nearBoundary >= filmtv.AutoApplyThreshold {
		t.Errorf("popularity rescued a below-bar match: %.6f >= %.2f", nearBoundary, filmtv.AutoApplyThreshold)
	}

	noYearMediocre := scorer.Score(
		sdk.Candidate{Title: "Crash Landing", Popularity: 1e9},
		sdk.ParsedMedia{Title: "Crash"},
	)
	if noYearMediocre >= filmtv.AutoApplyThreshold {
		t.Errorf("popularity rescued a mediocre no-year title: %.6f >= %.2f", noYearMediocre, filmtv.AutoApplyThreshold)
	}
}

// TestNoYearNonExactStillResolves guards the other side of the penalty: a
// docked no-year score must stay above the corpus resolution floor (0.5) so the
// candidate can still rank and resolve — it is deferred to manual review, not
// discarded. A containment match (0.85) docked by the 0.25 penalty is 0.60.
func TestNoYearNonExactStillResolves(t *testing.T) {
	scorer := filmtv.NewScorer()
	score := scorer.Score(
		sdk.Candidate{Title: "The Grand Adventure"},
		sdk.ParsedMedia{Title: "The Grand Adventure Part1"},
	)
	const resolutionFloor = 0.5
	if score < resolutionFloor {
		t.Errorf("containment no-year score %.4f fell below the %.2f resolution floor; "+
			"the fail-closed penalty must defer to manual review, not discard the candidate",
			score, resolutionFloor)
	}
	if score >= filmtv.AutoApplyThreshold {
		t.Errorf("containment no-year score %.4f still auto-applies (>= %.2f); "+
			"a no-year non-exact match must not clear the auto-apply bar",
			score, filmtv.AutoApplyThreshold)
	}
}

// TestApostrophesDoNotSplitTitleWords pins that a release name that drops an
// apostrophe ("A Mothers Son") scores against the provider's spelling ("A
// Mother's Son", straight or curly) exactly as against its own (CORE-2313).
func TestApostrophesDoNotSplitTitleWords(t *testing.T) {
	scorer := filmtv.NewScorer()
	parsed := sdk.ParsedMedia{Title: "A Mothers Son", Class: "episode", Sequence: "1", Index: 1}
	exact := scorer.Score(sdk.Candidate{Title: "A Mothers Son", Year: 2023, Sequence: 1, Index: 1}, parsed)
	for _, title := range []string{"A Mother's Son", "A Mother’s Son", "A Mother‘s Son", "A Motherʼs Son", "A Mother`s Son"} {
		if got := scorer.Score(sdk.Candidate{Title: title, Year: 2023, Sequence: 1, Index: 1}, parsed); got != exact {
			t.Errorf("Score(%q) = %.4f, want the exact-spelling score %.4f", title, got, exact)
		}
	}
	if exact < filmtv.AutoApplyThreshold {
		t.Errorf("exact episode score %.4f is below the auto-apply bar %.4f", exact, filmtv.AutoApplyThreshold)
	}
}

// TestSeparatorApostropheJoinsItsWord pins that a release name which turned
// the apostrophe into a separator ("Ocean_s_Eleven") still matches the title.
func TestSeparatorApostropheJoinsItsWord(t *testing.T) {
	scorer := filmtv.NewScorer()
	parsed, ok := filmtv.New().Parse("Ocean_s_Eleven.2001.mkv", sdk.ParseProbe{})
	if !ok {
		t.Fatal("Ocean_s_Eleven.2001.mkv refused")
	}
	if got := scorer.Score(sdk.Candidate{Title: "Ocean's Eleven", Year: 2001}, parsed); got != 1 {
		t.Errorf("Score(%q vs Ocean's Eleven) = %.4f, want 1", parsed.Title, got)
	}
}

// TestTowerShowTitles pins the CORE-2311 root cause and its fail-closed
// limits. Show-level TV candidates for real library names scored below the
// auto-apply bar because a no-year episode lost 0.25 on any non-exact title and
// 0.05 more because a show-level candidate carries no ordinals. Spelling
// variants now score as the title; a country qualifier on one side only does
// not, because a candidate carries no origin country to tell "The Office" UK
// from "The Office" US, so it needs a year (or the same qualifier) to apply.
func TestTowerShowTitles(t *testing.T) {
	scorer := filmtv.NewScorer()
	cases := []struct {
		parsed    string
		year      int
		candidate string
		candYear  int
		want      float64
	}{
		{"Its Always Sunny in Philadelphia", 0, "It's Always Sunny in Philadelphia", 2005, 0.99},
		{"Key and Peele", 0, "Key & Peele", 2012, 0.99},
		{"The Office (US)", 0, "The Office (US)", 2005, 0.99},
		{"The Office USA", 0, "The Office (US)", 2005, 0.99},
		{"The Traitors (UK)", 0, "The Traitors (UK)", 2022, 0.99},
		// One-sided qualifier, no year: both national versions score alike
		// and neither may auto-apply.
		{"The Traitors UK", 0, "The Traitors", 2022, 0.64},
		{"The Traitors UK", 0, "The Traitors", 2023, 0.64},
		{"The Office UK", 0, "The Office", 2001, 0.64},
		{"The Office UK", 0, "The Office", 2005, 0.64},
		{"Shameless", 0, "Shameless (US)", 2011, 0.64},
		{"Shameless", 0, "Shameless (UK)", 2004, 0.64},
		// A year tells them apart.
		{"The Office UK", 2001, "The Office", 2001, 0.92},
		{"The Office UK", 2001, "The Office", 2005, 0.62},
	}
	for _, tc := range cases {
		parsed := sdk.ParsedMedia{Title: tc.parsed, Year: tc.year, Class: "episode", Sequence: "1", Index: 1}
		got := scorer.Score(sdk.Candidate{Title: tc.candidate, Year: tc.candYear}, parsed)
		if math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("Score(%q %d vs %q %d) = %.4f, want %.4f", tc.parsed, tc.year, tc.candidate, tc.candYear, got, tc.want)
		}
	}
	// Not the same show: another country's version, a lower-case word that
	// only looks like a country ("Us"), and a film title's last word.
	for _, tc := range []struct {
		parsed sdk.ParsedMedia
		cand   string
	}{
		{sdk.ParsedMedia{Title: "The Office (UK)", Class: "episode", Sequence: "1", Index: 1}, "The Office (US)"},
		{sdk.ParsedMedia{Title: "This Is", Class: "episode", Sequence: "1", Index: 1}, "This Is Us"},
		{sdk.ParsedMedia{Title: "Between", Class: "episode", Sequence: "1", Index: 1}, "Between Us"},
		{sdk.ParsedMedia{Title: "Among Us", Class: "movie"}, "Among"},
		{sdk.ParsedMedia{Title: "Crash", Class: "episode", Sequence: "1", Index: 1}, "Crash Landing on You"},
	} {
		if got := scorer.Score(sdk.Candidate{Title: tc.cand}, tc.parsed); got >= filmtv.AutoApplyThreshold {
			t.Errorf("Score(%q vs %q) = %.4f, want below the auto-apply bar", tc.parsed.Title, tc.cand, got)
		}
	}
}

// TestRegionFoldFromFilename pins the reviewer's case end to end: a no-year
// "The.Office.UK" file must not auto-apply to either "The Office".
func TestRegionFoldFromFilename(t *testing.T) {
	parsed, ok := filmtv.New().Parse("/media/TV/The.Office.UK.S01E01.mkv", sdk.ParseProbe{})
	if !ok || parsed.Title != "The Office UK" {
		t.Fatalf("Parse = %+v, %v; want title The Office UK", parsed, ok)
	}
	scorer := filmtv.NewScorer()
	for _, year := range []int{2001, 2005} {
		if got := scorer.Score(sdk.Candidate{Title: "The Office", Year: year}, parsed); got >= filmtv.AutoApplyThreshold {
			t.Errorf("Score(The Office %d) = %.4f, want below the auto-apply bar", year, got)
		}
	}
}
