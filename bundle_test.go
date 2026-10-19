package filmtv_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"
	"github.com/beamfall/plugin-sdk/scrobble"

	filmtv "github.com/beamfall/plugin-filmtv"
)

// TestBundleValid pins that the declarative bundle passes the SDK load-time
// validator — the gate every declaration change must clear.
func TestBundleValid(t *testing.T) {
	if err := sdk.ValidateBundle(filmtv.Bundle()); err != nil {
		t.Fatalf("Bundle() fails SDK validation: %v", err)
	}
}

// TestLayoutSectionsDoNotShareSource pins that no home Layout repeats itself:
// two Sections on the same Layout that read the identical Source (same
// Kind, EntityKind, Value, FieldFilters, and EntityFilter) render the same
// items twice on one page. A "filter-bar" Section is exempt — its Source
// only supplies the facet vocabulary for the adjacent grid it narrows, and
// it declares no item rows of its own to repeat.
func TestLayoutSectionsDoNotShareSource(t *testing.T) {
	for _, layout := range filmtv.Bundle().Layouts {
		bySource := map[string]string{}
		for _, section := range layout.Sections {
			if section.Kind == "filter-bar" {
				continue
			}
			key := sourceKey(section.Source)
			if prior, ok := bySource[key]; ok {
				t.Errorf("layout %q: sections %q and %q both read source %s — the same titles render on both", layout.ID, prior, section.ID, key)
				continue
			}
			bySource[key] = section.ID
		}
	}
}

func sourceKey(source sdk.SourceSpec) string {
	fieldFilters, _ := json.Marshal(source.FieldFilters)
	entityFilter, _ := json.Marshal(source.EntityFilter)
	return fmt.Sprintf("%s|entityKind=%s|value=%s|fieldFilters=%s|entityFilter=%s",
		source.Kind, source.EntityKind, source.Value, fieldFilters, entityFilter)
}

func TestProviderIDLookupTemplates(t *testing.T) {
	want := map[string]map[string]string{
		"tmdb-movie":  {"tmdb": "/movie/{id}?append_to_response=credits"},
		"tmdb-tv":     {"tmdb": "/tv/{id}?append_to_response=credits"},
		"tvdb-series": {"tvdb": "/series/{id}"},
	}
	for _, scraper := range filmtv.Bundle().Scrapers {
		templates, ok := want[scraper.ID]
		if !ok {
			continue
		}
		if !reflect.DeepEqual(scraper.LookupByIDTemplate, templates) {
			t.Errorf("%s LookupByIDTemplate = %#v, want %#v", scraper.ID, scraper.LookupByIDTemplate, templates)
		}
		delete(want, scraper.ID)
	}
	if len(want) > 0 {
		t.Errorf("missing scrapers with provider-ID lookup templates: %#v", want)
	}
}

// TestScrobbleTargetDeclaration pins the exact plugin-owned class-to-shape
// mapping. Unknown or absent class values deliberately have no fallback.
func TestScrobbleTargetDeclaration(t *testing.T) {
	spec := filmtv.Bundle().ScrobbleTarget
	if spec == nil {
		t.Fatal("Bundle().ScrobbleTarget is nil")
	}
	want := &sdk.ScrobbleTargetSpec{
		MediaField: "class",
		Mappings: []sdk.ScrobbleTargetMapping{
			{Value: "movie", Hint: scrobble.TargetStandalone},
			{Value: "episode", Hint: scrobble.TargetIndexed},
		},
	}
	if !reflect.DeepEqual(spec, want) {
		t.Fatalf("Bundle().ScrobbleTarget = %#v, want %#v", spec, want)
	}

	for _, value := range []string{"", "Movie", "unknown"} {
		if hint, ok := scrobbleTargetHint(spec, value); ok {
			t.Errorf("class %q unexpectedly maps to %q", value, hint)
		}
	}
}

func scrobbleTargetHint(spec *sdk.ScrobbleTargetSpec, value string) (scrobble.TargetHint, bool) {
	for _, mapping := range spec.Mappings {
		if mapping.Value == value {
			return mapping.Hint, true
		}
	}
	return "", false
}

