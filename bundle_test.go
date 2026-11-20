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

func TestPlexImporterDeclaresSourceCategories(t *testing.T) {
	for _, importer := range filmtv.Bundle().Importers {
		if importer.ID != "plex" {
			continue
		}
		want := filmtv.NewPlexSource("plex.db").Categories()
		if !reflect.DeepEqual(importer.Categories, want) {
			t.Errorf("Plex importer categories = %v, want %v", importer.Categories, want)
		}
		return
	}
	t.Fatal("bundle is missing the Plex importer")
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
		"tmdb-movie":  {"tmdb": "/movie/{id}?append_to_response=credits,release_dates"},
		"tmdb-tv":     {"tmdb": "/tv/{id}?append_to_response=credits,content_ratings"},
		"tvdb-series": {"tvdb": "/series/{id}/extended?meta=translations", "imdb": "/search/remoteid/{id}", "tmdb": "/search/remoteid/{id}"},
		"tvmaze-show": {"tvdb": "/lookup/shows?thetvdb={id}", "imdb": "/lookup/shows?imdb={id}"},
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
// Media field, the episode-still artwork slot, and the guest-star credit
// scoped to Episode Media.
func TestMETAPARITY16(t *testing.T) {
	b := filmtv.Bundle()

	// 1. Network field: a "network" text Media field hydrated by the
	//    tmdb-tv scraper. The filter-bar declares no fixed network
	//    options (TestFilterBarDeclaresNoFixedNetworkOptions).
	if _, ok := mediaFieldType(b, "network"); !ok {
		t.Errorf("no `network` media field declared")
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

// TestFilterBarDeclaresNoFixedNetworkOptions pins CORE-1711: filter-bar
// options are fixed field/value pairs, so a declared network or studio
// value narrows a Library without it to an empty grid. Only the closed
// `class` vocabulary may be declared.
func TestFilterBarDeclaresNoFixedNetworkOptions(t *testing.T) {
	for _, l := range filmtv.Bundle().Layouts {
		for _, s := range l.Sections {
			for _, o := range s.FilterOptions {
				if o.Field != "class" {
					t.Errorf("layout %q section %q declares fixed %q option %q; only `class` options may be declared",
						l.ID, s.ID, o.Field, o.Value)
				}
			}
		}
	}
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
			if em.Container {
				// Container maps hydrate the show and season (TV only);
				// TestTMDBTVContainerHydration pins them.
				continue
			}
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

// TestTMDBCreditAttributes pins CORE-2503 and CORE-2432: both TMDB scrapers
// write each cast member's character and billing order on the credited-in row
// and sort by billing, then import only the key crew jobs, carrying job and
// department, after every cast credit. The two scrapers' person maps match.
func TestTMDBCreditAttributes(t *testing.T) {
	b := filmtv.Bundle()
	declared := map[string]bool{}
	for _, k := range b.EntityKinds {
		for _, r := range k.Relations {
			if k.Name == "person" && r.Name == "credited-in" {
				for _, a := range r.Attributes {
					declared[a] = true
				}
			}
		}
	}
	for _, a := range []string{"character", "billing", "job", "department"} {
		if !declared[a] {
			t.Errorf("credited-in does not declare attribute %q", a)
		}
	}

	personMaps := map[string][]sdk.EntityMap{}
	for _, scraperID := range []string{"tmdb-movie", "tmdb-tv"} {
		for _, em := range scraperByID(t, scraperID).EntityMaps {
			if em.Kind == "person" {
				personMaps[scraperID] = append(personMaps[scraperID], em)
			}
		}
	}
	movie := personMaps["tmdb-movie"]
	if len(movie) != 2 {
		t.Fatalf("tmdb-movie person maps = %d, want cast and crew", len(movie))
	}
	if !reflect.DeepEqual(personMaps["tmdb-tv"], movie) {
		t.Errorf("tmdb-tv person maps = %+v, want tmdb-movie's %+v", personMaps["tmdb-tv"], movie)
	}

	cast, crew := movie[0], movie[1]
	if cast.Path != "credits.cast" || cast.OrdinalField != "billing" || cast.OrdinalOffset != 0 ||
		!reflect.DeepEqual(cast.RelationAttributeMap, map[string]string{"character": "character", "billing": "order"}) {
		t.Errorf("cast map = %+v, want character and billing (order) attributes, ordered by billing", cast)
	}
	wantJobs := []string{"Director", "Screenplay", "Writer", "Producer", "Original Music Composer", "Director of Photography"}
	if crew.Path != "credits.crew" || crew.Relation != "credited-in" || crew.NamePath != "name" ||
		crew.ArtworkPath != "profile_path" || crew.LookupTemplate != "" ||
		!reflect.DeepEqual(crew.RelationAttributeMap, map[string]string{"job": "job", "department": "department"}) ||
		!reflect.DeepEqual(crew.ElementFilter, map[string][]string{"job": wantJobs}) {
		t.Errorf("crew map = %+v, want key jobs with job and department, no lookups", crew)
	}
	if crew.OrdinalOffset < 1000 {
		t.Errorf("crew OrdinalOffset = %d, want every crew credit after the cast", crew.OrdinalOffset)
	}
}

// TestTMDBTVContainerHydration pins CORE-2335: tmdb-tv hydrates the show from
// the series document root and the season from seasons[] by the parsed
// season, and fetches the episode's own document by season and episode
// number for its summary, year and still — never its title.
func TestTMDBTVContainerHydration(t *testing.T) {
	scraper := scraperByID(t, "tmdb-tv")
	containers := map[string]sdk.EntityMap{}
	for _, em := range scraper.EntityMaps {
		if em.Container {
			containers[em.Kind] = em
		}
	}
	show, season := containers["show"], containers["season"]
	if !show.Container || show.Path != "" || show.ArtworkPath != "poster_path" ||
		show.FieldMap["summary"] != "overview" || show.FieldMap["first-air-date"] != "first_air_date" {
		t.Errorf("show container map = %+v, want root poster_path, summary and first_air_date", show)
	}
	if !season.Container || season.Path != "seasons[season_number={sequence}]" || season.ArtworkPath != "poster_path" ||
		season.FieldMap["overview"] != "overview" || season.FieldMap["air-date"] != "air_date" {
		t.Errorf("season container map = %+v, want seasons[season_number={sequence}] poster_path, overview and air_date", season)
	}
	if scraper.ItemLookupTemplate != "/tv/{id}/season/{sequence}/episode/{index}" {
		t.Errorf("ItemLookupTemplate = %q, want the per-episode document", scraper.ItemLookupTemplate)
	}
	if scraper.ItemFieldMap["summary"] != "overview" || scraper.ItemFieldMap["year"] != "air_date" {
		t.Errorf("ItemFieldMap = %v, want summary=overview and year=air_date", scraper.ItemFieldMap)
	}
	if scraper.ItemFieldMap["media."+filmtv.EpisodeTitleField] != "name" {
		t.Errorf("ItemFieldMap = %v, want media.episode-title=name", scraper.ItemFieldMap)
	}
	if _, mapped := scraper.ItemFieldMap["title"]; mapped {
		t.Error("ItemFieldMap maps title; the episode keeps the parser's title")
	}
	if scraper.ItemArtworkMap["thumb"] != "still_path" {
		t.Errorf("ItemArtworkMap = %v, want thumb=still_path", scraper.ItemArtworkMap)
	}
	if scraper.Transforms["year"] != "year-from-date" {
		t.Errorf("year transform = %q, want year-from-date for the episode air date", scraper.Transforms["year"])
	}
	if err := sdk.ValidateBundle(filmtv.Bundle()); err != nil {
		t.Fatalf("ValidateBundle: %v", err)
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

// TestCredentialedScrapersDeclareAttribution: TMDB and TheTVDB require a
// visible credit with a link wherever their metadata is shown (CORE-1126).
func TestCredentialedScrapersDeclareAttribution(t *testing.T) {
	want := map[string]string{
		"tmdb-movie":  "https://www.themoviedb.org",
		"tmdb-tv":     "https://www.themoviedb.org",
		"tvdb-series": "https://www.thetvdb.com",
	}
	for _, scraper := range filmtv.Bundle().Scrapers {
		link, ok := want[scraper.ID]
		if !ok {
			continue
		}
		delete(want, scraper.ID)
		if scraper.Attribution == nil {
			t.Errorf("scraper %s declares no attribution", scraper.ID)
			continue
		}
		if scraper.Attribution.URL != link {
			t.Errorf("scraper %s attribution link = %q, want %q", scraper.ID, scraper.Attribution.URL, link)
		}
		if scraper.Attribution.Text.Resolve("") == "" {
			t.Errorf("scraper %s attribution has no text", scraper.ID)
		}
	}
	for id := range want {
		t.Errorf("scraper %s not found", id)
	}
}

// TestTVDBNeedsSubscriberPIN: TheTVDB's free project key only works with each
// user's own subscriber PIN, so the PIN is the scraper's AuthRef (no PIN, no
// TheTVDB) and the project key rides beside it as a second login secret
// (CORE-1129). TheTVDB is optional: without a PIN Core omits the scraper and
// TV identifies through tmdb-tv on the embedded TMDB key (CORE-2311).
func TestTVDBNeedsSubscriberPIN(t *testing.T) {
	for _, scraper := range filmtv.Bundle().Scrapers {
		if scraper.ID != "tvdb-series" {
			continue
		}
		if scraper.AuthRef != "tvdb-subscriber-pin" {
			t.Errorf("tvdb-series AuthRef = %q, want tvdb-subscriber-pin", scraper.AuthRef)
		}
		if !scraper.OptionalAuth {
			t.Error("tvdb-series must be optional-auth so a server without a PIN does not list it as a missing key")
		}
		login := scraper.Login
		if login == nil {
			t.Fatal("tvdb-series declares no login")
		}
		if login.SecretField != "pin" {
			t.Errorf("tvdb-series login SecretField = %q, want pin", login.SecretField)
		}
		if got := login.SecretFields["apikey"]; got != "tvdb-api-key" {
			t.Errorf("tvdb-series login apikey AuthRef = %q, want tvdb-api-key", got)
		}
		return
	}
	t.Fatal("scraper tvdb-series not found")
}

// TestHomeLeadsWithShowsShelf pins that the home layout's first catalog shelf
// after Continue Watching and Next Up is the show Entities, one card per
// show, ahead of the per-episode recently-added shelf (CORE-1786).
func TestHomeLeadsWithShowsShelf(t *testing.T) {
	var home *sdk.LayoutSpec
	layouts := filmtv.Bundle().Layouts
	for i := range layouts {
		if layouts[i].ID == "home" {
			home = &layouts[i]
		}
	}
	if home == nil {
		t.Fatal("no home layout")
	}
	index := map[string]int{}
	for i, section := range home.Sections {
		index[section.ID] = i
	}
	shows, ok := index["browse-shows"]
	if !ok {
		t.Fatal("home layout has no browse-shows section")
	}
	section := home.Sections[shows]
	if section.Kind != "shelf" || section.Source.Kind != "entity-items" || section.Source.EntityKind != "show" {
		t.Fatalf("browse-shows = kind %q source %+v, want a shelf over entity-items of kind show", section.Kind, section.Source)
	}
	if shows != index["next-up"]+1 || shows >= index["recently-added"] {
		t.Fatalf("browse-shows at %d, want directly after next-up (%d) and before recently-added (%d)", shows, index["next-up"], index["recently-added"])
	}
}

// TestHomeRowTitlesNameNoClass pins that the home layout's recently-added and
// all-items rows carry class-neutral titles: the layout serves Film and TV
// Libraries alike, so a Films Library must not read "Movies & Episodes"
// (CORE-2320).
func TestHomeRowTitlesNameNoClass(t *testing.T) {
	want := map[string]sdk.LocaleString{
		"recently-added": {"en": "Recently Added", "fr": "Ajouts récents"},
		"browse-all":     {"en": "All Titles", "fr": "Tous les titres"},
	}
	for _, layout := range filmtv.Bundle().Layouts {
		if layout.ID != "home" {
			continue
		}
		for _, section := range layout.Sections {
			title, ok := want[section.ID]
			if !ok {
				continue
			}
			if !reflect.DeepEqual(section.Title, title) {
				t.Errorf("home section %s title = %v, want %v", section.ID, section.Title, title)
			}
			delete(want, section.ID)
		}
	}
	for id := range want {
		t.Errorf("home layout has no %s section", id)
	}
}

// TestBundleMediaSubtitle pins the CORE-1226 subtitle template and how a host
// renders it for an episode and a movie.
func TestBundleMediaSubtitle(t *testing.T) {
	segments := filmtv.Bundle().MediaSubtitle
	episode := map[string]string{"sequence": "5", "index": "3", filmtv.EpisodeTitleField: "Reunion"}
	lookup := func(values map[string]string) func(string) (string, bool) {
		return func(name string) (string, bool) {
			value, ok := values[name]
			return value, ok
		}
	}
	if got := sdk.RenderMediaSubtitle(segments, "en", lookup(episode)); got != "S5 · E3 · Reunion" {
		t.Errorf("episode subtitle = %q, want S5 · E3 · Reunion", got)
	}
	if got := sdk.RenderMediaSubtitle(segments, "fr", lookup(episode)); got != "S5 · É3 · Reunion" {
		t.Errorf("fr episode subtitle = %q", got)
	}
	delete(episode, filmtv.EpisodeTitleField)
	if got := sdk.RenderMediaSubtitle(segments, "en", lookup(episode)); got != "S5 · E3" {
		t.Errorf("untitled episode subtitle = %q, want S5 · E3", got)
	}
	if got := sdk.RenderMediaSubtitle(segments, "en", lookup(nil)); got != "" {
		t.Errorf("movie subtitle = %q, want empty", got)
	}
}

// TestTMDBRatingMappings pins the ratings slice (CORE-2399): the community
// score fields are declared as numbers, both TMDB scrapers map score and
// votes, movies map the US certification onto the policy and display fields,
// and series map their US TV rating onto the display field only, because TV
// ratings sit outside the MPAA scale that orders Profile ceilings.
func TestTMDBRatingMappings(t *testing.T) {
	b := filmtv.Bundle()
	declared := map[string]string{}
	for _, field := range b.MediaFields {
		declared[field.Name] = field.Type
	}
	for name, typ := range map[string]string{
		filmtv.CommunityRatingField:      "number",
		filmtv.CommunityRatingVotesField: "number",
		"certification":                  "text",
		"certification-display":          "text",
	} {
		if declared[name] != typ {
			t.Errorf("MediaField %q type = %q, want %q", name, declared[name], typ)
		}
	}

	const usMovie = "release_dates.results[iso_3166_1=US].release_dates[type=3].certification" +
		"||release_dates.results[iso_3166_1=US].release_dates[type=4].certification" +
		"||release_dates.results[iso_3166_1=US].release_dates.0.certification"
	score := sdk.MediaFieldKeyPrefix + filmtv.CommunityRatingField
	votes := sdk.MediaFieldKeyPrefix + filmtv.CommunityRatingVotesField
	want := map[string]map[string]string{
		"tmdb-movie": {score: "vote_average", votes: "vote_count",
			"media.certification": usMovie, "media.certification-display": usMovie},
		"tmdb-tv": {score: "vote_average", votes: "vote_count",
			"media.certification-display": "content_ratings.results[iso_3166_1=US].rating"},
	}
	for _, s := range b.Scrapers {
		pins, ok := want[s.ID]
		if !ok {
			continue
		}
		for key, path := range pins {
			if got := s.FieldMap[key]; got != path {
				t.Errorf("%s FieldMap[%q] = %q, want %q", s.ID, key, got, path)
			}
		}
		if s.ID == "tmdb-tv" {
			if got, ok := s.FieldMap["media.certification"]; ok {
				t.Errorf("tmdb-tv maps media.certification = %q; TV ratings must stay display-only", got)
			}
			for key, path := range map[string]string{score: "vote_average", votes: "vote_count"} {
				if got := s.ItemFieldMap[key]; got != path {
					t.Errorf("tmdb-tv ItemFieldMap[%q] = %q, want %q", key, got, path)
				}
			}
		}
		delete(want, s.ID)
	}
	if len(want) > 0 {
		t.Errorf("missing scrapers: %v", want)
	}
}

// CORE-2535: this layout also serves TV Libraries, including unmatched files
// that default to movie. Only deliberate catalog browsing should offer that split.
func TestHomeHasNoMovieOnlyRail(t *testing.T) {
	home, ok := filmtv.Bundle().BrowseLayout()
	if !ok {
		t.Fatal("missing browse layout")
	}
	all := false
	for _, section := range home.Sections {
		if section.Kind != "filter-bar" && section.Source.FieldFilters["class"] == "movie" {
			t.Errorf("movie-only home section %q promotes unmatched files in TV Libraries", section.ID)
		}
		if section.ID == "browse-all" && section.Source.Kind == "all-items" && len(section.Source.FieldFilters) == 0 {
			all = true
		}
	}
	if !all {
		t.Error("loose Media lost their deliberate All Titles browse")
	}
}
