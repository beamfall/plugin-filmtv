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
		{Name: "nfo", Source: nfoSource, WantCategories: []string{sdk.CategoryWatchState, sdk.CategoryCollections}, WantItems: nfoItems},
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

	t.Run("emby", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "library.db")
		seedFilmTVDatabase(t, path, []string{
			`CREATE TABLE TypedBaseItems (Path TEXT, UserDataKey TEXT, RunTimeTicks INTEGER)`,
			`CREATE TABLE UserDatas (key TEXT, Played INTEGER, PlaybackPositionTicks INTEGER, LastPlayedDate TEXT)`,
			`INSERT INTO UserDatas VALUES ('one', 1, 120000000, '2024-01-02T03:04:05Z')`,
			`INSERT INTO TypedBaseItems VALUES ('/z.mkv', 'one', 900000000)`,
			`INSERT INTO TypedBaseItems VALUES ('/a.mkv', 'one', 900000000)`,
		})
		assertFilmTVStreamMatchesItems(t, NewEmbySource(path))
	})

	t.Run("jellyfin", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "jellyfin.db")
		seedFilmTVDatabase(t, path, []string{
			`CREATE TABLE BaseItems (Id TEXT NOT NULL PRIMARY KEY, IsFolder INTEGER NOT NULL, Path TEXT, RunTimeTicks INTEGER)`,
			`CREATE TABLE UserData (ItemId TEXT NOT NULL, UserId TEXT NOT NULL, CustomDataKey TEXT NOT NULL,
				Played INTEGER NOT NULL, PlaybackPositionTicks INTEGER NOT NULL, LastPlayedDate TEXT)`,
			`INSERT INTO BaseItems VALUES ('z', 0, '/z.mkv', 900000000), ('a', 0, '/a.mkv', 900000000)`,
			`INSERT INTO UserData VALUES
				('z', 'U1', 'z-uuid', 1, 120000000, '2024-01-02 03:04:05'),
				('a', 'U1', 'a-uuid', 1, 120000000, '2024-01-02 03:04:05')`,
		})
		assertFilmTVStreamMatchesItems(t, NewJellyfinSource(path))
	})
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
		t.Fatalf("%T does not implement ItemsBySourceUser", source)
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

