package mediaimport

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// NFOEntry is everything one Kodi-family NFO root element carries. Root is
// the root element's name: a film, "tvshow", "season" or "episodedetails". A
// multi-episode file carries one entry per episode root.
type NFOEntry struct {
	Root          string
	Title         string
	OriginalTitle string
	SortTitle     string
	ShowTitle     string
	Plot          string
	Outline       string
	Tagline       string
	// Rating is the default rating on a 0-10 scale (the one marked default,
	// else the first), with its vote count; HasRating is false when the NFO
	// carried none.
	Rating     float64
	Votes      int
	HasRating  bool
	UserRating float64
	MPAA       string
	Status     string
	Genres     []string
	Studios    []string
	Tags       []string
	Directors  []string
	Writers    []string
	Set        string
	Year       int
	// Premiered is the release or first-air date, Aired an episode's air
	// date; zero when absent or unparseable.
	Premiered time.Time
	Aired     time.Time
	// RuntimeMinutes is the runtime the NFO states, in minutes.
	RuntimeMinutes int
	Actors         []NFOActor
	Trailer        string
	DateAdded      time.Time
	PlayCount      int
	Watched        bool
	LastPlayed     time.Time
	Season         int
	Episode        int
	HasSeason      bool
	HasEpisode     bool
}

// NFOActor is one <actor> credit. Order is the billing position; HasOrder is
// false when the NFO gave none.
type NFOActor struct {
	Name     string
	Role     string
	Order    int
	HasOrder bool
	Thumb    string
}

// ErrNoNFORoot reports a document that decoded cleanly but holds no XML
// element, such as an NFO that is only a provider URL.
var ErrNoNFORoot = errors.New("nfo: no root element")

// ParseNFODocument reads every root element of a Kodi-family NFO. It is
// tolerant: a malformed or truncated document returns the entries and fields
// decoded before the fault, and an error only when nothing at all was read.
// The XML encoding declaration is honoured for ISO-8859-1 and Windows-1252.
func ParseNFODocument(r io.Reader) ([]NFOEntry, error) {
	roots, err := readNFOTree(r)
	var entries []NFOEntry
	for _, root := range roots {
		entries = append(entries, entryFromNode(root))
	}
	if len(entries) == 0 {
		if err == nil {
			err = ErrNoNFORoot
		}
		return nil, err
	}
	return entries, nil
}

// nfoNode is one element of the tolerant tree. closed is false for an element
// the input ended (or broke) inside; its own text is then dropped as possibly
// cut, while its closed children are kept.
type nfoNode struct {
	name     string
	attrs    map[string]string
	text     strings.Builder
	children []*nfoNode
	closed   bool
}

// readNFOTree decodes the document into root nodes, stopping at the first
// syntax error or truncation and keeping what was built.
func readNFOTree(r io.Reader) ([]*nfoNode, error) {
	br := bufio.NewReader(r)
	if bom, _ := br.Peek(3); bytes.Equal(bom, []byte{0xEF, 0xBB, 0xBF}) {
		_, _ = br.Discard(3)
	}
	decoder := xml.NewDecoder(br)
	decoder.Strict = false
	decoder.Entity = xml.HTMLEntity
	decoder.CharsetReader = nfoCharsetReader
	var roots []*nfoNode
	var stack []*nfoNode
	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return roots, nil
			}
			return roots, fmt.Errorf("nfo: %w", err)
		}
		switch t := token.(type) {
		case xml.StartElement:
			node := &nfoNode{name: strings.ToLower(t.Name.Local)}
			for _, attr := range t.Attr {
				if node.attrs == nil {
					node.attrs = map[string]string{}
				}
				node.attrs[strings.ToLower(attr.Name.Local)] = attr.Value
			}
			if len(stack) == 0 {
				roots = append(roots, node)
			} else {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, node)
			}
			stack = append(stack, node)
		case xml.EndElement:
			if len(stack) == 0 {
				continue
			}
			stack[len(stack)-1].closed = true
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].text.Write(t)
			}
		}
	}
}

// value is the node's trimmed text, or "" for an unclosed node.
func (n *nfoNode) value() string {
	if n == nil || !n.closed {
		return ""
	}
	return strings.TrimSpace(n.text.String())
}

func (n *nfoNode) child(name string) *nfoNode {
	for _, child := range n.children {
		if child.name == name {
			return child
		}
	}
	return nil
}

