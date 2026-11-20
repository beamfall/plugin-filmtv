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

func TestPlexSourceItemsBySourceUser(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plex.db")
	seedFilmTVDatabase(t, path, []string{
		`CREATE TABLE media_parts (id INTEGER PRIMARY KEY, media_item_id INTEGER, file TEXT)`,
		`CREATE TABLE media_items (id INTEGER PRIMARY KEY, metadata_item_id INTEGER, duration INTEGER)`,
		`CREATE TABLE metadata_items (id INTEGER PRIMARY KEY, guid TEXT)`,
		`CREATE TABLE metadata_item_settings (account_id INTEGER, guid TEXT, view_offset INTEGER, view_count INTEGER, last_viewed_at INTEGER, rating REAL)`,
		`CREATE TABLE accounts (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE tags (id INTEGER PRIMARY KEY, tag TEXT, tag_type INTEGER)`,
		`CREATE TABLE taggings (metadata_item_id INTEGER, tag_id INTEGER, "index" INTEGER)`,
		`INSERT INTO accounts VALUES (1, 'Alice'), (2, 'Bob'), (3, 'Empty')`,
		`INSERT INTO metadata_items VALUES (1, 'shared'), (2, 'rated'), (3, NULL), (4, 'collection-only')`,
		`INSERT INTO media_items VALUES (1, 1, 90000), (2, 1, 91000), (3, 2, 120000), (4, 3, 120000), (5, 4, 120000)`,
		`INSERT INTO media_parts VALUES (1, 1, '/movie.mkv'), (2, 1, '/part2.mkv'), (3, 2, '/version.mkv'),
			(4, 3, '/zero-rated.mkv'), (5, 4, '/ignore.mkv'), (6, 5, '/collection-only.mkv')`,
		`INSERT INTO metadata_item_settings VALUES (1, 'shared', 0, 2, 1700000000, 8.5),
			(2, 'shared', 12500, 0, 1700000010, NULL), (1, 'rated', NULL, 0, NULL, 0),
			(1, NULL, 89000, 1, 1700000030, 10), (99, 'shared', 18000, 0, 1700000020, 10)`,
		`INSERT INTO tags VALUES (1, 'Favorites', 2), (2, 'Weekend', 2), (3, 'Genre', 1)`,
		`INSERT INTO taggings VALUES (1, 1, 3), (1, 2, 0), (1, 3, 4), (4, 2, 7)`,
	})
	source := NewPlexSource(path)
	wantCategories := []string{sdk.CategoryWatchState, sdk.CategoryRatings, sdk.CategoryCollections}
	if got := source.Categories(); !reflect.DeepEqual(got, wantCategories) {
		t.Fatalf("Plex source categories = %v, want %v", got, wantCategories)
	}
	bySourceUser, ok := source.(interface {
		ItemsBySourceUser(context.Context) (map[string][]sdk.ForeignItem, error)
	})
	if !ok {
		t.Fatal("Plex source does not implement ItemsBySourceUser")
	}
	got, err := bySourceUser.ItemsBySourceUser(context.Background())
	if err != nil {
		t.Fatalf("ItemsBySourceUser: %v", err)
	}
	want := map[string][]sdk.ForeignItem{"Alice#1": nil, "Bob#2": nil, "Empty#3": nil}
	collections := []sdk.ForeignItem{{
		Path:             "/collection-only.mkv",
		CollectionMember: sdk.CollectionMember{Present: true, Name: "Weekend", Position: 7},
	}}
	for _, file := range []struct {
		path     string
		duration int
	}{
		{path: "/movie.mkv", duration: 90},
		{path: "/part2.mkv", duration: 90},
		{path: "/version.mkv", duration: 91},
	} {
		want["Alice#1"] = append(want["Alice#1"], sdk.ForeignItem{
			Path: file.path,
			WatchState: sdk.WatchState{
				Present: true, Watched: true, DurationSeconds: file.duration, OccurredAt: time.Unix(1700000000, 0).UTC(),
			},
			Rating: sdk.Rating{Present: true, Value: 8.5, Scale: 10},
		})
		want["Bob#2"] = append(want["Bob#2"], sdk.ForeignItem{
			Path: file.path,
			WatchState: sdk.WatchState{
				Present: true, PositionSeconds: 12, DurationSeconds: file.duration, OccurredAt: time.Unix(1700000010, 0).UTC(),
			},
		})
		collections = append(collections,
			sdk.ForeignItem{Path: file.path, CollectionMember: sdk.CollectionMember{Present: true, Name: "Favorites", Position: 3}},
			sdk.ForeignItem{Path: file.path, CollectionMember: sdk.CollectionMember{Present: true, Name: "Weekend", Position: 0}},
		)
	}
	want["Alice#1"] = append(want["Alice#1"], sdk.ForeignItem{
		Path: "/zero-rated.mkv", Rating: sdk.Rating{Present: true, Value: 0, Scale: 10},
	})
	for account, items := range want {
		want[account] = append(items, collections...)
	}
	if len(got) != len(want) {
		t.Fatalf("ItemsBySourceUser returned %d accounts, want %d", len(got), len(want))
	}
	for account, items := range want {
		if !reflect.DeepEqual(got[account], items) {
			t.Errorf("items for %s = %#v, want %#v", account, got[account], items)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := bySourceUser.ItemsBySourceUser(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled ItemsBySourceUser error = %v, want context.Canceled", err)
	}
	assertFilmTVStreamMatchesItems(t, source)
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
