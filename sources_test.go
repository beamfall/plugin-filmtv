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
		{
			Name:           "plex",
			Source:         NewPlexSource(seedPlexWatchDatabase(t)),
			WantCategories: []string{sdk.CategoryWatchState, sdk.CategoryRatings, sdk.CategoryCollections},
			WantItems: []sdk.ForeignItem{
				{Path: "/a.mkv", WatchState: sdk.WatchState{
					Present: true, PositionSeconds: 12, DurationSeconds: 90, Watched: true, OccurredAt: rows[0].LastPlayed.UTC(),
				}},
				{Path: "/z.mkv", WatchState: sdk.WatchState{
					Present: true, PositionSeconds: 12, DurationSeconds: 90, Watched: true, OccurredAt: rows[0].LastPlayed.UTC(),
				}},
			},
		},
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
		assertFilmTVStreamMatchesItems(t, NewPlexSource(seedPlexWatchDatabase(t)))
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

func seedPlexWatchDatabase(t *testing.T) string {
	t.Helper()
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
	return path
}

func TestPlexSourceItemsBySourceUser(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plex.db")
	schema, err := os.ReadFile("internal/foreignimport/testdata/plex-accounts.sql")
	if err != nil {
		t.Fatalf("read Plex fixture: %v", err)
	}
	seedFilmTVDatabase(t, path, []string{string(schema)})
	wantCategories := []string{sdk.CategoryWatchState, sdk.CategoryRatings, sdk.CategoryCollections}
	source := NewPlexSourceForCategories(path, wantCategories)
	if got := source.Categories(); !reflect.DeepEqual(got, wantCategories) {
		t.Fatalf("Plex source categories = %v, want %v", got, wantCategories)
	}
	bySourceUser := requireSourceUserItems(t, source)
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

	t.Run("watch-state", func(t *testing.T) {
		watchCategories := []string{sdk.CategoryWatchState}
		watchSources := []struct {
			name       string
			source     sdk.Source //nolint:staticcheck // deliberate SDK 0.x compatibility assertion
			categories []string
		}{
			{name: "legacy factory", source: NewPlexSource(path), categories: wantCategories},
			{name: "requested", source: NewPlexSourceForCategories(path, watchCategories), categories: watchCategories},
			{name: "nil selection", source: NewPlexSourceForCategories(path, nil), categories: watchCategories},
		}
		for _, test := range watchSources {
			t.Run(test.name, func(t *testing.T) {
				if got := test.source.Categories(); !reflect.DeepEqual(got, test.categories) {
					t.Errorf("categories = %v, want %v", got, test.categories)
				}
				legacy, err := test.source.Items(context.Background()) //nolint:staticcheck // deliberate SDK 0.x compatibility assertion
				// The aggregate reader sees three watch settings for each of the three file paths.
				if err != nil || len(legacy) != 9 {
					t.Fatalf("compatibility Items returned %d items, error %v, want 9 watch-state items", len(legacy), err)
				}
				for _, item := range legacy {
					if !item.WatchState.Present || !reflect.DeepEqual(item, sdk.ForeignItem{Path: item.Path, WatchState: item.WatchState}) {
						t.Errorf("compatibility item includes data outside watch state: %#v", item)
					}
				}
				if streamed, err := collectFilmTVStream(test.source); err != nil || !reflect.DeepEqual(streamed, legacy) {
					t.Errorf("streamed items = %#v, error %v, want %#v", streamed, err, legacy)
				}
				items, err := requireSourceUserItems(t, test.source).ItemsBySourceUser(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				if len(items) != len(want) {
					t.Errorf("ItemsBySourceUser returned %d accounts, want %d", len(items), len(want))
				}
				for _, account := range []string{"Alice#1", "Bob#2"} {
					var wantWatch []sdk.ForeignItem
					for _, item := range want[account] {
						if item.WatchState.Present {
							wantWatch = append(wantWatch, sdk.ForeignItem{Path: item.Path, WatchState: item.WatchState})
						}
					}
					if !reflect.DeepEqual(items[account], wantWatch) {
						t.Errorf("watch-state items for %s = %#v, want %#v", account, items[account], wantWatch)
					}
				}
				if empty, ok := items["Empty#3"]; !ok || len(empty) != 0 {
					t.Errorf("watch-state items for Empty#3 = %#v, present = %t", empty, ok)
				}
			})
		}
	})

	t.Run("empty selection", func(t *testing.T) {
		source := NewPlexSourceForCategories(path, []string{})
		if got := source.Categories(); len(got) != 0 {
			t.Errorf("categories = %v, want none", got)
		}
		if items, err := source.Items(context.Background()); err != nil || len(items) != 0 { //nolint:staticcheck // deliberate SDK 0.x compatibility assertion
			t.Errorf("compatibility Items = %#v, error %v, want no items", items, err)
		}
		if streamed, err := collectFilmTVStream(source); err != nil || len(streamed) != 0 {
			t.Errorf("streamed items = %#v, error %v, want no items", streamed, err)
		}
		items, err := requireSourceUserItems(t, source).ItemsBySourceUser(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != len(want) {
			t.Errorf("ItemsBySourceUser returned %d accounts, want %d", len(items), len(want))
		}
		for account := range want {
			if userItems, ok := items[account]; !ok || len(userItems) != 0 {
				t.Errorf("items for %s = %#v, present = %t, want an empty account", account, userItems, ok)
			}
		}
	})

	t.Run("collections", func(t *testing.T) {
		source := NewPlexSourceForCategories(path, []string{sdk.CategoryCollections})
		if got := source.Categories(); !reflect.DeepEqual(got, []string{sdk.CategoryCollections}) {
			t.Errorf("collections-only source categories = %v", got)
		}
		items, err := requireSourceUserItems(t, source).ItemsBySourceUser(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		for account := range want {
			if !reflect.DeepEqual(items[account], collections) {
				t.Errorf("collection items for %s = %#v, want %#v", account, items[account], collections)
			}
		}
		if legacy, err := collectFilmTVStream(source); err != nil || len(legacy) != 0 {
			t.Errorf("legacy stream on collections-only source = %#v, %v", legacy, err)
		}
	})

	t.Run("ratings", func(t *testing.T) {
		source := NewPlexSourceForCategories(path, []string{sdk.CategoryRatings})
		items, err := requireSourceUserItems(t, source).ItemsBySourceUser(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		var wantRatings []sdk.ForeignItem
		for _, item := range want["Alice#1"] {
			if item.Rating.Present {
				wantRatings = append(wantRatings, sdk.ForeignItem{Path: item.Path, Rating: item.Rating})
			}
		}
		if !reflect.DeepEqual(items["Alice#1"], wantRatings) || len(items["Bob#2"]) != 0 || len(items["Empty#3"]) != 0 {
			t.Errorf("ratings-only items = %#v, want only Alice#1's ratings %#v", items, wantRatings)
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := bySourceUser.ItemsBySourceUser(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled ItemsBySourceUser error = %v, want context.Canceled", err)
	}
	assertFilmTVStreamMatchesItems(t, source)
}

type sourceUserItems interface {
	ItemsBySourceUser(context.Context) (map[string][]sdk.ForeignItem, error)
}

func requireSourceUserItems(t *testing.T, source sdk.Source) sourceUserItems { //nolint:staticcheck // deliberate SDK 0.x compatibility assertion
	t.Helper()
	bySourceUser, ok := source.(sourceUserItems)
	if !ok {
		t.Fatal("Plex source does not implement ItemsBySourceUser")
	}
	return bySourceUser
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
