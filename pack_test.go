package filmtv

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type fakePackReader struct {
	members   map[string][]string
	entities  map[string][]PackEntity
	watched   map[string]bool
	memberErr error
	entityErr error
	watchErr  error
	calls     []string
}

func (f *fakePackReader) GroupMembers(_ context.Context, ref string) ([]string, error) {
	f.calls = append(f.calls, "members:"+ref)
	if f.memberErr != nil {
		return nil, f.memberErr
	}
	return f.members[ref], nil
}

func (f *fakePackReader) EntitiesOf(_ context.Context, parent, kind, relation string) ([]PackEntity, error) {
	f.calls = append(f.calls, "entities:"+parent+":"+kind+":"+relation)
	if f.entityErr != nil {
		return nil, f.entityErr
	}
	return f.entities[parent], nil
}

func (f *fakePackReader) Watched(_ context.Context, ids []string) (map[string]bool, error) {
	if f.watchErr != nil {
		return nil, f.watchErr
	}
	out := map[string]bool{}
	for _, id := range ids {
		if f.watched[id] {
			out[id] = true
		}
	}
	return out, nil
}

func numbered(id string, n float64) PackEntity {
	return PackEntity{ID: id, Numbers: map[string]float64{seasonNumberField: n}}
}

// seriesFixture is a show with seasons stored out of order, an unnumbered
// season, a specials season flagged and one numbered zero.
func seriesFixture() *fakePackReader {
	return &fakePackReader{
		entities: map[string][]PackEntity{
			"show-1": {
				{ID: "s-extra"},
				numbered("s-2", 2),
				{ID: "s-flag", Numbers: map[string]float64{seasonNumberField: 9}, Flags: map[string]bool{seasonSpecialsField: true}},
				numbered("s-1", 1),
				numbered("s-0", 0),
			},
		},
		members: map[string][]string{
			"s-1":     {"e1", "e2", "e3"},
			"s-2":     {"e4", "e5", "e2"},
			"s-extra": {"x1"},
			"s-0":     {"z1"},
			"s-flag":  {"z2", ""},
		},
	}
}

func TestPackResolversCoverEveryKind(t *testing.T) {
	got := PackResolvers()
	for _, kind := range []string{PackKindSeason, PackKindSeries, PackKindNextUnwatched} {
		if got[kind] == nil {
			t.Fatalf("PackResolvers() missing %q", kind)
		}
	}
	if len(got) != 3 {
		t.Fatalf("PackResolvers() has %d kinds, want 3", len(got))
	}
}

func TestSeasonPackKeepsStoredOrder(t *testing.T) {
	r := &fakePackReader{members: map[string][]string{"s-1": {"e3", "e1", "e3", "", "e2"}}}
	got, err := seasonPackResolver{}.Resolve(context.Background(), " s-1 ", r)
	if err != nil {
		t.Fatal(err)
	}
	want := PackResolved{MediaIDs: []string{"e3", "e1", "e2"}, Label: "Season"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Resolve = %+v, want %+v", got, want)
	}
}

func TestSeasonPackRejectsEmptyRefAndPropagatesErrors(t *testing.T) {
	if _, err := (seasonPackResolver{}).Resolve(context.Background(), "  ", &fakePackReader{}); err == nil {
		t.Fatal("empty ref: want error")
	}
	boom := errors.New("not visible")
	if _, err := (seasonPackResolver{}).Resolve(context.Background(), "s-1", &fakePackReader{memberErr: boom}); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
}

func TestSeriesPackOrdersSeasonsThenSpecials(t *testing.T) {
	r := seriesFixture()
	got, err := seriesPackResolver{}.Resolve(context.Background(), "show-1", r)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"e1", "e2", "e3", "e4", "e5", "x1", "z2", "z1"}
	if !reflect.DeepEqual(got.MediaIDs, want) || got.Label != "Series" {
		t.Fatalf("Resolve = %+v, want %v / Series", got, want)
	}
	if r.calls[0] != "entities:show-1:season:"+SeasonOfRelation {
		t.Fatalf("first read = %q, want the season-of lookup", r.calls[0])
	}
}

