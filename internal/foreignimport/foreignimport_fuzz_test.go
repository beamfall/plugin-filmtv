package foreignimport

import (
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func FuzzReadForeignWatchRows(f *testing.F) {
	seeds := []struct {
		dialect            byte
		path               string
		position, duration int64
		lastPlayedUnix     int64
		lastPlayedText     string
		watched            bool
	}{
		{0, "/media/films/Watched.mkv", 0, 5_400_000, 1_614_834_367, "", true},
		{0, "/media/films/Resume.mkv", 120_000, 6_000_000, 0, "", false},
		{1, "/media/tv/S01E01.mkv", 0, 18_000_000_000, 0, "2026-06-10T12:34:56.0000000Z", true},
		{2, "/media/tv/S01E02.mkv", 1_200_000_000, 18_000_000_000, 0, "not-a-timestamp", false},
		{1, "", 0, 0, 0, "", false},
		{2, "路径/剧集.mkv", -1, -1, -1, "9999-99-99T99:99:99Z", true},
	}
	for _, seed := range seeds {
		f.Add(
			seed.dialect,
			seed.path,
			seed.position,
			seed.duration,
			seed.lastPlayedUnix,
			seed.lastPlayedText,
			seed.watched,
		)
	}

	f.Fuzz(func(
		t *testing.T,
		dialect byte,
		path string,
		position int64,
		duration int64,
		lastPlayedUnix int64,
		lastPlayedText string,
		watched bool,
	) {
		path = boundedFuzzString(path)
		lastPlayedText = boundedFuzzString(lastPlayedText)
		dialect %= 3

		databaseDirectory, err := os.MkdirTemp(t.TempDir(), "foreign-")
		if err != nil {
			t.Fatalf("create fuzz database directory: %v", err)
		}
		databasePath := filepath.Join(databaseDirectory, "source.db")
		writeForeignFuzzDatabase(
			t,
			databasePath,
			dialect,
			path,
			position,
			duration,
			lastPlayedUnix,
			lastPlayedText,
			watched,
		)

		read := func() ([]WatchRow, error) {
			switch dialect {
			case 0:
				return ReadPlex(databasePath)
			case 1:
				return ReadJellyfin(databasePath)
			default:
				return ReadEmby(databasePath)
			}
		}

		rows, err := read()
		again, againErr := read()
		if err != nil || againErr != nil {
			t.Fatalf("valid foreign import fixture returned errors: first=%v second=%v", err, againErr)
		}
		if !reflect.DeepEqual(rows, again) {
			t.Fatalf("foreign import rows are not deterministic: first=%#v second=%#v", rows, again)
		}

		wantRow := path != "" && (watched || position > 0)
		if !wantRow {
			if len(rows) != 0 {
				t.Fatalf("untouched/empty-path input returned rows: %#v", rows)
			}
			return
		}
		if len(rows) != 1 {
			t.Fatalf("foreign import returned %d rows, want 1: %#v", len(rows), rows)
		}
		if rows[0].Path != path || rows[0].Watched != watched {
			t.Fatalf("foreign import changed row identity/state: got %#v path=%q watched=%v", rows[0], path, watched)
		}
		unit := int64(1_000)
		if dialect != 0 {
			unit = ticksPerSecond
		}
		if rows[0].PositionSeconds != position/unit || rows[0].DurationSeconds != duration/unit {
			t.Fatalf("foreign import changed time units: got %#v position=%d duration=%d unit=%d", rows[0], position, duration, unit)
		}
		if dialect == 0 {
			if lastPlayedUnix > 0 && !rows[0].LastPlayed.Equal(time.Unix(lastPlayedUnix, 0).UTC()) {
				t.Fatalf("Plex timestamp changed: got %v want unix %d", rows[0].LastPlayed, lastPlayedUnix)
			}
			if lastPlayedUnix <= 0 && !rows[0].LastPlayed.IsZero() {
				t.Fatalf("non-positive Plex timestamp became %v", rows[0].LastPlayed)
			}
			return
		}
		parsed, parseErr := time.Parse(time.RFC3339Nano, lastPlayedText)
		if parseErr == nil && !rows[0].LastPlayed.Equal(parsed.UTC()) {
			t.Fatalf("Emby-family timestamp changed: got %v want %v", rows[0].LastPlayed, parsed.UTC())
		}
		if parseErr != nil && !rows[0].LastPlayed.IsZero() {
			t.Fatalf("malformed Emby-family timestamp became %v", rows[0].LastPlayed)
		}
	})
}

func boundedFuzzString(value string) string {
	const limit = 4 << 10
	if len(value) > limit {
		return value[:limit]
	}
	return value
}

func writeForeignFuzzDatabase(
	t *testing.T,
	path string,
	dialect byte,
	mediaPath string,
	position int64,
	duration int64,
	lastPlayedUnix int64,
	lastPlayedText string,
	watched bool,
) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+escapeSQLiteURIPath(path))
	if err != nil {
		t.Fatalf("open fuzz database: %v", err)
	}
	defer func() { _ = db.Close() }()

	played := 0
	if watched {
		played = 1
	}
	if dialect == 0 {
		statements := []string{
			`CREATE TABLE metadata_items (id INTEGER PRIMARY KEY, guid TEXT)`,
			`CREATE TABLE media_items (id INTEGER PRIMARY KEY, metadata_item_id INTEGER, duration INTEGER)`,
			`CREATE TABLE media_parts (media_item_id INTEGER, file TEXT)`,
			`CREATE TABLE metadata_item_settings (guid TEXT, view_offset INTEGER, view_count INTEGER, last_viewed_at INTEGER)`,
		}
		for _, statement := range statements {
			if _, err := db.Exec(statement); err != nil {
				t.Fatalf("create Plex fuzz schema: %v", err)
			}
		}
		if _, err := db.Exec(`INSERT INTO metadata_items (id, guid) VALUES (1, 'plex://fuzz/1')`); err != nil {
			t.Fatalf("insert Plex metadata: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO media_items (id, metadata_item_id, duration) VALUES (1, 1, ?)`, duration); err != nil {
			t.Fatalf("insert Plex media: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO media_parts (media_item_id, file) VALUES (1, ?)`, mediaPath); err != nil {
			t.Fatalf("insert Plex path: %v", err)
		}
		if _, err := db.Exec(
			`INSERT INTO metadata_item_settings (guid, view_offset, view_count, last_viewed_at) VALUES ('plex://fuzz/1', ?, ?, ?)`,
			position,
			played,
			lastPlayedUnix,
		); err != nil {
			t.Fatalf("insert Plex state: %v", err)
		}
		return
	}

	if _, err := db.Exec(`CREATE TABLE TypedBaseItems (Path TEXT, UserDataKey TEXT, RunTimeTicks INTEGER)`); err != nil {
		t.Fatalf("create Emby-family item schema: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE UserDatas (key TEXT, Played INTEGER, PlaybackPositionTicks INTEGER, LastPlayedDate TEXT)`); err != nil {
		t.Fatalf("create Emby-family state schema: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO TypedBaseItems (Path, UserDataKey, RunTimeTicks) VALUES (?, 'fuzz-key', ?)`, mediaPath, duration); err != nil {
		t.Fatalf("insert Emby-family item: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO UserDatas (key, Played, PlaybackPositionTicks, LastPlayedDate) VALUES ('fuzz-key', ?, ?, ?)`,
		played,
		position,
		lastPlayedText,
	); err != nil {
		t.Fatalf("insert Emby-family state: %v", err)
	}
}
