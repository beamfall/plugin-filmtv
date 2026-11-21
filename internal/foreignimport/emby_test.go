package foreignimport

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// emby410Schema is the part of the Emby 4.10 `library.db` the reader uses,
// with the STRICT tables and keys observed in the official 4.10.2 image
// (CORE-2678 evidence). Columns the reader ignores are trimmed.
var emby410Schema = []string{
	`CREATE TABLE MediaItems (Id INTEGER PRIMARY KEY, type INT NOT NULL, ParentId INT NULL,
		Path TEXT NULL, Name TEXT NULL, RunTimeTicks INTEGER NULL, UserDataKeyId INT NULL) STRICT`,
	`CREATE TABLE UserDatas (UserDataKeyId INT NOT NULL, userId INT NOT NULL, rating REAL NULL,
		played INT NOT NULL DEFAULT 0, playCount INT NOT NULL DEFAULT 0, isFavorite INT NOT NULL DEFAULT 0,
		playbackPositionTicks INTEGER NOT NULL DEFAULT 0, LastPlayedDateInt INT NULL,
		PRIMARY KEY (UserDataKeyId, userId)) STRICT, WITHOUT ROWID`,
	`CREATE TABLE ItemLinks2 (ItemId INT NOT NULL, Type INT NOT NULL, LinkedId INT NOT NULL, LinkOrder INT NOT NULL,
		PRIMARY KEY (ItemId, Type, LinkedId)) STRICT, WITHOUT ROWID`,
}

var embyUsersSchema = `CREATE TABLE LocalUsersv2 (Id INTEGER PRIMARY KEY AUTOINCREMENT,
	guid BLOB NOT NULL, data TEXT NOT NULL) STRICT`

// emby410Fixture records the CORE-2678 Emby scenario. Users 1 to 4 are in
// users.db (4 with unreadable data); user 9 is not.
func emby410Fixture(t *testing.T, extra ...string) string {
	t.Helper()
	stmts := append(append([]string(nil), emby410Schema...),
		`INSERT INTO MediaItems (Id, type, Path, Name, RunTimeTicks, UserDataKeyId) VALUES
			(1, 1, '/', 'root', NULL, NULL),
			(4, 4, '/var/lib/emby/data/collections', 'Collections', NULL, NULL),
			(10, 5, '/media/films/Arrival.mkv', 'Arrival', 60000000000, 100),
			(11, 5, '/media/films/Heat.mkv', 'Heat', 90000000000, 101),
			(12, 8, '/media/tv/S01E01.mkv', 'Pilot', 18000000000, 102),
			(13, 8, '/media/tv/S01E02.mkv', 'Second', 18000000000, 103),
			(14, 5, NULL, 'Pathless', 18000000000, 105),
			(20, 9, '/var/lib/emby/data/collections/Heist Night [boxset]', 'Heist Night', NULL, NULL),
			(21, 6, '/media/tv/Night Shift', 'Night Shift', NULL, 104),
			(22, 9, '/var/lib/emby/data/collections/Pair [boxset]', 'Pair', NULL, NULL),
			(23, 9, '/var/lib/emby/data/collections/x [boxset]', '', NULL, NULL)`,
		`INSERT INTO UserDatas (UserDataKeyId, userId, rating, played, isFavorite, playbackPositionTicks, LastPlayedDateInt) VALUES
			(100, 1, NULL, 1, 0, 0, 1763700000),
			(101, 1, NULL, 0, 1, 0, NULL),
			(102, 1, NULL, 0, 0, 120000000, NULL),
			(101, 2, NULL, 0, 1, 70000000, NULL),
			(100, 2, 8.5, 0, 0, 0, NULL),
			(104, 1, NULL, 1, 0, 0, 1763700000),
			(105, 1, NULL, 1, 0, 0, NULL),
			(103, 9, NULL, 1, 0, 0, 0)`,
		`INSERT INTO ItemLinks2 (ItemId, Type, LinkedId, LinkOrder) VALUES
			(11, 6, 20, 0), (10, 6, 20, 0), (13, 6, 20, 0), (21, 6, 20, 0), (14, 6, 20, 0),
			(11, 3, 20, 0), (13, 6, 22, 1), (12, 6, 22, 0), (11, 6, 23, 0), (12, 6, 4, 0)`,
	)
	path := fixtureDB(t, "library.db", append(stmts, extra...)...)
	execFixture(t, filepath.Join(filepath.Dir(path), "users.db"),
		embyUsersSchema,
		`INSERT INTO LocalUsersv2 (Id, guid, data) VALUES
			(1, X'01', '{"Name":"owner"}'), (2, X'02', '{"Name":"second"}'),
			(3, X'03', '{"Name":"empty"}'), (4, X'04', 'not json')`)
	return path
}

