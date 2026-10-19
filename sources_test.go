package filmtv

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/beamfall/plugin-filmtv/internal/foreignimport"
	sdk "github.com/beamfall/plugin-sdk"
	"github.com/beamfall/plugin-sdk/sdktest"
)

func TestImportSourcesHonorCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tests := map[string]sdk.Source{ //nolint:staticcheck // deliberate SDK 0.x compatibility path for pre-streaming consumers
		"sqlite": NewPlexSource(filepath.Join(t.TempDir(), "missing.db")),
		"nfo":    NewNFOSource(t.TempDir()),
	}
	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := source.Items(ctx); !errors.Is(err, context.Canceled) { //nolint:staticcheck // deliberate SDK 0.x compatibility assertion
				t.Fatalf("canceled import error = %v, want context.Canceled", err)
			}
		})
	}
}

func TestImporterStreamingCancellationAndCheckpoint(t *testing.T) {
	rows := []foreignimport.WatchRow{{
		Path:            "/movie.mkv",
		PositionSeconds: 12,
		DurationSeconds: 120,
		Watched:         true,
		LastPlayed:      time.Unix(1_700_000_000, 0),
	}}
	reader := func(context.Context, string) ([]foreignimport.WatchRow, error) {
		return append([]foreignimport.WatchRow(nil), rows...), nil
	}
	streamer := func(
		ctx context.Context,
		_ string,
		visit func(foreignimport.WatchRow) error,
	) error {
		for _, row := range rows {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := visit(row); err != nil {
				return err
			}
		}
		return nil
	}
	watchItems := []sdk.ForeignItem{{
		Path: "/movie.mkv",
		WatchState: sdk.WatchState{
			Present:         true,
			PositionSeconds: 12,
			DurationSeconds: 120,
			Watched:         true,
			OccurredAt:      rows[0].LastPlayed,
		},
	}}
	traktPath := filepath.Join(t.TempDir(), "trakt.csv")
	if err := os.WriteFile(traktPath, []byte("watched_at,tmdb\n2024-03-04T05:06:07Z,123\n"), 0o600); err != nil {
		t.Fatalf("write Trakt fixture: %v", err)
	}
	traktItems, err := NewTraktSource(traktPath).Items(context.Background()) //nolint:staticcheck // deliberate SDK 0.x compatibility assertion
	if err != nil {
		t.Fatalf("parse Trakt fixture: %v", err)
	}
	nfoRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(nfoRoot, "movie.mkv"), nil, 0o600); err != nil {
		t.Fatalf("write NFO-tree fixture: %v", err)
	}
	nfoSource := NewNFOSource(nfoRoot)
	nfoItems, err := nfoSource.Items(context.Background()) //nolint:staticcheck // deliberate SDK 0.x compatibility assertion
	if err != nil {
		t.Fatalf("parse NFO-tree fixture: %v", err)
	}
	vectors := []sdktest.ImporterVector{
		{Name: "plex", Source: dbWatchSource{read: reader, stream: streamer}, WantCategories: []string{sdk.CategoryWatchState}, WantItems: watchItems},
		{Name: "emby", Source: dbWatchSource{read: reader, stream: streamer}, WantCategories: []string{sdk.CategoryWatchState}, WantItems: watchItems},
		{Name: "jellyfin", Source: dbWatchSource{read: reader, stream: streamer}, WantCategories: []string{sdk.CategoryWatchState}, WantItems: watchItems},
		{Name: "nfo", Source: nfoSource, WantCategories: []string{sdk.CategoryCollections}, WantItems: nfoItems},
		{Name: "trakt", Source: NewTraktSource(traktPath), WantCategories: []string{sdk.CategoryWatchState}, WantItems: traktItems},
	}
	for _, vector := range vectors {
		items, err := vector.Source.Items(context.Background()) //nolint:staticcheck // deliberate SDK 0.x compatibility assertion
		if err != nil {
			t.Fatalf("%s compatibility Items: %v", vector.Name, err)
		}
		streamed, err := collectFilmTVStream(vector.Source)
		if err != nil {
			t.Fatalf("%s stream: %v", vector.Name, err)
		}
		if !reflect.DeepEqual(streamed, items) {
			t.Fatalf("%s streamed items differ from compatibility Items:\nstream: %#v\nitems:  %#v", vector.Name, streamed, items)
		}
	}
	sdktest.RunImporterConformance(t, vectors)
}