func TestJellyfinAndEmbySourcesItemsBySourceUser(t *testing.T) {
	jellyfinPath := filepath.Join(t.TempDir(), "jellyfin.db")
	seedFilmTVDatabase(t, jellyfinPath, []string{
		`CREATE TABLE BaseItems (Id TEXT NOT NULL PRIMARY KEY, IsFolder INTEGER NOT NULL, Path TEXT,
			RunTimeTicks INTEGER, Type TEXT NOT NULL, Name TEXT)`,
		`CREATE TABLE UserData (ItemId TEXT NOT NULL, UserId TEXT NOT NULL, CustomDataKey TEXT NOT NULL,
			Played INTEGER NOT NULL, PlaybackPositionTicks INTEGER NOT NULL, LastPlayedDate TEXT, Rating REAL)`,
		`CREATE TABLE Users (Id TEXT NOT NULL PRIMARY KEY, Username TEXT NOT NULL)`,
		`CREATE TABLE LinkedChildren (ParentId TEXT NOT NULL, SortOrder INTEGER NOT NULL, ChildId TEXT NOT NULL)`,
		`INSERT INTO Users VALUES ('U1', 'owner'), ('U2', 'idle')`,
		`INSERT INTO BaseItems VALUES
			('a', 0, '/a.mkv', 900000000, 'MediaBrowser.Controller.Entities.Movies.Movie', 'A'),
			('b', 0, '/b.mkv', 900000000, 'MediaBrowser.Controller.Entities.Movies.Movie', 'B'),
			('s', 1, '/s', NULL, 'MediaBrowser.Controller.Entities.Movies.BoxSet', 'Set')`,
		`INSERT INTO UserData VALUES ('a', 'U1', 'ka', 1, 0, '2024-01-02 03:04:05', NULL),
			('b', 'U1', 'kb', 0, 0, NULL, 7.5)`,
		`INSERT INTO LinkedChildren VALUES ('s', 3, 'b')`,
	})
	embyPath := filepath.Join(t.TempDir(), "library.db")
	seedFilmTVDatabase(t, embyPath, []string{
		`CREATE TABLE MediaItems (Id INTEGER PRIMARY KEY, type INT, ParentId INT, Path TEXT, Name TEXT,
			RunTimeTicks INTEGER, UserDataKeyId INT)`,
		`CREATE TABLE UserDatas (UserDataKeyId INT, userId INT, rating REAL, played INT, playCount INT,
			isFavorite INT, playbackPositionTicks INTEGER, LastPlayedDateInt INT)`,
		`CREATE TABLE ItemLinks2 (ItemId INT, Type INT, LinkedId INT, LinkOrder INT)`,
		`INSERT INTO MediaItems (Id, type, Path, Name, RunTimeTicks, UserDataKeyId) VALUES
			(10, 5, '/a.mkv', 'A', 900000000, 100), (11, 5, '/b.mkv', 'B', 900000000, 101),
			(20, 9, '/c [boxset]', 'Set', NULL, NULL)`,
		`INSERT INTO UserDatas VALUES (100, 1, NULL, 1, 1, 0, 0, 1704164645), (101, 1, 9, 0, 0, 1, 0, NULL)`,
		`INSERT INTO ItemLinks2 VALUES (11, 6, 20, 0)`,
	})
	watched := sdk.ForeignItem{Path: "/a.mkv", WatchState: sdk.WatchState{Present: true, Watched: true,
		DurationSeconds: 90, OccurredAt: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)}}
	jellyfinMember := sdk.ForeignItem{Path: "/b.mkv", CollectionMember: sdk.CollectionMember{Present: true, Name: "Set", Position: 3}}
	embyMember := sdk.ForeignItem{Path: "/b.mkv", CollectionMember: sdk.CollectionMember{Present: true, Name: "Set"}}
	all := []string{sdk.CategoryWatchState, sdk.CategoryRatings, sdk.CategoryCollections}
	embyAll := []string{sdk.CategoryWatchState, sdk.CategoryCollections}
	for _, test := range []struct {
		name       string
		source     sdk.Source //nolint:staticcheck // deliberate SDK 0.x compatibility assertion
		categories []string
		want       map[string][]sdk.ForeignItem
	}{
		{name: "jellyfin all", source: NewJellyfinSourceForCategories(jellyfinPath, all), categories: all, want: map[string][]sdk.ForeignItem{
			"owner#U1": {watched, {Path: "/b.mkv", Rating: sdk.Rating{Present: true, Value: 7.5, Scale: 10}}, jellyfinMember},
			"idle#U2":  {jellyfinMember},
		}},
		{name: "jellyfin ratings", source: NewJellyfinSourceForCategories(jellyfinPath, []string{sdk.CategoryRatings}),
			categories: []string{sdk.CategoryRatings}, want: map[string][]sdk.ForeignItem{
				"owner#U1": {{Path: "/b.mkv", Rating: sdk.Rating{Present: true, Value: 7.5, Scale: 10}}},
				"idle#U2":  nil,
			}},
		{name: "jellyfin legacy factory", source: NewJellyfinSource(jellyfinPath),
			categories: all, want: map[string][]sdk.ForeignItem{"owner#U1": {watched}, "idle#U2": nil}},
		{name: "jellyfin nil selection", source: NewJellyfinSourceForCategories(jellyfinPath, nil),
			categories: []string{sdk.CategoryWatchState}, want: map[string][]sdk.ForeignItem{
				"owner#U1": {watched}, "idle#U2": nil,
			}},
		{name: "emby legacy factory", source: NewEmbySource(embyPath), categories: embyAll,
			want: map[string][]sdk.ForeignItem{"#1": {watched}}},
		{name: "emby all", source: NewEmbySourceForCategories(embyPath, all), categories: embyAll, want: map[string][]sdk.ForeignItem{
			"#1": {watched, embyMember},
		}},
		{name: "emby collections", source: NewEmbySourceForCategories(embyPath, []string{sdk.CategoryCollections}),
			categories: []string{sdk.CategoryCollections}, want: map[string][]sdk.ForeignItem{"#1": {embyMember}}},
		{name: "emby collections with user", source: NewEmbySourceForCategories(embyPath, []string{sdk.CategoryWatchState, sdk.CategoryCollections}),
			categories: embyAll, want: map[string][]sdk.ForeignItem{"#1": {watched, embyMember}}},
		{name: "emby ratings unsupported", source: NewEmbySourceForCategories(embyPath, []string{sdk.CategoryRatings}),
			categories: []string{}, want: map[string][]sdk.ForeignItem{"#1": nil}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := test.source.Categories(); !reflect.DeepEqual(got, test.categories) {
				t.Fatalf("categories = %#v, want %#v", got, test.categories)
			}
			got, err := requireSourceUserItems(t, test.source).ItemsBySourceUser(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			for account, items := range got {
				if len(items) == 0 {
					got[account] = nil
				}
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("items = %#v\nwant  %#v", got, test.want)
			}
		})
	}
}
