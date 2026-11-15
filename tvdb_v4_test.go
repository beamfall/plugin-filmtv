package filmtv_test

import (
	"regexp"
	"strings"
	"testing"

	filmtv "github.com/beamfall/plugin-filmtv"
	sdk "github.com/beamfall/plugin-sdk"
)

func tvdbSeries(t *testing.T) sdk.ScraperSpec {
	t.Helper()
	for _, scraper := range filmtv.Bundle().Scrapers {
		if scraper.ID == "tvdb-series" {
			return scraper
		}
	}
	t.Fatal("scraper tvdb-series not found")
	return sdk.ScraperSpec{}
}

// TestTVDBPresentsTheSubscriberPIN pins CORE-1127: the AuthRef field is
// labelled as the subscriber PIN and links to TheTVDB's subscription page.
func TestTVDBPresentsTheSubscriberPIN(t *testing.T) {
	help := tvdbSeries(t).CredentialHelp
	if help == nil {
		t.Fatal("tvdb-series declares no credential help")
	}
	if help.Label["en"] != "TheTVDB subscriber PIN" || help.HelpLabel["en"] != "Subscribe at thetvdb.com" {
		t.Errorf("credential help labels = %#v / %#v", help.Label, help.HelpLabel)
	}
	if help.HelpURL != "https://www.thetvdb.com/subscribe" {
		t.Errorf("credential help URL = %q", help.HelpURL)
	}
}

// TestTVDBFollowsTheV4API pins CORE-1128's declarations, confirmed against
// TheTVDB v4 swagger 4.8.0: year-filtered search, the extended record with
// translations in the configured language, remote-ID lookups, and extra
// artwork whose type ids come from /artwork/types rather than literals.
func TestTVDBFollowsTheV4API(t *testing.T) {
	scraper := tvdbSeries(t)
	if scraper.SearchTemplate != "/search?query={query}&type=series&year={year}" {
		t.Errorf("SearchTemplate = %q, want the year filter", scraper.SearchTemplate)
	}
	if scraper.LookupTemplate != "/series/{id}/extended?meta=translations" {
		t.Errorf("LookupTemplate = %q, want the extended record with translations", scraper.LookupTemplate)
	}
	for field, translation := range map[string]string{
		"title":   "data.translations.nameTranslations[language={config.metadata-language}].name||data.name",
		"summary": "data.translations.overviewTranslations[language={config.metadata-language}].overview||data.overview",
	} {
		if !strings.HasPrefix(scraper.FieldMap[field], translation) {
			t.Errorf("FieldMap[%s] = %q, want the configured translation then the original", field, scraper.FieldMap[field])
		}
	}
	language := false
	for _, field := range filmtv.Bundle().ConfigFields {
		if field.Name == "metadata-language" {
			language = field.Default == "eng" && field.Type == "text"
		}
	}
	if !language {
		t.Error("bundle declares no metadata-language config field defaulting to eng")
	}
	if doc, ok := scraper.ReferenceDocuments["artwork-types"]; !ok || doc.Path != "/artwork/types" || doc.CacheHours != 0 {
		t.Errorf("artwork-types reference document = %#v, want /artwork/types cached for the default week", doc)
	}
	literalType := regexp.MustCompile(`type=\d`)
	for slot, slug := range map[string]string{"backdrop": "background", "banner": "banner", "logo": "clearlogo"} {
		path := scraper.ArtworkMap[slot]
		if !strings.Contains(path, "{ref.artwork-types:data[slug="+slug+",recordType=series].id}") || literalType.MatchString(path) {
			t.Errorf("ArtworkMap[%s] = %q, want the %s type id from /artwork/types", slot, path, slug)
		}
	}
	if !strings.HasPrefix(scraper.ArtworkMap["poster"], "data.image") {
		t.Errorf("poster = %q, want the base-record image", scraper.ArtworkMap["poster"])
	}
}
