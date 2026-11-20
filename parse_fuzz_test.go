package filmtv

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	sdk "github.com/beamfall/plugin-sdk"
)

func FuzzParseFilename(f *testing.F) {
	for _, seed := range []string{
		"The.Matrix.1999.1080p.mkv",
		"Some.Show.S02E05-E07.2020.WEB-DL.mkv",
		"Season 3 Episode 9.mp4",
		"-Fringe (2008).mkv",
		"Juego de Tronos - Temp.2 [Cap.209].mkv",
		"Star.Wars.Episode.1.10-bit.1080p.mkv",
		"Naruto - 014 - Episode 1.14.mkv",
		"Show Saison 2 E\u0301pisode 4.mkv",
		"Fringe.T01E01-E02.Pilot.mkv",
		"Show - E14 - Episode 14 - 316.mkv",
		"Show.Episode.1.02.Bits.and.Pieces.mkv",
		"Naruto - 014 - Episode 1.14 - The Bridge.mkv",
		"Show/Season 1/Show Épisode 4.mkv",
		"Season 2/MosGaz.(08.seriya).2012.mkv",
		"Show - Episode 1.22 - T02E22.mkv",
		"[DEADBEEF] (2007) [CAFEBABE]-YIFY.mkv",
		"KK.Movie.2003.mkv",
		"Pioneer One - S01E01 - Earthfall.mkv",
		"Show Name - 1x01-1x02 - Pilot (Part 1) [1080p].mkv",
		"Show (2010)/Season 01/S01E01E02-E03.Internal.WEB-DL.mkv",
		"[Group] - S01 E01 - .PROPER.mkv",
		"Night Harbor (2004) - [Director's Cut].mkv",
		"Night Harbor (2004) - 2160p HDR.mkv",
		"Synthetic Daily 2023-03-01.mkv",
		"/tv/One Piece/One Piece - 1071.mkv",
		"/anime/Hunter x Hunter (2011)/Hunter x Hunter (2011) - 12.mkv",
		"/tv/Show - 12 - Class of 1999.mkv",
		"/movies/2012.2009.1080p.BluRay.x264.mkv",
		"/tv/Space 1999/Season 01/Space.1999.0001.mkv",
		"[Fansub].[Group (1999)] (2000) Title.mkv",
		"[REC] (2007) PROPER.[DEADBEEF].mkv",
		"/tv/Show_-_2011_-_12.mkv",
		"(2000) The Emperor's New Groove.avi",
		"[G] (2000) 1984.1984 2160p.mkv",
		"/Downloads/[SubsPlease] Frieren - 12v2 (1080p) [ABCD1234].mkv",
		"Synthetic_Daily_2024_02_29 - Guest Night.mkv",
		"The IT Crowd/Season 1/1e01-Yesterday's Jam.mkv",
		"Synthetic Show/Season 3/3 01 From Hell.mkv",
		"Dealbreakers Talk Show 0001.mkv",
		"Naruto - 001 [1080p].mkv",
		"0 0000",
		"Show/Show - 001 - Title.mkv",
		"[Grp] Show - 001 [ABCD1234].mkv",
		"Show/Specials/1 01 X.mkv",
		"TV/Movie.1080p.DD5.1e01.mkv",
		"TV/VID_0001.mp4",
		"Downloads/Show - 001 [ABCD1234].mkv",
		"Home/Wedding/Wedding_0002.mp4",
		"Media/TV/Movies/Bond - 007.mkv",
		"TV/Show/Season 2/Show 1e01.mkv",
		"Show/season01-banner.jpg",
		"",
	} {
		f.Add(seed)
	}

	parser := New()
	f.Fuzz(func(t *testing.T, path string) {
		if len(path) > 64<<10 {
			return
		}
		got, ok := parser.Parse(path, sdk.ParseProbe{})
		again, againOK := parser.Parse(path, sdk.ParseProbe{})
		if ok != againOK || !reflect.DeepEqual(got, again) {
			t.Fatalf("Parse(%q) is not deterministic: (%+v, %v) then (%+v, %v)", path, got, ok, again, againOK)
		}
		if err := sdk.ValidateVersionLabel(got); err != nil {
			t.Fatalf("Parse(%q) version label contract: %v", path, err)
		}
		fields := parser.ParseMediaFields(path, sdk.ParseProbe{})
		if !reflect.DeepEqual(fields, parser.ParseMediaFields(path, sdk.ParseProbe{})) {
			t.Fatalf("ParseMediaFields(%q) is not deterministic", path)
		}
		if !ok {
			if !reflect.DeepEqual(got, sdk.ParsedMedia{}) {
				t.Fatalf("Parse(%q) returned non-zero refusal: %+v", path, got)
			}
			if fields != nil {
				t.Fatalf("ParseMediaFields(%q) returned fields for a refused parse: %+v", path, fields)
			}
			return
		}
		if got.Title == "" || got.Title != strings.TrimSpace(got.Title) {
			t.Fatalf("Parse(%q) returned invalid title %q", path, got.Title)
		}
		if got.Title != cleanTitle(got.Title) {
			t.Fatalf("Parse(%q) returned a title with dangling separators: %q", path, got.Title)
		}
		if got.IndexEnd != 0 && got.IndexEnd <= got.Index {
			t.Fatalf("Parse(%q) returned an index end that is not after the index: %+v", path, got)
		}
		for _, field := range fields {
			if got.Class != "episode" || len(fields) != 1 || field.Name != EpisodeTitleField {
				t.Fatalf("ParseMediaFields(%q) = %+v for class %q", path, fields, got.Class)
			}
			if field.Text == "" || field.Text != cleanTitle(field.Text) {
				t.Fatalf("ParseMediaFields(%q) returned an unclean episode title %q", path, field.Text)
			}
		}
		// A file under a TV library folder that is no episode keeps no class:
		// it is never a film.
		unclassified := got.Class == "" && nearestLibraryFolder(filepath.Dir(path)) == "tv"
		if got.Class != "movie" && got.Class != "episode" && !unclassified {
			t.Fatalf("Parse(%q) returned unknown class %q", path, got.Class)
		}
		if got.Year != 0 && (got.Year < 1900 || got.Year > 2099) {
			t.Fatalf("Parse(%q) returned out-of-contract year %d", path, got.Year)
		}
		typed := parser.ParseTypedMediaFields(path, sdk.ParseProbe{})
		dated := false
		for _, field := range typed {
			switch {
			case got.Class != "episode":
				t.Fatalf("ParseTypedMediaFields(%q) = %+v for class %q", path, typed, got.Class)
			case field.Name == AirDateField && !field.Date.IsZero() && field.Text == "":
				dated = true
			case field.Name != EpisodeTitleField || !field.Date.IsZero() || field.Text != cleanTitle(field.Text) || field.Text == "":
				t.Fatalf("ParseTypedMediaFields(%q) returned an out-of-contract field %+v", path, field)
			}
		}
		if dated && (got.Sequence != "" || got.Index != 0 || got.IndexEnd != 0) {
			t.Fatalf("Parse(%q) returned ordinals for a dated episode: %+v", path, got)
		}
		if got.AbsoluteOrdinal != 0 && (got.Class != "episode" || got.Sequence != "" || got.Index != got.AbsoluteOrdinal || got.AbsoluteOrdinal < 1) {
			t.Fatalf("Parse(%q) returned an out-of-contract absolute episode: %+v", path, got)
		}
		if got.Class == "episode" && got.Sequence == "" && !dated && got.AbsoluteOrdinal == 0 {
			t.Fatalf("Parse(%q) returned an episode without a sequence, an absolute ordinal or an air date: %+v", path, got)
		}
		if title := parser.MediaTitle(path, sdk.ParseProbe{}); (title != "") != (got.Class == "episode") {
			t.Fatalf("MediaTitle(%q) = %q for class %q", path, title, got.Class)
		}
		// An absolute episode number runs to four digits ("One Piece - 1071");
		// a season's episode index to three.
		maxIndex := 999
		if got.AbsoluteOrdinal != 0 {
			maxIndex = 9999
		}
		if got.Index < 0 || got.Index > maxIndex || got.IndexEnd < 0 || got.IndexEnd > 999 {
			t.Fatalf("Parse(%q) returned an out-of-contract index: %+v", path, got)
		}
	})
}

