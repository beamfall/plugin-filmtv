package foreignimport

import (
	"database/sql"
	"os"
	"path/filepath"
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

func TestReadJellyfinMapsPathKeyedWatchState(t *testing.T) {
	path := fixtureDB(t, "jellyfin.db",
		`CREATE TABLE TypedBaseItems (Id TEXT PRIMARY KEY, Path TEXT, UserDataKey TEXT, RunTimeTicks INTEGER)`,
		`CREATE TABLE UserDatas (key TEXT, Played INTEGER, PlaybackPositionTicks INTEGER, LastPlayedDate TEXT)`,
		// Completed, with a .NET-style RFC3339 timestamp.
		`INSERT INTO TypedBaseItems (Id, Path, UserDataKey, RunTimeTicks) VALUES ('a', '/media/tv/S01E01.mkv', 'key-a', 18000000000)`,
		`INSERT INTO UserDatas (key, Played, PlaybackPositionTicks, LastPlayedDate) VALUES ('key-a', 1, 0, '2026-06-10T12:34:56.0000000Z')`,
		// In-progress: 120s = 1_200_000_000 ticks.
		`INSERT INTO TypedBaseItems (Id, Path, UserDataKey, RunTimeTicks) VALUES ('b', '/media/tv/S01E02.mkv', 'key-b', 18000000000)`,
		`INSERT INTO UserDatas (key, Played, PlaybackPositionTicks, LastPlayedDate) VALUES ('key-b', 0, 1200000000, '')`,
		// Untouched: excluded.
		`INSERT INTO TypedBaseItems (Id, Path, UserDataKey, RunTimeTicks) VALUES ('c', '/media/tv/S01E03.mkv', 'key-c', 18000000000)`,
		`INSERT INTO UserDatas (key, Played, PlaybackPositionTicks, LastPlayedDate) VALUES ('key-c', 0, 0, '')`,
	)

	rows, err := ReadJellyfin(path)
	if err != nil {
		t.Fatalf("ReadJellyfin: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: %#v", len(rows), rows)
	}
	first := rows[0] // S01E01.mkv (sorted)
	if first.Path != "/media/tv/S01E01.mkv" || !first.Watched || first.DurationSeconds != 1800 {
		t.Fatalf("first row = %#v", first)
	}
	if !first.LastPlayed.Equal(time.Date(2026, 6, 10, 12, 34, 56, 0, time.UTC)) {
		t.Fatalf("first LastPlayed = %v", first.LastPlayed)
	}
	second := rows[1]
	if second.Path != "/media/tv/S01E02.mkv" || second.Watched || second.PositionSeconds != 120 {
		t.Fatalf("second row = %#v", second)
	}
	if !second.LastPlayed.IsZero() {
		t.Fatalf("second LastPlayed = %v, want zero (empty source)", second.LastPlayed)
	}
}

func TestReadEmbySharesJellyfinDialect(t *testing.T) {
	// Emby is the sibling schema: same TypedBaseItems/UserDatas
	// shape, so the shared reader resolves it identically.
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
	path := fixtureDB(t, "jellyfin-malformed.db",
		`CREATE TABLE TypedBaseItems (Id TEXT PRIMARY KEY, Path TEXT, UserDataKey TEXT, RunTimeTicks INTEGER)`,
		`CREATE TABLE UserDatas (key TEXT, Played INTEGER, PlaybackPositionTicks INTEGER, LastPlayedDate TEXT)`,
		// Good row with a valid timestamp.
		`INSERT INTO TypedBaseItems (Id, Path, UserDataKey, RunTimeTicks) VALUES ('a', '/media/tv/Good.mkv', 'key-a', 18000000000)`,
		`INSERT INTO UserDatas (key, Played, PlaybackPositionTicks, LastPlayedDate) VALUES ('key-a', 1, 0, '2026-06-10T12:34:56.0000000Z')`,
		// Bad row: malformed (non-RFC3339) LastPlayedDate.
		`INSERT INTO TypedBaseItems (Id, Path, UserDataKey, RunTimeTicks) VALUES ('b', '/media/tv/Bad.mkv', 'key-b', 18000000000)`,
		`INSERT INTO UserDatas (key, Played, PlaybackPositionTicks, LastPlayedDate) VALUES ('key-b', 1, 0, 'not-a-timestamp')`,
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
		t.Fatal("ReadJellyfin on a non-Jellyfin db: error = nil, want a query error")
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