func entryFromNode(root *nfoNode) NFOEntry {
	entry := NFOEntry{Root: root.name}
	var legacyRating, legacyVotes string
	for _, child := range root.children {
		value := child.value()
		switch child.name {
		case "title":
			entry.Title = firstNonEmpty(entry.Title, value)
		case "originaltitle":
			entry.OriginalTitle = firstNonEmpty(entry.OriginalTitle, value)
		case "sorttitle":
			entry.SortTitle = firstNonEmpty(entry.SortTitle, value)
		case "showtitle":
			entry.ShowTitle = firstNonEmpty(entry.ShowTitle, value)
		case "plot":
			entry.Plot = firstNonEmpty(entry.Plot, value)
		case "outline":
			entry.Outline = firstNonEmpty(entry.Outline, value)
		case "tagline":
			entry.Tagline = firstNonEmpty(entry.Tagline, value)
		case "ratings":
			readRatings(child, &entry)
		case "rating":
			legacyRating = firstNonEmpty(legacyRating, value)
		case "votes":
			legacyVotes = firstNonEmpty(legacyVotes, value)
		case "userrating":
			if v, ok := parseNumber(value); ok && v > 0 {
				entry.UserRating = v
			}
		case "mpaa", "certification":
			entry.MPAA = firstNonEmpty(entry.MPAA, value)
		case "status":
			entry.Status = firstNonEmpty(entry.Status, value)
		case "genre":
			entry.Genres = appendSplit(entry.Genres, value)
		case "studio":
			entry.Studios = appendSplit(entry.Studios, value)
		case "tag":
			entry.Tags = appendUnique(entry.Tags, value)
		case "director":
			entry.Directors = appendUnique(entry.Directors, value)
		case "writer", "credits":
			entry.Writers = appendUnique(entry.Writers, value)
		case "set", "collection":
			// Current form <set><name>…</name></set>; older files put the
			// name directly in <set>, and some writers say <collection>.
			if name := child.child("name").value(); name != "" {
				entry.Set = name
			} else if len(child.children) == 0 && value != "" {
				entry.Set = value
			}
		case "year":
			if y := parseNFOYear(value); y > 0 {
				entry.Year = y
			}
		case "premiered":
			if d, ok := parseNFODate(value); ok {
				entry.Premiered = d
			}
		case "aired":
			if d, ok := parseNFODate(value); ok {
				entry.Aired = d
			}
		case "runtime":
			if v, ok := parseNumber(value); ok && v > 0 {
				entry.RuntimeMinutes = int(v)
			}
		case "actor":
			if actor, ok := readActor(child); ok {
				entry.Actors = append(entry.Actors, actor)
			}
		case "trailer":
			entry.Trailer = firstNonEmpty(entry.Trailer, value)
		case "dateadded":
			if d, ok := parseNFODate(value); ok {
				entry.DateAdded = d
			}
		case "playcount":
			if v, err := strconv.Atoi(value); err == nil && v > 0 {
				entry.PlayCount = v
			}
		case "watched":
			entry.Watched = strings.EqualFold(value, "true") || value == "1"
		case "lastplayed":
			if d, ok := parseNFODate(value); ok {
				entry.LastPlayed = d
			}
		case "season":
			if v, err := strconv.Atoi(value); err == nil && v >= 0 {
				entry.Season, entry.HasSeason = v, true
			}
		case "episode":
			if v, err := strconv.Atoi(value); err == nil && v >= 0 {
				entry.Episode, entry.HasEpisode = v, true
			}
		}
	}
	if !entry.HasRating {
		if v, ok := parseNumber(legacyRating); ok && v > 0 {
			entry.Rating, entry.HasRating = v, true
			if votes, ok := parseVotes(legacyVotes); ok {
				entry.Votes = votes
			}
		}
	}
	if entry.Year == 0 {
		if !entry.Premiered.IsZero() {
			entry.Year = entry.Premiered.Year()
		} else if !entry.Aired.IsZero() {
			entry.Year = entry.Aired.Year()
		}
	}
	return entry
}

// readRatings takes the <rating default="true"> under <ratings>, else the
// first one, scaled to 0-10 by its max.
func readRatings(ratings *nfoNode, entry *NFOEntry) {
	var chosen *nfoNode
	for _, rating := range ratings.children {
		if rating.name != "rating" {
			continue
		}
		if _, ok := parseNumber(rating.child("value").value()); !ok {
			continue
		}
		if chosen == nil || strings.EqualFold(rating.attrs["default"], "true") {
			chosen = rating
		}
	}
	if chosen == nil {
		return
	}
	value, _ := parseNumber(chosen.child("value").value())
	if scale, ok := parseNumber(chosen.attrs["max"]); ok && scale > 0 && scale != 10 {
		value = value * 10 / scale
	}
	if value <= 0 {
		return
	}
	entry.Rating, entry.HasRating = value, true
	if votes, ok := parseVotes(chosen.child("votes").value()); ok {
		entry.Votes = votes
	}
}

