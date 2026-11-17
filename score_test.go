package filmtv_test

import (
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
			name:          "candidate carries no ordinals: no penalty, title+year carry",
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
	if !(matching == missing && missing > wrong) {
		t.Errorf("ordinal ordering matching %.4f == missing %.4f > wrong %.4f not satisfied", matching, missing, wrong)
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

// TestTowerShowTitlesAutoApply pins the CORE-2311 root cause: show-level TV
// candidates for real library names scored below the auto-apply bar because a
// no-year episode lost 0.25 on any non-exact title and 0.05 more because a
// show-level candidate carries no ordinals. Each row is the parsed show title
// from a tower file against the TMDB candidate it should match.
func TestTowerShowTitlesAutoApply(t *testing.T) {
	scorer := filmtv.NewScorer()
	cases := []struct {
		parsed, candidate string
		want              float64
	}{
		{"The Traitors UK", "The Traitors", 0.9},
		{"The Traitors (UK)", "The Traitors", 0.9},
		{"Its Always Sunny in Philadelphia", "It's Always Sunny in Philadelphia", 1},
		{"Key and Peele", "Key & Peele", 1},
		{"The Office (US)", "The Office", 0.9},
		{"The Office (US)", "The Office (US)", 1},
		{"The Office USA", "The Office (US)", 1},
	}
	for _, tc := range cases {
		parsed := sdk.ParsedMedia{Title: tc.parsed, Class: "episode", Sequence: "1", Index: 1}
		got := scorer.Score(sdk.Candidate{Title: tc.candidate, Year: 2022}, parsed)
		if got != tc.want || got < filmtv.AutoApplyThreshold {
			t.Errorf("Score(%q vs %q) = %.4f, want %.4f (auto-applies)", tc.parsed, tc.candidate, got, tc.want)
		}
	}
	// A different country's version is not the same show, and a film title
	// keeps its last word ("Among Us" is not "Among").
	for _, tc := range []struct {
		parsed sdk.ParsedMedia
		cand   string
	}{
		{sdk.ParsedMedia{Title: "The Office (UK)", Class: "episode", Sequence: "1", Index: 1}, "The Office (US)"},
		{sdk.ParsedMedia{Title: "Among Us", Class: "movie"}, "Among"},
		{sdk.ParsedMedia{Title: "Crash", Class: "episode", Sequence: "1", Index: 1}, "Crash Landing on You"},
	} {
		if got := scorer.Score(sdk.Candidate{Title: tc.cand}, tc.parsed); got >= filmtv.AutoApplyThreshold {
			t.Errorf("Score(%q vs %q) = %.4f, want below the auto-apply bar", tc.parsed.Title, tc.cand, got)
		}
	}
}
