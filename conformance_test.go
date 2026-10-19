package filmtv_test

import (
	"reflect"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"
	"github.com/beamfall/plugin-sdk/sdktest"

	filmtv "github.com/beamfall/plugin-filmtv"
)

// TestParseConformance pins the Film & TV filename parser against the SDK scan
// contract. A change that shifts any of these
// guesses reds CI before it ships.
func TestParseConformance(t *testing.T) {
	sdktest.RunConformance(t, filmtv.New(), []sdktest.ParseVector{
		{
			Name:   "movie with year and release noise",
			Path:   "The.Matrix.1999.1080p.BluRay.x264.mkv",
			Want:   sdk.ParsedMedia{Title: "The Matrix", Class: "movie", Year: 1999},
			WantOK: true,
		},
		{
			Name:   "parenthesized year",
			Path:   "Inception (2010).mkv",
			Want:   sdk.ParsedMedia{Title: "Inception", Class: "movie", Year: 2010},
			WantOK: true,
		},
		{
			Name:   "episode SxxEyy",
			Path:   "Some.Show.S02E05.720p.HDTV.mkv",
			Want:   sdk.ParsedMedia{Title: "Some Show", Class: "episode", Sequence: "2", Index: 5},
			WantOK: true,
		},
		{
			Name:   "multi-episode range",
			Path:   "Show.S01E01-E02.mkv",
			Want:   sdk.ParsedMedia{Title: "Show", Class: "episode", Sequence: "1", Index: 1, IndexEnd: 2},
			WantOK: true,
		},
		{
			Name:   "noise-only name has no title",
			Path:   "1080p.x264.mkv",
			Want:   sdk.ParsedMedia{},
			WantOK: false,
		},
	})
}

// TestScoreConformance pins the match scorer's confidence model.
func TestScoreConformance(t *testing.T) {
	matrix := sdk.ParsedMedia{Title: "The Matrix", Year: 1999}
	sdktest.RunScoreConformance(t, filmtv.NewScorer(), []sdktest.ScoreVector{
		{Name: "exact title + year", CandTitle: "The Matrix", CandYear: 1999, Parsed: matrix, Want: 1.0},
		{Name: "exact title, +/-1 year", CandTitle: "The Matrix", CandYear: 2000, Parsed: matrix, Want: 0.85},
		{Name: "exact title, conflicting year", CandTitle: "The Matrix", CandYear: 2010, Parsed: matrix, Want: 0.7},
		{Name: "title miss is zero", CandTitle: "Inception", CandYear: 1999, Parsed: matrix, Want: 0.0},
		{Name: "no parsed year: title carries", CandTitle: "The Matrix", CandYear: 1999,
			Parsed: sdk.ParsedMedia{Title: "The Matrix"}, Want: 1.0},
		// Containment with no year: 0.85 similarity docked by the
		// noCorroborationPenalty (0.25) to 0.60 — still above the 0.5 corpus
		// resolution floor so it can rank/resolve, but below AutoApplyThreshold
		// (0.75) so it will not auto-apply on title alone (fail-closed).
		{Name: "containment, no year", CandTitle: "The Matrix", CandYear: 0,
			Parsed: sdk.ParsedMedia{Title: "Matrix"}, Want: 0.60},
		// Franchise-sequel false positive: a short parsed title ("Matrix", e.g. from "Matrix.mkv"
		// with no year) is contained in a longer, WRONG candidate title ("The
		// Matrix Revolutions"). Containment (0.85) minus the no-year penalty
		// (0.25) is 0.60 — now BELOW AutoApplyThreshold (0.75), so this wrong
		// candidate no longer auto-applies; it routes to the Unmatched queue.
		// This pins that fail-closed behavior.
		{Name: "containment false positive: short title in wrong longer title, no year",
			CandTitle: "The Matrix Revolutions", CandYear: 0,
			Parsed: sdk.ParsedMedia{Title: "Matrix"}, Want: 0.60},
		// Near-AutoApplyThreshold boundary: containment (0.85) combined with a
		// +/-1 year drift (0.5) lands at 0.745 — just BELOW the 0.75 auto-apply
		// gate. Pins the boundary so a change to the 0.85 containment constant,
		// the 0.5 year-drift constant, or the 0.75 threshold itself is caught
		// here rather than silently flipping auto-apply behavior at the edge.
		{Name: "near-threshold boundary: containment + year drift stays below 0.75",
			CandTitle: "The Matrix Revolutions", CandYear: 2000,
			Parsed: sdk.ParsedMedia{Title: "Matrix", Year: 1999}, Want: 0.745},
	})
}

// TestBundleConformance pins the declarative bundle + locale invariants.
func TestBundleConformance(t *testing.T) {
	sdktest.RunBundleConformance(t, filmtv.Bundle())
	sdktest.RequireEntityKindVocab(t, filmtv.Bundle(), []sdktest.KindVocab{
		{Name: "show", Archetype: "container"},
		{Name: "season", Archetype: "container"},
		{Name: "film-series", Archetype: "container"},
		{Name: "person", Archetype: "person"},
		{Name: "studio", Archetype: "category"},
		{Name: "genre", Archetype: "category"},
	})
}

func TestBundleLabelMaps(t *testing.T) {
	want := []sdk.LabelMapSpec{
		{Label: "action", Tag: "action"},
		{Label: "comedy", Tag: "comedy"},
		{Label: "drama", Tag: "drama"},
		{Label: "horror", Tag: "horror"},
		{Label: "romance", Tag: "romance"},
		{Label: "science fiction", Tag: "science-fiction"},
	}
	if got := filmtv.Bundle().LabelMaps; !reflect.DeepEqual(got, want) {
		t.Fatalf("LabelMaps = %#v, want %#v", got, want)
	}
}