func FuzzParseTraktCSV(f *testing.F) {
	for _, seed := range []string{
		"title,watched_at,tmdb,imdb,tvdb\nExample,2024-03-04T05:06:07Z,123,tt456,\n",
		"2024-03-04,123,tt456,789\n",
		"title,watched_at,tmdb\n\"quoted, title\",invalid,0\n",
		"",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			return
		}
		got, err := ParseTraktCSV(bytes.NewReader(data))
		again, againErr := ParseTraktCSV(bytes.NewReader(data))
		if errorText(err) != errorText(againErr) || !reflect.DeepEqual(got, again) {
			t.Fatalf("ParseTraktCSV is not deterministic: (%+v, %v) then (%+v, %v)", got, err, again, againErr)
		}
		if err != nil {
			if got != nil {
				t.Fatalf("ParseTraktCSV returned items with error: %+v, %v", got, err)
			}
			return
		}
		if len(got) > len(data)+1 {
			t.Fatalf("ParseTraktCSV expanded %d input bytes into %d rows", len(data), len(got))
		}
		for row, item := range got {
			if !item.WatchState.Present || !item.WatchState.Watched {
				t.Fatalf("row %d has invalid watch state: %+v", row, item.WatchState)
			}
			if !item.WatchState.OccurredAt.IsZero() && item.WatchState.OccurredAt.Location() != time.UTC {
				t.Fatalf("row %d timestamp is not UTC: %v", row, item.WatchState.OccurredAt)
			}
			seen := map[string]bool{}
			for _, id := range item.ExternalIDs {
				if id.Scheme != SchemeTMDB && id.Scheme != SchemeIMDB && id.Scheme != SchemeTVDB {
					t.Fatalf("row %d has unknown ID scheme %q", row, id.Scheme)
				}
				if id.Value == "" || id.Value == "0" || id.Value != strings.TrimSpace(id.Value) {
					t.Fatalf("row %d has invalid external ID %+v", row, id)
				}
				key := id.Scheme + "\x00" + id.Value
				if seen[key] {
					t.Fatalf("row %d has duplicate external ID %+v", row, id)
				}
				seen[key] = true
			}
		}
	})
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
