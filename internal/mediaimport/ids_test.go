package mediaimport

import "testing"

// TestSchemeConsistency pins the bare external-ID scheme literals used in
// this package (ids.go, nfo.go) against the canonical values of
// filmtv.SchemeTMDB / SchemeIMDB / SchemeTVDB (trakt_scrobble.go).
//
// mediaimport cannot import the filmtv package directly: filmtv already
// imports mediaimport (see nfotree.go), so a back-import would create an
// import cycle. This test guards against silent drift instead — if the
// canonical constants in trakt_scrobble.go ever change, update the expected
// values below to match and audit every literal in this package.
func TestSchemeConsistency(t *testing.T) {
	const (
		wantSchemeTMDB = "tmdb"
		wantSchemeIMDB = "imdb"
		wantSchemeTVDB = "tvdb"
	)

	got, ok := normalizeExternalID("tmdb", "1")
	if !ok || got.Source != wantSchemeTMDB {
		t.Fatalf("tmdb source = %q, want %q", got.Source, wantSchemeTMDB)
	}
	got, ok = normalizeExternalID("themoviedb", "1")
	if !ok || got.Source != wantSchemeTMDB {
		t.Fatalf("themoviedb normalizes to %q, want %q", got.Source, wantSchemeTMDB)
	}
	got, ok = normalizeExternalID("imdb", "tt1")
	if !ok || got.Source != wantSchemeIMDB {
		t.Fatalf("imdb source = %q, want %q", got.Source, wantSchemeIMDB)
	}
	got, ok = normalizeExternalID("internetmoviedatabase", "tt1")
	if !ok || got.Source != wantSchemeIMDB {
		t.Fatalf("internetmoviedatabase normalizes to %q, want %q", got.Source, wantSchemeIMDB)
	}
	got, ok = normalizeExternalID("tvdb", "1")
	if !ok || got.Source != wantSchemeTVDB {
		t.Fatalf("tvdb source = %q, want %q", got.Source, wantSchemeTVDB)
	}
	got, ok = normalizeExternalID("thetvdb", "1")
	if !ok || got.Source != wantSchemeTVDB {
		t.Fatalf("thetvdb normalizes to %q, want %q", got.Source, wantSchemeTVDB)
	}

	if len(preferredIDSourceOrder) != 3 ||
		preferredIDSourceOrder[0] != wantSchemeTMDB ||
		preferredIDSourceOrder[1] != wantSchemeIMDB ||
		preferredIDSourceOrder[2] != wantSchemeTVDB {
		t.Fatalf("preferredIDSourceOrder = %#v, want [%q %q %q]",
			preferredIDSourceOrder, wantSchemeTMDB, wantSchemeIMDB, wantSchemeTVDB)
	}
}
