package filmtv

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
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

// NFOHydration is what the NFO sidecar of one media file declares. Found is
// false when the file has no film or episode NFO it can use. Unreadable is
// true when an NFO exists but could not be read in full: the plan then carries
// nothing, and Core keeps what an earlier read imported and retries. Signature
// identifies the bytes the plan was parsed from ("" with no NFO), in the form
// NFOSignature computes from the file system. Title, SortTitle, Summary and
// Year are the built-in Media fields.
type NFOHydration struct {
	Found       bool
	Unreadable  bool              `json:",omitzero"`
	Signature   string            `json:"-"`
	Title       string            `json:",omitzero"`
	SortTitle   string            `json:",omitzero"`
	Summary     string            `json:",omitzero"`
	Year        int               `json:",omitzero"`
	MediaFields []SidecarField    `json:",omitzero"`
	Relations   []SidecarRelation `json:",omitzero"`
}

// SidecarRelationKind names one Entity kind and relation an NFO plan writes.
type SidecarRelationKind struct {
	FromKind string
	Name     string
}

// NFOOwnedRelations lists every relation an NFO plan can carry. Core replaces
// the rows an earlier NFO import wrote under each of them, so a relation the
// NFO stopped naming (its last actor removed) goes away too.
func NFOOwnedRelations() []SidecarRelationKind {
	return []SidecarRelationKind{
		{FromKind: "genre", Name: "genre-of"},
		{FromKind: "studio", Name: "produced"},
		{FromKind: "person", Name: "credited-in"},
		{FromKind: "film-series", Name: FilmSeriesFilmRelation},
	}
}

// LocalNFOHydration reads the NFO sidecar of mediaPath: its own <stem>.nfo,
// else the folder film NFO when the file is the folder's main feature. A
// malformed document contributes what parsed. Container sidecars (tvshow.nfo,
// season.nfo) are not read: Core has no field provenance for the show and
// season Entities they describe. A failed sidecar lookup reads as unreadable,
// never as no sidecar.
func LocalNFOHydration(mediaPath string) NFOHydration {
	path, err := mediaNFOPath(mediaPath)
	if err != nil {
		return NFOHydration{Unreadable: true}
	}
	if path == "" {
		return NFOHydration{}
	}
	entries, signature, err := readNFOSnapshot(path)
	if err != nil {
		return NFOHydration{Unreadable: true}
	}
	out := mediaHydration(entries)
	out.Signature = signature
	return out
}

