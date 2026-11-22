package mediaimport

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func parseOne(t *testing.T, doc string) NFOEntry {
	t.Helper()
	entries, err := ParseNFODocument(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("ParseNFODocument: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	return entries[0]
}

func TestParseNFODocumentReadsEveryField(t *testing.T) {
	entry := parseOne(t, `<?xml version="1.0" encoding="UTF-8"?>
<Movie>
  <title>Title</title><title>Ignored second title</title>
  <originaltitle>Original</originaltitle>
  <sorttitle>Sort</sorttitle>
  <showtitle>Show</showtitle>
  <plot>Plot &amp; more&nbsp;text</plot>
  <outline>Outline</outline>
  <tagline>Tagline</tagline>
  <ratings>
    <rating name="a" max="5"><value>4</value><votes>10</votes></rating>
    <rating name="b" max="100" default="true"><value>75</value><votes>12,345</votes></rating>
    <rating name="c"><value>none</value></rating>
    <other/>
  </ratings>
  <rating>1.0</rating>
  <userrating>8.5</userrating>
  <certification>US:R</certification>
  <status>Ended</status>
  <genre>Drama / Crime</genre><genre>drama</genre>
  <studio>One</studio>
  <tag>a</tag><tag>A</tag><tag> </tag>
  <director>D</director>
  <writer>W1</writer><credits>W2</credits>
  <set><name>Series</name></set>
  <premiered>2020-02-03</premiered>
  <aired>2020-02-04 08:00:00</aired>
  <runtime>95.5</runtime>
  <actor><name>Actor</name><role>Role</role><order>2</order><thumb>t.jpg</thumb></actor>
  <actor><role>Nameless</role></actor>
  <actor><name>Second</name><order>-1</order></actor>
  <trailer>http://example/t</trailer>
  <dateadded>2021-05-06T07:08:09</dateadded>
  <playcount>3</playcount>
  <watched>1</watched>
  <lastplayed>2022-01-02T03:04:05Z</lastplayed>
  <season>2</season><episode>0</episode>
</Movie>`)
	want := NFOEntry{
		Root: "movie", Title: "Title", OriginalTitle: "Original", SortTitle: "Sort", ShowTitle: "Show",
		Plot: "Plot & more text", Outline: "Outline", Tagline: "Tagline",
		Rating: 7.5, Votes: 12345, HasRating: true, UserRating: 8.5, MPAA: "US:R", Status: "Ended",
		Genres: []string{"Drama", "Crime"}, Studios: []string{"One"}, Tags: []string{"a"},
		Directors: []string{"D"}, Writers: []string{"W1", "W2"}, Set: "Series", Year: 2020,
		Premiered:      time.Date(2020, 2, 3, 0, 0, 0, 0, time.UTC),
		Aired:          time.Date(2020, 2, 4, 8, 0, 0, 0, time.UTC),
		RuntimeMinutes: 95,
		Actors: []NFOActor{
			{Name: "Actor", Role: "Role", Order: 2, HasOrder: true, Thumb: "t.jpg"},
			{Name: "Second"},
		},
		Trailer:    "http://example/t",
		DateAdded:  time.Date(2021, 5, 6, 7, 8, 9, 0, time.UTC),
		PlayCount:  3,
		Watched:    true,
		LastPlayed: time.Date(2022, 1, 2, 3, 4, 5, 0, time.UTC),
		Season:     2, HasSeason: true, Episode: 0, HasEpisode: true,
	}
	if !reflect.DeepEqual(entry, want) {
		t.Fatalf("entry =\n%+v\nwant\n%+v", entry, want)
	}
}

func TestParseNFODocumentLegacyRatingAndPlainSet(t *testing.T) {
	entry := parseOne(t, `<tvshow><rating>8,2</rating><votes>1.500</votes><set>Plain</set><aired>2019-01-01</aired><watched>false</watched></tvshow>`)
	if !entry.HasRating || entry.Rating != 8.2 || entry.Votes != 1500 {
		t.Fatalf("legacy rating = %v/%v/%v", entry.HasRating, entry.Rating, entry.Votes)
	}
	if entry.Set != "Plain" || entry.Year != 2019 || entry.Watched {
		t.Fatalf("set/year/watched = %q/%d/%v", entry.Set, entry.Year, entry.Watched)
	}
	// A rating that parses to zero or below is no rating at all.
	if entry := parseOne(t, `<movie><ratings><rating><value>0</value></rating></ratings><rating>bad</rating></movie>`); entry.HasRating {
		t.Fatalf("zero rating kept: %+v", entry)
	}
	if entry := parseOne(t, `<movie><set><name></name><overview>x</overview></set></movie>`); entry.Set != "" {
		t.Fatalf("set without a name = %q", entry.Set)
	}
}

func TestParseNFODocumentMultipleRoots(t *testing.T) {
	entries, err := ParseNFODocument(strings.NewReader(`<episodedetails><title>One</title><episode>1</episode></episodedetails>
<episodedetails><title>Two</title><episode>2</episode></episodedetails>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Title != "One" || entries[1].Episode != 2 {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestParseNFODocumentKeepsWhatParsedBeforeTruncation(t *testing.T) {
	entries, err := ParseNFODocument(strings.NewReader(`<movie><title>Kept</title><genre>Drama</genre><actor><name>Named</name><role>Cut of`))
	if err != nil {
		t.Fatalf("truncated document error = %v, want the parsed part", err)
	}
	entry := entries[0]
	if entry.Title != "Kept" || len(entry.Genres) != 1 {
		t.Fatalf("entry = %+v", entry)
	}
	if len(entry.Actors) != 1 || entry.Actors[0].Name != "Named" || entry.Actors[0].Role != "" {
		t.Fatalf("actors = %+v, want the closed name without the cut role", entry.Actors)
	}
	// A syntax fault mid-document behaves the same way.
	entries, err = ParseNFODocument(strings.NewReader(`<movie><title>Before</title><plot>a < b</plot><tagline>After</tagline></movie>`))
	if err != nil || entries[0].Title != "Before" {
		t.Fatalf("malformed = %+v, %v", entries, err)
	}
}

func TestParseNFODocumentErrorsWhenNothingParsed(t *testing.T) {
	for _, doc := range []string{"", "   ", "<", "</movie>", `<?xml version="1.0" encoding="UTF-16"?><movie/>`} {
		if entries, err := ParseNFODocument(strings.NewReader(doc)); err == nil || entries != nil {
			t.Errorf("ParseNFODocument(%q) = %v, %v; want an error", doc, entries, err)
		}
	}
}

func TestParseNFODocumentHonoursEncodingDeclaration(t *testing.T) {
	latin1 := append([]byte(`<?xml version="1.0" encoding="ISO-8859-1"?><movie><title>Caf`), 0xE9)
	latin1 = append(latin1, []byte(` `)...)
	latin1 = append(latin1, 0x80) // a C1 control in ISO-8859-1, the euro sign in Windows-1252
	latin1 = append(latin1, []byte(`</title></movie>`)...)
	entries, err := ParseNFODocument(bytes.NewReader(latin1))
	if err != nil || entries[0].Title != "Café \u0080" {
		t.Fatalf("latin-1 title = %+v, %v", entries, err)
	}
	cp1252 := bytes.Replace(latin1, []byte("ISO-8859-1"), []byte("windows-1252"), 1)
	entries, err = ParseNFODocument(bytes.NewReader(cp1252))
	if err != nil || entries[0].Title != "Café €" {
		t.Fatalf("windows-1252 title = %+v, %v", entries, err)
	}
	bom := append([]byte{0xEF, 0xBB, 0xBF}, []byte(`<?xml version="1.0" encoding="utf-8"?><movie><title>Bom</title></movie>`)...)
	if entries, err := ParseNFODocument(bytes.NewReader(bom)); err != nil || entries[0].Title != "Bom" {
		t.Fatalf("BOM title = %+v, %v", entries, err)
	}
}

func TestSingleByteReaderFillsSmallBuffers(t *testing.T) {
	reader, err := nfoCharsetReader("latin1", bytes.NewReader([]byte{'a', 0xE9, 'b'}))
	if err != nil {
		t.Fatal(err)
	}
	var out []byte
	buf := make([]byte, 1)
	for {
		n, err := reader.Read(buf)
		out = append(out, buf[:n]...)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if string(out) != "aéb" {
		t.Fatalf("decoded = %q", out)
	}
}

func TestParseNFODateRejectsImplausibleValues(t *testing.T) {
	for _, value := range []string{"", "0000-00-00", "1601-01-01", "soon"} {
		if _, ok := parseNFODate(value); ok {
			t.Errorf("parseNFODate(%q) accepted", value)
		}
	}
	if _, ok := parseVotes("-3"); ok {
		t.Error("negative votes accepted")
	}
}

func TestParseNFODocumentCollectionAndEarlyYear(t *testing.T) {
	entries, err := ParseNFODocument(strings.NewReader("<" + "mo" + "vie><year>1896</year><collection>Lumiere Shorts</collection></" + "mo" + "vie>"))
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].Year != 1896 || entries[0].Set != "Lumiere Shorts" {
		t.Fatalf("year %d set %q, want 1896 and Lumiere Shorts", entries[0].Year, entries[0].Set)
	}
}

func TestParseNFODocumentURLOnlyHasNoRoot(t *testing.T) {
	if _, err := ParseNFODocument(strings.NewReader("https://www.imdb.com/title/tt0000001/\n")); !errors.Is(err, ErrNoNFORoot) {
		t.Fatalf("err = %v, want ErrNoNFORoot", err)
	}
}

// TestParseNFODocumentSplitsLegacyCrew: Kodi splits <director> and <credits>
// on its " / " item separator (Codex r1).
func TestParseNFODocumentSplitsLegacyCrew(t *testing.T) {
	entries, err := ParseNFODocument(strings.NewReader("<movie><director>Ann One / Bo Two</director>" +
		"<credits>Cy Three / Di Four</credits><writer>Cy Three</writer></movie>"))
	if err != nil {
		t.Fatal(err)
	}
	got := entries[0]
	if !slices.Equal(got.Directors, []string{"Ann One", "Bo Two"}) || !slices.Equal(got.Writers, []string{"Cy Three", "Di Four"}) {
		t.Fatalf("directors %q writers %q", got.Directors, got.Writers)
	}
}

// TestParseNFODocumentDropsNonFiniteNumbers (Codex r1).
func TestParseNFODocumentDropsNonFiniteNumbers(t *testing.T) {
	for _, doc := range []string{
		"<movie><ratings><rating><value>NaN</value></rating></ratings></movie>",
		"<movie><ratings><rating><value>NaN</value></rating><rating><value>7.5</value></rating></ratings></movie>",
		"<movie><rating>+Inf</rating><userrating>NaN</userrating><runtime>Inf</runtime></movie>",
	} {
		entries, err := ParseNFODocument(strings.NewReader(doc))
		if err != nil {
			t.Fatal(err)
		}
		got := entries[0]
		if (got.HasRating && got.Rating != 7.5) || got.UserRating != 0 || got.RuntimeMinutes != 0 {
			t.Errorf("%s: rating %v/%v user %v runtime %v", doc, got.Rating, got.HasRating, got.UserRating, got.RuntimeMinutes)
		}
	}
}
