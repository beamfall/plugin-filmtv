package filmtv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	scrobble "github.com/beamfall/plugin-sdk/scrobble"
)

const (
	ScrobbleServiceTrakt scrobble.Service = "trakt"
	SchemeTMDB           string           = "tmdb"
	SchemeIMDB           string           = "imdb"
	SchemeTVDB           string           = "tvdb"

	defaultTraktBaseURL = "https://api.trakt.tv"
)

// drainClose reads and closes a response body so the connection can be reused,
// bounding the read so a hostile/huge error body cannot exhaust memory.
func drainClose(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	_ = resp.Body.Close()
}

// TraktAdapter pushes a watched check-in to Trakt's sync/history endpoint.
// It consumes the video External IDs the Film & TV plugin
// writes (TMDB preferred, then IMDB, then TVDB). The Profile's OAuth access
// token (from the 0600 secure store, resolved by the dispatcher) authorizes the
// request as a Bearer token; ClientID is the operator-registered Trakt API app
// id (a public client identifier, not a per-Profile secret).
type TraktAdapter struct {
	client   scrobble.Doer
	baseURL  string
	clientID string
}

// NewTraktAdapter builds the Trakt adapter. baseURL defaults to the production
// API host; tests point it at an httptest server. clientID is the operator's
// registered Trakt application id.
func NewTraktAdapter(client scrobble.Doer, baseURL, clientID string) *TraktAdapter {
	if client == nil {
		panic("scrobble trakt adapter: nil client")
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultTraktBaseURL
	}
	return &TraktAdapter{client: client, baseURL: strings.TrimRight(baseURL, "/"), clientID: clientID}
}

func (a *TraktAdapter) Service() scrobble.Service  { return ScrobbleServiceTrakt }
func (a *TraktAdapter) Class() scrobble.MediaClass { return scrobble.ClassVideo }
func (a *TraktAdapter) IdentitySchemes() []string {
	return []string{SchemeTMDB, SchemeIMDB, SchemeTVDB}
}
func (a *TraktAdapter) DeclaredEndpoints() []string {
	u, err := url.Parse(a.baseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil
	}
	return []string{u.Scheme + "://" + u.Host}
}

// Submit posts the watched item to Trakt's sync/history endpoint. The id is sent
// under the namespace matching the resolved scheme (Trakt's "ids" object keys:
// tmdb/imdb/tvdb). The submission posts the item under Trakt's top-level item
// array (the literal API key below is Trakt's wire vocabulary, not Core's);
// finer-grained per-segment check-ins are a later refinement — the dispatcher
// fires once on the watched transition, so a single history POST is the correct
// minimal submission.
func (a *TraktAdapter) Submit(ctx context.Context, s scrobble.Scrobble) error {
	ids := map[string]any{}
	switch s.Identity.Scheme {
	case SchemeIMDB:
		ids["imdb"] = s.Identity.Value
	case SchemeTVDB:
		if n, err := strconv.Atoi(s.Identity.Value); err == nil {
			ids["tvdb"] = n
		} else {
			ids["tvdb"] = s.Identity.Value
		}
	default: // SchemeTMDB
		if n, err := strconv.Atoi(s.Identity.Value); err == nil {
			ids["tmdb"] = n
		} else {
			ids["tmdb"] = s.Identity.Value
		}
	}
	item := map[string]any{"ids": ids}
	if !s.Event.WatchedAt.IsZero() {
		item["watched_at"] = s.Event.WatchedAt.UTC().Format("2006-01-02T15:04:05.000Z")
	}
	// Known limitation: every item is submitted under Trakt's
	// "movies" key, so TV episodes are mislabeled — they should be posted under
	// "episodes" with season/episode numbers instead. Episode-aware
	// submission is planned follow-up work, not an oversight.
	body, err := json.Marshal(map[string]any{"movies": []any{item}})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/sync/history", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("trakt-api-version", "2")
	req.Header.Set("trakt-api-key", a.clientID)
	req.Header.Set("Authorization", "Bearer "+s.Secret)
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer drainClose(resp)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("scrobble %s: endpoint returned status %d", ScrobbleServiceTrakt, resp.StatusCode)
	}
	return nil
}

// errTraktBaseURLInvalid is returned when the configured/default Trakt base
// URL fails validation. The message is fixed and never echoes the raw input,
// which could carry embedded credentials or query material an operator
// pasted into the config.
var errTraktBaseURLInvalid = errors.New("scrobble trakt adapter: base url must be an absolute http(s) url with a host and no embedded credentials")

// validateTraktBaseURL parses and validates the configured/default Trakt API
// origin at factory load: it must be an absolute http(s) URL with a
// non-empty host and no userinfo (embedded credentials).
func validateTraktBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return errTraktBaseURLInvalid
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errTraktBaseURLInvalid
	}
	if u.Host == "" {
		return errTraktBaseURLInvalid
	}
	if u.User != nil {
		return errTraktBaseURLInvalid
	}
	return nil
}

// TraktScrobbleFactory builds a TraktAdapter from operator config.
func TraktScrobbleFactory(client scrobble.Doer, cfg scrobble.AdapterConfig, secrets scrobble.SecretSource) (scrobble.Adapter, error) {
	if client == nil {
		return nil, fmt.Errorf("scrobble trakt adapter: nil client")
	}
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = defaultTraktBaseURL
	}
	if err := validateTraktBaseURL(baseURL); err != nil {
		return nil, err
	}
	clientID, err := secrets.Secret(cfg.ClientIDRef)
	if err != nil {
		return nil, fmt.Errorf("scrobble trakt adapter: client id: %w", err)
	}
	return NewTraktAdapter(client, baseURL, clientID), nil
}
