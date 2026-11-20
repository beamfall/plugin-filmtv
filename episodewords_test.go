package filmtv_test

import (
	"reflect"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"

	filmtv "github.com/beamfall/plugin-filmtv"
)

// TestEpisodeMarkerWords pins the season and episode markers of other
// languages (CORE-2702): a season word paired with an episode word, TxxEyy and
// "Episode S.EE" are read anywhere; a lone episode word, a bare "Eyy" and "N
// seriya" are read only in a season folder, which supplies the season. A film
// named with one of the words stays a film.
func TestEpisodeMarkerWords(t *testing.T) {
	parser := filmtv.New()
	episode := func(title string, season string, index int) sdk.ParsedMedia {
		return sdk.ParsedMedia{Title: title, Class: "episode", Sequence: season, Index: index}
	}
	cases := []struct {
		path string
		want sdk.ParsedMedia
		ok   bool
	}{
		// The ticket's names, each in its show's season folder.
		{"/tv/Anubis/Season 01/Anubis saison 01 episode 38.mkv", episode("Anubis", "1", 38), true},
		{"/tv/Le Monde Incroyable de Gumball/Season 5/Le Monde Incroyable de Gumball - Saison 5 Ep 14.mkv", episode("Le Monde Incroyable de Gumball", "5", 14), true},
		{"/tv/El Chema/Season 1/El Chema Temporada 1 Capitulo 25.mkv", episode("El Chema", "1", 25), true}, //nolint:misspell // Spanish "capitulo", the ticket's name
		{"/tv/Juego de Tronos/Season 2/Juego de Tronos - Temp.2 [Cap.209].mkv", episode("Juego de Tronos", "2", 9), true},
		{"/tv/My Little Pony/Season 2/My Little Pony - T02E22.mp4", episode("My Little Pony", "2", 22), true},
		{"/tv/Desperate Housewives/Season 1/Desperate Housewives - Episode 1.22 - Goodbye for now.avi", sdk.ParsedMedia{Title: "Desperate Housewives", Class: "episode", Sequence: "1", Index: 22}, true},
		{"/tv/Mob Psycho 100/Season 2/Mob.Psycho.100.II.E10.720p.WEB.mkv", episode("Mob Psycho 100 II", "2", 10), true},
		{"/tv/MosGaz/Season 1/MosGaz.(08.seriya).2012.mkv", episode("MosGaz", "1", 8), true},
		// The paired forms need no folder.
		{"/downloads/Anubis saison 01 episode 38.mkv", episode("Anubis", "1", 38), true},
		{"/downloads/Show Staffel 2 Folge 3.mkv", episode("Show", "2", 3), true},
		{"/downloads/Show Stagione 3 Episodio 4.mkv", episode("Show", "3", 4), true},
		{"/downloads/Show Sezon 1 Ep 5.mkv", episode("Show", "1", 5), true},
		{"/downloads/Show Temporada 1 Capítulo 6.mkv", episode("Show", "1", 6), true},
		{"/downloads/Show Saison 2 Épisode 7.mkv", episode("Show", "2", 7), true},
		{"/downloads/My Little Pony - T02E22.mp4", episode("My Little Pony", "2", 22), true},
		{"/downloads/Desperate Housewives - Episode 1.22.avi", episode("Desperate Housewives", "1", 22), true},
		// "Cap" packs the season only when the first digit is it.
		{"/downloads/Show Temp 1 Cap 209.mkv", episode("Show", "1", 209), true},
		{"/tv/Show/Season 1/Show - Cap.105.mkv", episode("Show", "1", 5), true},
		// A lone episode word in a season folder, or "Specials" (season 0).
		{"/tv/Show/Season 3/Show Folge 12.mkv", episode("Show", "3", 12), true},
		{"/tv/Show/Season 1/Show - Épisode 4.mkv", episode("Show", "1", 4), true},
		{"/tv/Show/Specials/Show Ep 2.mkv", episode("Show", "0", 2), true},
		// A number after "Episode" is not a dotted season with a resolution.
		{"/downloads/Show.Episode.12.720p.mkv", sdk.ParsedMedia{Title: "Show Episode 12", Class: "movie"}, true},
		// Films named with the words stay films.
		{"/movies/Temporada (2015).mkv", sdk.ParsedMedia{Title: "Temporada", Class: "movie", Year: 2015}, true},
		{"/movies/Saison.2019.1080p.mkv", sdk.ParsedMedia{Title: "Saison", Class: "movie", Year: 2019}, true},
		{"/movies/T2 (1991).mkv", sdk.ParsedMedia{Title: "T2", Class: "movie", Year: 1991}, true},
		{"/movies/E10.mkv", sdk.ParsedMedia{Title: "E10", Class: "movie"}, true},
		{"/downloads/E10.mkv", sdk.ParsedMedia{Title: "E10", Class: "movie"}, true},
		{"/movies/Ep 14.mkv", sdk.ParsedMedia{Title: "Ep 14", Class: "movie"}, true},
		{"/movies/Folge 12 (2010).mkv", sdk.ParsedMedia{Title: "Folge 12", Class: "movie", Year: 2010}, true},
		// A season word must start a word.
		// Review cases: the first marker wins over a later one, an episode
		// number never runs into a letter or digit, and a bit depth is no
		// "Episode S.EE" code.
		{"/movies/Star.Wars.Episode.1.10-bit.1080p.mkv", sdk.ParsedMedia{Title: "Star Wars Episode 1", Class: "movie"}, true},
		{"/anime/Naruto/Naruto - 014 - Episode 1.14.mkv", sdk.ParsedMedia{Title: "Naruto", Class: "episode", Index: 14, AbsoluteOrdinal: 14}, true},
		{"/tv/Pokemon/Season 3/Pokemon 3x01 - Episode 3.01.mkv", episode("Pokemon", "3", 1), true},
		{"/tv/Show/Season 1/Show - Episode 14 - 316.mkv", episode("Show", "1", 14), true},
		{"/downloads/One Piece Temporada 1 Episodio 1071.mkv", episode("One Piece", "1", 1071), true},
		{"/downloads/Anubis saison 1 ep 720p.mkv", sdk.ParsedMedia{Title: "Anubis saison 1 ep", Class: "movie"}, true},
		{"/downloads/Dark Folge 2.04.mkv", episode("Dark", "2", 4), true},
		{"/downloads/Fringe.T01E01-E02.Pilot.mkv", sdk.ParsedMedia{Title: "Fringe", Class: "episode", Sequence: "1", Index: 1, IndexEnd: 2}, true},
		{"/downloads/Show Saison 2 E\u0301pisode 4.mkv", episode("Show", "2", 4), true},
		{"/tv/Show/Season 3/Show Folge-03.mkv", episode("Show", "3", 3), true},
		{"/tv/Show/Season 1/Show - 05 - The Cap 3 Heist.mkv", sdk.ParsedMedia{Title: "Show", Class: "episode", Index: 5, AbsoluteOrdinal: 5}, true},
		{"/tv/E60/Season 1/E60 - 014.mkv", sdk.ParsedMedia{Title: "E60", Class: "episode", Index: 14, AbsoluteOrdinal: 14}, true},
		{"/movies/Star.Wars.Ep.1.10.bit.1080p.mkv", sdk.ParsedMedia{Title: "Star Wars Ep 1", Class: "movie"}, true},
		{"/downloads/Fringe.Episode.1.22-Bittersweet.mkv", episode("Fringe", "1", 22), true},
		{"/downloads/Fringe.T01E01-T01E02.Pilot.mkv", sdk.ParsedMedia{Title: "Fringe", Class: "episode", Sequence: "1", Index: 1, IndexEnd: 2}, true},
		{"/downloads/[Fansub] Show Season 1 Episode 03v2.mkv", episode("Show", "1", 3), true},
		{"/downloads/Show Episode 1.10-bit - Episode 2.04.mkv", episode("Show Episode 1 10-bit", "2", 4), true},
		{"/tv/E60/Season 1/E60 - 1x01-1x02 - Pilot.mkv", sdk.ParsedMedia{Title: "E60", Class: "episode", Sequence: "1", Index: 1, IndexEnd: 2}, true},
		{"/tv/E60/Season 1/[ESPN] E60 - 014.mkv", sdk.ParsedMedia{Title: "E60", Class: "episode", Index: 14, AbsoluteOrdinal: 14}, true},
		{"/tv/Fringe/Season 1/Episode 1.22 - 022.mkv", episode("Fringe", "1", 22), true},
		{"/movies/Star.Wars.Ep.1.10-bits.1080p.mkv", sdk.ParsedMedia{Title: "Star Wars Ep 1 10-bits", Class: "movie"}, true},
		{"/tv/Show/Season 1/Show - E14 - Episode 14 - 316.mkv", sdk.ParsedMedia{Title: "Show - E14", Class: "episode", Sequence: "1", Index: 14}, true},
		{"/downloads/Show.Episode.1.02.Bits.and.Pieces.mkv", sdk.ParsedMedia{Title: "Show", Class: "episode", Sequence: "1", Index: 2}, true},
		{"/tv/Show/Season 1/Show.E14.Episode.14 - The Cap 3 Heist.mkv", sdk.ParsedMedia{Title: "Show E14", Class: "episode", Sequence: "1", Index: 14}, true},
		{"/tv/Show/Season 2/Show Episode 1.10-bit - Episode 2.04.mkv", sdk.ParsedMedia{Title: "Show Episode 1 10-bit", Class: "episode", Sequence: "2", Index: 4}, true},
		{"/tv/Show/Season 1/Show - Episode 14 - 1x14-1x15 - Finale.mkv", sdk.ParsedMedia{Title: "Show - Episode 14", Class: "episode", Sequence: "1", Index: 14, IndexEnd: 15}, true},
		{"/movies/Star.Wars.Episode.1.24-bit.FLAC.mkv", sdk.ParsedMedia{Title: "Star Wars Episode 1 24-bit FLAC", Class: "movie"}, true},
		{"/downloads/Contemp 2 Ep 3.mkv", sdk.ParsedMedia{Title: "Contemp 2 Ep 3", Class: "movie"}, true},
		// Episode zero is no episode.
		{"/tv/Show/Season 1/Show Ep 0.mkv", sdk.ParsedMedia{Title: "Show Ep 0"}, true},
	}
	for _, c := range cases {
		got, ok := parser.Parse(c.path, sdk.ParseProbe{})
		if ok != c.ok || (ok && !reflect.DeepEqual(got, c.want)) {
			t.Errorf("Parse(%q) = %+v, %v; want %+v, %v", c.path, got, ok, c.want, c.ok)
		}
	}
}