func execFixture(t *testing.T, path string, stmts ...string) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open fixture %s: %v", path, err)
	}
	defer func() { _ = db.Close() }()
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec fixture stmt %q: %v", stmt, err)
		}
	}
}

var (
	embyArrivalWatched = UserItem{WatchRow: WatchRow{Path: "/media/films/Arrival.mkv", Watched: true,
		DurationSeconds: 6000, LastPlayed: time.Unix(1763700000, 0).UTC()}, HasWatchState: true}
	embyPilotResume = UserItem{WatchRow: WatchRow{Path: "/media/tv/S01E01.mkv", PositionSeconds: 12,
		DurationSeconds: 1800}, HasWatchState: true}
	embyHeatResume = UserItem{WatchRow: WatchRow{Path: "/media/films/Heat.mkv", PositionSeconds: 7,
		DurationSeconds: 9000}, HasWatchState: true}
	embySecondWatched = UserItem{WatchRow: WatchRow{Path: "/media/tv/S01E02.mkv", Watched: true,
		DurationSeconds: 1800}, HasWatchState: true}
	embyMembers = []CollectionMember{
		{Path: "/media/films/Arrival.mkv", Name: "Heist Night", Position: 0},
		{Path: "/media/films/Heat.mkv", Name: "Heist Night", Position: 1},
		{Path: "/media/tv/S01E01.mkv", Name: "Pair", Position: 0},
		{Path: "/media/tv/S01E02.mkv", Name: "Heist Night", Position: 2},
		{Path: "/media/tv/S01E02.mkv", Name: "Pair", Position: 1},
	}
)

func TestReadEmbyByUser(t *testing.T) {
	path := emby410Fixture(t)
	all := ReadOptions{WatchState: true, Ratings: true, Collections: true}
	users, members, err := ReadEmbyByUser(context.Background(), path, all)
	if err != nil {
		t.Fatal(err)
	}
	// A favourite and a stored rating import nothing; series state is skipped.
	want := map[string][]UserItem{
		"owner#1":  {embyArrivalWatched, embyPilotResume},
		"second#2": {embyHeatResume},
		"empty#3":  nil,
		"#4":       nil,
		"#9":       {embySecondWatched},
	}
	if !reflect.DeepEqual(users, want) {
		t.Errorf("users = %#v\nwant    %#v", users, want)
	}
	if !reflect.DeepEqual(members, embyMembers) {
		t.Errorf("members = %#v\nwant      %#v", members, embyMembers)
	}

	t.Run("collections only", func(t *testing.T) {
		users, members, err := ReadEmbyByUser(context.Background(), path, ReadOptions{Collections: true})
		if err != nil {
			t.Fatal(err)
		}
		want := map[string][]UserItem{"owner#1": nil, "second#2": nil, "empty#3": nil, "#4": nil}
		if !reflect.DeepEqual(users, want) || !reflect.DeepEqual(members, embyMembers) {
			t.Errorf("users = %#v, members = %#v", users, members)
		}
	})

	t.Run("path reader", func(t *testing.T) {
		rows, err := ReadEmby(path)
		if err != nil {
			t.Fatal(err)
		}
		want := []WatchRow{embyArrivalWatched.WatchRow, embyHeatResume.WatchRow,
			embyPilotResume.WatchRow, embySecondWatched.WatchRow}
		if !reflect.DeepEqual(rows, want) {
			t.Errorf("rows = %#v\nwant %#v", rows, want)
		}
	})
}

