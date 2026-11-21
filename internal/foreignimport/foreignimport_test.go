package foreignimport

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// fixtureDB runs a sequence of DDL/DML statements against a fresh on-disk SQLite
// file and returns its path, building a minimal foreign-server fixture DB.
func fixtureDB(t *testing.T, name string, stmts ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open fixture %s: %v", name, err)
	}
	defer func() { _ = db.Close() }()
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec fixture stmt %q: %v", stmt, err)
		}
	}
	return path
}

func TestReadPlexMapsPathKeyedWatchState(t *testing.T) {
	path := fixtureDB(t, "plex.db",
		`CREATE TABLE metadata_items (id INTEGER PRIMARY KEY, guid TEXT, duration INTEGER)`,
		`CREATE TABLE media_items (id INTEGER PRIMARY KEY, metadata_item_id INTEGER, duration INTEGER)`,
		`CREATE TABLE media_parts (id INTEGER PRIMARY KEY, media_item_id INTEGER, file TEXT)`,
		`CREATE TABLE metadata_item_settings (id INTEGER PRIMARY KEY, guid TEXT, view_offset INTEGER, view_count INTEGER, last_viewed_at INTEGER)`,
		// A completed item: view_count > 0, no resume offset, with a timestamp.
		`INSERT INTO metadata_items (id, guid, duration) VALUES (1, 'plex://movie/aaa', 5400000)`,
		`INSERT INTO media_items (id, metadata_item_id, duration) VALUES (10, 1, 5400000)`,
		`INSERT INTO media_parts (id, media_item_id, file) VALUES (100, 10, '/media/films/Watched.mkv')`,
		`INSERT INTO metadata_item_settings (id, guid, view_offset, view_count, last_viewed_at) VALUES (1, 'plex://movie/aaa', 0, 2, 1614834367)`,
		// An in-progress item: view_offset in milliseconds, no completion.
		`INSERT INTO metadata_items (id, guid, duration) VALUES (2, 'plex://movie/bbb', 6000000)`,
		`INSERT INTO media_items (id, metadata_item_id, duration) VALUES (20, 2, 6000000)`,
		`INSERT INTO media_parts (id, media_item_id, file) VALUES (200, 20, '/media/films/Resume.mkv')`,
		`INSERT INTO metadata_item_settings (id, guid, view_offset, view_count, last_viewed_at) VALUES (2, 'plex://movie/bbb', 120000, 0, 0)`,
		// An untouched item: no settings row → excluded by the WHERE clause.
		`INSERT INTO metadata_items (id, guid, duration) VALUES (3, 'plex://movie/ccc', 100000)`,
		`INSERT INTO media_items (id, metadata_item_id, duration) VALUES (30, 3, 100000)`,
		`INSERT INTO media_parts (id, media_item_id, file) VALUES (300, 30, '/media/films/Untouched.mkv')`,
	)

	rows, err := ReadPlex(path)
	if err != nil {
		t.Fatalf("ReadPlex: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (untouched excluded): %#v", len(rows), rows)
	}
	// Sorted by path: Resume.mkv before Watched.mkv.
	resume := rows[0]
	if resume.Path != "/media/films/Resume.mkv" || resume.Watched || resume.PositionSeconds != 120 || resume.DurationSeconds != 6000 {
		t.Fatalf("resume row = %#v", resume)
	}
	watched := rows[1]
	if watched.Path != "/media/films/Watched.mkv" || !watched.Watched || watched.PositionSeconds != 0 {
		t.Fatalf("watched row = %#v", watched)
	}
	if !watched.LastPlayed.Equal(time.Unix(1614834367, 0).UTC()) {
		t.Fatalf("watched LastPlayed = %v, want unix 1614834367", watched.LastPlayed)
	}
}

