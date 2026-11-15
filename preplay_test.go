package filmtv

import (
	"context"
	"errors"
	"slices"
	"testing"
)

type fakePrePlayReader struct {
	classes    map[string]string
	extras     []PrePlayExtra
	watched    map[string]bool
	classErr   error
	classErrOn int
	classCalls int
	extrasErr  error
	watchedErr error
}

func (f *fakePrePlayReader) MediaClasses(_ context.Context, ids []string) (map[string]string, error) {
	f.classCalls++
	if f.classErr != nil && f.classCalls >= f.classErrOn {
		return nil, f.classErr
	}
	out := map[string]string{}
	for _, id := range ids {
		if class, ok := f.classes[id]; ok {
			out[id] = class
		}
	}
	return out, nil
}

func (f *fakePrePlayReader) Extras(context.Context, string) ([]PrePlayExtra, error) {
	return f.extras, f.extrasErr
}

func (f *fakePrePlayReader) Watched(_ context.Context, ids []string) (map[string]bool, error) {
	if f.watchedErr != nil {
		return nil, f.watchedErr
	}
	out := map[string]bool{}
	for _, id := range ids {
		out[id] = f.watched[id]
	}
	return out, nil
}

func libraryReader() *fakePrePlayReader {
	return &fakePrePlayReader{
		classes: map[string]string{
			"feature": "movie", "a": "movie", "b": "movie", "c": "movie", "ep": "episode",
		},
		extras: []PrePlayExtra{
			{MediaID: "t-feature", ParentID: "feature", Path: "/lib/Feature (2001)/Feature-trailer.mkv"},
			{MediaID: "t-a2", ParentID: "a", Path: "/lib/A (2002)/trailers/second.mkv"},
			{MediaID: "t-a1", ParentID: "a", Path: "/lib/A (2002)/A-Trailer.MP4"},
			{MediaID: "t-b", ParentID: "b", Path: `C:\lib\B (2003)\Trailers\b.mkv`},
			{MediaID: "f-c", ParentID: "c", Path: "/lib/C (2004)/featurettes/C-featurette.mkv"},
			{MediaID: "t-ep", ParentID: "ep", Path: "/tv/Show/Season 01/ep-trailer.mkv"},
		},
		watched: map[string]bool{},
	}
}

func TestPlanPrePlayPicksOneTrailerPerOtherFilm(t *testing.T) {
	plan, err := PlanPrePlay(context.Background(), libraryReader(), PrePlayRequest{FeatureID: "feature", Count: 5, Seed: 7})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Eligible || plan.Label != PrePlayLabel {
		t.Fatalf("plan = %+v, want eligible with label", plan)
	}
	got := slices.Clone(plan.MediaIDs)
	slices.Sort(got)
	if want := []string{"t-a1", "t-b"}; !slices.Equal(got, want) {
		t.Fatalf("trailers = %v, want %v", got, want)
	}
}

func TestPlanPrePlayIneligibleFeatures(t *testing.T) {
	for _, feature := range []string{"ep", "hidden"} {
		plan, err := PlanPrePlay(context.Background(), libraryReader(), PrePlayRequest{FeatureID: feature, Count: 3})
		if err != nil {
			t.Fatal(err)
		}
		if plan.Eligible || len(plan.MediaIDs) != 0 {
			t.Fatalf("%s: plan = %+v, want ineligible and empty", feature, plan)
		}
	}
}

func TestPlanPrePlayZeroCountStaysEligible(t *testing.T) {
	plan, err := PlanPrePlay(context.Background(), libraryReader(), PrePlayRequest{FeatureID: "feature", Count: -2})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Eligible || len(plan.MediaIDs) != 0 {
		t.Fatalf("plan = %+v, want eligible with no trailers", plan)
	}
}

func TestPlanPrePlayOnlyOwnTrailerYieldsNone(t *testing.T) {
	reader := libraryReader()
	reader.extras = reader.extras[:1]
	plan, err := PlanPrePlay(context.Background(), reader, PrePlayRequest{FeatureID: "feature", Count: 3})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Eligible || len(plan.MediaIDs) != 0 {
		t.Fatalf("plan = %+v, want eligible with no trailers", plan)
	}
}