func collectFilmTVStream(source sdk.Source) ([]sdk.ForeignItem, error) { //nolint:staticcheck // deliberate SDK 0.x compatibility path for pre-streaming consumers
	streaming := source.(interface {
		Stream(context.Context, string, func(context.Context, []sdk.ForeignItem, string) error) error
	})
	items := []sdk.ForeignItem{}
	err := streaming.Stream(context.Background(), "", func(
		_ context.Context,
		page []sdk.ForeignItem,
		_ string,
	) error {
		items = append(items, page...)
		return nil
	})
	return items, err
}

func TestDatabaseImporterStreamsMatchCompatibilityItems(t *testing.T) {
	t.Run("plex", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "plex.db")
		seedFilmTVDatabase(t, path, []string{
			`CREATE TABLE media_parts (id INTEGER PRIMARY KEY, media_item_id INTEGER, file TEXT)`,
			`CREATE TABLE media_items (id INTEGER PRIMARY KEY, metadata_item_id INTEGER, duration INTEGER)`,
			`CREATE TABLE metadata_items (id INTEGER PRIMARY KEY, guid TEXT)`,
			`CREATE TABLE metadata_item_settings (guid TEXT, view_offset INTEGER, view_count INTEGER, last_viewed_at INTEGER)`,
			`INSERT INTO media_items VALUES (1, 1, 90000)`,
			`INSERT INTO metadata_items VALUES (1, 'guid-1')`,
			`INSERT INTO metadata_item_settings VALUES ('guid-1', 12000, 1, 1700000000)`,
			`INSERT INTO media_parts VALUES (2, 1, '/z.mkv')`,
			`INSERT INTO media_parts VALUES (1, 1, '/a.mkv')`,
		})
		assertFilmTVStreamMatchesItems(t, NewPlexSource(path))
	})

	for _, dialect := range []struct {
		name   string
		source func(string) sdk.Source //nolint:staticcheck // deliberate SDK 0.x compatibility path for pre-streaming consumers
	}{
		{name: "emby", source: NewEmbySource},
		{name: "jellyfin", source: NewJellyfinSource},
	} {
		t.Run(dialect.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), dialect.name+".db")
			seedFilmTVDatabase(t, path, []string{
				`CREATE TABLE TypedBaseItems (Path TEXT, UserDataKey TEXT, RunTimeTicks INTEGER)`,
				`CREATE TABLE UserDatas (key TEXT, Played INTEGER, PlaybackPositionTicks INTEGER, LastPlayedDate TEXT)`,
				`INSERT INTO UserDatas VALUES ('one', 1, 120000000, '2024-01-02T03:04:05Z')`,
				`INSERT INTO TypedBaseItems VALUES ('/z.mkv', 'one', 900000000)`,
				`INSERT INTO TypedBaseItems VALUES ('/a.mkv', 'one', 900000000)`,
			})
			assertFilmTVStreamMatchesItems(t, dialect.source(path))
		})
	}
}

func seedFilmTVDatabase(t *testing.T, path string, statements []string) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(context.Background(), statement); err != nil {
			_ = db.Close()
			t.Fatalf("seed fixture: %v", err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close fixture: %v", err)
	}
}

func assertFilmTVStreamMatchesItems(t *testing.T, source sdk.Source) { //nolint:staticcheck // deliberate SDK 0.x compatibility path for pre-streaming consumers
	t.Helper()
	items, err := source.Items(context.Background()) //nolint:staticcheck // deliberate SDK 0.x compatibility assertion
	if err != nil {
		t.Fatalf("compatibility Items: %v", err)
	}
	streamed, err := collectFilmTVStream(source)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if !reflect.DeepEqual(streamed, items) {
		t.Fatalf("streamed items differ from compatibility Items:\nstream: %#v\nitems:  %#v", streamed, items)
	}
}