func TestReadPlexByAccount(t *testing.T) {
	schema, err := os.ReadFile("testdata/plex-accounts.sql")
	if err != nil {
		t.Fatalf("read Plex fixture: %v", err)
	}
	path := fixtureDB(t, "plex.db", string(schema))
	options := PlexReadOptions{WatchState: true, Ratings: true, Collections: true}
	accounts, collections, err := ReadPlexByAccount(context.Background(), path, options)
	if err != nil {
		t.Fatal(err)
	}

	rating, zeroRating := 8.5, 0.0
	wantAccounts := map[string][]PlexAccountItem{"Alice#1": nil, "Bob#2": nil, "Empty#3": nil}
	wantCollections := []PlexCollectionMember{{Path: "/collection-only.mkv", Name: "Weekend", Position: 7}}
	for _, file := range []struct {
		path     string
		duration int64
	}{
		{path: "/movie.mkv", duration: 90},
		{path: "/part2.mkv", duration: 90},
		{path: "/version.mkv", duration: 91},
	} {
		wantAccounts["Alice#1"] = append(wantAccounts["Alice#1"], PlexAccountItem{
			WatchRow: WatchRow{
				Path: file.path, DurationSeconds: file.duration, Watched: true, LastPlayed: time.Unix(1700000000, 0).UTC(),
			},
			HasWatchState: true,
			Rating:        &rating,
		})
		wantAccounts["Bob#2"] = append(wantAccounts["Bob#2"], PlexAccountItem{
			WatchRow: WatchRow{
				Path: file.path, DurationSeconds: file.duration, PositionSeconds: 12, LastPlayed: time.Unix(1700000010, 0).UTC(),
			},
			HasWatchState: true,
		})
		wantCollections = append(wantCollections,
			PlexCollectionMember{Path: file.path, Name: "Favorites", Position: 3},
			PlexCollectionMember{Path: file.path, Name: "Weekend", Position: 0},
		)
	}
	wantAccounts["Alice#1"] = append(wantAccounts["Alice#1"], PlexAccountItem{
		WatchRow: WatchRow{Path: "/zero-rated.mkv", DurationSeconds: 120},
		Rating:   &zeroRating,
	})
	if !reflect.DeepEqual(accounts, wantAccounts) {
		t.Errorf("account items = %#v, want %#v", accounts, wantAccounts)
	}
	if !reflect.DeepEqual(collections, wantCollections) {
		t.Errorf("collection memberships = %#v, want %#v", collections, wantCollections)
	}

	t.Run("collections without settings", func(t *testing.T) {
		path := fixtureDB(t, "plex.db", string(schema), "DROP TABLE metadata_item_settings")
		accounts, collections, err := ReadPlexByAccount(context.Background(), path, PlexReadOptions{Collections: true})
		if err != nil {
			t.Fatal(err)
		}
		wantAccounts := map[string][]PlexAccountItem{"Alice#1": nil, "Bob#2": nil, "Empty#3": nil}
		if !reflect.DeepEqual(accounts, wantAccounts) {
			t.Errorf("account items = %#v, want %#v", accounts, wantAccounts)
		}
		if !reflect.DeepEqual(collections, wantCollections) {
			t.Errorf("collection memberships = %#v, want %#v", collections, wantCollections)
		}
	})
}

func TestReadPlexByAccountCategoryQueries(t *testing.T) {
	schema, err := os.ReadFile("testdata/plex-accounts.sql")
	if err != nil {
		t.Fatalf("read Plex fixture: %v", err)
	}
	path := fixtureDB(t, "plex.db", string(schema), "DROP TABLE taggings")
	accounts, collections, err := ReadPlexByAccount(context.Background(), path, PlexReadOptions{WatchState: true})
	if err != nil {
		t.Fatalf("watch-state-only read without a collection table: %v", err)
	}
	if len(accounts["Alice#1"]) != 3 || len(accounts["Bob#2"]) != 3 || len(collections) != 0 {
		t.Fatalf("watch-state-only read: accounts = %#v, collections = %#v", accounts, collections)
	}
	if _, _, err := ReadPlexByAccount(context.Background(), path, PlexReadOptions{Collections: true}); err == nil ||
		!strings.Contains(err.Error(), "query plex collections") {
		t.Fatalf("collections read without a collection table: got %v, want collection query error", err)
	}
}

func TestReadPlexByAccountQueryErrors(t *testing.T) {
	schema, err := os.ReadFile("testdata/plex-accounts.sql")
	if err != nil {
		t.Fatalf("read Plex fixture: %v", err)
	}
	for _, test := range []struct {
		name      string
		mutation  string
		wantError string
	}{
		{name: "missing accounts", mutation: "DROP TABLE accounts", wantError: "query plex account settings"},
		{name: "invalid rating", mutation: "UPDATE metadata_item_settings SET rating = 'invalid' WHERE account_id = 1", wantError: "scan plex account settings"},
		{name: "invalid collection position", mutation: `UPDATE taggings SET "index" = 'invalid' WHERE tag_id = 2`, wantError: "scan plex collection membership"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := fixtureDB(t, "plex.db", string(schema), test.mutation)
			options := PlexReadOptions{WatchState: true, Ratings: true, Collections: true}
			accounts, collections, err := ReadPlexByAccount(context.Background(), path, options)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("ReadPlexByAccount error = %v, want %q", err, test.wantError)
			}
			if accounts != nil || collections != nil {
				t.Fatalf("failed read returned accounts = %#v, collections = %#v", accounts, collections)
			}
		})
	}
}

// jellyfin12Schema is the part of the Jellyfin 12 `jellyfin.db` layout the
// reader uses, with the column types, nullability and UserData primary key
// observed in the official 12.4 image (CORE-2677 evidence). The DEFAULTs on
// columns the reader ignores, and the missing foreign keys, are fixture
// conveniences.
var jellyfin12Schema = []string{
	`CREATE TABLE "BaseItems" (
		"Id" TEXT NOT NULL CONSTRAINT "PK_BaseItems" PRIMARY KEY,
		"IsFolder" INTEGER NOT NULL,
		"IsVirtualItem" INTEGER NOT NULL DEFAULT 0,
		"Path" TEXT NULL,
		"RunTimeTicks" INTEGER NULL,
		"Type" TEXT NOT NULL)`,
	`CREATE TABLE "UserData" (
		"ItemId" TEXT NOT NULL,
		"UserId" TEXT NOT NULL,
		"CustomDataKey" TEXT NOT NULL,
		"IsFavorite" INTEGER NOT NULL DEFAULT 0,
		"LastPlayedDate" TEXT NULL,
		"PlayCount" INTEGER NOT NULL DEFAULT 0,
		"PlaybackPositionTicks" INTEGER NOT NULL,
		"Played" INTEGER NOT NULL,
		CONSTRAINT "PK_UserData" PRIMARY KEY ("ItemId", "UserId", "CustomDataKey"))`,
}

