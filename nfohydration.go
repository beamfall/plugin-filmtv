package filmtv

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/beamfall/plugin-filmtv/internal/mediaimport"
	sdk "github.com/beamfall/plugin-sdk"
)

// Local NFO import (CORE-0361). Core's hydration seam writes what an NFO
// sidecar declares as locally sourced fields: they outrank any provider
// scrape, and they unlock (keeping their values) when the NFO is removed.
const (
	// NFOSignatureField is the declared text Media field Core records the
	// NFOSignature under, so a rescan re-reads the sidecars only when one
	// appeared, changed or went away.
	NFOSignatureField = "nfo-signature"
)

// Media fields the NFO import writes beyond the ones the scrapers share.
const (
	outlineField    = "outline"
	userRatingField = "user-rating"
	tagsField       = "tags"
	runtimeField    = "runtime"
	trailerField    = "trailer"
	dateAddedField  = "date-added"
)

// nfoFilmRoot is the root element of a film NFO, split like the folder NFO
// name (see AGENTS.md).
const nfoFilmRoot = "mo" + "vie"

// Crew credits sort after every billed cast credit, as the TMDB crew map does.
const nfoCrewOrdinalOffset = tmdbCrewOrdinalOffset

// SidecarField is one typed value read from a local sidecar. Type is a
// declared field type: "text", "number" or "date".
type SidecarField struct {
	Name   string
	Type   string
	Text   string    `json:",omitzero"`
	Number float64   `json:",omitzero"`
	Date   time.Time `json:",omitzero"`
}

// SidecarRelation is one Entity→Media relation read from a local sidecar: the
// Entity of FromKind titled FromTitle relates to the Media through Name.
type SidecarRelation struct {
	FromKind    string
	FromTitle   string
	Name        string
	Attributes  map[string]string `json:",omitzero"`
	SortOrder   *int              `json:",omitzero"`
	Cardinality string            `json:",omitzero"`
}

// NFOHydration is what the NFO sidecars of one media file declare. Found is
// false when the file has no film or episode NFO of its own; the container
// fields can still be set from a season.nfo or tvshow.nfo. Title, SortTitle,
// Summary and Year are the built-in Media fields; GroupFields are season
// Entity fields and ParentFields show Entity fields.
type NFOHydration struct {
	Found        bool
	Title        string            `json:",omitzero"`
	SortTitle    string            `json:",omitzero"`
	Summary      string            `json:",omitzero"`
	Year         int               `json:",omitzero"`
	MediaFields  []SidecarField    `json:",omitzero"`
	Relations    []SidecarRelation `json:",omitzero"`
	GroupFields  []SidecarField    `json:",omitzero"`
	ParentFields []SidecarField    `json:",omitzero"`
}

// LocalNFOHydration reads the NFO sidecars of mediaPath: its own <stem>.nfo
// (else the folder film NFO) for the Media, the season.nfo beside it for its
// season and the nearest tvshow.nfo for its show. Unreadable or unparseable
// sidecars count as absent; a malformed one contributes what parsed.
func LocalNFOHydration(mediaPath string) NFOHydration {
	var out NFOHydration
	if entries, ok := readNFOEntries(mediaNFOPath(mediaPath)); ok {
		out = mediaHydration(entries)
	}
	if !out.Found || out.isEpisode() {
		dir := filepath.Dir(mediaPath)
		if entries, ok := readNFOEntries(filepath.Join(dir, "season.nfo")); ok {
			out.GroupFields = seasonFields(entries[0])
		}
		if path, ok := nearestShowNFO(dir); ok {
			if entries, ok := readNFOEntries(path); ok {
				out.ParentFields = showFields(entries[0])
			}
		}
	}
	return out
}

// isEpisode reports whether the Media NFO was an episode's.
func (h NFOHydration) isEpisode() bool {
	for _, field := range h.MediaFields {
		if field.Name == EpisodeTitleField || field.Name == AirDateField {
			return true
		}
	}
	return false
}