func TestPlanPrePlayUnwatchedFirst(t *testing.T) {
	for seed := range uint64(20) {
		reader := libraryReader()
		reader.watched = map[string]bool{"a": true}
		plan, err := PlanPrePlay(context.Background(), reader, PrePlayRequest{FeatureID: "feature", Count: 1, Seed: seed})
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(plan.MediaIDs, []string{"t-b"}) {
			t.Fatalf("seed %d: trailers = %v, want [t-b]", seed, plan.MediaIDs)
		}
	}
}

func TestPlanPrePlaySeedIsDeterministicAndCapped(t *testing.T) {
	reader := &fakePrePlayReader{classes: map[string]string{"feature": "movie"}, watched: map[string]bool{}}
	for i := range 15 {
		id := string(rune('a' + i))
		reader.classes[id] = "movie"
		reader.extras = append(reader.extras, PrePlayExtra{MediaID: "t-" + id, ParentID: id, Path: "/lib/" + id + "/trailers/x.mkv"})
	}
	first, err := PlanPrePlay(context.Background(), reader, PrePlayRequest{FeatureID: "feature", Count: 99, Seed: 42})
	if err != nil {
		t.Fatal(err)
	}
	second, err := PlanPrePlay(context.Background(), reader, PrePlayRequest{FeatureID: "feature", Count: 99, Seed: 42})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.MediaIDs) != PrePlayMaxTrailers || !slices.Equal(first.MediaIDs, second.MediaIDs) {
		t.Fatalf("plans differ or not capped: %v vs %v", first.MediaIDs, second.MediaIDs)
	}
	other, err := PlanPrePlay(context.Background(), reader, PrePlayRequest{FeatureID: "feature", Count: 99, Seed: 43})
	if err != nil {
		t.Fatal(err)
	}
	if slices.Equal(first.MediaIDs, other.MediaIDs) {
		t.Fatalf("different seeds gave the same order %v", other.MediaIDs)
	}
}

func TestPlanPrePlayErrors(t *testing.T) {
	boom := errors.New("boom")
	ctx := context.Background()
	if _, err := PlanPrePlay(ctx, nil, PrePlayRequest{FeatureID: "feature"}); err == nil {
		t.Fatal("nil reader: want error")
	}
	if _, err := PlanPrePlay(ctx, libraryReader(), PrePlayRequest{FeatureID: " "}); err == nil {
		t.Fatal("blank feature: want error")
	}
	cases := map[string]func(*fakePrePlayReader){
		"feature class": func(r *fakePrePlayReader) { r.classErr, r.classErrOn = boom, 1 },
		"parent class":  func(r *fakePrePlayReader) { r.classErr, r.classErrOn = boom, 2 },
		"extras":        func(r *fakePrePlayReader) { r.extrasErr = boom },
		"watched":       func(r *fakePrePlayReader) { r.watchedErr = boom },
	}
	for name, mutate := range cases {
		reader := libraryReader()
		mutate(reader)
		if _, err := PlanPrePlay(ctx, reader, PrePlayRequest{FeatureID: "feature", Count: 2}); !errors.Is(err, boom) {
			t.Fatalf("%s: err = %v, want boom", name, err)
		}
	}
}

func TestPlanPrePlayNoTrailersInLibrary(t *testing.T) {
	reader := libraryReader()
	reader.extras = []PrePlayExtra{{MediaID: "f-c", ParentID: "c", Path: "/lib/C/extras/c.mkv"}, {MediaID: "", ParentID: "a", Path: "x-trailer.mkv"}}
	plan, err := PlanPrePlay(context.Background(), reader, PrePlayRequest{FeatureID: "feature", Count: 3})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Eligible || len(plan.MediaIDs) != 0 {
		t.Fatalf("plan = %+v, want eligible with no trailers", plan)
	}
}

func TestIsTrailerPath(t *testing.T) {
	for path, want := range map[string]bool{
		"/lib/A/A-trailer.mkv":                 true,
		"/lib/A/A-TRAILER":                     true,
		"/lib/A/trailers/teaser.mkv":           true,
		`D:\lib\A\Trailers\teaser.mkv`:         true,
		"/lib/A/trailers/featurettes/x.mkv":    false,
		"/lib/A/featurettes/A-featurette.mkv":  false,
		"/lib/A/A.mkv":                         false,
		"/lib/A/extras/A-sample.mkv":           false,
		"/trailers/lib/A/extras/interview.mkv": false,
	} {
		if got := IsTrailerPath(path); got != want {
			t.Errorf("IsTrailerPath(%q) = %v, want %v", path, got, want)
		}
	}
}
