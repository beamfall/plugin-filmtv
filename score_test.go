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
// full match below the auto-apply bar (wrong-episode is fail-closed), while an
// agreement outranks a same-title candidate that carries no ordinals.
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
			name:          "candidate carries no ordinals: small fallback penalty, title+year carry",
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