func readActor(node *nfoNode) (NFOActor, bool) {
	actor := NFOActor{
		Name:  node.child("name").value(),
		Role:  node.child("role").value(),
		Thumb: node.child("thumb").value(),
	}
	if order, err := strconv.Atoi(node.child("order").value()); err == nil && order >= 0 {
		actor.Order, actor.HasOrder = order, true
	}
	return actor, actor.Name != ""
}

// nfoDateLayouts are the date shapes Kodi-family tools write.
var nfoDateLayouts = []string{"2006-01-02", "2006-01-02 15:04:05", "2006-01-02T15:04:05", time.RFC3339}

func parseNFODate(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range nfoDateLayouts {
		if d, err := time.Parse(layout, value); err == nil && d.Year() > 1800 {
			return d.UTC(), true
		}
	}
	return time.Time{}, false
}

func parseNumber(value string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(value), ",", "."), 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// parseVotes accepts "12345" and the thousands-grouped "12,345".
func parseVotes(value string) (int, bool) {
	cleaned := strings.NewReplacer(",", "", ".", "", " ", "").Replace(value)
	v, err := strconv.Atoi(cleaned)
	if err != nil || v < 0 {
		return 0, false
	}
	return v, true
}

func firstNonEmpty(current, next string) string {
	if current != "" {
		return current
	}
	return next
}

// appendSplit adds each " / "-separated value (the legacy multi-value form)
// once.
func appendSplit(values []string, value string) []string {
	for _, part := range strings.Split(value, " / ") {
		values = appendUnique(values, part)
	}
	return values
}

func appendUnique(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if strings.EqualFold(existing, value) {
			return values
		}
	}
	return append(values, value)
}

// nfoCharsetReader transcodes the single-byte encodings NFO writers declare
// to UTF-8. UTF-8 and ASCII pass through.
func nfoCharsetReader(charset string, input io.Reader) (io.Reader, error) {
	switch strings.ToLower(strings.TrimSpace(charset)) {
	case "utf-8", "utf8", "us-ascii", "ascii":
		return input, nil
	case "iso-8859-1", "iso8859-1", "latin1", "latin-1", "l1", "iso_8859-1":
		return &singleByteReader{src: bufio.NewReader(input)}, nil
	case "windows-1252", "cp1252", "x-cp1252":
		return &singleByteReader{src: bufio.NewReader(input), cp1252: true}, nil
	default:
		return nil, fmt.Errorf("nfo: unsupported encoding %q", charset)
	}
}

// cp1252High maps Windows-1252 bytes 0x80-0x9F to their code points; the
// five undefined bytes map to themselves as in ISO-8859-1.
var cp1252High = [32]rune{
	0x20AC, 0x81, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021,
	0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0x8D, 0x017D, 0x8F,
	0x90, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014,
	0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0x9D, 0x017E, 0x0178,
}

type singleByteReader struct {
	src     *bufio.Reader
	cp1252  bool
	pending []byte
}

func (r *singleByteReader) Read(p []byte) (int, error) {
	n := 0
	for n < len(p) {
		if len(r.pending) > 0 {
			copied := copy(p[n:], r.pending)
			r.pending = r.pending[copied:]
			n += copied
			continue
		}
		b, err := r.src.ReadByte()
		if err != nil {
			if n > 0 {
				return n, nil
			}
			return 0, err
		}
		ch := rune(b)
		if r.cp1252 && b >= 0x80 && b <= 0x9F {
			ch = cp1252High[b-0x80]
		}
		var buf [utf8.UTFMax]byte
		r.pending = append(r.pending[:0], buf[:utf8.EncodeRune(buf[:], ch)]...)
	}
	return n, nil
}

// parseNFOYear reads <year>: a bare four-digit year from 1870 (the first
// films) on, else the first 19xx/20xx year in the text.
func parseNFOYear(value string) int {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) == 4 {
		if y, err := strconv.Atoi(trimmed); err == nil && y >= 1870 {
			return y
		}
	}
	return parseYear(value)
}
