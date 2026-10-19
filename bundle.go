// Package filmtv is the first-party Film & TV Domain Plugin for Beamfall: one
// bundle covering both movies and episodes. It is a single plugin rather than
// separate "movies" and "TV" plugins because real libraries have mixed
// folders, and the parse stage must classify movie-vs-episode per file
// regardless. Film & TV vocabulary lives here, never in Core or the SDK.
//
// Bundle (this file) is pure declarations: entity kinds against the fixed
// archetype vocabulary, a home Layout against the fixed Section/Source
// vocabularies, and declarative scrapers. A Movie and an Episode are Media,
// never Entities. The plugin's compiled hooks (the filename Parser in parse.go,
// the match Scorer in score.go) sit beside the data bundle, not inside it.
package filmtv

import (
	sdk "github.com/beamfall/plugin-sdk"
	"github.com/beamfall/plugin-sdk/scrobble"
)

// tmdbMaxCastLookups caps the per-identification secondary /person/{id}
// detail fetches the credited-in EntityMap issues against credits.cast
// (credits.cast): a credits document can list 50+ names, but the
// detail fetch fires once per name, so this bounds identification to the
// top-billed cast rather than an unbounded roster of requests.
const tmdbMaxCastLookups = 10

// Bundle returns the complete declarative Film & TV contribution.
func Bundle() sdk.Bundle {
	return sdk.Bundle{
		ID:          PluginID,
		Version:     "0.1.4",
		SDKFloor:    "0.3.2",
		SDKCeiling:  "0.3.2",
		DisplayName: sdk.LocaleString{"en": "Film & TV", "fr": "Films et séries"},
		// Per-item display labels key off the typed Media field
		// `class: movie | episode`; MediaLabel stays the class-neutral
		// fallback for unmapped class values.
		MediaLabel:   sdk.LocaleString{"en": "Video", "fr": "Vidéo"},
		MediaClasses: []string{"video"},
		// Library accent (a theme token): a warm cinema amber,
		// distinct from the beam brand accent. Clients derive the
		// artwork-fallback tint and filter selection state from it.
		Accent: "#C9803F",
		// The `class` keystone: the parse/identification stage
		// classifies every file movie-vs-episode and writes this Media field.
		// The values are this plugin's data, never core vocabulary.
		MediaFields: []sdk.FieldSpec{
			{Name: "class", Type: "text",
				Label: sdk.LocaleString{"en": "Class", "fr": "Classe"}},
			// Certification (regulatory label, policy — never the TMDB
			// score) on the Media item itself; the scale below orders
			// these values for Profile policy ceilings.
			{Name: "certification", Type: "text",
				Label: sdk.LocaleString{"en": "Certification", "fr": "Classification"}},
			// Incumbent display fields: the attached display metadata
			// Plex/Emby/Jellyfin surface, declared as typed plugin fields
			// so identification does not silently drop them. These are
			// display-only — none feed the Profile policy ceiling (that stays the "certification" field + scale above).
			{Name: "original-title", Type: "text", Searchable: true,
				Label: sdk.LocaleString{"en": "Original Title", "fr": "Titre original"}},
			{Name: "tagline", Type: "text",
				Label: sdk.LocaleString{"en": "Tagline", "fr": "Accroche"}},
			// The per-region certification display set (distinct from
			// the enforcement ceiling scale — this is the
			// human-facing label to show, not the ordered policy value).
			{Name: "certification-display", Type: "text",
				Label: sdk.LocaleString{"en": "Rating", "fr": "Classification affichée"}},
			// release-date vs air-date: a movie carries a
			// theatrical release date, an episode/show an air date — kept
			// as separate fields so neither is coerced into the other.
			{Name: "release-date", Type: "date", Sortable: true,
				Label: sdk.LocaleString{"en": "Release Date", "fr": "Date de sortie"}},
			{Name: "air-date", Type: "date", Sortable: true,
				Label: sdk.LocaleString{"en": "Air Date", "fr": "Date de diffusion"}},
			// Production status: Returning/Ended/Canceled —
			// display data written verbatim from TMDB's `status`.
			{Name: "production-status", Type: "text", Filterable: true,
				Label: sdk.LocaleString{"en": "Status", "fr": "Statut"}},
			// Network (the network side of the Studio Kind's
			// production-company-vs-network distinction). TMDB TV carries a
			// series' originating networks separately from its production
			// companies; the primary network name lands here so the home
			// filter-bar can offer a network browse facet keyed on the
			// `class`-neutral Media field. Movies carry no network — the
			// field stays absent there, keeping the facet TV-scoped.
			{Name: "network", Type: "text", Filterable: true,
				Label: sdk.LocaleString{"en": "Network", "fr": "Chaîne"}},
		},
		MediaClassLabels: map[string]sdk.LocaleString{
			"movie":   {"en": "Movie", "fr": "Film"},
			"episode": {"en": "Episode", "fr": "Épisode"},
		},
		// Scrobble adapters consume only bounded target shapes. The plugin-owned
		// class field selects that shape exactly; absent and unknown values stay
		// unmapped so Core never guesses from Film/TV vocabulary.
		ScrobbleTarget: &sdk.ScrobbleTargetSpec{
			MediaField: "class",
			Mappings: []sdk.ScrobbleTargetMapping{
				{Value: "movie", Hint: scrobble.TargetStandalone},
				{Value: "episode", Hint: scrobble.TargetIndexed},
			},
		},
		// The US MPA movie scale, low→high (the ordered scale Profile
		// ceilings compare against; country × media-family scale tables arrive with the
		// certification-region setting later).
		CertificationScale: []string{"G", "PG", "PG-13", "R", "NC-17"},
		EntityKinds: []sdk.EntityKind{
			{
				// Show: ordered Seasons.
				Name:            "show",
				DisplayName:     sdk.LocaleString{"en": "Show", "fr": "Série"},
				Archetype:       "container",
				OrderedChildren: true,
				Fields: []sdk.FieldSpec{
					{Name: "first-air-date", Type: "date", Sortable: true,
						Label: sdk.LocaleString{"en": "First Air Date", "fr": "Première diffusion"}},
					{Name: "status", Type: "text", Filterable: true,
						Label: sdk.LocaleString{"en": "Status", "fr": "Statut"}},
					{Name: "certification", Type: "text",
						Label: sdk.LocaleString{"en": "Certification", "fr": "Classification"}},
					{Name: "tmdb-id", Type: "text",
						Label: sdk.LocaleString{"en": "TMDB ID", "fr": "Identifiant TMDB"}},
				},
				Relations: []sdk.RelationSpec{
					// Cast/crew is a relation with attributes, never a Kind:
					// role/character/billing/department
					// distinguish actor/director/writer.
					{Name: "credit", ToKind: "person",
						Attributes: []string{"role", "character", "billing", "department"}},
					{Name: "produced-by", ToKind: "studio"},
					{Name: "in-genre", ToKind: "genre"},
				},
			},
			{
				// Season: ordered Episode Media, play-next.
				// TMDB Season 0 is a real Season named "Specials", sorted
				// last, excluded from play-next auto-advance — the
				// sort/auto-advance exclusion is server policy, not
				// expressible in this declaration.
				Name:            "season",
				DisplayName:     sdk.LocaleString{"en": "Season", "fr": "Saison"},
				Archetype:       "container",
				OrderedChildren: true,
				Fields: []sdk.FieldSpec{
					{Name: "number", Type: "number", Sortable: true,
						Label: sdk.LocaleString{"en": "Number", "fr": "Numéro"}},
					{Name: "air-date", Type: "date",
						Label: sdk.LocaleString{"en": "Air Date", "fr": "Date de diffusion"}},
					{Name: "specials", Type: "flag",
						Label: sdk.LocaleString{"en": "Specials", "fr": "Épisodes spéciaux"}},
				},
			},
			{
				// Film Series: TMDB's "Collection" concept, renamed because
				// core Collection means user-curated. Ordered
				// grid by release.
				Name:            "film-series",
				DisplayName:     sdk.LocaleString{"en": "Film Series", "fr": "Saga de films"},
				Archetype:       "container",
				OrderedChildren: true,
				Fields: []sdk.FieldSpec{
					{Name: "tmdb-id", Type: "text",
						Label: sdk.LocaleString{"en": "TMDB ID", "fr": "Identifiant TMDB"}},
				},
			},
			{
				// One Kind for all humans; the credit relation's attributes
				// distinguish actor/director/writer.
				Name:        "person",
				DisplayName: sdk.LocaleString{"en": "Person", "fr": "Personne"},
				Archetype:   "person",
				Fields: []sdk.FieldSpec{
					{Name: "birth-date", Type: "date",
						Label: sdk.LocaleString{"en": "Birth Date", "fr": "Date de naissance"}},
					{Name: "death-date", Type: "date",
						Label: sdk.LocaleString{"en": "Death Date", "fr": "Date de décès"}},
					{Name: "birthplace", Type: "text",
						Label: sdk.LocaleString{"en": "Birthplace", "fr": "Lieu de naissance"}},
					{Name: "biography", Type: "text", Searchable: true,
						Label: sdk.LocaleString{"en": "Biography", "fr": "Biographie"}},
					{Name: "homepage", Type: "url",
						Label: sdk.LocaleString{"en": "Homepage", "fr": "Site officiel"}},
				},
				Relations: []sdk.RelationSpec{
					// Guest-star credit scoped to the Episode Media, not the
					// Show container (per-episode credits — writer/director/
					// guest stars — attach at the episode, the declared
					// extension point beyond the Season relation). A
					// person guest-starring in one episode links to that
					// episode's Media through this Person→Media relation,
					// distinct from the show-level `credit` relation on the show
					// container; the `character` attribute carries the guest
					// role.
					{Name: "guest-starred-in", ToKind: sdk.MediaKindName,
						Attributes: []string{"character"}},
					// Full cast/crew credit: the
					// tmdb-movie/tmdb-tv scrapers identify Media directly, never
					// a "show" Entity, so the show-level `credit` relation above
					// is unreachable by any declared scraper — EntityMap only
					// hydrates a Media-targeted relation declared on the Kind it
					// instantiates. This Person→Media relation is the one the
					// scraper's credits.cast EntityMap actually writes; the
					// role/character/billing/department attributes match
					// `credit`'s set so a future Show detail Layout can read
					// either shape.
					{Name: "credited-in", ToKind: sdk.MediaKindName,
						Attributes: []string{"role", "character", "billing", "department"}},
				},
			},
			{
				// One category Kind for production companies and networks;
				// the function field carries the distinction.
				Name:        "studio",
				DisplayName: sdk.LocaleString{"en": "Studio", "fr": "Studio"},
				Archetype:   "category",
				Fields: []sdk.FieldSpec{
					{Name: "function", Type: "text", Filterable: true,
						Label: sdk.LocaleString{"en": "Function", "fr": "Fonction"}},
					{Name: "homepage", Type: "url",
						Label: sdk.LocaleString{"en": "Homepage", "fr": "Site officiel"}},
				},
				Relations: []sdk.RelationSpec{
					// Studio → Media: the per-item production-company link
					// identification hydrates (the show-level "produced-by"
					// above covers Show → Studio).
					{Name: "produced", ToKind: sdk.MediaKindName},
				},
			},
			{
				Name:        "genre",
				DisplayName: sdk.LocaleString{"en": "Genre", "fr": "Genre"},
				Archetype:   "category",
				Relations: []sdk.RelationSpec{
					// Genre → Media: the per-item genre link identification
					// hydrates from TMDB's genres list.
					{Name: "genre-of", ToKind: sdk.MediaKindName},
				},
			},
		},
		// The Library home. The class-split "Movies" / "Episodes" browse
		// grids are the mixed-Library split, expressed as the Source
		// contract's declarative FieldFilters over the `class` keystone.
		// The genre shelves are the Source contract's EntityFilter: "Media related to the genre entity
		// named N in this Library" (the Comedy shelf below proves it; more
		// shelves are a curation pass, not new vocabulary). The catch-all
		// grid stays last: Media not yet identified carries no class field
		// and would otherwise be unreachable. Section titles ship as a
		// full locale map: en + fr today, with more locales to follow.
		Layouts: []sdk.LayoutSpec{
			{ID: "home", Role: "browse", Sections: []sdk.SectionSpec{
				// The hero reads "most-played":
				// "recently-added" is already the shelf below, and a shared
				// source between two home Sections repeats the same titles
				// in both places by construction. "most-played" is a Source
				// Kind the SDK already declares (source.go) and no other
				// home Section reads, so the hero spotlights the
				// household's most-watched titles instead of a duplicate
				// of the shelf.
				{ID: "home-hero", Kind: "hero",
					Title:       sdk.LocaleString{"en": "Most Played", "fr": "Les plus regardés"},
					ArtworkSlot: "backdrop", Source: sdk.SourceSpec{Kind: "most-played"}},
				{ID: "continue-watching", Kind: "shelf",
					Title:       sdk.LocaleString{"en": "Continue Watching", "fr": "Reprendre la lecture"},
					ArtworkSlot: "poster", Source: sdk.SourceSpec{Kind: "continue-watching"}},
				// next-up sits beside continue-watching, never merged with
				// it (Plex's merged On Deck is the cautionary tale).
				{ID: "next-up", Kind: "shelf",
					Title:       sdk.LocaleString{"en": "Next Up", "fr": "À suivre"},
					ArtworkSlot: "poster", Source: sdk.SourceSpec{Kind: "next-up"}},
				{ID: "recently-added", Kind: "shelf",
					Title:       sdk.LocaleString{"en": "New Movies & Episodes", "fr": "Nouveaux films et épisodes"},
					ArtworkSlot: "poster", Source: sdk.SourceSpec{Kind: "recently-added"}},
				// The per-genre shelf: catalog feed narrowed to
				// the Media the named genre entity relates to. "Comedy" is
				// TMDB's canonical genre name, hydrated verbatim by the
				// EntityMaps above — the static name is safe because TMDB's
				// genre vocabulary is fixed and the tmdb-movie scraper writes
				// it untranslated regardless of operator locale.
				{ID: "comedy", Kind: "shelf",
					Title:       sdk.LocaleString{"en": "Comedy", "fr": "Comédie"},
					ArtworkSlot: "poster",
					Source: sdk.SourceSpec{Kind: "all-items",
						EntityFilter: &sdk.EntityFilter{Kind: "genre", Name: "Comedy"}}},
				{ID: "browse-movies", Kind: "grid",
					Title:       sdk.LocaleString{"en": "Movies", "fr": "Films"},
					ArtworkSlot: "poster",
					Source: sdk.SourceSpec{Kind: "all-items",
						FieldFilters: map[string]any{"class": "movie"}}},
				{ID: "browse-episodes", Kind: "grid",
					Title:       sdk.LocaleString{"en": "Episodes", "fr": "Épisodes"},
					ArtworkSlot: "poster",
					Source: sdk.SourceSpec{Kind: "all-items",
						FieldFilters: map[string]any{"class": "episode"}}},
				// The filter-bar's declared options narrow the adjacent grid
				// (browse-all, directly below) by the `class` keystone —
				// view-time FieldFilters the user picks instead of the
				// declaration-time split above.
				{ID: "browse-filter", Kind: "filter-bar",
					Title:  sdk.LocaleString{"en": "Browse", "fr": "Parcourir"},
					Source: sdk.SourceSpec{Kind: "all-items"},
					FilterOptions: []sdk.FilterOption{
						{Field: "class", Value: "movie",
							Label: sdk.LocaleString{"en": "Movies", "fr": "Films"}},
						{Field: "class", Value: "episode",
							Label: sdk.LocaleString{"en": "Episodes", "fr": "Épisodes"}},
						// The network browse facet (the network side of the
						// Studio distinction): the same filter-bar mechanism
						// narrows the adjacent grid to a broadcast network's
						// Media. Networks are plugin data hydrated verbatim by
						// the tmdb-tv scraper's media.network mapping below — the
						// built-in options are the major US networks TMDB names
						// untranslated; more are a curation pass, not new
						// vocabulary.
						{Field: "network", Value: "HBO",
							Label: sdk.L("HBO")},
						{Field: "network", Value: "Netflix",
							Label: sdk.L("Netflix")},
					}},
				// "all-items" here carries no FieldFilters, so it returns
				// every Media row — movie
				// and episode class values, never a show-level Entity (a
				// Movie and an Episode are Media, never Entities, per the
				// package doc above). The label names exactly that.
				{ID: "browse-all", Kind: "grid",
					Title:       sdk.LocaleString{"en": "All Movies & Episodes", "fr": "Tous les films et épisodes"},
					ArtworkSlot: "poster", Source: sdk.SourceSpec{Kind: "all-items"}},
			}},
			// The Season detail Layout (episode list, thumb + synopsis) —
			// the proving case for additional Layouts.
			// The "episodes of this Season" feed is the entity-scoped
			// entity-media Source: the Media related to the entity the
			// detail view is scoped to.
			{ID: "season-detail", Role: "detail", EntityKind: "season",
				Sections: []sdk.SectionSpec{
					{ID: "episodes", Kind: "list",
						Title:       sdk.LocaleString{"en": "Episodes", "fr": "Épisodes"},
						ArtworkSlot: "thumb", Source: sdk.SourceSpec{Kind: "entity-media"}},
				}},
			// The Movie detail Layout (info). The
			// info Section is where the incumbent display fields surface —
			// original title, tagline, release date, per-region certification
			// display, and production status render off the Media item this
			// detail view is scoped to (the entity-media Source, as on the
			// season list). This is the Media-item default detail (no
			// EntityKind): a Movie and an Episode are Media, never Entities.
			{ID: "movie-detail", Role: "detail",
				Sections: []sdk.SectionSpec{
					{ID: "movie-info", Kind: "info",
						Title:  sdk.LocaleString{"en": "Details", "fr": "Détails"},
						Source: sdk.SourceSpec{Kind: "entity-media"}},
				}},
			// The Show detail Layout (info · Seasons · cast). Its info
			// Section surfaces the same display fields at
			// the show-container level (a returning/ended status, first-air
			// vs original title). Bound to the show Kind so clients pick this
			// detail Layout for a Show entity.
			{ID: "show-detail", Role: "detail", EntityKind: "show",
				Sections: []sdk.SectionSpec{
					{ID: "show-info", Kind: "info",
						Title:  sdk.LocaleString{"en": "Details", "fr": "Détails"},
						Source: sdk.SourceSpec{Kind: "entity-media"}},
				}},
		},
		// TMDB is the primary source: operator-registered key,
		// named by AuthRef (Console config field), never embedded. The v3
		// key travels as the classic api_key query parameter (the
		// "query:api_key" auth style). Identification is the full
		// declarative chain: SearchTemplate → results candidates (the search-element
		// and details-document shapes share keys, so one FieldMap serves
		// both) → the chosen candidate's `id` feeds LookupTemplate.
		//
		// TVDB is declared as a data-only addition — pure declarations,
		// no hooks. Its v4
		// POST /login JWT exchange is the declarative "login-bearer" style.
		// TVDB's search elements (top-level name/overview/image_url) diverge
		// from its data-enveloped details document, so the search leg gets
		// its own SearchFieldMap/SearchArtworkMap — the full chain works.
		Scrapers: []sdk.ScraperSpec{
			{
				ID:          "tmdb-movie",
				DisplayName: sdk.L("TMDB (Movies)"),
				Kind:        "http-json",
				Endpoint:    "https://api.themoviedb.org/3",
				AuthRef:     "tmdb-api-key",
				AuthStyle:   "query:api_key",
				// GET /search/movie — {"results": [{id, title, overview,
				// release_date, poster_path, backdrop_path}, …]}.
				SearchTemplate:  "/search/movie?query={query}",
				CandidatesPath:  "results",
				CandidateIDPath: "id",
				// GET /movie/{id} — TMDB movie details shape (same keys as
				// a search element). append_to_response=credits folds the
				// cast/crew list onto this same document
				// rather than a separate /movie/{id}/credits request — the
				// credited-in EntityMap below resolves credits.cast against it.
				LookupTemplate: "/movie/{id}?append_to_response=credits",
				LookupByIDTemplate: map[string]string{
					"tmdb": "/movie/{id}?append_to_response=credits",
				},
				FieldMap: map[string]string{
					"title":   "title",
					"summary": "overview",
					// release_date is "YYYY-MM-DD" → year transform below.
					"year": "release_date",
					// popularity is TMDB's ranking float, carried onto the
					// Candidate as the scorer's tiebreak-only signal (never
					// persisted as Media metadata — a candidate-signal bare key).
					"popularity": "popularity",
					// Incumbent display fields. release_date
					// also lands raw on media.release-date (untransformed,
					// distinct from an episode's air-date); status is
					// "Released"/"In Production"/… on movies.
					"media.original-title":    "original_title",
					"media.tagline":           "tagline",
					"media.release-date":      "release_date",
					"media.production-status": "status",
				},
				Transforms: map[string]string{"year": "year-from-date"},
				// poster_path / backdrop_path are site-relative ("/abc.jpg");
				// composed onto the TMDB image CDN prefix.
				ArtworkBaseURL: "https://image.tmdb.org/t/p/original",
				ArtworkMap: map[string]string{
					"poster":   "poster_path",
					"backdrop": "backdrop_path",
				},
				// Identifying through the movie scraper IS the movie-vs-episode
				// classification: a successful apply writes the
				// `class` keystone. The "media." prefix is the SDK's namespacing
				// contract — plugin Media fields, never bare built-in keys.
				ImpliedMediaFields: map[string]string{"media.class": "movie"},
				// GET /movie/{id} carries genres: [{id, name}, …] and
				// production_companies: [{id, name, …}, …] — applying an
				// identification hydrates each name as an Entity in the
				// Media's Library and links the declared Media relation.
				EntityMaps: []sdk.EntityMap{
					{Kind: "genre", Relation: "genre-of", Path: "genres", NamePath: "name"},
					{Kind: "studio", Relation: "produced", Path: "production_companies", NamePath: "name"},
					// credits.cast (folded on by append_to_response above)
					// carries only id/name/character per person — a name-only
					// hydration would leave Person an empty entity shell. The
					// secondary GET /person/{id} lookup
					// fetches the field-populated document per top-billed cast
					// member, capped at tmdbMaxCastLookups.
					{Kind: "person", Relation: "credited-in", Path: "credits.cast",
						NamePath: "name", ArtworkPath: "profile_path",
						LookupTemplate: "/person/{id}", LookupIDPath: "id",
						MaxLookups: tmdbMaxCastLookups,
						FieldMap: map[string]string{
							"birth-date": "birthday",
							"death-date": "deathday",
							"birthplace": "place_of_birth",
							"biography":  "biography",
							"homepage":   "homepage",
						}},
				},
			},
			{
				ID:          "tmdb-tv",
				DisplayName: sdk.L("TMDB (TV)"),
				Kind:        "http-json",
				Endpoint:    "https://api.themoviedb.org/3",
				AuthRef:     "tmdb-api-key",
				AuthStyle:   "query:api_key",
				// GET /search/tv — {"results": [{id, name, overview,
				// first_air_date, poster_path, backdrop_path}, …]}.
				SearchTemplate:  "/search/tv?query={query}",
				CandidatesPath:  "results",
				CandidateIDPath: "id",
				// GET /tv/{id} — TMDB TV series details shape.
				// append_to_response=credits folds the cast/crew list onto
				// this document, matching tmdb-movie —
				// the credited-in EntityMap below resolves credits.cast.
				LookupTemplate: "/tv/{id}?append_to_response=credits",
				LookupByIDTemplate: map[string]string{
					"tmdb": "/tv/{id}?append_to_response=credits",
				},
				FieldMap: map[string]string{
					"title":   "name",
					"summary": "overview",
					"year":    "first_air_date",
					// popularity is TMDB's ranking float, carried onto the
					// Candidate as the scorer's tiebreak-only signal (never
					// persisted as Media metadata — a candidate-signal bare key).
					"popularity": "popularity",
					// Incumbent display fields. TV keys the
					// original title off original_name and the date off
					// first_air_date (the air-date field, not release-date);
					// status is Returning Series/Ended/Canceled — the
					// production-status values verbatim.
					"media.original-title":    "original_name",
					"media.tagline":           "tagline",
					"media.air-date":          "first_air_date",
					"media.production-status": "status",
					// Network browse facet: the series' primary
					// originating network name — TMDB's networks[] is the
					// network side of the Studio distinction, separate from
					// production_companies. The first element is the primary
					// network the home filter-bar facet keys on.
					"media.network": "networks.0.name",
				},
				Transforms:     map[string]string{"year": "year-from-date"},
				ArtworkBaseURL: "https://image.tmdb.org/t/p/original",
				ArtworkMap: map[string]string{
					"poster":   "poster_path",
					"backdrop": "backdrop_path",
					// Episode still slot (the season list's "thumb +
					// synopsis"). TMDB's episode still lands on still_path;
					// composed onto the same image CDN prefix and surfaced in
					// the fixed "thumb" artwork slot the season-detail episode
					// list already renders.
					"thumb": "still_path",
				},
				// A TV-shaped identification means the file is an episode.
				ImpliedMediaFields: map[string]string{"media.class": "episode"},
				// GET /tv/{id} carries genres: [{id, name}, …] and
				// production_companies: [{id, name, …}, …] too — same
				// shape as the movie details document, so applying an
				// identification hydrates each name as an Entity in the
				// Media's Library and links the declared Media relation.
				EntityMaps: []sdk.EntityMap{
					{Kind: "genre", Relation: "genre-of", Path: "genres", NamePath: "name"},
					{Kind: "studio", Relation: "produced", Path: "production_companies", NamePath: "name"},
					// Same shape as tmdb-movie's credited-in map above
					// (TestTMDBTVEntityMaps pins the two scrapers' EntityMaps
					// stay identical in kind/relation/path/name-path).
					{Kind: "person", Relation: "credited-in", Path: "credits.cast",
						NamePath: "name", ArtworkPath: "profile_path",
						LookupTemplate: "/person/{id}", LookupIDPath: "id",
						MaxLookups: tmdbMaxCastLookups,
						FieldMap: map[string]string{
							"birth-date": "birthday",
							"death-date": "deathday",
							"birthplace": "place_of_birth",
							"biography":  "biography",
							"homepage":   "homepage",
						}},
				},
			},
			{
				ID:          "tvdb-series",
				DisplayName: sdk.L("TheTVDB (Series)"),
				Kind:        "http-json",
				Endpoint:    "https://api4.thetvdb.com/v4",
				AuthRef:     "tvdb-api-key",
				// TVDB v4 auth: POST /login {"apikey": <secret>} →
				// {"data": {"token": <JWT>}}, then Bearer — pure data.
				AuthStyle: "login-bearer",
				Login: &sdk.LoginSpec{
					Path:        "/login",
					SecretField: "apikey",
					TokenPath:   "data.token",
				},
				// GET /search?query=…&type=series — {"status": "success",
				// "data": [{objectID, tvdb_id, name, overview, year,
				// image_url, …}, …]}: flat elements, no per-item envelope.
				SearchTemplate: "/search?query={query}&type=series",
				CandidatesPath: "data",
				// tvdb_id is the bare numeric id ("121361") that /series/{id}
				// takes; the element's `id` is the prefixed objectID form
				// ("series-121361") and cannot feed the lookup.
				CandidateIDPath: "tvdb_id",
				// GET /series/{id} — TVDB v4 wraps payloads in
				// {"status": "success", "data": {...}}.
				LookupTemplate: "/series/{id}",
				LookupByIDTemplate: map[string]string{
					"tvdb": "/series/{id}",
				},
				FieldMap: map[string]string{
					"title":   "data.name",
					"summary": "data.overview",
					// TVDB v4 carries year as a plain string already.
					"year": "data.year",
				},
				ArtworkMap: map[string]string{
					// data.image is an absolute artworks.thetvdb.com URL.
					"poster": "data.image",
				},
				// The search elements are flat — per-template maps carry the
				// divergent shape (top-level keys, image_url for art).
				SearchFieldMap: map[string]string{
					"title":   "name",
					"summary": "overview",
					"year":    "year",
				},
				SearchArtworkMap: map[string]string{
					// image_url is absolute, like data.image on the lookup.
					"poster": "image_url",
				},
				// Series-shaped, like tmdb-tv: the file is an episode.
				ImpliedMediaFields: map[string]string{"media.class": "episode"},
			},
		},
		// The three Library family scopes this plugin offers:
		// one plugin, three Library "types". A manager picks one at creation;
		// the scope tells identification the family deterministically instead
		// of guessing per file. "films" pins every file to class=movie,
		// "tvshows" pins class=episode; "mixed" is the default and keeps
		// today's per-file parse classification (the catch-all that preserves
		// existing behavior when no scope is chosen). The values are this
		// plugin's data, keyed off — but never schema-bound to — the `class`
		// keystone above; "family scope" is the generic core vocabulary.
		FamilyScopes: []sdk.FamilyScopeSpec{
			{Value: "films",
				DisplayName: sdk.LocaleString{"en": "Films", "fr": "Films"}},
			{Value: "tvshows",
				DisplayName: sdk.LocaleString{"en": "TV Shows", "fr": "Séries"}},
			{Value: "mixed", Default: true,
				DisplayName: sdk.LocaleString{"en": "Mixed", "fr": "Mixte"}},
		},
		LabelMaps: []sdk.LabelMapSpec{
			{Label: "action", Tag: "action"},
			{Label: "comedy", Tag: "comedy"},
			{Label: "drama", Tag: "drama"},
			{Label: "horror", Tag: "horror"},
			{Label: "romance", Tag: "romance"},
			{Label: "science fiction", Tag: "science-fiction"},
		},
		// Migration Wizard importers: the four Film & TV sources a user
		// switching from another media server most often carries history
		// from. These are declarations only — the matching compiled
		// sdk.Source implementations live in sources.go, and Core's resolver
		// wires (filmtv, importerID) → the constructor there. Categories come
		// from the fixed import-category vocabulary; "additive" is the only
		// conflict strategy (re-runnable, gap-fill / advance-only).
		//
		// Plex/Emby/Jellyfin read the foreign server's SQLite database
		// (export-file input) and carry watch-state — resume position +
		// completion + the historical timestamp. NFO walks a
		// sidecar tree (export-file input = the library folder) and carries its
		// TMDB/IMDB parse-hint ids + local artwork; it is
		// declared under "collections" as the closest fixed category for that
		// metadata-hint material (the v1 handler applies engagement for
		// watch-state only, so an NFO run reports the match rate and pins matched
		// files while identification consumes the ids).
		Importers: []sdk.ImporterSpec{
			{
				ID:          "plex",
				DisplayName: sdk.LocaleString{"en": "Plex", "fr": "Plex"},
				Categories:  []string{"watch-state"},
				Input:       "export-file",
				Conflict:    "additive",
			},
			{
				ID:          "emby",
				DisplayName: sdk.LocaleString{"en": "Emby", "fr": "Emby"},
				Categories:  []string{"watch-state"},
				Input:       "export-file",
				Conflict:    "additive",
			},
			{
				ID:          "jellyfin",
				DisplayName: sdk.LocaleString{"en": "Jellyfin", "fr": "Jellyfin"},
				Categories:  []string{"watch-state"},
				Input:       "export-file",
				Conflict:    "additive",
			},
			{
				ID:          "nfo",
				DisplayName: sdk.LocaleString{"en": "NFO sidecars", "fr": "Fichiers NFO"},
				Categories:  []string{"collections"},
				Input:       "export-file",
				Conflict:    "additive",
			},
			{
				ID:          "trakt",
				DisplayName: sdk.LocaleString{"en": "Trakt history", "fr": "Historique Trakt"},
				Categories:  []string{"watch-state"},
				Input:       "export-file",
				Conflict:    "additive",
			},
		},
	}
}