// TestMETAPARITY16 pins three incumbent-parity declarations: the network
// browse facet, the episode-still artwork slot, and the guest-star credit
// scoped to Episode Media.
func TestMETAPARITY16(t *testing.T) {
	b := filmtv.Bundle()

	// 1. Network browse facet: a "network" text Media field feeds a
	//    home filter-bar option, hydrated by the tmdb-tv scraper.
	if _, ok := mediaFieldType(b, "network"); !ok {
		t.Errorf("no `network` media field declared for the browse facet")
	}
	if !filterBarHasField(b, "network") {
		t.Errorf("home filter-bar declares no `network` browse-facet option")
	}
	if tmdbTVFieldMap(b)["media.network"] == "" {
		t.Errorf("tmdb-tv scraper does not map media.network")
	}

	// 2. Episode-still slot: tmdb-tv surfaces the still in the fixed
	//    "thumb" artwork slot the season-detail episode list renders.
	if tmdbTVArtworkMap(b)["thumb"] == "" {
		t.Errorf("tmdb-tv scraper declares no episode-still `thumb` artwork slot")
	}

	// 3. Guest-star credit scoped to Episode Media: a Person→Media
	//    relation distinct from the show-level `credit` container relation.
	if !personRelationToMedia(b, "guest-starred-in") {
		t.Errorf("person kind declares no `guest-starred-in` Person→Media relation")
	}
}

func mediaFieldType(b sdk.Bundle, name string) (string, bool) {
	for _, f := range b.MediaFields {
		if f.Name == name {
			return f.Type, true
		}
	}
	return "", false
}

func filterBarHasField(b sdk.Bundle, field string) bool {
	for _, l := range b.Layouts {
		for _, s := range l.Sections {
			for _, o := range s.FilterOptions {
				if o.Field == field {
					return true
				}
			}
		}
	}
	return false
}

func personRelationToMedia(b sdk.Bundle, name string) bool {
	for _, k := range b.EntityKinds {
		if k.Name != "person" {
			continue
		}
		for _, r := range k.Relations {
			if r.Name == name && r.ToKind == sdk.MediaKindName {
				return true
			}
		}
	}
	return false
}

func tmdbTVFieldMap(b sdk.Bundle) map[string]string {
	for _, s := range b.Scrapers {
		if s.ID == "tmdb-tv" {
			return s.FieldMap
		}
	}
	return nil
}

func tmdbTVArtworkMap(b sdk.Bundle) map[string]string {
	for _, s := range b.Scrapers {
		if s.ID == "tmdb-tv" {
			return s.ArtworkMap
		}
	}
	return nil
}

func personEntityMap(b sdk.Bundle, scraperID string) (sdk.EntityMap, bool) {
	for _, s := range b.Scrapers {
		if s.ID != scraperID {
			continue
		}
		for _, em := range s.EntityMaps {
			if em.Kind == "person" {
				return em, true
			}
		}
	}
	return sdk.EntityMap{}, false
}