// NFOSignature identifies the NFO sidecar LocalNFOHydration reads for
// mediaPath by its absolute path, its size and its modification time, without
// reading it. It is "" when there is none, so it changes when the sidecar
// appears, changes or is removed. A lookup that fails for any reason other
// than the file not existing is an error, never "".
func NFOSignature(mediaPath string) (string, error) {
	path, err := mediaNFOPath(mediaPath)
	if err != nil || path == "" {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		// The sidecar was found a moment ago: gone now is a change mid-lookup.
		return "", fmt.Errorf("nfo sidecar %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("nfo sidecar %q: %w", path, errNFOChanged)
	}
	return nfoSignature(path, info), nil
}

// nfoSignature is the signature of the sidecar at path with the given stat.
func nfoSignature(path string, info os.FileInfo) string {
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	return path + ":" + strconv.FormatInt(info.Size(), 10) + ":" +
		strconv.FormatInt(info.ModTime().UnixNano(), 10)
}

// nfoWatchState is the play the media file's NFO records: watched when its
// playcount is above zero or <watched> is true, at <lastplayed> when given.
// It is the zero value (no watch-state) otherwise, and an error when the NFO
// exists but cannot be read. The play count itself is not carried:
// sdk.WatchState has no field for it.
func nfoWatchState(mediaPath string) (sdk.WatchState, error) {
	path, err := mediaNFOPath(mediaPath)
	if err != nil || path == "" {
		return sdk.WatchState{}, err
	}
	entries, _, err := readNFOSnapshot(path)
	if err != nil {
		return sdk.WatchState{}, fmt.Errorf("nfo sidecar %q: %w", path, err)
	}
	for _, entry := range entries {
		if entry.Root != nfoFilmRoot && entry.Root != "episodedetails" {
			continue
		}
		if entry.PlayCount > 0 || entry.Watched {
			return sdk.WatchState{Present: true, Watched: true, OccurredAt: entry.LastPlayed}, nil
		}
	}
	return sdk.WatchState{}, nil
}

// mediaNFOPath returns the per-file sidecar, else the folder film NFO when
// mediaPath is the folder's main feature, else "". Looking up a candidate
// that fails for any reason but its absence is an error.
func mediaNFOPath(mediaPath string) (string, error) {
	dir := filepath.Dir(mediaPath)
	stem := strings.TrimSuffix(filepath.Base(mediaPath), filepath.Ext(mediaPath))
	candidates := []string{filepath.Join(dir, stem+".nfo")}
	if !isExtraStem(stem) {
		candidates = append(candidates, filepath.Join(dir, nfoFilmRoot+".nfo"))
	}
	for _, candidate := range candidates {
		found, err := isRegularFile(candidate)
		if err != nil {
			return "", fmt.Errorf("nfo sidecar %q: %w", candidate, err)
		}
		if found {
			return candidate, nil
		}
	}
	return "", nil
}

// reExtraLabel matches the Kodi extras suffix of a file stem, folded by
// notVersionKey: "-trailer", "-behindthescenes", "-deleted" and the rest, or a
// stem that is only such a label ("trailer.mkv").
var reExtraLabel = regexp.MustCompile(`^(?:trailer|sample|featurette|behindthescene|deleted|deletedscene|interview|scene|short|clip|extra|other|blooper|outtake|gagreel|makingof|teaser|bonus)s?\d*$`)

// looseExtraLabels are the suffixes Jellyfin also accepts after a dot, an
// underscore or a space ("Film sample.mkv"); the rest need a hyphen.
var looseExtraLabels = []string{"trailer", "sample"}

// isExtraStem reports whether a file stem names an extra rather than the
// main feature.
func isExtraStem(stem string) bool {
	if reExtraLabel.MatchString(notVersionKey(stem)) {
		return true
	}
	if at := strings.LastIndex(stem, "-"); at >= 0 && reExtraLabel.MatchString(notVersionKey(stem[at+1:])) {
		return true
	}
	at := strings.LastIndexAny(stem, " ._")
	return at >= 0 && slices.Contains(looseExtraLabels, asciiLower(stem[at+1:]))
}

// errNFODangling reports a sidecar symlink whose target is gone.
var errNFODangling = errors.New("nfo symlink target is missing")

// isRegularFile reports whether path names a regular file. Only a clean
// not-found, or a path the system refuses as too long (no file can live
// there), is false with no error: any other failure is returned, so a sidecar
// behind a folder that denies traversal, or a symlink whose target moved,
// never reads as absent.
func isRegularFile(path string) (bool, error) {
	info, err := os.Stat(path)
	switch {
	case err == nil:
		return info.Mode().IsRegular(), nil
	case errors.Is(err, fs.ErrNotExist):
		_, lerr := os.Lstat(path)
		switch {
		case lerr == nil:
			return false, errNFODangling
		case errors.Is(lerr, fs.ErrNotExist):
			return false, nil
		}
		return false, lerr
	case tooLong(filepath.Dir(path), err):
		return entryInFolder(path)
	}
	return false, err
}

// errNFOUnreachable reports a sidecar that exists in its folder but that the
// system cannot open by its full path: the path, or its symlink target, is
// past the length limit.
var errNFOUnreachable = errors.New("nfo sidecar path is past the system's length limit")

// entryInFolder looks for path's entry after a full-path lookup was refused
// as too long. A symlink whose target is too long, or a sidecar created
// relative to its folder past the full-path limit, is an error, never
// absence. A name too long for any entry to have is absent.
func entryInFolder(path string) (bool, error) {
	dir := filepath.Dir(path)
	_, err := os.Lstat(path)
	switch {
	case err == nil:
		return false, errNFOUnreachable
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	case !tooLong(dir, err):
		return false, err
	}
	// A path as long made of "." steps resolves, so the name alone is past
	// the folder's limit and nothing can carry it.
	steps := strings.Repeat(string(filepath.Separator)+".", (len(path)-len(dir)+1)/2)
	if _, err := os.Lstat(dir + steps); err == nil {
		return false, nil
	}
	// Only the full path is past the limit: look inside the folder. A folder
	// that cannot be opened to look leaves the sidecar unknown, an error.
	root, err := os.OpenRoot(dir)
	if err != nil {
		return false, err
	}
	defer root.Close()
	_, err = root.Lstat(filepath.Base(path))
	switch {
	case err == nil:
		return false, errNFOUnreachable
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	}
	return false, err
}

// tooLong reports whether err is the error dir's filesystem gives for a name
// past its length limits, as a stat of a name no filesystem holds shows: the
// plugin cannot import syscall to name it. A probe that fails as not found or
// access denied matches nothing.
func tooLong(dir string, err error) bool {
	_, probe := os.Lstat(filepath.Join(dir, strings.Repeat("n", 8192)))
	var pathErr *fs.PathError
	if !errors.As(probe, &pathErr) || errors.Is(probe, fs.ErrNotExist) || errors.Is(probe, fs.ErrPermission) {
		return false
	}
	return errors.Is(err, pathErr.Err)
}

// errNFOChanged reports a sidecar that changed while it was read.
var errNFOChanged = errors.New("nfo changed while it was read")

// errNFOTooLarge reports a sidecar past the read limit: reading only its
// head would drop what follows (credits) as if the file had removed it.
var errNFOTooLarge = errors.New("nfo larger than the read limit")

// afterNFORead runs between reading a sidecar and checking it is unchanged;
// tests replace it to edit the file mid-read.
var afterNFORead = func(string) {}

// readNFOSnapshot parses one sidecar, bounded like the identification read,
// and returns the signature of the bytes it parsed. It fails when the file
// cannot be read, is larger than the read limit, changed during the read (its
// stat before and after differ, or the path now names another file), or holds
// an XML syntax error before
// any root element. A file with no XML element at all (a bare provider URL)
// parses to no entries.
func readNFOSnapshot(path string) ([]mediaimport.NFOEntry, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = f.Close() }()
	before, err := f.Stat()
	if err != nil {
		return nil, "", err
	}
	if !before.Mode().IsRegular() {
		return nil, "", errNFOChanged
	}
	data, err := io.ReadAll(io.LimitReader(f, maxNFOFileBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) > maxNFOFileBytes {
		return nil, "", errNFOTooLarge
	}
	afterNFORead(path)
	after, err := f.Stat()
	if err != nil {
		return nil, "", err
	}
	current, err := os.Stat(path)
	if err != nil {
		return nil, "", err
	}
	if !sameNFOStat(before, after) || !sameNFOStat(before, current) || !os.SameFile(before, current) {
		return nil, "", errNFOChanged
	}
	entries, err := mediaimport.ParseNFODocument(bytes.NewReader(data))
	if err != nil && !errors.Is(err, mediaimport.ErrNoNFORoot) {
		return nil, "", err
	}
	return entries, nfoSignature(path, before), nil
}

func sameNFOStat(a, b os.FileInfo) bool {
	return a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
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
	out := NFOHydration{Found: true, SortTitle: first.SortTitle, Summary: strings.Join(plots, "\n\n"), Year: first.Year}
	if out.Year == 0 && !aired.IsZero() {
		out.Year = aired.Year()
	}
	out.MediaFields = appendText(out.MediaFields, EpisodeTitleField, strings.Join(titles, " / "))
	out.MediaFields = appendText(out.MediaFields, "original-title", first.OriginalTitle)
	out.MediaFields = appendDate(out.MediaFields, AirDateField, aired)
	out.MediaFields = append(out.MediaFields, commonFields(withAllTags(entries))...)
	out.Relations = creditRelations(entries)
	return out
}

// withAllTags is the first entry carrying the tags of every entry, so a tag
// named only by a later episode root still reaches the plan.
func withAllTags(entries []mediaimport.NFOEntry) mediaimport.NFOEntry {
	merged := entries[0]
	merged.Tags = nil
	for _, entry := range entries {
		for _, tag := range entry.Tags {
			if !slices.ContainsFunc(merged.Tags, func(have string) bool { return strings.EqualFold(have, tag) }) {
				merged.Tags = append(merged.Tags, tag)
			}
		}
	}
	return merged
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
// policy certification only when it is a US or unqualified value of the
// certification scale: a foreign rating never sets or lowers the US ceiling.
func certificationFields(mpaa string) []SidecarField {
	label, policy := selectCertification(mpaa)
	if label == "" {
		return nil
	}
	fields := []SidecarField{{Name: "certification-display", Type: "text", Text: label}}
	if policy && slices.Contains(certificationScale, label) {
		fields = append(fields, SidecarField{Name: "certification", Type: "text", Text: label})
	}
	return fields
}

// selectCertification picks one label from a Kodi rating list ("Rated PG-13",
// "US:PG-13", "CA:PG / US:R"): the US entry, else the first unqualified entry,
// else the first entry. policy is false for the last case, a rating that names
// only other countries.
func selectCertification(mpaa string) (label string, policy bool) {
	var fallback string
	for _, entry := range strings.Split(mpaa, " / ") {
		country, value := splitCertification(entry)
		if value == "" {
			continue
		}
		switch {
		case isUSQualifier(country):
			return value, true
		case country == "" && !policy:
			label, policy = value, true
		case fallback == "":
			fallback = value
		}
	}
	if policy {
		return label, true
	}
	return fallback, false
}

// reCertificationQualifier matches the country before a rating's colon: a
// code or a name ("US", "Germany", "United States", "Côte d’Ivoire",
// "Guinea-Bissau").
var reCertificationQualifier = regexp.MustCompile(`^[\pL][\pL\p{M} .'’-]{0,39}$`)

// splitCertification splits "US:Rated R" or "Germany:FSK 12" into its
// lowercased country and the bare label. A label with no country before a
// colon is unqualified.
func splitCertification(entry string) (country, label string) {
	label = strings.TrimSpace(entry)
	if prefix, rest, ok := strings.Cut(label, ":"); ok && reCertificationQualifier.MatchString(strings.TrimSpace(prefix)) {
		country, label = strings.ToLower(strings.TrimSpace(prefix)), strings.TrimSpace(rest)
	}
	if rest, ok := strings.CutPrefix(label, "Rated "); ok {
		label = strings.TrimSpace(rest)
	}
	return country, label
}

// isUSQualifier reports whether a lowercased rating qualifier names the US.
func isUSQualifier(country string) bool {
	switch strings.NewReplacer(".", "", " ", "").Replace(country) {
	case "us", "usa", "unitedstates", "unitedstatesofamerica":
		return true
	}
	return false
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

func appendText(fields []SidecarField, name, value string) []SidecarField {
	if value = strings.TrimSpace(value); value == "" {
		return fields
	}
	return append(fields, SidecarField{Name: name, Type: "text", Text: value})
}

func appendNumber(fields []SidecarField, name string, value float64) []SidecarField {
	if value <= 0 || math.IsInf(value, 0) || math.IsNaN(value) {
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