const (
	jellyfinMovie   = "MediaBrowser.Controller.Entities.Movies.Movie"
	jellyfinEpisode = "MediaBrowser.Controller.Entities.TV.Episode"
	jellyfinSeries  = "MediaBrowser.Controller.Entities.TV.Series"
	jellyfinOwner   = "C77F9F38-62CF-4530-A763-EA3ADA622927"
	jellyfinSecond  = "7396E9F1-2D18-4486-B3E7-EFD69E5C449A"
)

// jellyfin12DB builds a Jellyfin 12 fixture from the observed schema plus the
// given rows.
func jellyfin12DB(t *testing.T, name string, stmts ...string) string {
	t.Helper()
	return fixtureDB(t, name, append(append([]string(nil), jellyfin12Schema...), stmts...)...)
}

func TestReadJellyfinMapsPathKeyedWatchState(t *testing.T) {
	path := jellyfin12DB(t, "jellyfin.db",
		`INSERT INTO BaseItems (Id, IsFolder, Path, RunTimeTicks, Type) VALUES
			('a', 0, '/media/tv/S01E01.mkv', 18000000000, '`+jellyfinEpisode+`'),
			('b', 0, '/media/tv/S01E02.mkv', 18000000000, '`+jellyfinEpisode+`'),
			('c', 0, '/media/tv/S01E03.mkv', 18000000000, '`+jellyfinEpisode+`'),
			('show', 1, '/media/tv', NULL, '`+jellyfinSeries+`'),
			('nopath', 0, NULL, 18000000000, '`+jellyfinMovie+`')`,
		// Completed by the owner, stored under three data keys as Jellyfin 12
		// writes it, with the stored timestamp form.
		`INSERT INTO UserData (ItemId, UserId, CustomDataKey, Played, PlaybackPositionTicks, LastPlayedDate) VALUES
			('a', '`+jellyfinOwner+`', '329865', 1, 0, '2026-06-10 12:34:56.0279689'),
			('a', '`+jellyfinOwner+`', 'a-uuid', 1, 0, '2026-06-10 12:34:56.0279689'),
			('a', '`+jellyfinOwner+`', 'tt2543164', 1, 0, '2026-06-10 12:34:56.0279689')`,
		// The second user stopped the same file at 50 s.
		`INSERT INTO UserData (ItemId, UserId, CustomDataKey, Played, PlaybackPositionTicks, LastPlayedDate) VALUES
			('a', '`+jellyfinSecond+`', '329865', 0, 500000000, NULL)`,
		// In progress at 120 s, two data keys, no timestamp.
		`INSERT INTO UserData (ItemId, UserId, CustomDataKey, Played, PlaybackPositionTicks, LastPlayedDate) VALUES
			('b', '`+jellyfinOwner+`', 'b-uuid', 0, 1200000000, NULL),
			('b', '`+jellyfinOwner+`', 'series001002', 0, 1200000000, NULL)`,
		// Untouched, a played folder and a played item without a path: excluded.
		`INSERT INTO UserData (ItemId, UserId, CustomDataKey, Played, PlaybackPositionTicks, LastPlayedDate) VALUES
			('c', '`+jellyfinOwner+`', 'c-uuid', 0, 0, NULL),
			('show', '`+jellyfinOwner+`', 'show-uuid', 1, 0, '2026-06-10 12:00:00.0000000'),
			('nopath', '`+jellyfinOwner+`', 'nopath-uuid', 1, 0, NULL)`,
	)

	rows, err := ReadJellyfin(path)
	if err != nil {
		t.Fatalf("ReadJellyfin: %v", err)
	}
	want := []WatchRow{
		// Same path: the second user's row sorts first by user ID.
		{Path: "/media/tv/S01E01.mkv", PositionSeconds: 50, DurationSeconds: 1800},
		{Path: "/media/tv/S01E01.mkv", Watched: true, DurationSeconds: 1800,
			LastPlayed: time.Date(2026, 6, 10, 12, 34, 56, 27968900, time.UTC)},
		{Path: "/media/tv/S01E02.mkv", PositionSeconds: 120, DurationSeconds: 1800},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %#v\nwant  %#v", rows, want)
	}
}