func TestReadEmbyByUserWithoutUserNames(t *testing.T) {
	for _, test := range []struct {
		name  string
		users func(t *testing.T, path string)
	}{
		{name: "no users.db", users: func(t *testing.T, path string) {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "no LocalUsersv2", users: func(t *testing.T, path string) {
			execFixture(t, path, "DROP TABLE LocalUsersv2")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := emby410Fixture(t)
			test.users(t, filepath.Join(filepath.Dir(path), "users.db"))
			users, _, err := ReadEmbyByUser(context.Background(), path, ReadOptions{WatchState: true})
			if err != nil {
				t.Fatal(err)
			}
			want := map[string][]UserItem{
				"#1": {embyArrivalWatched, embyPilotResume},
				"#2": {embyHeatResume},
				"#9": {embySecondWatched},
			}
			if !reflect.DeepEqual(users, want) {
				t.Errorf("users = %#v\nwant    %#v", users, want)
			}
		})
	}

	t.Run("users.db not readable", func(t *testing.T) {
		path := emby410Fixture(t)
		users := filepath.Join(filepath.Dir(path), "users.db")
		if err := os.Remove(users); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(users, []byte("not a database"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := ReadEmbyByUser(context.Background(), path, ReadOptions{WatchState: true}); err == nil ||
			!strings.Contains(err.Error(), "emby users") {
			t.Fatalf("error = %v, want an emby users error", err)
		}
	})
}

func TestReadEmbyLegacyLayoutByUser(t *testing.T) {
	legacy := func(t *testing.T, userColumn string, rows ...string) string {
		return fixtureDB(t, "library.db", append([]string{
			`CREATE TABLE TypedBaseItems (Id TEXT PRIMARY KEY, Path TEXT, UserDataKey TEXT, RunTimeTicks INTEGER)`,
			`CREATE TABLE UserDatas (key TEXT` + userColumn + `, Played INTEGER, PlaybackPositionTicks INTEGER, LastPlayedDate)`,
			`INSERT INTO TypedBaseItems VALUES ('a', '/a.mkv', 'ka', 9000000000), ('b', '/b.mkv', 'kb', 9000000000)`,
		}, rows...)...)
	}
	at := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	t.Run("with userId", func(t *testing.T) {
		path := legacy(t, ", userId INTEGER",
			`INSERT INTO UserDatas VALUES ('ka', 1, 1, 0, '2024-01-02T03:04:05Z'),
				('kb', 2, 0, 50000000, CAST('2024-01-02T03:04:05Z' AS BLOB)), ('kb', 1, 0, 0, NULL),
				('ka', NULL, 1, 0, NULL)`)
		users, members, err := ReadEmbyByUser(context.Background(), path,
			ReadOptions{WatchState: true, Collections: true})
		if err != nil {
			t.Fatal(err)
		}
		want := map[string][]UserItem{
			"#1": {{WatchRow: WatchRow{Path: "/a.mkv", Watched: true, DurationSeconds: 900, LastPlayed: at}, HasWatchState: true}},
			"#2": {{WatchRow: WatchRow{Path: "/b.mkv", PositionSeconds: 5, DurationSeconds: 900, LastPlayed: at}, HasWatchState: true}},
			"#":  {{WatchRow: WatchRow{Path: "/a.mkv", Watched: true, DurationSeconds: 900}, HasWatchState: true}},
		}
		if !reflect.DeepEqual(users, want) || members != nil {
			t.Errorf("users = %#v, members = %#v\nwant %#v", users, members, want)
		}
	})
	t.Run("lower-case table names", func(t *testing.T) {
		path := fixtureDB(t, "library.db",
			`CREATE TABLE typedbaseitems (Path TEXT, UserDataKey TEXT, RunTimeTicks INTEGER)`,
			`CREATE TABLE userdatas (key TEXT, Played INTEGER, PlaybackPositionTicks INTEGER, LastPlayedDate TEXT)`,
			`INSERT INTO typedbaseitems VALUES ('/a.mkv', 'ka', 9000000000)`,
			`INSERT INTO userdatas VALUES ('ka', 1, 0, NULL)`)
		rows, err := ReadEmby(path)
		if err != nil || len(rows) != 1 || rows[0].Path != "/a.mkv" {
			t.Fatalf("rows = %#v, err = %v", rows, err)
		}
	})
	t.Run("without userId", func(t *testing.T) {
		path := legacy(t, "", `INSERT INTO UserDatas VALUES ('ka', 1, 0, 'not a time')`)
		users, _, err := ReadEmbyByUser(context.Background(), path, ReadOptions{WatchState: true})
		if err != nil {
			t.Fatal(err)
		}
		want := map[string][]UserItem{
			"#": {{WatchRow: WatchRow{Path: "/a.mkv", Watched: true, DurationSeconds: 900}, HasWatchState: true}},
		}
		if !reflect.DeepEqual(users, want) {
			t.Errorf("users = %#v\nwant %#v", users, want)
		}
	})
}

func TestEmbyLastPlayed(t *testing.T) {
	for _, test := range []struct {
		value any
		want  time.Time
	}{
		{nil, time.Time{}},
		{int64(0), time.Time{}},
		{int64(-1), time.Unix(-1, 0).UTC()},
		{int64(1763700000), time.Unix(1763700000, 0).UTC()},
		{"not a time", time.Time{}},
		{"2024-01-02T03:04:05Z", time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)},
		{[]byte("2024-01-02T03:04:05Z"), time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)},
	} {
		if got := embyLastPlayed(test.value); !got.Equal(test.want) {
			t.Errorf("embyLastPlayed(%#v) = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestReadEmbySchemaErrors(t *testing.T) {
	for _, test := range []struct {
		name    string
		stmts   []string
		options ReadOptions
		want    string
	}{
		{name: "unknown", stmts: []string{`CREATE TABLE Other (Id INTEGER)`}, want: "missing table MediaItems"},
		{name: "4.10 without UserDatas", stmts: emby410Schema[:1], want: "missing table UserDatas"},
		{name: "legacy without key", stmts: []string{
			`CREATE TABLE TypedBaseItems (Path TEXT, UserDataKey TEXT, RunTimeTicks INTEGER)`,
			`CREATE TABLE UserDatas (Played INTEGER, PlaybackPositionTicks INTEGER, LastPlayedDate TEXT)`,
		}, want: "missing column UserDatas.key"},
		{
			name:    "collections without ItemLinks2",
			stmts:   emby410Schema[:2],
			options: ReadOptions{Collections: true},
			want:    "missing table ItemLinks2",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := fixtureDB(t, "library.db", test.stmts...)
			users, members, err := ReadEmbyByUser(context.Background(), path, test.options)
			if err == nil || !errors.Is(err, errEmbyUnknown) || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
			if users != nil || members != nil {
				t.Fatalf("failed read returned users = %#v, members = %#v", users, members)
			}
			if _, err := ReadEmby(path); test.options.Collections != (err == nil) {
				t.Fatalf("ReadEmby error = %v", err)
			}
		})
	}

	t.Run("watch state without ItemLinks2", func(t *testing.T) {
		path := fixtureDB(t, "library.db", emby410Schema[:2]...)
		if _, _, err := ReadEmbyByUser(context.Background(), path, ReadOptions{WatchState: true}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("unreadable rows", func(t *testing.T) {
		for _, test := range []struct {
			name, stmt, want string
			options          ReadOptions
		}{
			{"membership", `DROP TABLE ItemLinks2`, "missing table ItemLinks2", ReadOptions{Collections: true}},
		} {
			t.Run(test.name, func(t *testing.T) {
				path := emby410Fixture(t, test.stmt)
				_, _, err := ReadEmbyByUser(context.Background(), path, test.options)
				if (test.want == "") != (err == nil) || (err != nil && !strings.Contains(err.Error(), test.want)) {
					t.Fatalf("error = %v, want %q", err, test.want)
				}
			})
		}
	})

	t.Run("unreadable duration", func(t *testing.T) {
		path := fixtureDB(t, "library.db",
			`CREATE TABLE TypedBaseItems (Path TEXT, UserDataKey TEXT, RunTimeTicks)`,
			`CREATE TABLE UserDatas (key TEXT, Played INTEGER, PlaybackPositionTicks INTEGER, LastPlayedDate TEXT)`,
			`INSERT INTO TypedBaseItems VALUES ('/a.mkv', 'ka', 9000000000), ('/b.mkv', 'kb', 'x')`,
			`INSERT INTO UserDatas VALUES ('ka', 1, 0, NULL), ('kb', 1, 0, NULL)`)
		users, members, err := ReadEmbyByUser(context.Background(), path, ReadOptions{WatchState: true})
		if err == nil || !strings.Contains(err.Error(), "scan emby row") || users != nil || members != nil {
			t.Fatalf("users = %#v, members = %#v, err = %v", users, members, err)
		}
	})

	t.Run("stops early", func(t *testing.T) {
		stop := errors.New("stop")
		calls := 0
		err := StreamEmbyWatchRows(context.Background(), emby410Fixture(t), func(WatchRow) error {
			calls++
			return stop
		})
		if !errors.Is(err, stop) || calls != 1 {
			t.Fatalf("err = %v after %d calls", err, calls)
		}
	})

	t.Run("canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, _, err := ReadEmbyByUser(ctx, emby410Fixture(t), ReadOptions{WatchState: true, Collections: true}); err == nil {
			t.Fatal("canceled read succeeded")
		}
	})
}
