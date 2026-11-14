package filmtv_test

import (
	"strings"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"

	filmtv "github.com/beamfall/plugin-filmtv"
)

// TestParseVersionLabel pins which filenames carry a version label (CORE-1676):
// only the `Title (Year) - <label>` convention, never a dotted release tail, a
// stacked part, an extra or an episode title. The title, class and year stay
// exactly what the unlabelled name parses to, so core can group the files.
func TestParseVersionLabel(t *testing.T) {
	long := strings.Repeat("a", sdk.MaxVersionLabelLen) + " tail"
	cases := []struct {
		path      string
		wantTitle string
		wantLabel string
	}{
		{"Night Harbor (2004) - 1080p.mkv", "Night Harbor", "1080p"},
		{"Night Harbor (2004) - 2160p HDR.mkv", "Night Harbor", "2160p HDR"},
		{"Night Harbor (2004) - Director's Cut.mkv", "Night Harbor", "Director's Cut"},
		{"Night Harbor (2004) - [Extended Edition].mkv", "Night Harbor", "Extended Edition"},
		{"Night Harbor (2004)  -  (Theatrical).mkv", "Night Harbor", "Theatrical"},
		{"Movies/Night Harbor (2004)/Night Harbor (2004) - 4K.mkv", "Night Harbor", "4K"},
		{"Night Harbor (2004) - 1080p {tmdb-123}.mkv", "Night Harbor", "1080p"},
		{"Night Harbor (2004) - " + long + ".mkv", "Night Harbor", strings.Repeat("a", sdk.MaxVersionLabelLen)},
		{"Night Harbor (2004).mkv", "Night Harbor", ""},
		{"Night.Harbor.2004.1080p.BluRay.x264.mkv", "Night Harbor", ""},
		{"Night Harbor 2004 - 1080p.mkv", "Night Harbor", ""},
		{"Night Harbor (2004)-1080p.mkv", "Night Harbor", ""},
		{"Night Harbor (2004) 1080p.mkv", "Night Harbor", ""},
		{"Night Harbor (2004) - .mkv", "Night Harbor", ""},
		{"Night Harbor (2004) - [].mkv", "Night Harbor", ""},
		{"Night Harbor (2004) - cd1.mkv", "Night Harbor", ""},
		{"Night Harbor (2004) - Part 2.mkv", "Night Harbor", ""},
		{"Night Harbor (2004) - trailer.mkv", "Night Harbor", ""},
		{"Night Harbor (2004) - Sample.mkv", "Night Harbor", ""},
		{"Night Harbor (2004) - trailer 2.mkv", "Night Harbor", ""},
		{"Night Harbor (2004) - Behind the Scenes.mkv", "Night Harbor", ""},
		{"Night Harbor (2004) - Deleted Scenes.mkv", "Night Harbor", ""},
		{"Night Harbor (2004) - Interviews.mkv", "Night Harbor", ""},
		{"Night Harbor (2004) - Bloopers.mkv", "Night Harbor", ""},
		{"Night Harbor (2004) - Making Of.mkv", "Night Harbor", ""},
		{"Night Harbor (2004) - Teaser 1.mkv", "Night Harbor", ""},
		{"Night Harbor (2004) - disc_1.mkv", "Night Harbor", ""},
		{"Coastline (2011) - S01E02 - Low Tide.mkv", "Coastline", ""},
	}
	p := filmtv.New()
	for _, c := range cases {
		got, ok := p.Parse(c.path, sdk.ParseProbe{})
		if !ok {
			t.Errorf("Parse(%q) refused", c.path)
			continue
		}
		if got.Title != c.wantTitle || got.VersionLabel != c.wantLabel {
			t.Errorf("Parse(%q) = title %q label %q, want title %q label %q", c.path, got.Title, got.VersionLabel, c.wantTitle, c.wantLabel)
		}
		if err := sdk.ValidateVersionLabel(got); err != nil {
			t.Errorf("Parse(%q): %v", c.path, err)
		}
	}
}

// TestParseVersionLabelKeepsGroupingKey proves two versions of one title parse
// to identical fields apart from the label, which is what core groups on.
func TestParseVersionLabelKeepsGroupingKey(t *testing.T) {
	p := filmtv.New()
	a, _ := p.Parse("Night Harbor (2004) - 1080p.mkv", sdk.ParseProbe{})
	b, _ := p.Parse("Night Harbor (2004) - 2160p.mkv", sdk.ParseProbe{})
	c, _ := p.Parse("Night Harbor (2004).mkv", sdk.ParseProbe{})
	a.VersionLabel, b.VersionLabel = "", ""
	if a.Title != b.Title || a.Year != b.Year || a.Class != b.Class || a.Title != c.Title || a.Year != c.Year || a.Class != c.Class {
		t.Fatalf("versions parse apart: %+v / %+v / %+v", a, b, c)
	}
}