func TestSeriesPackErrors(t *testing.T) {
	ctx := context.Background()
	if _, err := (seriesPackResolver{}).Resolve(ctx, "", &fakePackReader{}); err == nil {
		t.Fatal("empty ref: want error")
	}
	boom := errors.New("hidden")
	if _, err := (seriesPackResolver{}).Resolve(ctx, "show-1", &fakePackReader{entityErr: boom}); !errors.Is(err, boom) {
		t.Fatalf("entity err = %v", err)
	}
	r := seriesFixture()
	r.memberErr = boom
	if _, err := (seriesPackResolver{}).Resolve(ctx, "show-1", r); !errors.Is(err, boom) {
		t.Fatalf("member err = %v", err)
	}
}

func TestNextUnwatchedPack(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name    string
		ref     string
		watched map[string]bool
		want    []string
	}{
		{"nothing watched", "show-1:3", nil, []string{"e1", "e2", "e3"}},
		{"resumes after last watched", "show-1:2", map[string]bool{"e1": true, "e2": true}, []string{"e3", "e4"}},
		{"skipped earlier episode ignored", "show-1:5", map[string]bool{"e3": true}, []string{"e4", "e5", "x1"}},
		{"finished later skipped", "show-1:2", map[string]bool{"e1": true, "e4": true}, []string{"e5", "x1"}},
		{"never includes specials", "show-1:50", map[string]bool{"x1": true}, []string{}},
		{"show ref may hold a colon", "show:1:1", nil, []string{"c1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := seriesFixture()
			r.entities["show:1"] = []PackEntity{numbered("c-1", 1)}
			r.members["c-1"] = []string{"c1", "c2"}
			r.watched = tc.watched
			got, err := nextUnwatchedPackResolver{}.Resolve(ctx, tc.ref, r)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.MediaIDs, tc.want) || got.Label != "Up Next" {
				t.Fatalf("Resolve = %+v, want %v", got, tc.want)
			}
		})
	}
}

func TestNextUnwatchedPackErrors(t *testing.T) {
	ctx := context.Background()
	for _, ref := range []string{"", "show-1", ":3", "show-1:", "show-1:0", "show-1:51", "show-1:x", "show-1:-2"} {
		if _, err := (nextUnwatchedPackResolver{}).Resolve(ctx, ref, seriesFixture()); err == nil {
			t.Fatalf("ref %q: want error", ref)
		}
	}
	boom := errors.New("down")
	r := seriesFixture()
	r.watchErr = boom
	if _, err := (nextUnwatchedPackResolver{}).Resolve(ctx, "show-1:1", r); !errors.Is(err, boom) {
		t.Fatalf("watch err = %v", err)
	}
	if _, err := (nextUnwatchedPackResolver{}).Resolve(ctx, "show-1:1", &fakePackReader{entityErr: boom}); !errors.Is(err, boom) {
		t.Fatalf("entity err = %v", err)
	}
}

func TestParseNextUnwatchedRefBounds(t *testing.T) {
	show, n, err := parseNextUnwatchedRef(" show-9:50 ")
	if err != nil || show != "show-9" || n != MaxNextUnwatched {
		t.Fatalf("parse = %q %d %v", show, n, err)
	}
	if _, _, err := parseNextUnwatchedRef("show-9:51"); err == nil || !strings.Contains(err.Error(), "1 to 50") {
		t.Fatalf("over max err = %v", err)
	}
}

func TestPackKindsStayNamespaced(t *testing.T) {
	for _, kind := range []string{PackKindSeason, PackKindSeries, PackKindNextUnwatched} {
		if !strings.HasPrefix(kind, PluginID+"-") {
			t.Fatalf("kind %q not prefixed with plugin id %q", kind, PluginID)
		}
	}
}
