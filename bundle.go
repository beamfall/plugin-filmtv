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

// tmdbKeyCrewJobs are the credits.crew jobs imported as Person credits
// (CORE-2503); the rest of a crew roster is not.
var tmdbKeyCrewJobs = []string{
	"Director", "Screenplay", "Writer", "Producer",
	"Original Music Composer", "Director of Photography",
}

// tmdbCrewOrdinalOffset sorts every crew credit after the cast: TMDB billing
// orders stay well under it.
const tmdbCrewOrdinalOffset = 1000

// tmdbKeyCrewMap hydrates the key crew of a TMDB credits document as
// credited-in Person credits carrying job and department, in TMDB's listing
// order after the cast. Crew elements carry the name and headshot; there is
// no /person lookup, so a crew-only Person stays field-less until a cast
// credit or another title hydrates it.
func tmdbKeyCrewMap() sdk.EntityMap {
	return sdk.EntityMap{Kind: "person", Relation: "credited-in", Path: "credits.crew",
		NamePath: "name", ArtworkPath: "profile_path",
		RelationAttributeMap: map[string]string{"job": "job", "department": "department"},
		ElementFilter:        map[string][]string{"job": tmdbKeyCrewJobs},
		OrdinalOffset:        tmdbCrewOrdinalOffset}
}

// Community score fields (CORE-2399): core-conventional Media field names the
// detail page reads for its score chip.
const (
	CommunityRatingField      = "community-rating"
	CommunityRatingVotesField = "community-rating-votes"
)

// TMDB lookups with the documents that carry ratings folded on: credits for
// the credited-in maps, release_dates (movies) and content_ratings (series)
// for the certification fields.
const (
	tmdbMovieLookup = "/movie/{id}?append_to_response=credits,release_dates"
	tmdbTVLookup    = "/tv/{id}?append_to_response=credits,content_ratings"
)

// tmdbMovieUSCertification picks the US theatrical (type 3) certification,
// else the digital (type 4) one, else the first US release's. Blank values
// fall through to the next alternative and a blank result is never stored, so
// a title with no US certification stays unrated.
const tmdbMovieUSCertification = "release_dates.results[iso_3166_1=US].release_dates[type=3].certification" +
	"||release_dates.results[iso_3166_1=US].release_dates[type=4].certification" +
	"||release_dates.results[iso_3166_1=US].release_dates.0.certification"