// TestEpisodeMarkerTitles pins the episode title after a marker: the same
// episode restated after it is no part of the title, another one is
// (CORE-2702).
func TestEpisodeMarkerTitles(t *testing.T) {
	parser := filmtv.New()
	for path, want := range map[string]string{
		"/tv/Pokemon/Season 3/Pokemon 3x01 - Episode 3.01 - Pilot.mkv":                 "Pilot",
		"/downloads/Fringe.T01E01-T01E02.Pilot.mkv":                                    "Pilot",
		"/downloads/Fringe.Episode.1.22-Bittersweet.mkv":                               "Bittersweet",
		"/tv/Pokemon/Season 3/Pokemon S03E01 - Episode 3.01 - Pilot.mkv":               "Pilot",
		"/tv/Fringe/Season 1/Fringe 1x14 - Episode 1.15.mkv":                           "Episode 1 15",
		"/downloads/Show.Episode.1.02.Bits.and.Pieces.mkv":                             "Bits and Pieces",
		"/tv/Naruto/Season 1/Naruto - 014 - Episode 1.14 - The Bridge.mkv":             "The Bridge",
		"/tv/Show/Season 1/Show.E14.Episode.14 - The Cap 3 Heist.mkv":                  "The Cap 3 Heist",
		"/tv/Newsnight/Season 2024/Newsnight - Episode 2024-05-12 - The Interview.mkv": "The Interview",
	} {
		got := ""
		for _, field := range parser.ParseTypedMediaFields(path, sdk.ParseProbe{}) {
			if field.Name == "episode-title" {
				got = field.Text
			}
		}
		if got != want {
			t.Errorf("episode title of %s = %q, want %q", path, got, want)
		}
	}
}
