package filmtv_test

import (
	"reflect"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"

	filmtv "github.com/beamfall/plugin-filmtv"
)

// TestAlternateTitle pins dual-titled names (CORE-2703): a Latin title and a
// title in another script, split by a bar or a full-width slash or bar, or
// with one in parentheses at the end of the other, parse to the Latin title,
// and the other is the AlternateTitleField Media field. Any other title stays
// whole, with no alternate title.
func TestAlternateTitle(t *testing.T) {
	parser := filmtv.New()
	movie := func(title string, year int) sdk.ParsedMedia {
		return sdk.ParsedMedia{Title: title, Class: "movie", Year: year}
	}
	cases := []struct {
		name      string
		path      string
		want      sdk.ParsedMedia
		alternate string
	}{
		{"bar", "/movies/Голубая волна | Blue Crush (2002) DVDRip.mkv", movie("Blue Crush", 2002), "Голубая волна"},
		{"dotted-bar", "/movies/Голубая.волна.|.Blue.Crush.2002.DVDRip.mkv", movie("Blue Crush", 2002), "Голубая волна"},
		{"full-width-slash", "/movies/Голубая волна ／ Blue Crush (2002).mkv", movie("Blue Crush", 2002), "Голубая волна"},
		{"full-width-bar", "/movies/Мост｜Bridge 2010.mkv", movie("Bridge", 2010), "Мост"},
		{"latin-first", "/movies/Blue Crush | Голубая волна (2002).mkv", movie("Blue Crush", 2002), "Голубая волна"},
		{"latin-in-parentheses", "/movies/О мышах и людях (Of Mice and Men) 1992.mkv", movie("Of Mice and Men", 1992), "О мышах и людях"},
		{"japanese-then-latin", "/movies/千と千尋の神隠し (Spirited Away) 2001.mkv", movie("Spirited Away", 2001), "千と千尋の神隠し"},
		{"native-in-parentheses", "/movies/Spirited Away (千と千尋の神隠し) 2001.mkv", movie("Spirited Away", 2001), "千と千尋の神隠し"},
		{"accented-latin", "/movies/Амели | Le Fabuleux Destin d'Amélie Poulain (2001).mkv", movie("Le Fabuleux Destin d'Amélie Poulain", 2001), "Амели"},
		{
			"episode-series", "/tv/Сериал | The Show S01E02 - Pilot.mkv",
			sdk.ParsedMedia{Title: "The Show", Class: "episode", Sequence: "1", Index: 2}, "Сериал",
		},
		{"full-width-parentheses", "/movies/千と千尋の神隠し（Spirited Away） (2001).mkv", movie("Spirited Away", 2001), "千と千尋の神隠し"},
		{"parentheses-in-latin-title", "/movies/Бёрдмэн | Birdman (or The Unexpected Virtue of Ignorance) (2014).mkv", movie("Birdman (or The Unexpected Virtue of Ignorance)", 2014), "Бёрдмэн"},
		{"year-starts-latin-title", "/movies/Космическая одиссея | 2001 A Space Odyssey 1968.mkv", movie("2001 A Space Odyssey", 1968), "Космическая одиссея"},
		{"common-script-letter", "/movies/LʼAvventura | Приключение (1960).mkv", movie("LʼAvventura", 1960), "Приключение"},
		{"single-han-alternate", "/movies/Ran (乱) 1985.mkv", movie("Ran", 1985), "乱"},
		{"punctuated-latin-title", "/movies/Баллада (R&B) 2010.mkv", movie("R&B", 2010), "Баллада"},
		{"checksum-after-bar", "/movies/Космическая одиссея | [DEADBEEF] 2001 A Space Odyssey 1968.mkv", movie("2001 A Space Odyssey", 1968), "Космическая одиссея"},
		{"bracketed-latin-title", "/movies/Репортаж | [REC] 2007.mkv", movie("[REC]", 2007), "Репортаж"},
		// Controls: no Latin title, two Latin titles, a mixed-script part, a
		// qualifier, or not two titles.
		{"region-qualifier", "/tv/Офис (US)/Season 01/S01E02 - Pilot.mkv", sdk.ParsedMedia{Title: "Офис (US)", Class: "episode", Sequence: "1", Index: 2}, ""},
		{"edition-qualifier", "/movies/Бразилия (Director's Cut) (1985).mkv", movie("Бразилия (Director's Cut)", 1985), ""},
		{"release-word-qualifier", "/movies/Амели (Web) 2001.mkv", movie("Амели (Web)", 2001), ""},
		// An edition tag before the year is no title text at all (CORE-2704).
		{"edition-tag-before-year", "/movies/Амели (Extended) 2001.mkv", movie("Амели", 2001), ""},
		{"edition-tags-before-year", "/movies/Бразилия (4K Remastered) (1985).mkv", movie("Бразилия", 1985), ""},
		{"edition-word-is-the-latin-title", "/movies/Пленка | Uncut (1997).mkv", movie("Uncut", 1997), "Пленка"},
		{"unlisted-region-qualifier", "/movies/Офис (DE) 2004.mkv", movie("Офис (DE)", 2004), ""},
		{"curly-apostrophe-edition", "/movies/Бразилия (Director’s Cut) 1985.mkv", movie("Бразилия (Director’s Cut)", 1985), ""},
		{"hyphenated-edition", "/movies/Бразилия (Special-Edition) 1985.mkv", movie("Бразилия (Special-Edition)", 1985), ""},
		{"limited-edition", "/movies/Бразилия (Limited Edition) 1985.mkv", movie("Бразилия (Limited Edition)", 1985), ""},
		// A title ending in an edition word reads as an edition: accepted.
		{"title-ending-in-cut", "/movies/Тёмная сторона (In the Cut) 2003.mkv", movie("Тёмная сторона (In the Cut)", 2003), ""},
		{"unmatched-inner-parenthesis", "/movies/Мост （Bridge(） 2010.mkv", movie("Мост （Bridge(）", 2010), ""},
		{"mismatched-parentheses", "/movies/Мост (Bridge） 2010.mkv", movie("Мост (Bridge）", 2010), ""},
		{"mixed-script-part", "/movies/Fate／Grand Order -絶対魔獣戦線バビロニア-.mkv", movie("Fate／Grand Order -絶対魔獣戦線バビロニア", 0), ""},
		{"non-latin-only", "/movies/Жихарка (2007).mkv", movie("Жихарка", 2007), ""},
		{"non-latin-in-parentheses", "/movies/Ёжик в тумане (Ёжик) 1975.mkv", movie("Ёжик в тумане (Ёжик)", 1975), ""},
		{"both-latin-parentheses", "/movies/Star Wars (Special Edition) 1977.mkv", movie("Star Wars (Special Edition)", 1977), ""},
		{"both-latin-bar", "/movies/Face|Off (1997).mkv", movie("Face|Off", 1997), ""},
		{"digits-in-parentheses", "/movies/Жихарка (2).mkv", movie("Жихарка (2)", 0), ""},
		{"nested-parentheses", "/movies/Наруто (ТВ) (Naruto) 2002.mkv", movie("Наруто (ТВ) (Naruto)", 2002), ""},
		{"three-titles", "/movies/Мост | Bridge | Brücke 2010.mkv", movie("Мост | Bridge | Brücke", 2010), ""},
		{"empty-side", "/movies/| Blue Crush (2002).mkv", movie("| Blue Crush", 2002), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parser.Parse(tc.path, sdk.ParseProbe{})
			if !ok || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Parse(%q) = %+v, %v; want %+v", tc.path, got, ok, tc.want)
			}
			var alternate string
			for _, field := range parser.ParseMediaFields(tc.path, sdk.ParseProbe{}) {
				if field.Name == filmtv.AlternateTitleField {
					alternate = field.Text
				}
			}
			var typed string
			for _, field := range parser.ParseTypedMediaFields(tc.path, sdk.ParseProbe{}) {
				if field.Name == filmtv.AlternateTitleField {
					typed = field.Text
				}
			}
			if alternate != tc.alternate || typed != tc.alternate {
				t.Errorf("alternate title of %q = %q (typed %q), want %q", tc.path, alternate, typed, tc.alternate)
			}
		})
	}
}

// TestAlternateTitleFieldDeclared pins that the alternate title is a declared,
// searchable text Media field: Core persists only declared Media fields.
func TestAlternateTitleFieldDeclared(t *testing.T) {
	for _, field := range filmtv.Bundle().MediaFields {
		if field.Name == filmtv.AlternateTitleField {
			if field.Type != "text" || !field.Searchable {
				t.Fatalf("%s is declared as %+v, want searchable text", filmtv.AlternateTitleField, field)
			}
			return
		}
	}
	t.Fatalf("bundle declares no %s Media field", filmtv.AlternateTitleField)
}
