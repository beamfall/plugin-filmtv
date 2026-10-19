package filmtv

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	scrobble "github.com/beamfall/plugin-sdk/scrobble"
)

func videoEvent() scrobble.WatchedEvent {
	return scrobble.WatchedEvent{
		ProfileID:   "profile-1",
		MediaID:     "media-1",
		LibraryID:   "library-films",
		Class:       scrobble.ClassVideo,
		Title:       "Sintel",
		Year:        2010,
		ExternalIDs: []scrobble.ExternalID{{Scheme: SchemeTMDB, Value: "45745"}},
		WatchedAt:   time.Unix(1_700_000_000, 0).UTC(),
	}
}

// TestTraktAdapterPostsBearerHistory: the Trakt adapter posts to /sync/history
// with the Profile's token as a Bearer credential and the tmdb id in the body.
func TestTraktAdapterPostsBearerHistory(t *testing.T) {
	var gotAuth, gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	a := NewTraktAdapter(srv.Client(), srv.URL, "client-id-xyz")
	err := a.Submit(context.Background(), scrobble.Scrobble{
		Event:    videoEvent(),
		Identity: scrobble.ExternalID{Scheme: SchemeTMDB, Value: "45745"},
		Secret:   "token-abc",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if gotAuth != "Bearer token-abc" {
		t.Errorf("Authorization header = %q, want Bearer token-abc", gotAuth)
	}
	if gotPath != "/sync/history" {
		t.Errorf("path = %q, want /sync/history", gotPath)
	}
	if !strings.Contains(gotBody, "45745") {
		t.Errorf("body missing tmdb id: %s", gotBody)
	}
}

// TestTraktAdapterNon2xxRetryable: a non-2xx response is an error so the
// dispatcher retries it.
func TestTraktAdapterNon2xxRetryable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	a := NewTraktAdapter(srv.Client(), srv.URL, "cid")
	if err := a.Submit(context.Background(), scrobble.Scrobble{Identity: scrobble.ExternalID{Scheme: SchemeTMDB, Value: "1"}}); err == nil {
		t.Fatal("a 500 response must be an error (retryable)")
	}
}

func TestTraktAdapterDeclaredEndpointsUsesBaseURLOrigin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()

	a := NewTraktAdapter(srv.Client(), srv.URL+"/nested", "cid")
	got := a.DeclaredEndpoints()
	if len(got) != 1 || got[0] != srv.URL {
		t.Fatalf("DeclaredEndpoints() = %#v, want %#v", got, []string{srv.URL})
	}
}

func TestTraktAdapterDeclaredEndpointsDefaultsToTrakt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()

	a := NewTraktAdapter(srv.Client(), "", "cid")
	got := a.DeclaredEndpoints()
	if len(got) != 1 || got[0] != defaultTraktBaseURL {
		t.Fatalf("DeclaredEndpoints() = %#v, want %#v", got, []string{defaultTraktBaseURL})
	}
}

func TestNewTraktAdapterPanicsOnNilClient(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewTraktAdapter(nil, ...) did not panic")
		}
	}()
	NewTraktAdapter(nil, "", "cid")
}

// stubSecretSource resolves any ref to a fixed secret; sufficient for factory
// tests that only exercise BaseURL validation.
type stubSecretSource struct{ secret string }

func (s stubSecretSource) Secret(string) (string, error) { return s.secret, nil }

// TestTraktScrobbleFactoryValidatesBaseURL: the factory validates the
// configured/default origin at load time — absolute http(s), non-empty host,
// no embedded userinfo — and preserves httptest injection for a valid URL.
func TestTraktScrobbleFactoryValidatesBaseURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()

	cases := []struct {
		name    string
		baseURL string
		wantErr bool
	}{
		{"valid httptest url preserved", srv.URL, false},
		{"empty defaults to production", "", false},
		{"missing scheme rejected", "api.trakt.tv", true},
		{"non-http(s) scheme rejected", "ftp://api.trakt.tv", true},
		{"empty host rejected", "https:///sync", true},
		{"userinfo rejected", "https://user:pass@api.trakt.tv", true},
		{"malformed url rejected", "https://api.trakt.tv/%zz", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := scrobble.AdapterConfig{BaseURL: tc.baseURL, ClientIDRef: "ref"}
			_, err := TraktScrobbleFactory(srv.Client(), cfg, stubSecretSource{secret: "cid"})
			if tc.wantErr && err == nil {
				t.Fatalf("TraktScrobbleFactory(%q): want error, got nil", tc.baseURL)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("TraktScrobbleFactory(%q): unexpected error: %v", tc.baseURL, err)
			}
		})
	}
}

// TestTraktScrobbleFactoryErrorNeverEchoesRawURL: the validation error is a
// fixed, bounded message that never leaks the raw (possibly credential- or
// query-bearing) input URL back to the caller.
func TestTraktScrobbleFactoryErrorNeverEchoesRawURL(t *testing.T) {
	const raw = "https://user:s3cr3t-pass@api.trakt.tv/?token=leak-me"
	cfg := scrobble.AdapterConfig{BaseURL: raw, ClientIDRef: "ref"}
	_, err := TraktScrobbleFactory(http.DefaultClient, cfg, stubSecretSource{secret: "cid"})
	if err == nil {
		t.Fatal("want error for userinfo-bearing base url, got nil")
	}
	if strings.Contains(err.Error(), "s3cr3t-pass") || strings.Contains(err.Error(), "leak-me") {
		t.Fatalf("error echoes credential/query material: %v", err)
	}
}

// TestTraktScrobbleFactorySubmitsThroughValidatedBaseURL: a factory-created
// adapter's DeclaredEndpoints reports the origin of the validated base URL,
// and Submit through that adapter actually reaches that origin — proving the
// validated URL, not some other value, is what the adapter uses end-to-end.
func TestTraktScrobbleFactorySubmitsThroughValidatedBaseURL(t *testing.T) {
	var gotPath, gotHost string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotHost = r.Host
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	cfg := scrobble.AdapterConfig{BaseURL: srv.URL, ClientIDRef: "ref"}
	adapter, err := TraktScrobbleFactory(srv.Client(), cfg, stubSecretSource{secret: "cid"})
	if err != nil {
		t.Fatalf("TraktScrobbleFactory: unexpected error: %v", err)
	}

	endpoints := adapter.DeclaredEndpoints()
	if len(endpoints) != 1 || endpoints[0] != srv.URL {
		t.Fatalf("DeclaredEndpoints() = %#v, want %#v", endpoints, []string{srv.URL})
	}

	err = adapter.Submit(context.Background(), scrobble.Scrobble{
		Event:    videoEvent(),
		Identity: scrobble.ExternalID{Scheme: SchemeTMDB, Value: "45745"},
		Secret:   "token-abc",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if gotPath != "/sync/history" {
		t.Errorf("path = %q, want /sync/history", gotPath)
	}
	wantHost := strings.TrimPrefix(strings.TrimPrefix(srv.URL, "https://"), "http://")
	if gotHost != wantHost {
		t.Errorf("host = %q, want %q", gotHost, wantHost)
	}
}