// NFOSignature identifies the NFO sidecars LocalNFOHydration reads for
// mediaPath by name, size and modification time. It is "" when there are
// none, so it changes when a sidecar appears, changes or is removed.
func NFOSignature(mediaPath string) string {
	dir := filepath.Dir(mediaPath)
	candidates := []string{mediaNFOPath(mediaPath), filepath.Join(dir, "season.nfo")}
	if path, ok := nearestShowNFO(dir); ok {
		candidates = append(candidates, path)
	}
	var parts []string
	for _, path := range candidates {
		if path == "" {
			continue
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		parts = append(parts, filepath.Base(path)+":"+strconv.FormatInt(info.Size(), 10)+":"+
			strconv.FormatInt(info.ModTime().UnixNano(), 10))
	}
	return strings.Join(parts, "|")
}

// nfoWatchState is the play the media file's own NFO records: watched when its
// playcount is above zero or <watched> is true, at <lastplayed> when given.
// It is the zero value (no watch-state) otherwise.
func nfoWatchState(mediaPath string) sdk.WatchState {
	entries, ok := readNFOEntries(mediaNFOPath(mediaPath))
	if !ok {
		return sdk.WatchState{}
	}
	for _, entry := range entries {
		if entry.Root != nfoFilmRoot && entry.Root != "episodedetails" {
			continue
		}
		if entry.PlayCount > 0 || entry.Watched {
			return sdk.WatchState{Present: true, Watched: true, OccurredAt: entry.LastPlayed}
		}
	}
	return sdk.WatchState{}
}

// mediaNFOPath returns the per-file sidecar, else the folder film NFO, else "".
func mediaNFOPath(mediaPath string) string {
	dir := filepath.Dir(mediaPath)
	stem := strings.TrimSuffix(filepath.Base(mediaPath), filepath.Ext(mediaPath))
	for _, candidate := range []string{filepath.Join(dir, stem+".nfo"), filepath.Join(dir, nfoFilmRoot+".nfo")} {
		if isRegularFile(candidate) {
			return candidate
		}
	}
	return ""
}

// nearestShowNFO finds tvshow.nfo in dir or its nearest ancestor holding one.
func nearestShowNFO(dir string) (string, bool) {
	for current := dir; ; current = filepath.Dir(current) {
		candidate := filepath.Join(current, "tvshow.nfo")
		if isRegularFile(candidate) {
			return candidate, true
		}
		if parent := filepath.Dir(current); parent == current {
			return "", false
		}
	}
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// readNFOEntries parses one sidecar, bounded like the identification read.
func readNFOEntries(path string) ([]mediaimport.NFOEntry, bool) {
	if path == "" {
		return nil, false
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer func() { _ = f.Close() }()
	entries, err := mediaimport.ParseNFODocument(io.LimitReader(f, maxNFOFileBytes))
	return entries, err == nil
}

// mediaHydration maps the film or episode roots of a Media NFO. Episode roots
// of a multi-episode file merge: titles join with " / ", plots with a blank
// line, credits and categories union.
func mediaHydration(entries []mediaimport.NFOEntry) NFOHydration {
	var films, episodes []mediaimport.NFOEntry
	for _, entry := range entries {
		switch entry.Root {
		case nfoFilmRoot:
			films = append(films, entry)
		case "episodedetails":
			episodes = append(episodes, entry)
		}
	}
	switch {
	case len(films) > 0:
		return filmHydration(films[0])
	case len(episodes) > 0:
		return episodeHydration(episodes)
	}
	return NFOHydration{}
}

func filmHydration(entry mediaimport.NFOEntry) NFOHydration {
	out := NFOHydration{Found: true, Title: entry.Title, SortTitle: entry.SortTitle, Summary: entry.Plot, Year: entry.Year}
	out.MediaFields = appendText(out.MediaFields, "original-title", entry.OriginalTitle)
	out.MediaFields = appendDate(out.MediaFields, "release-date", entry.Premiered)
	out.MediaFields = append(out.MediaFields, commonFields(entry)...)
	out.Relations = creditRelations([]mediaimport.NFOEntry{entry})
	if entry.Set != "" {
		out.Relations = append(out.Relations, SidecarRelation{FromKind: "film-series", FromTitle: entry.Set,
			Name: FilmSeriesFilmRelation, Cardinality: "one"})
	}
	return out
}

func episodeHydration(entries []mediaimport.NFOEntry) NFOHydration {
	first := entries[0]
	var titles, plots []string
	for _, entry := range entries {
		titles = appendNonEmpty(titles, entry.Title)
		plots = appendNonEmpty(plots, entry.Plot)
	}
	aired := first.Aired
	if aired.IsZero() {
		aired = first.Premiered
	}
	out := NFOHydration{Found: true, Summary: strings.Join(plots, "\n\n")}
	if !aired.IsZero() {
		out.Year = aired.Year()
	}
	out.MediaFields = appendText(out.MediaFields, EpisodeTitleField, strings.Join(titles, " / "))
	out.MediaFields = appendText(out.MediaFields, "original-title", first.OriginalTitle)
	out.MediaFields = appendDate(out.MediaFields, AirDateField, aired)
	out.MediaFields = append(out.MediaFields, commonFields(first)...)
	out.Relations = creditRelations(entries)
	return out
}

// commonFields are the Media fields a film and an episode NFO share.
func commonFields(entry mediaimport.NFOEntry) []SidecarField {
	var fields []SidecarField
	fields = appendText(fields, "tagline", entry.Tagline)
	fields = appendText(fields, outlineField, entry.Outline)
	if entry.HasRating {
		fields = appendNumber(fields, CommunityRatingField, entry.Rating)
		if entry.Votes > 0 {
			fields = appendNumber(fields, CommunityRatingVotesField, float64(entry.Votes))
		}
	}
	fields = appendNumber(fields, userRatingField, entry.UserRating)
	fields = appendText(fields, tagsField, strings.Join(entry.Tags, ", "))
	fields = appendNumber(fields, runtimeField, float64(entry.RuntimeMinutes))
	fields = appendText(fields, trailerField, entry.Trailer)
	fields = appendDate(fields, dateAddedField, entry.DateAdded)
	fields = append(fields, certificationFields(entry.MPAA)...)
	return fields
}

// certificationFields writes the NFO's rating label for display, and as the
// policy certification only when it is a value of the certification scale.
func certificationFields(mpaa string) []SidecarField {
	label := normalizeCertification(mpaa)
	if label == "" {
		return nil
	}
	fields := []SidecarField{{Name: "certification-display", Type: "text", Text: label}}
	if slices.Contains(certificationScale, label) {
		fields = append(fields, SidecarField{Name: "certification", Type: "text", Text: label})
	}
	return fields
}

// normalizeCertification reduces Kodi rating labels ("Rated PG-13",
// "US:PG-13", "US:R / UK:15") to the first bare label.
func normalizeCertification(mpaa string) string {
	label, _, _ := strings.Cut(mpaa, " / ")
	label = strings.TrimSpace(label)
	if country, rest, ok := strings.Cut(label, ":"); ok && len(country) <= 3 {
		label = strings.TrimSpace(rest)
	}
	if rest, ok := strings.CutPrefix(label, "Rated "); ok {
		label = strings.TrimSpace(rest)
	}
	return label
}

// creditRelations links genres, studios and people to the Media. Each person
// gets one credited-in relation: Core keeps one relation per Entity and Media,
// so an actor who also directed carries both credits' attributes.
func creditRelations(entries []mediaimport.NFOEntry) []SidecarRelation {
	var relations []SidecarRelation
	seen := map[string]int{}
	add := func(kind, title, name string) *SidecarRelation {
		key := kind + "\x00" + strings.ToLower(title)
		if at, ok := seen[key]; ok {
			return &relations[at]
		}
		seen[key] = len(relations)
		relations = append(relations, SidecarRelation{FromKind: kind, FromTitle: title, Name: name})
		return &relations[len(relations)-1]
	}
	for _, entry := range entries {
		for _, genre := range entry.Genres {
			add("genre", genre, "genre-of")
		}
		for _, studio := range entry.Studios {
			add("studio", studio, "produced")
		}
	}
	crew := 0
	for _, entry := range entries {
		for i, actor := range entry.Actors {
			relation := add("person", actor.Name, "credited-in")
			order := i
			if actor.HasOrder {
				order = actor.Order
			}
			mergeAttribute(relation, "character", actor.Role)
			mergeAttribute(relation, "billing", strconv.Itoa(order))
			setSortOrder(relation, order)
		}
	}
	for _, entry := range entries {
		for _, credit := range []struct {
			names       []string
			job, sector string
		}{{entry.Directors, "Director", "Directing"}, {entry.Writers, "Writer", "Writing"}} {
			for _, name := range credit.names {
				relation := add("person", name, "credited-in")
				mergeAttribute(relation, "job", credit.job)
				mergeAttribute(relation, "department", credit.sector)
				setSortOrder(relation, nfoCrewOrdinalOffset+crew)
				crew++
			}
		}
	}
	return relations
}

// mergeAttribute adds value to a relation attribute, joining distinct values
// with ", " as Core's identification merge does.
func mergeAttribute(relation *SidecarRelation, name, value string) {
	if value == "" {
		return
	}
	if relation.Attributes == nil {
		relation.Attributes = map[string]string{}
	}
	current := relation.Attributes[name]
	if current == "" {
		relation.Attributes[name] = value
		return
	}
	if slices.Contains(strings.Split(current, ", "), value) {
		return
	}
	relation.Attributes[name] = current + ", " + value
}

// setSortOrder keeps the lowest (best-billed) order a relation was given.
func setSortOrder(relation *SidecarRelation, order int) {
	if relation.SortOrder == nil || order < *relation.SortOrder {
		relation.SortOrder = &order
	}
}

// seasonFields maps a season.nfo to season Entity fields.
func seasonFields(entry mediaimport.NFOEntry) []SidecarField {
	var fields []SidecarField
	fields = appendText(fields, "overview", entry.Plot)
	aired := entry.Premiered
	if aired.IsZero() {
		aired = entry.Aired
	}
	return appendDate(fields, "air-date", aired)
}

// showFields maps a tvshow.nfo to show Entity fields.
func showFields(entry mediaimport.NFOEntry) []SidecarField {
	var fields []SidecarField
	fields = appendText(fields, "summary", entry.Plot)
	fields = appendDate(fields, "first-air-date", entry.Premiered)
	fields = appendText(fields, "status", entry.Status)
	if len(entry.Studios) > 0 {
		fields = appendText(fields, "network", entry.Studios[0])
	}
	fields = append(fields, certificationFields(entry.MPAA)...)
	if entry.HasRating {
		fields = appendNumber(fields, CommunityRatingField, entry.Rating)
		if entry.Votes > 0 {
			fields = appendNumber(fields, CommunityRatingVotesField, float64(entry.Votes))
		}
	}
	return fields
}

func appendText(fields []SidecarField, name, value string) []SidecarField {
	if value = strings.TrimSpace(value); value == "" {
		return fields
	}
	return append(fields, SidecarField{Name: name, Type: "text", Text: value})
}

func appendNumber(fields []SidecarField, name string, value float64) []SidecarField {
	if value <= 0 {
		return fields
	}
	return append(fields, SidecarField{Name: name, Type: "number", Number: value})
}

func appendDate(fields []SidecarField, name string, value time.Time) []SidecarField {
	if value.IsZero() {
		return fields
	}
	return append(fields, SidecarField{Name: name, Type: "date", Date: value})
}

func appendNonEmpty(values []string, value string) []string {
	if value = strings.TrimSpace(value); value == "" {
		return values
	}
	return append(values, value)
}
