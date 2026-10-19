package filmtv

import (
	"path/filepath"
	"reflect"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"
)

var _ sdk.ContextFilenameParser = (*Parser)(nil)

func TestParserReadsProviderIDTagsFromFilenameAndAncestors(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		provider string
		id       string
		title    string
	}{
		{name: "tmdb folder", path: "The Matrix (1999) {tmdb-603}/The.Matrix.1999.mkv", provider: "tmdb", id: "603", title: "The Matrix"},
		{name: "tmdbid folder", path: "Films/{tmdbid-604}/Title.mkv", provider: "tmdb", id: "604", title: "Title"},
		{name: "tmdbid filename square", path: "Films/Title [tmdbid-605].mkv", provider: "tmdb", id: "605", title: "Title"},
		{name: "imdb filename", path: "Films/Title {imdb-tt0133093}.mkv", provider: "imdb", id: "tt0133093", title: "Title"},
		{name: "imdbid ancestor square", path: "Films/[imdbid-tt0133094]/Season/Title.mkv", provider: "imdb", id: "tt0133094", title: "Title"},
		{name: "tvdb filename", path: "Shows/Title {tvdb-81189}.mkv", provider: "tvdb", id: "81189", title: "Title"},
		{name: "tvdbid ancestor square", path: "Shows/[tvdbid-81190]/Season 01/Title.mkv", provider: "tvdb", id: "81190", title: "Title"},
		{name: "anidb ancestor", path: "Anime/{anidb-69}/Season/Title.mkv", provider: "anidb", id: "69", title: "Title"},
	}

	parser := New()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parser.Parse(tt.path, sdk.ParseProbe{})
			if !ok {
				t.Fatalf("Parse(%q) refused", tt.path)
			}
			want := map[string]string{tt.provider: tt.id}
			if !reflect.DeepEqual(got.ProviderIDs, want) {
				t.Fatalf("ProviderIDs = %#v, want %#v", got.ProviderIDs, want)
			}
			if got.HintSource != sdk.ParseHintSourceFilename {
				t.Fatalf("HintSource = %q, want filename", got.HintSource)
			}
			if got.Title != tt.title {
				t.Fatalf("Title = %q, want %q", got.Title, tt.title)
			}
		})
	}
}

func TestParserNFOProviderIDsBeatFilenameTag(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "The Matrix (1999) {tmdb-999}")
	media := filepath.Join(dir, "The.Matrix.1999.mkv")
	write(t, media, "")
	write(t, filepath.Join(dir, "movie.nfo"), `<movie><uniqueid type="imdb">tt0133093</uniqueid></movie>`)

	got, ok := New().Parse(media, sdk.ParseProbe{})
	if !ok {
		t.Fatal("Parse refused valid media path")
	}
	want := map[string]string{"imdb": "tt0133093"}
	if !reflect.DeepEqual(got.ProviderIDs, want) {
		t.Fatalf("ProviderIDs = %#v, want NFO IDs %#v", got.ProviderIDs, want)
	}
	if got.HintSource != sdk.ParseHintSourceNFO {
		t.Fatalf("HintSource = %q, want nfo", got.HintSource)
	}
}

func TestParserReadsEachNFOSidecarConvention(t *testing.T) {
	tests := []struct {
		name   string
		media  string
		nfo    string
		body   string
		wantID string
	}{
		{name: "stem sibling", media: "Film.mkv", nfo: "Film.nfo", body: `<movie><tmdbid>1</tmdbid></movie>`, wantID: "1"},
		{name: "movie folder", media: "Film.mkv", nfo: "movie.nfo", body: `<movie><tmdbid>2</tmdbid></movie>`, wantID: "2"},
		{name: "tvshow ancestor", media: filepath.Join("Season 01", "Show.S01E01.mkv"), nfo: "tvshow.nfo", body: `<tvshow><tmdbid>3</tmdbid></tvshow>`, wantID: "3"},
		{name: "url only", media: "Film.mkv", nfo: "movie.nfo", body: "https://www.themoviedb.org/movie/603", wantID: "603"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			media := filepath.Join(root, tt.media)
			write(t, media, "")
			write(t, filepath.Join(root, tt.nfo), tt.body)
			got, ok := New().Parse(media, sdk.ParseProbe{})
			if !ok {
				t.Fatal("Parse refused valid media path")
			}
			if got.ProviderIDs["tmdb"] != tt.wantID || got.HintSource != sdk.ParseHintSourceNFO {
				t.Fatalf("hint = %#v/%q, want tmdb=%s/nfo", got.ProviderIDs, got.HintSource, tt.wantID)
			}
		})
	}
}

func TestParserIgnoresMalformedNFOWithoutError(t *testing.T) {
	root := t.TempDir()
	media := filepath.Join(root, "The.Matrix.1999.mkv")
	write(t, media, "")
	write(t, filepath.Join(root, "movie.nfo"), `<movie><uniqueid type="tmdb">603`)

	got, ok := New().Parse(media, sdk.ParseProbe{})
	if !ok {
		t.Fatal("Parse refused because the NFO was malformed")
	}
	if got.ProviderIDs != nil || got.HintSource != sdk.ParseHintSourceNone {
		t.Fatalf("malformed NFO hint = %#v/%q, want none", got.ProviderIDs, got.HintSource)
	}
}

func TestParseWithContextPreservesParseResult(t *testing.T) {
	parser := New()
	path := "The Matrix (1999) {tmdb-603}/The.Matrix.1999.mkv"
	want, wantOK := parser.Parse(path, sdk.ParseProbe{})
	got, gotOK := parser.ParseWithContext(path, sdk.ParseProbe{}, sdk.ParseContext{DirectoryMediaFileCount: sdk.DirectoryMediaFileCountMultiple})
	if gotOK != wantOK || !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseWithContext = %#v/%v, want Parse %#v/%v", got, gotOK, want, wantOK)
	}
}