func TestReadJellyfinPicksNewestDataKeyCopy(t *testing.T) {
	path := jellyfin12DB(t, "jellyfin-copies.db",
		`INSERT INTO BaseItems (Id, IsFolder, Path, RunTimeTicks, Type) VALUES
			('x', 0, '/media/films/Unmarked.mkv', 300000000, '`+jellyfinMovie+`'),
			('y', 0, '/media/films/Resumed.mkv', 300000000, '`+jellyfinMovie+`'),
			('z', 0, '/media/films/Tied.mkv', 300000000, '`+jellyfinMovie+`'),
			('s', 0, '/media/films/Spelled.mkv', 300000000, '`+jellyfinMovie+`'),
			('u', 0, '/media/films/Unreadable.mkv', 300000000, '`+jellyfinMovie+`'),
			('d2', 0, '/media/films/Shared.mkv', 600000000, '`+jellyfinMovie+`'),
			('d1', 0, '/media/films/Shared.mkv', 300000000, '`+jellyfinMovie+`')`,
		// Two spellings of one instant tie on time, so the played copy wins.
		`INSERT INTO UserData (ItemId, UserId, CustomDataKey, Played, PlaybackPositionTicks, LastPlayedDate) VALUES
			('s', '`+jellyfinOwner+`', 'k1', 1, 0, '2026-11-21 13:00:00'),
			('s', '`+jellyfinOwner+`', 'k2', 0, 0, '2026-11-21 13:00:00.0000000')`,
		// An unreadable time ranks below a readable one.
		`INSERT INTO UserData (ItemId, UserId, CustomDataKey, Played, PlaybackPositionTicks, LastPlayedDate) VALUES
			('u', '`+jellyfinOwner+`', 'k1', 0, 0, 'not-a-timestamp'),
			('u', '`+jellyfinOwner+`', 'k2', 1, 0, '2026-11-21 13:00:00.0000000')`,
		// Two items on one path for one user stay two records, in item ID order.
		`INSERT INTO UserData (ItemId, UserId, CustomDataKey, Played, PlaybackPositionTicks, LastPlayedDate) VALUES
			('d2', '`+jellyfinOwner+`', 'k1', 0, 200000000, NULL),
			('d1', '`+jellyfinOwner+`', 'k1', 0, 100000000, NULL)`,
		// A newer unplayed copy wins over an older played one: no record.
		`INSERT INTO UserData (ItemId, UserId, CustomDataKey, Played, PlaybackPositionTicks, LastPlayedDate) VALUES
			('x', '`+jellyfinOwner+`', 'k1', 1, 0, '2026-01-01 10:00:00.0000000'),
			('x', '`+jellyfinOwner+`', 'k2', 0, 0, '2026-02-01 10:00:00.0000000')`,
		// With no timestamps, a played copy wins, then the larger position.
		`INSERT INTO UserData (ItemId, UserId, CustomDataKey, Played, PlaybackPositionTicks, LastPlayedDate) VALUES
			('y', '`+jellyfinOwner+`', 'k1', 0, 100000000, NULL),
			('y', '`+jellyfinOwner+`', 'k2', 0, 200000000, NULL),
			('z', '`+jellyfinOwner+`', 'k1', 0, 100000000, NULL),
			('z', '`+jellyfinOwner+`', 'k2', 1, 0, NULL)`,
	)

	rows, err := ReadJellyfin(path)
	if err != nil {
		t.Fatalf("ReadJellyfin: %v", err)
	}
	at := time.Date(2026, 11, 21, 13, 0, 0, 0, time.UTC)
	want := []WatchRow{
		{Path: "/media/films/Resumed.mkv", PositionSeconds: 20, DurationSeconds: 30},
		{Path: "/media/films/Shared.mkv", PositionSeconds: 10, DurationSeconds: 30},
		{Path: "/media/films/Shared.mkv", PositionSeconds: 20, DurationSeconds: 60},
		{Path: "/media/films/Spelled.mkv", Watched: true, DurationSeconds: 30, LastPlayed: at},
		{Path: "/media/films/Tied.mkv", Watched: true, DurationSeconds: 30},
		{Path: "/media/films/Unreadable.mkv", Watched: true, DurationSeconds: 30, LastPlayed: at},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %#v\nwant  %#v", rows, want)
	}
}

// TestReadJellyfinMatchesLegacyReader proves the Jellyfin 12 reader returns the
// records the legacy reader returns for the same library: the one recorded for
// CORE-2677 (two films, two episodes, two users), laid out both ways.
func TestReadJellyfinMatchesLegacyReader(t *testing.T) {
	legacy := fixtureDB(t, "library.db",
		`CREATE TABLE TypedBaseItems (Id TEXT PRIMARY KEY, Path TEXT, UserDataKey TEXT, RunTimeTicks INTEGER)`,
		`CREATE TABLE UserDatas (key TEXT, userId INTEGER, Played INTEGER, PlaybackPositionTicks INTEGER, LastPlayedDate TEXT)`,
		`INSERT INTO TypedBaseItems VALUES
			('arrival', '/media/films/Arrival (2016)/Arrival (2016).mkv', '329865', 300230000),
			('heat', '/media/films/Heat (1995)/Heat (1995).mkv', '949', NULL),
			('e01', '/media/tv/Night Shift (2020)/Season 01/Night Shift (2020) - S01E01.mkv', 'ns001001', 200230000),
			('e02', '/media/tv/Night Shift (2020)/Season 01/Night Shift (2020) - S01E02.mkv', 'ns001002', 200230000)`,
		`INSERT INTO UserDatas VALUES
			('329865', 1, 1, 0, '2026-11-21T13:42:00.8301004Z'),
			('329865', 2, 0, 50000000, NULL),
			('949', 2, 0, 70000000, NULL),
			('ns001001', 1, 0, 120000000, NULL),
			('ns001002', 1, 1, 0, '2026-11-21T13:42:00.9321923Z')`,
	)
	current := jellyfin12DB(t, "jellyfin.db",
		`INSERT INTO BaseItems (Id, IsFolder, Path, RunTimeTicks, Type) VALUES
			('arrival', 0, '/media/films/Arrival (2016)/Arrival (2016).mkv', 300230000, '`+jellyfinMovie+`'),
			('heat', 0, '/media/films/Heat (1995)/Heat (1995).mkv', NULL, '`+jellyfinMovie+`'),
			('e01', 0, '/media/tv/Night Shift (2020)/Season 01/Night Shift (2020) - S01E01.mkv', 200230000, '`+jellyfinEpisode+`'),
			('e02', 0, '/media/tv/Night Shift (2020)/Season 01/Night Shift (2020) - S01E02.mkv', 200230000, '`+jellyfinEpisode+`')`,
		`INSERT INTO UserData (ItemId, UserId, CustomDataKey, Played, PlaybackPositionTicks, LastPlayedDate) VALUES
			('arrival', '`+jellyfinOwner+`', '329865', 1, 0, '2026-11-21 13:42:00.8301004'),
			('arrival', '`+jellyfinOwner+`', 'arrival-uuid', 1, 0, '2026-11-21 13:42:00.8301004'),
			('arrival', '`+jellyfinOwner+`', 'tt2543164', 1, 0, '2026-11-21 13:42:00.8301004'),
			('arrival', '`+jellyfinSecond+`', '329865', 0, 50000000, NULL),
			('arrival', '`+jellyfinSecond+`', 'arrival-uuid', 0, 50000000, NULL),
			('arrival', '`+jellyfinSecond+`', 'tt2543164', 0, 50000000, NULL),
			('heat', '`+jellyfinSecond+`', 'heat-uuid', 0, 70000000, NULL),
			('e01', '`+jellyfinOwner+`', 'e01-uuid', 0, 120000000, NULL),
			('e01', '`+jellyfinOwner+`', 'series001001', 0, 120000000, NULL),
			('e02', '`+jellyfinOwner+`', 'e02-uuid', 1, 0, '2026-11-21 13:42:00.9321923'),
			('e02', '`+jellyfinOwner+`', 'series001002', 1, 0, '2026-11-21 13:42:00.9321923')`,
	)

	want, err := ReadEmby(legacy)
	if err != nil {
		t.Fatalf("legacy reader: %v", err)
	}
	got, err := ReadJellyfin(current)
	if err != nil {
		t.Fatalf("ReadJellyfin: %v", err)
	}
	// The legacy reader orders by path only, so compare as sets.
	sortWatchRows(want)
	sortWatchRows(got)
	if len(got) != 5 || !reflect.DeepEqual(got, want) {
		t.Fatalf("Jellyfin 12 rows differ from legacy rows:\njellyfin 12: %#v\nlegacy:      %#v", got, want)
	}
}