// Bundle returns the complete declarative Film & TV contribution.
func Bundle() sdk.Bundle {
	return sdk.Bundle{
		ID:          PluginID,
		Version:     "0.1.4",
		SDKFloor:    "0.4.7",
		SDKCeiling:  "0.4.7",
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
			// An episode's own name ("Earthfall"), read from the filename by
			// the parser (parse.go ParseMediaFields). The parsed title stays the
			// series name; the display title (local.go MediaTitle) carries the
			// episode code and this name.
			{Name: EpisodeTitleField, Type: "text", Searchable: true,
				Label: sdk.LocaleString{"en": "Episode Title", "fr": "Titre de l'épisode"}},
			// The other title of a dual-titled name ("Голубая волна | Blue
			// Crush"), read from the filename by the parser: the parsed title
			// is the Latin one, so library search still finds the native one.
			{Name: AlternateTitleField, Type: "text", Searchable: true,
				Label: sdk.LocaleString{"en": "Alternate Title", "fr": "Titre alternatif"}},
			// Set by Core's hydration seam once a file's local show and
			// season containers are written (local.go).
			{Name: HydratedField, Type: "flag",
				Label: sdk.LocaleString{"en": "Grouped Locally", "fr": "Regroupé localement"}},
			{Name: "tagline", Type: "text",
				Label: sdk.LocaleString{"en": "Tagline", "fr": "Accroche"}},
			// The per-region certification display set (distinct from
			// the enforcement ceiling scale — this is the
			// human-facing label to show, not the ordered policy value).
			{Name: "certification-display", Type: "text",
				Label: sdk.LocaleString{"en": "Rating", "fr": "Classification affichée"}},
			// release-date vs air-date: a movie carries a
			// theatrical release date, an episode/show an air date — kept
			// as separate fields so neither is coerced into the other. The
			// parser also fills air-date for a dated episode (AirDateField).
			{Name: "release-date", Type: "date", Sortable: true,
				Label: sdk.LocaleString{"en": "Release Date", "fr": "Date de sortie"}},
			{Name: AirDateField, Type: "date", Sortable: true,
				Label: sdk.LocaleString{"en": "Air Date", "fr": "Date de diffusion"}},
			// Production status: Returning/Ended/Canceled —
			// display data written verbatim from TMDB's `status`.
			{Name: "production-status", Type: "text", Filterable: true,
				Label: sdk.LocaleString{"en": "Status", "fr": "Statut"}},
			// Network (the network side of the Studio Kind's
			// production-company-vs-network distinction). TMDB TV carries a
			// series' originating networks separately from its production
			// companies; the primary network name lands here as a
			// filterable, `class`-neutral Media field. The home filter-bar
			// declares no fixed network options (CORE-1711). Movies carry
			// no network — the field stays absent there.
			{Name: "network", Type: "text", Filterable: true,
				Label: sdk.LocaleString{"en": "Network", "fr": "Chaîne"}},
			// Community score (CORE-2399): the provider's 0-10 user score
			// and its vote count, display-only and never a policy value.
			// Core reads these core-conventional names for the detail
			// page's score chip; the label names the source.
			{Name: CommunityRatingField, Type: "number", Sortable: true,
				Label: sdk.LocaleString{"en": "TMDB rating", "fr": "Note TMDB"}},
			{Name: CommunityRatingVotesField, Type: "number",
				Label: sdk.LocaleString{"en": "TMDB votes", "fr": "Votes TMDB"}},
		},
		MediaClassLabels: map[string]sdk.LocaleString{
			"movie":   {"en": "Movie", "fr": "Film"},
			"episode": {"en": "Episode", "fr": "Épisode"},
		},
		// How sibling Media differ on a card (CORE-1226): season and
		// episode ordinals plus the episode's own title. A movie carries none
		// of these values, so it renders no subtitle.
		MediaSubtitle: []sdk.LocaleString{
			{"en": "S{sequence}", "fr": "S{sequence}"},
			{"en": "E{index}", "fr": "É{index}"},
			{"en": "{" + EpisodeTitleField + "}"},
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
		// How this catalog presents to external automation tools through
		// Core's read-only compatibility surfaces (CORE-2681). The tokens are
		// this plugin's data; Core echoes them and never spells them. Unmapped
		// class values and kinds are not listed, and the default "mixed" scope
		// carries no collection token.
		Interop: &sdk.InteropSpec{
			MediaField: "class",
			Media: []sdk.InteropMapping{
				{Value: "movie", Type: "Movie"},
				{Value: "episode", Type: "Episode"},
			},
			Kinds: []sdk.InteropMapping{
				{Value: "show", Type: "Series"},
				{Value: "season", Type: "Season"},
			},
			Scopes: []sdk.InteropMapping{
				{Value: "films", Type: "movies"},
				{Value: "tvshows", Type: "tvshows"},
			},
		},
		// The US MPA movie scale, low→high (the ordered scale Profile
		// ceilings compare against; country × media-family scale tables arrive with the
		// certification-region setting later).
		CertificationScale: []string{"G", "PG", "PG-13", "R", "NC-17"},
		EntityKinds: []sdk.EntityKind{
			{
				// Show: ordered Seasons.
				Name:              "show",
				DisplayName:       sdk.LocaleString{"en": "Show", "fr": "Série"},
				PluralDisplayName: sdk.LocaleString{"en": "Shows", "fr": "Séries"},
				Archetype:         "container",
				OrderedChildren:   true,
				Fields: []sdk.FieldSpec{
					{Name: "first-air-date", Type: "date", Sortable: true,
						Label: sdk.LocaleString{"en": "First Air Date", "fr": "Première diffusion"}},
					{Name: "last-air-date", Type: "date", Sortable: true,
						Label: sdk.LocaleString{"en": "Last Air Date", "fr": "Dernière diffusion"}},
					{Name: "status", Type: "text", Filterable: true,
						Label: sdk.LocaleString{"en": "Status", "fr": "Statut"}},
					{Name: "network", Type: "text", Filterable: true,
						Label: sdk.LocaleString{"en": "Network", "fr": "Chaîne"}},
					{Name: "certification", Type: "text",
						Label: sdk.LocaleString{"en": "Certification", "fr": "Classification"}},
					// The display rating and community score core's detail
					// hero reads as chips, as on Media (CORE-2399, CORE-2514).
					{Name: "certification-display", Type: "text",
						Label: sdk.LocaleString{"en": "Rating", "fr": "Classification affichée"}},
					{Name: CommunityRatingField, Type: "number", Sortable: true,
						Label: sdk.LocaleString{"en": "TMDB rating", "fr": "Note TMDB"}},
					{Name: CommunityRatingVotesField, Type: "number",
						Label: sdk.LocaleString{"en": "TMDB votes", "fr": "Votes TMDB"}},
					{Name: "tmdb-id", Type: "text",
						Label: sdk.LocaleString{"en": "TMDB ID", "fr": "Identifiant TMDB"}},
					// summary, not overview: core's detail hero renders the
					// summary field as the overview (CORE-2514).
					{Name: "summary", Type: "text",
						Label: sdk.LocaleString{"en": "Overview", "fr": "Synopsis"}},
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
				Name:              "season",
				DisplayName:       sdk.LocaleString{"en": "Season", "fr": "Saison"},
				PluralDisplayName: sdk.LocaleString{"en": "Seasons", "fr": "Saisons"},
				Archetype:         "container",
				OrderedChildren:   true,
				Fields: []sdk.FieldSpec{
					{Name: "number", Type: "number", Sortable: true,
						Label: sdk.LocaleString{"en": "Number", "fr": "Numéro"}},
					{Name: "air-date", Type: "date",
						Label: sdk.LocaleString{"en": "Air Date", "fr": "Date de diffusion"}},
					{Name: "specials", Type: "flag",
						Label: sdk.LocaleString{"en": "Specials", "fr": "Épisodes spéciaux"}},
					{Name: "overview", Type: "text",
						Label: sdk.LocaleString{"en": "Overview", "fr": "Synopsis"}},
				},
				// Local grouping (local.go): Core writes these from the
				// filename when no provider has matched the file, so local
				// episodes still land in a season of a show. One season per
				// episode; the season title is unique only within its show.
				Relations: []sdk.RelationSpec{
					{Name: SeasonEpisodeRelation, ToKind: sdk.MediaKindName, Cardinality: "one"},
					{Name: SeasonOfRelation, ToKind: "show"},
				},
			},
			{
				// Film Series: TMDB's "Collection" concept, renamed because
				// core Collection means user-curated. Ordered
				// grid by release.
				Name:              "film-series",
				DisplayName:       sdk.LocaleString{"en": "Film Series", "fr": "Saga de films"},
				PluralDisplayName: sdk.LocaleString{"en": "Film Series", "fr": "Sagas de films"},
				Archetype:         "container",
				OrderedChildren:   true,
				Fields: []sdk.FieldSpec{
					{Name: "tmdb-id", Type: "text",
						Label: sdk.LocaleString{"en": "TMDB ID", "fr": "Identifiant TMDB"}},
				},
			},
			{
				// One Kind for all humans; the credit relation's attributes
				// distinguish actor/director/writer.
				Name:              "person",
				DisplayName:       sdk.LocaleString{"en": "Person", "fr": "Personne"},
				PluralDisplayName: sdk.LocaleString{"en": "People", "fr": "Personnes"},
				Archetype:         "person",
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
						Attributes: []string{"role", "character", "billing", "department", "job"}},
				},
			},
			{
				// One category Kind for production companies and networks;
				// the function field carries the distinction.
				Name:              "studio",
				DisplayName:       sdk.LocaleString{"en": "Studio", "fr": "Studio"},
				PluralDisplayName: sdk.LocaleString{"en": "Studios", "fr": "Studios"},
				Archetype:         "category",
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
				Name:              "genre",
				DisplayName:       sdk.LocaleString{"en": "Genre", "fr": "Genre"},
				PluralDisplayName: sdk.LocaleString{"en": "Genres", "fr": "Genres"},
				Archetype:         "category",
				Relations: []sdk.RelationSpec{
					// Genre → Media: the per-item genre link identification
					// hydrates from TMDB's genres list.
					{Name: "genre-of", ToKind: sdk.MediaKindName},
				},
			},
		},
		// The Library home is shared by Film and TV Libraries. A fixed
		// movie-only rail would promote unmatched movie-class files in a
		// TV Library (CORE-2535); deliberate class browsing stays in the
		// filter bar and All Titles grid.
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
				// Shows, local or matched: one card per show Entity of this
				// Library, ahead of the per-episode "recently-added" shelf, so
				// a TV Library's first catalog row is the shows people follow
				// (CORE-1786). A show opens on its seasons; See All lists every
				// show. No other home Section reads this Source.
				{ID: "browse-shows", Kind: "shelf",
					Title:       sdk.LocaleString{"en": "Shows", "fr": "Séries"},
					ArtworkSlot: "poster",
					Source:      sdk.SourceSpec{Kind: "entity-items", EntityKind: "show"}},
				{ID: "recently-added", Kind: "shelf",
					Title:       sdk.LocaleString{"en": "Recently Added", "fr": "Ajouts récents"},
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
						// No network or studio chips: a filter-bar option is a
						// fixed field/value pair, and fixed network names would
						// narrow a Library without them to an empty grid. Network
						// chips return when Core can derive options from the
						// values present in the Library (CORE-1711).
					}},
				// "all-items" here carries no FieldFilters, so it returns
				// every Media row — movie
				// and episode class values, never a show-level Entity (a
				// Movie and an Episode are Media, never Entities, per the
				// package doc above). The layout serves Film and TV
				// Libraries alike, so the label names no class: a Films
				// Library must not read "Movies & Episodes" (CORE-2320).
				{ID: "browse-all", Kind: "grid",
					Title:       sdk.LocaleString{"en": "All Titles", "fr": "Tous les titres"},
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
		// TheTVDB returns a series' name and overview in its original
		// language; tvdb-series reads the translation in this language and
		// falls back to the original (CORE-1128). TheTVDB language codes are
		// three-letter ISO 639-2 ("eng", "fra", "spa").
		ConfigFields: []sdk.ConfigFieldSpec{
			{Name: "metadata-language", Type: "text", Label: sdk.L("TheTVDB metadata language (ISO 639-2 code)"), Default: "eng"},
		},
		// TMDB is the primary source and works out of the box: official
		// builds carry Beamfall's own key for the AuthRef, and an operator
		// key saved in the Console replaces it. The plugin itself never
		// embeds a key. The v3
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
				Attribution: tmdbAttribution,
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
				// release_dates (CORE-2399) folds the per-region release
				// certifications on too, for the certification fields below.
				LookupTemplate: tmdbMovieLookup,
				LookupByIDTemplate: map[string]string{
					"tmdb": tmdbMovieLookup,
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
					// Ratings (CORE-2399): TMDB's community score and the US
					// MPAA certification, which feeds the Profile ceiling.
					sdk.MediaFieldKeyPrefix + CommunityRatingField:      "vote_average",
					sdk.MediaFieldKeyPrefix + CommunityRatingVotesField: "vote_count",
					"media.certification":                               tmdbMovieUSCertification,
					"media.certification-display":                       tmdbMovieUSCertification,
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
						},
						// The character and TMDB's billing order ride on the
						// relation row (CORE-2432); credits sort by billing.
						RelationAttributeMap: map[string]string{"character": "character", "billing": "order"},
						OrdinalField:         "billing"},
					tmdbKeyCrewMap(),
				},
			},
			{
				ID:          "tmdb-tv",
				DisplayName: sdk.L("TMDB (TV)"),
				Attribution: tmdbAttribution,
				Kind:        "http-json",
				Endpoint:    "https://api.themoviedb.org/3",
				AuthRef:     "tmdb-api-key",
				AuthStyle:   "query:api_key",
				// GET /search/tv — {"results": [{id, name, overview,
				// first_air_date, poster_path, backdrop_path}, …]}.
				SearchTemplate:  "/search/tv?query={query}",
				CandidatesPath:  "results",
				CandidateIDPath: "id",
				Children: []sdk.ChildListing{
					{
						Level:        "season",
						ListTemplate: "/tv/{id}",
						ItemsPath:    "seasons",
						Key:          "season_number",
						Label:        "name",
						Sequence:     "season_number",
						Year:         "air_date",
						Artwork:      "poster_path",
					},
					{
						Level:        "episode",
						ListTemplate: "/tv/{id}/season/{parent}",
						ItemsPath:    "episodes",
						Key:          "episode_number",
						Label:        "name",
						Sequence:     "season_number",
						Index:        "episode_number",
						Year:         "air_date",
						Artwork:      "still_path",
					},
				},
				// GET /tv/{id} — TMDB TV series details shape.
				// append_to_response=credits folds the cast/crew list onto
				// this document, matching tmdb-movie —
				// the credited-in EntityMap below resolves credits.cast.
				// content_ratings (CORE-2399) folds the per-region TV
				// ratings on for media.certification-display below.
				LookupTemplate: tmdbTVLookup,
				LookupByIDTemplate: map[string]string{
					"tmdb": tmdbTVLookup,
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
					// original title off original_name; status is Returning
					// Series/Ended/Canceled — the production-status values
					// verbatim. first_air_date is the series premiere, not
					// this episode's air date, so it is never mapped onto
					// media.air-date: that would overwrite the parsed air
					// date of a date-named episode (CORE-1713).
					"media.original-title":    "original_name",
					"media.tagline":           "tagline",
					"media.production-status": "status",
					// The series' primary originating network name —
					// TMDB's networks[] is the network side of the Studio
					// distinction, separate from production_companies. The
					// first element is the primary network.
					"media.network": "networks.0.name",
					// Ratings (CORE-2399). US TV ratings (TV-MA, TV-14) sit
					// outside the MPAA scale that orders Profile ceilings, so
					// they are display-only: certification stays unset.
					sdk.MediaFieldKeyPrefix + CommunityRatingField:      "vote_average",
					sdk.MediaFieldKeyPrefix + CommunityRatingVotesField: "vote_count",
					"media.certification-display":                       "content_ratings.results[iso_3166_1=US].rating",
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
					// list already renders. The series document has no
					// still_path; the per-episode document below supplies it.
					"thumb": "still_path",
				},
				// GET /tv/{id}/season/{s}/episode/{e} — the episode's own
				// document, fetched with the parsed season (sequence) and
				// episode (index); skipped when either is unknown (a
				// date-named episode). Its overview, air date and still
				// override the series values. The title is never mapped: the
				// episode keeps the parser's title (CORE-1713).
				ItemLookupTemplate: "/tv/{id}/season/{sequence}/episode/{index}",
				ItemFieldMap: map[string]string{
					"summary": "overview",
					"year":    "air_date",
					// The episode's own name, so a filename without one
					// still renders a subtitle title (CORE-1226).
					sdk.MediaFieldKeyPrefix + EpisodeTitleField: "name",
					// The episode's own community score (CORE-2399).
					sdk.MediaFieldKeyPrefix + CommunityRatingField:      "vote_average",
					sdk.MediaFieldKeyPrefix + CommunityRatingVotesField: "vote_count",
				},
				ItemArtworkMap: map[string]string{"thumb": "still_path"},
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
						},
						// The character and TMDB's billing order ride on the
						// relation row (CORE-2432); credits sort by billing.
						RelationAttributeMap: map[string]string{"character": "character", "billing": "order"},
						OrdinalField:         "billing"},
					tmdbKeyCrewMap(),
					// Container maps hydrate the show and season the local
					// mapper built for this episode; they create nothing. The
					// series document root carries the show's poster and
					// overview, and seasons[] each season's (CORE-2335). The
					// show also takes its details and backdrop, the TMDB title
					// in place of the folder spelling (kept when name is
					// absent), and links to the genres and cast the maps above
					// hydrate from the same document (CORE-2514, CORE-2513).
					{Kind: "show", Container: true, ArtworkPath: "poster_path",
						ArtworkMap: map[string]string{"backdrop": "backdrop_path"},
						TitlePath:  "name",
						FieldMap: map[string]string{
							"summary":                 "overview",
							"first-air-date":          "first_air_date",
							"last-air-date":           "last_air_date",
							"status":                  "status",
							"network":                 "networks.0.name",
							"certification-display":   "content_ratings.results[iso_3166_1=US].rating",
							CommunityRatingField:      "vote_average",
							CommunityRatingVotesField: "vote_count",
						},
						ContainerRelations: map[string]string{
							"in-genre": "genre-of",
							"credit":   "credited-in",
						}},
					{Kind: "season", Container: true, Path: "seasons[season_number={sequence}]",
						ArtworkPath: "poster_path",
						FieldMap: map[string]string{
							"overview": "overview",
							"air-date": "air_date",
						}},
				},
			},
			{
				ID:          "tvdb-series",
				DisplayName: sdk.L("TheTVDB (Series)"),
				Attribution: tvdbAttribution,
				Kind:        "http-json",
				Endpoint:    "https://api4.thetvdb.com/v4",
				// TheTVDB's free project key works only with each user's own
				// subscriber PIN, so the PIN gates the scraper: no saved PIN,
				// no TheTVDB. The project key rides beside it — Beamfall's
				// bundled one in official builds, or the operator's own.
				// TheTVDB is optional: without a PIN it is skipped rather
				// than shown as a missing required key. It does not change
				// which scraper identifies TV; tmdb-tv comes first.
				AuthRef:      "tvdb-subscriber-pin",
				OptionalAuth: true,
				// The Console and setup label the AuthRef field as the PIN it
				// is and link to the subscription page (CORE-1127).
				CredentialHelp: &sdk.CredentialHelp{
					Label:     sdk.L("TheTVDB subscriber PIN"),
					HelpURL:   "https://www.thetvdb.com/subscribe",
					HelpLabel: sdk.L("Subscribe at thetvdb.com"),
				},
				// TVDB v4 auth: POST /login {"apikey": <key>, "pin": <PIN>} →
				// {"data": {"token": <JWT>}}, then Bearer — pure data.
				AuthStyle: "login-bearer",
				Login: &sdk.LoginSpec{
					Path:         "/login",
					SecretField:  "pin",
					SecretFields: map[string]string{"apikey": "tvdb-api-key"},
					TokenPath:    "data.token",
				},
				// GET /search?query=…&type=series&year=… — {"status": "success",
				// "data": [{objectID, tvdb_id, name, overview, year,
				// image_url, …}, …]}: flat elements, no per-item envelope.
				// year narrows same-titled series; the host drops it when the
				// file carries no year (v4 swagger 4.8.0, CORE-1128).
				SearchTemplate: "/search?query={query}&type=series&year={year}",
				CandidatesPath: "data",
				// tvdb_id is the bare numeric id ("121361") that /series/{id}
				// takes; the element's `id` is the prefixed objectID form
				// ("series-121361") and cannot feed the lookup.
				CandidateIDPath: "tvdb_id",
				Children: []sdk.ChildListing{
					{
						Level:        "season",
						ListTemplate: "/series/{id}/extended",
						ItemsPath:    "data.seasons[?type.type=official]",
						Key:          "id",
						Label:        "name",
						Sequence:     "number",
						Year:         "year",
						Artwork:      "image",
					},
					{
						Level:        "episode",
						ListTemplate: "/seasons/{parent}/extended",
						ItemsPath:    "data.episodes",
						Key:          "id",
						Label:        "name",
						Sequence:     "seasonNumber",
						Index:        "number",
						Year:         "aired",
						Artwork:      "image",
					},
				},
				// GET /series/{id}/extended?meta=translations — TVDB v4 wraps
				// payloads in {"status": "success", "data": {...}}; the
				// extended record adds translations and every artwork.
				LookupTemplate: "/series/{id}/extended?meta=translations",
				// IMDb and TMDB ids resolve through /search/remoteid/{id},
				// which answers {"data": [{"series": {...}}]} with a base
				// record (no overview, translations as language codes only),
				// so it only yields the series id; the host then maps that
				// series' extended record like any lookup.
				LookupByIDTemplate: map[string]string{
					"tvdb": "/series/{id}/extended?meta=translations",
					"imdb": "/search/remoteid/{id}",
					"tmdb": "/search/remoteid/{id}",
				},
				// The tvdb id fetches the extended record itself.
				LookupByIDRecordIDPaths: map[string]string{
					"imdb": "data.0.series.id",
					"tmdb": "data.0.series.id",
				},
				FieldMap: map[string]string{
					// The configured language's translation, else the
					// record's original-language value.
					"title":   "data.translations.nameTranslations[language={config.metadata-language}].name||data.name",
					"summary": "data.translations.overviewTranslations[language={config.metadata-language}].overview||data.overview",
					// TVDB v4 carries year as a plain string already.
					"year": "data.year",
				},
				ArtworkMap: map[string]string{
					// data.image is an absolute artworks.thetvdb.com URL and
					// the best poster (v4 README), so it needs no artwork call.
					"poster": "data.image",
					// Extra artwork by type id, read from /artwork/types at
					// run time rather than hard-coded.
					"backdrop": tvdbArtwork("background"),
					"banner":   tvdbArtwork("banner"),
					"logo":     tvdbArtwork("clearlogo"),
				},
				// /artwork/types is static; the host caches it for a week.
				ReferenceDocuments: map[string]sdk.ReferenceDocument{
					"artwork-types": {Path: "/artwork/types"},
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
			// TVmaze needs no key or account (tvmaze.com/api, read
			// 2026-11-14). Summaries are HTML and stay unmapped.
			{
				ID:          "tvmaze-show",
				DisplayName: sdk.L("TVmaze"),
				Attribution: tvmazeAttribution,
				Kind:        "http-json",
				Endpoint:    "https://api.tvmaze.com",
				// GET /search/shows — a bare array of {score, show} results.
				SearchTemplate:  "/search/shows?q={query}",
				CandidatesPath:  "$",
				CandidateIDPath: "show.id",
				SearchFieldMap: map[string]string{
					"title":         "show.name",
					"year":          "show.premiered",
					"media.network": "show.network.name",
				},
				SearchArtworkMap: map[string]string{"poster": "show.image.original"},
				Children: []sdk.ChildListing{
					{
						Level:        "season",
						ListTemplate: "/shows/{id}/seasons",
						ItemsPath:    "$",
						Key:          "id",
						Label:        "name",
						Sequence:     "number",
						Year:         "premiereDate",
						Artwork:      "image.medium",
					},
					{
						Level:        "episode",
						ListTemplate: "/seasons/{parent}/episodes",
						ItemsPath:    "$",
						Key:          "id",
						Label:        "name",
						Sequence:     "season",
						// Specials have a null number. They stay in their airing
						// season and the picker asks the operator for an index.
						Index:   "number",
						Year:    "airdate",
						Artwork: "image.medium",
					},
				},
				// GET /shows/{id}?embed=episodes — the show document with its
				// episode list under _embedded.episodes (the list the
				// absolute-episode mapper reads, see absolute.go).
				LookupTemplate: "/shows/{id}?embed=episodes",
				// /lookup/shows answers with a same-origin redirect to the
				// show document, which the engine follows.
				LookupByIDTemplate: map[string]string{
					"tvdb": "/lookup/shows?thetvdb={id}",
					"imdb": "/lookup/shows?imdb={id}",
				},
				FieldMap: map[string]string{
					"title": "name",
					// premiered is "YYYY-MM-DD" → year transform below. It
					// is the show's premiere, so like tmdb-tv it does not map
					// onto the episode's media.air-date (CORE-1713).
					"year":          "premiered",
					"media.network": "network.name",
				},
				Transforms: map[string]string{"year": "year-from-date"},
				ArtworkMap: map[string]string{
					// image.original is an absolute static.tvmaze.com URL.
					"poster": "image.original",
				},
				ImpliedMediaFields: map[string]string{"media.class": "episode"},
				// genres is a plain string array: each element is the name.
				EntityMaps: []sdk.EntityMap{
					{Kind: "genre", Relation: "genre-of", Path: "genres", NamePath: ""},
				},
				// TVmaze allows at least 20 calls per 10 s per address.
				MinRequestIntervalMS: tvmazeRequestIntervalMS,
			},
			// AniList (docs.anilist.co terms of use and rate limiting, read
			// 2026-11-14) is reached with each operator's own access token,
			// so the operator is the API user. Without a token both scrapers
			// are left out and send nothing. AniList splits anime into one
			// entry per season or film, so the series and movie scrapers
			// mirror the TMDB split by format. Descriptions are HTML and
			// stay unmapped, like TVmaze's.
			{
				ID:                   "anilist-series",
				DisplayName:          sdk.L("AniList (Series)"),
				Kind:                 "http-graphql",
				Endpoint:             "https://graphql.anilist.co",
				AuthRef:              anilistTokenRef,
				OptionalAuth:         true,
				SearchTemplate:       anilistSearchQuery("TV, TV_SHORT, ONA, OVA, SPECIAL"),
				CandidatesPath:       "Page.media",
				CandidateIDPath:      "id",
				LookupTemplate:       anilistLookupQuery,
				FieldMap:             anilistFieldMap,
				ArtworkMap:           anilistArtworkMap,
				SearchFieldMap:       anilistSearchFieldMap,
				SearchArtworkMap:     anilistSearchArtworkMap,
				ImpliedMediaFields:   map[string]string{"media.class": "episode"},
				EntityMaps:           anilistEntityMaps,
				MinRequestIntervalMS: anilistRequestIntervalMS,
			},
			{
				ID:                   "anilist-movie",
				DisplayName:          sdk.L("AniList (Movies)"),
				Kind:                 "http-graphql",
				Endpoint:             "https://graphql.anilist.co",
				AuthRef:              anilistTokenRef,
				OptionalAuth:         true,
				SearchTemplate:       anilistSearchQuery("MOVIE"),
				CandidatesPath:       "Page.media",
				CandidateIDPath:      "id",
				LookupTemplate:       anilistLookupQuery,
				FieldMap:             anilistFieldMap,
				ArtworkMap:           anilistArtworkMap,
				SearchFieldMap:       anilistSearchFieldMap,
				SearchArtworkMap:     anilistSearchArtworkMap,
				ImpliedMediaFields:   map[string]string{"media.class": "movie"},
				EntityMaps:           anilistEntityMaps,
				MinRequestIntervalMS: anilistRequestIntervalMS,
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
		// Core's Migration Wizard resolves each Film & TV importer to its
		// compiled source. The SDK defines the category vocabulary and the
		// additive conflict strategy (re-runnable, gap-fill / advance-only).
		//
		// Plex/Emby/Jellyfin read the foreign server's SQLite database
		// (export-file input) and carry watch-state — resume position +
		// completion + the historical timestamp — per source user. Plex and
		// Jellyfin also carry per-user ratings, and all three carry
		// library-wide collection memberships. Emby advertises no ratings: its
		// API never stored one when observed (CORE-2678). Core's v1 handler
		// applies watch-state. Applying ratings and collections lands
		// separately in Core.
		//
		// NFO walks a sidecar tree (the library folder) and declares TMDB/IMDB
		// identification hints and local artwork under "collections". An NFO
		// run reports the match rate and pins matched files for identification.
		Importers: []sdk.ImporterSpec{
			{
				ID:          "plex",
				DisplayName: sdk.LocaleString{"en": "Plex", "fr": "Plex"},
				Categories:  []string{sdk.CategoryWatchState, sdk.CategoryRatings, sdk.CategoryCollections},
				Input:       "export-file",
				Conflict:    "additive",
			},
			{
				ID:          "emby",
				DisplayName: sdk.LocaleString{"en": "Emby", "fr": "Emby"},
				Categories:  []string{sdk.CategoryWatchState, sdk.CategoryCollections},
				Input:       "export-file",
				Conflict:    "additive",
			},
			{
				ID:          "jellyfin",
				DisplayName: sdk.LocaleString{"en": "Jellyfin", "fr": "Jellyfin"},
				Categories:  []string{sdk.CategoryWatchState, sdk.CategoryRatings, sdk.CategoryCollections},
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

// tvdbArtwork maps one TheTVDB series artwork type, by slug, to the image
// of the first extended-record artwork of that type.
func tvdbArtwork(slug string) string {
	return "data.artworks[type={ref.artwork-types:data[slug=" + slug + ",recordType=series].id}].image"
}

// The credits TMDB and TheTVDB require wherever their metadata is shown
// (themoviedb.org API FAQ; thetvdb.com/api-information#attribution, both read
// 2026-11-11). TheTVDB's text is its published sample; a negotiated license
// may set different wording.
var (
	tmdbAttribution = &sdk.Attribution{
		Text: sdk.L("This product uses the TMDB API but is not endorsed or certified by TMDB."),
		URL:  "https://www.themoviedb.org",
	}
	tvdbAttribution = &sdk.Attribution{
		Text: sdk.L("Metadata provided by TheTVDB. Please consider adding missing information or subscribing."),
		URL:  "https://www.thetvdb.com",
	}
)

// TVmaze licenses its data CC BY-SA 4.0 and asks for a credit that links back
// (tvmaze.com/api#licensing, read 2026-11-14).
var tvmazeAttribution = &sdk.Attribution{
	Text: sdk.L("TV data provided by TVmaze, licensed CC BY-SA 4.0."),
	URL:  "https://www.tvmaze.com",
}

// Request spacing inside each source's published limit: TVmaze allows at least
// 20 calls per 10 s per address; AniList allows 90 per minute, currently
// lowered to 30. Core spaces requests per scraper ID, and the two AniList
// scrapers share one token and address, so each takes half the budget.
const (
	tvmazeRequestIntervalMS  = 500
	anilistRequestIntervalMS = 4000
)

// anilistTokenRef names the operator's own AniList access token.
const anilistTokenRef = "anilist-access-token"

// anilistSearchQuery is the GraphQL search body for one set of AniList media
// formats. The engine escapes the substituted {query} as a string value.
func anilistSearchQuery(formats string) string {
	return `query { Page(perPage: 10) { media(search: "{query}", type: ANIME, format_in: [` + formats + `]) ` +
		`{ id title { romaji } startDate { year } coverImage { extraLarge } popularity } } }`
}

// anilistLookupQuery fetches one AniList entry; the response's data object is
// the lookup document, so every lookup path starts at Media.
const anilistLookupQuery = `query { Media(id: {id}, type: ANIME) { id title { romaji native } ` +
	`startDate { year } coverImage { extraLarge } bannerImage popularity genres ` +
	`studios(isMain: true) { nodes { name } } } }`

var (
	anilistFieldMap = map[string]string{
		"title":                "Media.title.romaji",
		"year":                 "Media.startDate.year",
		"popularity":           "Media.popularity",
		"media.original-title": "Media.title.native",
	}
	anilistArtworkMap = map[string]string{
		"poster":   "Media.coverImage.extraLarge",
		"backdrop": "Media.bannerImage",
	}
	anilistSearchFieldMap = map[string]string{
		"title":      "title.romaji",
		"year":       "startDate.year",
		"popularity": "popularity",
	}
	anilistSearchArtworkMap = map[string]string{
		"poster": "coverImage.extraLarge",
	}
	anilistEntityMaps = []sdk.EntityMap{
		{Kind: "genre", Relation: "genre-of", Path: "Media.genres", NamePath: ""},
		{Kind: "studio", Relation: "produced", Path: "Media.studios.nodes", NamePath: "name"},
	}
)