// TestTMDBPersonFieldHydration pins full Person hydration: a TMDB credits.cast
// entry carries only id/name/character, so a Person hydrated straight from it
// would be an empty entity shell (a name with no biographical fields). Both
// TMDB scrapers must declare the secondary /person/{id} detail-fetch shape
// (LookupTemplate/LookupIDPath/MaxLookups) and a FieldMap that actually
// populates declared Person fields — proving the relation direction
// (Person→Media `credited-in`, not the unreachable show-level `credit`) and
// the detail-fetch shape both wire up in production.
func TestTMDBPersonFieldHydration(t *testing.T) {
	b := filmtv.Bundle()

	if !personRelationToMedia(b, "credited-in") {
		t.Fatalf("person kind declares no `credited-in` Person→Media relation")
	}

	personFields := map[string]bool{}
	for _, k := range b.EntityKinds {
		if k.Name != "person" {
			continue
		}
		for _, f := range k.Fields {
			personFields[f.Name] = true
		}
	}

	for _, scraperID := range []string{"tmdb-movie", "tmdb-tv"} {
		em, ok := personEntityMap(b, scraperID)
		if !ok {
			t.Fatalf("%s scraper declares no person EntityMap", scraperID)
		}
		if em.Relation != "credited-in" {
			t.Errorf("%s person EntityMap relation = %q, want credited-in", scraperID, em.Relation)
		}
		if em.LookupTemplate == "" || em.LookupIDPath == "" || em.MaxLookups <= 0 {
			t.Errorf("%s person EntityMap declares no bounded secondary detail fetch (LookupTemplate=%q LookupIDPath=%q MaxLookups=%d)",
				scraperID, em.LookupTemplate, em.LookupIDPath, em.MaxLookups)
		}
		if len(em.FieldMap) == 0 {
			t.Errorf("%s person EntityMap has no FieldMap — hydrates a name-only empty entity shell", scraperID)
		}
		for field := range em.FieldMap {
			if !personFields[field] {
				t.Errorf("%s person EntityMap FieldMap targets undeclared person field %q", scraperID, field)
			}
		}
		if _, ok := em.FieldMap["biography"]; !ok {
			t.Errorf("%s person EntityMap does not populate biography — not field-populated in production", scraperID)
		}
	}
}

// TestTMDBTVEntityMaps pins genre/studio parity for TV: the tmdb-tv scraper's GET
// /tv/{id} details document carries genres and production_companies just
// like the movie details document, so it must hydrate the same genre/studio
// EntityMaps as the tmdb-movie scraper.
func TestTMDBTVEntityMaps(t *testing.T) {
	b := filmtv.Bundle()

	var tvMaps, movieMaps []string
	for _, s := range b.Scrapers {
		var kinds []string
		for _, em := range s.EntityMaps {
			kinds = append(kinds, em.Kind+":"+em.Relation+":"+em.Path+":"+em.NamePath)
		}
		switch s.ID {
		case "tmdb-tv":
			tvMaps = kinds
		case "tmdb-movie":
			movieMaps = kinds
		}
	}

	if len(tvMaps) == 0 {
		t.Fatalf("tmdb-tv scraper has no EntityMaps; want genre + studio maps matching tmdb-movie")
	}
	if len(movieMaps) == 0 {
		t.Fatalf("test setup broken: tmdb-movie scraper has no EntityMaps")
	}
	if len(tvMaps) != len(movieMaps) {
		t.Fatalf("tmdb-tv EntityMaps = %v, want same shape as tmdb-movie %v", tvMaps, movieMaps)
	}
	for i, want := range movieMaps {
		if tvMaps[i] != want {
			t.Errorf("tmdb-tv EntityMaps[%d] = %q, want %q (movie scraper pattern)", i, tvMaps[i], want)
		}
	}
}

// wantBundleVersion is the release semver this head carries: the release step
// tags the head `v`+wantBundleVersion, so a bundle Version left behind by a
// tag bump fails here instead of shipping a stale identity.
const wantBundleVersion = "0.1.4"

// TestBundleVersionMatchesReleaseTag pins Bundle().Version to the semver of
// the release tag applied to this head.
func TestBundleVersionMatchesReleaseTag(t *testing.T) {
	if got := filmtv.Bundle().Version; got != wantBundleVersion {
		t.Fatalf("Bundle().Version = %q, want %q", got, wantBundleVersion)
	}
}

// TestBundleCompatibleWithPinnedSDK pins the declared [SDKFloor, SDKCeiling]
// range against the SDK identity this module pins, so a bundle whose range
// excludes its own pinned SDK fails here rather than at core load time
// (sdk.CompatibleWithSDK).
func TestBundleCompatibleWithPinnedSDK(t *testing.T) {
	b := filmtv.Bundle()
	if !sdk.CompatibleWithSDK(b, sdk.Version) {
		t.Fatalf("Bundle() compat range [%s, %s] excludes pinned SDK %s", b.SDKFloor, b.SDKCeiling, sdk.Version)
	}
}