func sortWatchRows(rows []WatchRow) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Path != rows[j].Path {
			return rows[i].Path < rows[j].Path
		}
		if rows[i].Watched != rows[j].Watched {
			return !rows[i].Watched
		}
		return rows[i].PositionSeconds < rows[j].PositionSeconds
	})
}

func TestReadJellyfinRefusesLegacyLibrary(t *testing.T) {
	path := fixtureDB(t, "library.db",
		`CREATE TABLE TypedBaseItems (Id TEXT PRIMARY KEY, Path TEXT, UserDataKey TEXT, RunTimeTicks INTEGER)`,
		`CREATE TABLE UserDatas (key TEXT, Played INTEGER, PlaybackPositionTicks INTEGER, LastPlayedDate TEXT)`,
		`INSERT INTO TypedBaseItems VALUES ('a', '/media/films/Old.mkv', 'key-a', 12000000000)`,
		`INSERT INTO UserDatas VALUES ('key-a', 1, 0, '')`,
	)
	rows, err := ReadJellyfin(path)
	if !errors.Is(err, errJellyfinLegacy) || rows != nil {
		t.Fatalf("ReadJellyfin(legacy) = %#v, %v; want errJellyfinLegacy", rows, err)
	}
	for _, phrase := range []string{"Jellyfin 12", "jellyfin.db", "upgrade"} {
		if !strings.Contains(err.Error(), phrase) {
			t.Fatalf("legacy error %q does not mention %q", err, phrase)
		}
	}
}

func TestReadJellyfinRefusesUnknownSchema(t *testing.T) {
	tests := []struct {
		name  string
		stmts []string
		want  string
	}{
		{"unrelated file", []string{`CREATE TABLE unrelated (x INTEGER)`}, "missing table BaseItems"},
		{"no UserData", jellyfin12Schema[:1], "missing table UserData"},
		{"no Played column", []string{
			jellyfin12Schema[0],
			`CREATE TABLE UserData (ItemId TEXT, UserId TEXT, CustomDataKey TEXT, PlaybackPositionTicks INTEGER, LastPlayedDate TEXT)`,
		}, "missing column UserData.Played"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := fixtureDB(t, "unknown.db", tt.stmts...)
			_, err := ReadJellyfin(path)
			if !errors.Is(err, errJellyfinUnknown) || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ReadJellyfin = %v; want errJellyfinUnknown naming %q", err, tt.want)
			}
		})
	}
}

func TestParseJellyfinTime(t *testing.T) {
	tests := map[string]time.Time{
		"2026-11-21 13:40:27.0279689":  time.Date(2026, 11, 21, 13, 40, 27, 27968900, time.UTC),
		"2026-11-21 13:40:27":          time.Date(2026, 11, 21, 13, 40, 27, 0, time.UTC),
		"2026-11-21T13:40:27.0279689Z": time.Date(2026, 11, 21, 13, 40, 27, 27968900, time.UTC),
		"2026-11-21T15:40:27.5+02:00":  time.Date(2026, 11, 21, 13, 40, 27, 500000000, time.UTC),
	}
	for value, want := range tests {
		got, err := parseJellyfinTime(value)
		if err != nil || !got.Equal(want) || got.Location() != time.UTC {
			t.Fatalf("parseJellyfinTime(%q) = %v, %v; want %v UTC", value, got, err, want)
		}
	}
	if _, err := parseJellyfinTime("not-a-timestamp"); err == nil {
		t.Fatal("parseJellyfinTime(malformed) error = nil")
	}
}

func TestReadEmbyReadsLegacyLibrary(t *testing.T) {
	// Emby keeps the TypedBaseItems/UserDatas layout that Jellyfin dropped in
	// 12; the Emby reader still reads it.
	path := fixtureDB(t, "emby.db",
		`CREATE TABLE TypedBaseItems (Id TEXT PRIMARY KEY, Path TEXT, UserDataKey TEXT, RunTimeTicks INTEGER)`,
		`CREATE TABLE UserDatas (key TEXT, Played INTEGER, PlaybackPositionTicks INTEGER, LastPlayedDate TEXT)`,
		`INSERT INTO TypedBaseItems (Id, Path, UserDataKey, RunTimeTicks) VALUES ('a', '/media/films/Emby.mkv', 'key-a', 12000000000)`,
		`INSERT INTO UserDatas (key, Played, PlaybackPositionTicks, LastPlayedDate) VALUES ('key-a', 1, 0, '')`,
	)
	rows, err := ReadEmby(path)
	if err != nil {
		t.Fatalf("ReadEmby: %v", err)
	}
	if len(rows) != 1 || rows[0].Path != "/media/films/Emby.mkv" || !rows[0].Watched {
		t.Fatalf("emby rows = %#v", rows)
	}
}

func TestReadJellyfinDegradesMalformedLastPlayedDate(t *testing.T) {
	// A single malformed LastPlayedDate must degrade that one row to a zero
	// LastPlayed rather than aborting the whole import: the
	// Plex path already tolerates an absent timestamp, and this path must
	// match it instead of erroring out the entire library.
	path := jellyfin12DB(t, "jellyfin-malformed.db",
		`INSERT INTO BaseItems (Id, IsFolder, Path, RunTimeTicks, Type) VALUES
			('a', 0, '/media/tv/Good.mkv', 18000000000, '`+jellyfinEpisode+`'),
			('b', 0, '/media/tv/Bad.mkv', 18000000000, '`+jellyfinEpisode+`')`,
		`INSERT INTO UserData (ItemId, UserId, CustomDataKey, Played, PlaybackPositionTicks, LastPlayedDate) VALUES
			('a', '`+jellyfinOwner+`', 'a-uuid', 1, 0, '2026-06-10 12:34:56.0000000'),
			('b', '`+jellyfinOwner+`', 'b-uuid', 1, 0, 'not-a-timestamp')`,
	)

	rows, err := ReadJellyfin(path)
	if err != nil {
		t.Fatalf("ReadJellyfin: %v, want nil (malformed row should degrade, not abort)", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (good row must still import): %#v", len(rows), rows)
	}
	bad := rows[0] // Bad.mkv (sorted before Good.mkv)
	if bad.Path != "/media/tv/Bad.mkv" || !bad.Watched {
		t.Fatalf("bad row = %#v", bad)
	}
	if !bad.LastPlayed.IsZero() {
		t.Fatalf("bad row LastPlayed = %v, want zero (malformed source degrades)", bad.LastPlayed)
	}
	good := rows[1]
	if good.Path != "/media/tv/Good.mkv" || !good.Watched {
		t.Fatalf("good row = %#v", good)
	}
	if !good.LastPlayed.Equal(time.Date(2026, 6, 10, 12, 34, 56, 0, time.UTC)) {
		t.Fatalf("good row LastPlayed = %v, want the parsed timestamp", good.LastPlayed)
	}
}

func TestReadForeignDBErrorsOnMissingTables(t *testing.T) {
	// A structurally-wrong file (wrong tables) must error, not import zero
	// silently — a wrong-file upload should surface.
	path := fixtureDB(t, "wrong.db", `CREATE TABLE unrelated (x INTEGER)`)
	if _, err := ReadPlex(path); err == nil {
		t.Fatal("ReadPlex on a non-Plex db: error = nil, want a query error")
	}
	if _, err := ReadJellyfin(path); err == nil {
		t.Fatal("ReadJellyfin on a non-Jellyfin db: error = nil, want a schema error")
	}
}

// TestOpenReadOnlyEscapesQuestionMarkAndHash guards against a real regression:
// modernc.org/sqlite's DSN parser splits the connection string at the FIRST
// '?' anywhere in it (see its newConn), so an operator-supplied export path
// containing a literal '?' or '#' would, if spliced in unescaped, truncate or
// corrupt the "?mode=ro&immutable=1" query — silently dropping the read-only
// guarantee. This asserts the DB still opens against the intended file (an
// import still succeeds) and that write access is genuinely refused.
func TestOpenReadOnlyEscapesQuestionMarkAndHash(t *testing.T) {
	// fixtureDB opens its own DSN unescaped, so it can't create the fixture
	// directly at a '?'/'#'-bearing path either (same DSN-splitting behavior
	// this test guards against). Build it at a safe name, then rename on disk
	// — a plain filesystem op, not a DSN parse — to get the weird path that
	// exercises openReadOnly's escaping.
	safe := fixtureDB(t, "safe.db",
		`CREATE TABLE metadata_items (id INTEGER PRIMARY KEY, guid TEXT, duration INTEGER)`,
		`CREATE TABLE media_items (id INTEGER PRIMARY KEY, metadata_item_id INTEGER, duration INTEGER)`,
		`CREATE TABLE media_parts (id INTEGER PRIMARY KEY, media_item_id INTEGER, file TEXT)`,
		`CREATE TABLE metadata_item_settings (id INTEGER PRIMARY KEY, guid TEXT, view_offset INTEGER, view_count INTEGER, last_viewed_at INTEGER)`,
		`INSERT INTO metadata_items (id, guid, duration) VALUES (1, 'plex://movie/aaa', 5400000)`,
		`INSERT INTO media_items (id, metadata_item_id, duration) VALUES (10, 1, 5400000)`,
		`INSERT INTO media_parts (id, media_item_id, file) VALUES (100, 10, '/media/films/Watched.mkv')`,
		`INSERT INTO metadata_item_settings (id, guid, view_offset, view_count, last_viewed_at) VALUES (1, 'plex://movie/aaa', 0, 2, 1614834367)`,
	)
	path := filepath.Join(filepath.Dir(safe), "Weird?Name#1.db")
	if err := os.Rename(safe, path); err != nil {
		t.Fatalf("rename fixture to '?'/'#' path: %v", err)
	}

	rows, err := ReadPlex(path)
	if err != nil {
		t.Fatalf("ReadPlex on a path containing '?'/'#': %v", err)
	}
	if len(rows) != 1 || rows[0].Path != "/media/films/Watched.mkv" {
		t.Fatalf("rows = %#v, want the one fixture row (DSN must resolve to the intended file)", rows)
	}

	db, err := openReadOnly(path)
	if err != nil {
		t.Fatalf("openReadOnly: %v", err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec("CREATE TABLE should_fail (x)"); err == nil {
		t.Fatal("write succeeded against a '?'/'#' path: mode=ro was dropped by DSN corruption")
	}
}

// TestEscapeSQLiteURIPath verifies the escaper encodes only '?', '#', and
// '%', leaving every other byte — including '/' and spaces — untouched. The
// '%' cases guard against the SQLite URI filename parser percent-decoding a
// literal "%23"/"%3F" already present in a real path back into '#'/'?'; a
// single-pass strings.Replacer doesn't rescan its own output, so encoding
// '%' first does not get re-encoded by the '?'/'#' replacements that follow.
func TestEscapeSQLiteURIPath(t *testing.T) {
	cases := map[string]string{
		"/a/b/c.db":       "/a/b/c.db",
		"/a b/c.db":       "/a b/c.db",
		"/a?b.db":         "/a%3Fb.db",
		"/a#b.db":         "/a%23b.db",
		"/a?b#c d.db":     "/a%3Fb%23c d.db",
		"/a%b.db":         "/a%25b.db",
		"/50%23done/x.db": "/50%2523done/x.db",
		"/a%3Fb.db":       "/a%253Fb.db",
	}
	for in, want := range cases {
		if got := escapeSQLiteURIPath(in); got != want {
			t.Errorf("escapeSQLiteURIPath(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestOpenReadOnlyEscapesPercent guards against a real regression: SQLite's
// URI filename parser percent-decodes the path, so a literal "%23" already
// present in a real path (e.g. a directory named "50%23done") would decode
// back to '#' and resolve the wrong file unless '%' is itself escaped first.
func TestOpenReadOnlyEscapesPercent(t *testing.T) {
	// fixtureDB opens its own DSN unescaped, so it can't create the fixture
	// directly at a '%'-bearing path either (same DSN-splitting/decoding
	// behavior this test guards against). Build it at a safe name, then
	// rename on disk — a plain filesystem op, not a DSN parse — to get the
	// weird path that exercises openReadOnly's escaping.
	safe := fixtureDB(t, "safe.db",
		`CREATE TABLE metadata_items (id INTEGER PRIMARY KEY, guid TEXT, duration INTEGER)`,
		`CREATE TABLE media_items (id INTEGER PRIMARY KEY, metadata_item_id INTEGER, duration INTEGER)`,
		`CREATE TABLE media_parts (id INTEGER PRIMARY KEY, media_item_id INTEGER, file TEXT)`,
		`CREATE TABLE metadata_item_settings (id INTEGER PRIMARY KEY, guid TEXT, view_offset INTEGER, view_count INTEGER, last_viewed_at INTEGER)`,
		`INSERT INTO metadata_items (id, guid, duration) VALUES (1, 'plex://movie/bbb', 5400000)`,
		`INSERT INTO media_items (id, metadata_item_id, duration) VALUES (11, 1, 5400000)`,
		`INSERT INTO media_parts (id, media_item_id, file) VALUES (101, 11, '/media/films/Watched2.mkv')`,
		`INSERT INTO metadata_item_settings (id, guid, view_offset, view_count, last_viewed_at) VALUES (1, 'plex://movie/bbb', 0, 2, 1614834367)`,
	)
	dir := filepath.Join(filepath.Dir(safe), "50%23done")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("mkdir literal '%%23' dir: %v", err)
	}
	path := filepath.Join(dir, "library.db")
	if err := os.Rename(safe, path); err != nil {
		t.Fatalf("rename fixture into literal '%%23' dir: %v", err)
	}

	rows, err := ReadPlex(path)
	if err != nil {
		t.Fatalf("ReadPlex on a path containing literal '%%23': %v", err)
	}
	if len(rows) != 1 || rows[0].Path != "/media/films/Watched2.mkv" {
		t.Fatalf("rows = %#v, want the one fixture row (DSN must resolve to the intended file)", rows)
	}

	db, err := openReadOnly(path)
	if err != nil {
		t.Fatalf("openReadOnly: %v", err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec("CREATE TABLE should_fail (x)"); err == nil {
		t.Fatal("write succeeded against a literal '%23' path: mode=ro was dropped by DSN corruption")
	}
}

func TestStreamJellyfinWatchRowsStopsEarly(t *testing.T) {
	path := jellyfin12DB(t, "jellyfin-stream.db",
		`INSERT INTO BaseItems (Id, IsFolder, Path, RunTimeTicks, Type) VALUES
			('a', 0, '/media/a.mkv', 300000000, '`+jellyfinMovie+`'),
			('b', 0, '/media/b.mkv', 300000000, '`+jellyfinMovie+`')`,
		`INSERT INTO UserData (ItemId, UserId, CustomDataKey, Played, PlaybackPositionTicks) VALUES
			('a', '`+jellyfinOwner+`', 'a-uuid', 1, 0),
			('b', '`+jellyfinOwner+`', 'b-uuid', 1, 0)`,
	)

	stop := errors.New("stop")
	visited := 0
	err := StreamJellyfinWatchRows(context.Background(), path, func(WatchRow) error {
		visited++
		return stop
	})
	if !errors.Is(err, stop) || visited != 1 {
		t.Fatalf("visit error: err = %v after %d rows; want stop after 1", err, visited)
	}

	ctx, cancel := context.WithCancel(context.Background())
	visited = 0
	err = StreamJellyfinWatchRows(ctx, path, func(WatchRow) error {
		visited++
		cancel()
		return nil
	})
	if !errors.Is(err, context.Canceled) || visited != 1 {
		t.Fatalf("cancel mid-stream: err = %v after %d rows; want context.Canceled after 1", err, visited)
	}

	if err := StreamJellyfinWatchRows(ctx, path, func(WatchRow) error { return nil }); err == nil {
		t.Fatal("cancelled context before the schema check: error = nil")
	}
	if _, err := ReadJellyfin(filepath.Join(t.TempDir(), "missing.db")); err == nil {
		t.Fatal("ReadJellyfin(missing file) error = nil")
	}
}

func TestReadJellyfinReportsUnreadableRow(t *testing.T) {
	path := jellyfin12DB(t, "jellyfin-bad-row.db",
		`INSERT INTO BaseItems (Id, IsFolder, Path, RunTimeTicks, Type) VALUES
			('a', 0, '/media/a.mkv', 'long', '`+jellyfinMovie+`')`,
		`INSERT INTO UserData (ItemId, UserId, CustomDataKey, Played, PlaybackPositionTicks) VALUES
			('a', '`+jellyfinOwner+`', 'a-uuid', 1, 0)`,
	)
	if _, err := ReadJellyfin(path); err == nil || !strings.Contains(err.Error(), "scan jellyfin row") {
		t.Fatalf("ReadJellyfin(non-numeric RunTimeTicks) = %v; want a scan error", err)
	}
}

func TestReadEmbyDegradesMalformedLastPlayedDate(t *testing.T) {
	path := fixtureDB(t, "library-malformed.db",
		`CREATE TABLE TypedBaseItems (Path TEXT, UserDataKey TEXT, RunTimeTicks INTEGER)`,
		`CREATE TABLE UserDatas (key TEXT, Played INTEGER, PlaybackPositionTicks INTEGER, LastPlayedDate TEXT)`,
		`INSERT INTO TypedBaseItems VALUES ('/media/Good.mkv', 'good', 18000000000), ('/media/Bad.mkv', 'bad', 18000000000)`,
		`INSERT INTO UserDatas VALUES ('good', 1, 0, '2026-06-10T12:34:56Z'), ('bad', 1, 0, 'not-a-timestamp')`,
	)
	rows, err := ReadEmby(path)
	if err != nil {
		t.Fatalf("ReadEmby: %v", err)
	}
	want := []WatchRow{
		{Path: "/media/Bad.mkv", Watched: true, DurationSeconds: 1800},
		{Path: "/media/Good.mkv", Watched: true, DurationSeconds: 1800,
			LastPlayed: time.Date(2026, 6, 10, 12, 34, 56, 0, time.UTC)},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %#v\nwant  %#v", rows, want)
	}

	bad := fixtureDB(t, "library-bad-row.db",
		`CREATE TABLE TypedBaseItems (Path TEXT, UserDataKey TEXT, RunTimeTicks INTEGER)`,
		`CREATE TABLE UserDatas (key TEXT, Played INTEGER, PlaybackPositionTicks INTEGER, LastPlayedDate TEXT)`,
		`INSERT INTO TypedBaseItems VALUES ('/media/a.mkv', 'a', 'long')`,
		`INSERT INTO UserDatas VALUES ('a', 1, 0, '')`,
	)
	if _, err := ReadEmby(bad); err == nil || !strings.Contains(err.Error(), "scan emby row") {
		t.Fatalf("ReadEmby(non-numeric RunTimeTicks) = %v; want a scan error", err)
	}
}
