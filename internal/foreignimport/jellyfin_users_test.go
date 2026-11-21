package foreignimport

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

// jellyfin12UserSchema adds the columns and tables the per-user reader uses,
// as observed in the official 12.4 image (CORE-2678 evidence).
var jellyfin12UserSchema = []string{
	`ALTER TABLE "BaseItems" ADD COLUMN "Name" TEXT NULL`,
	`ALTER TABLE "UserData" ADD COLUMN "Likes" INTEGER NULL`,
	`ALTER TABLE "UserData" ADD COLUMN "Rating" REAL NULL`,
	`CREATE TABLE "Users" (
		"Id" TEXT NOT NULL CONSTRAINT "PK_Users" PRIMARY KEY,
		"Username" TEXT NOT NULL)`,
	`CREATE TABLE "LinkedChildren" (
		"ParentId" TEXT NOT NULL,
		"SortOrder" INTEGER NOT NULL,
		"ChildId" TEXT NOT NULL,
		"ChildType" INTEGER NOT NULL DEFAULT 0,
		CONSTRAINT "PK_LinkedChildren" PRIMARY KEY ("ParentId", "SortOrder"))`,
}

const (
	jellyfinThird    = "0B3E1D52-5C11-4F7B-9E57-2B8C4D7A9F10"
	jellyfinDeleted  = "DEADBEEF-0000-4000-8000-000000000000"
	jellyfinPlaylist = "MediaBrowser.Controller.Playlists.Playlist"
)

// jellyfinUserFixture records the CORE-2678 scenario: a rated-only film, a
// favourite-only film, a like, a rating on a lower-ranked copy, and one BoxSet.
func jellyfinUserFixture(t *testing.T, extra ...string) string {
	t.Helper()
	stmts := append(append([]string(nil), jellyfin12UserSchema...),
		`INSERT INTO Users (Id, Username) VALUES
			('`+jellyfinOwner+`', 'owner'), ('`+jellyfinSecond+`', 'second'), ('`+jellyfinThird+`', 'empty')`,
		`INSERT INTO BaseItems (Id, IsFolder, Path, RunTimeTicks, Type, Name) VALUES
			('arrival', 0, '/media/films/Arrival.mkv', 60000000000, '`+jellyfinMovie+`', 'Arrival'),
			('heat', 0, '/media/films/Heat.mkv', 90000000000, '`+jellyfinMovie+`', 'Heat'),
			('ep1', 0, '/media/tv/S01E01.mkv', 18000000000, '`+jellyfinEpisode+`', 'Pilot'),
			('ep2', 0, '/media/tv/S01E02.mkv', 18000000000, '`+jellyfinEpisode+`', 'Second'),
			('show', 1, '/media/tv', NULL, '`+jellyfinSeries+`', 'Night Shift'),
			('nopath', 0, NULL, 18000000000, '`+jellyfinMovie+`', 'Pathless'),
			('heist', 1, '/config/data/collections/Heist Night [boxset]', NULL, '`+jellyfinBoxSetType+`', 'Heist Night'),
			('pair', 1, '/config/data/collections/Pair [boxset]', NULL, '`+jellyfinBoxSetType+`', 'Pair'),
			('unnamed', 1, '/config/data/collections/x [boxset]', NULL, '`+jellyfinBoxSetType+`', ''),
			('list', 1, '/config/data/playlists/List', NULL, '`+jellyfinPlaylist+`', 'List')`,
		// The owner rated Arrival 8.5, written to every data key, and never played it.
		`INSERT INTO UserData (ItemId, UserId, CustomDataKey, Played, PlaybackPositionTicks, Rating, Likes) VALUES
			('arrival', '`+jellyfinOwner+`', '329865', 0, 0, 8.5, 1),
			('arrival', '`+jellyfinOwner+`', 'arrival-uuid', 0, 0, 8.5, 1),
			('arrival', '`+jellyfinOwner+`', 'tt2543164', 0, 0, 8.5, 1)`,
		// A favourite with no rating or watch state is not imported.
		`INSERT INTO UserData (ItemId, UserId, CustomDataKey, IsFavorite, Played, PlaybackPositionTicks) VALUES
			('heat', '`+jellyfinOwner+`', 'heat-uuid', 1, 0, 0)`,
		`INSERT INTO UserData (ItemId, UserId, CustomDataKey, Played, PlaybackPositionTicks) VALUES
			('ep1', '`+jellyfinOwner+`', 'ep1-uuid', 0, 120000000)`,
		// The newest copy carries the watch state, an older one the rating.
		`INSERT INTO UserData (ItemId, UserId, CustomDataKey, Played, PlaybackPositionTicks, LastPlayedDate, Rating) VALUES
			('ep2', '`+jellyfinOwner+`', 'ep2-a', 1, 0, '2026-11-21 10:00:00', NULL),
			('ep2', '`+jellyfinOwner+`', 'ep2-b', 0, 0, NULL, 4)`,
		// The second user's ID is stored in lower case; the Users join ignores case.
		`INSERT INTO UserData (ItemId, UserId, CustomDataKey, IsFavorite, Played, PlaybackPositionTicks, LastPlayedDate, Rating, Likes) VALUES
			('heat', '`+strings.ToLower(jellyfinSecond)+`', 'heat-uuid', 1, 1, 0, '2026-11-20 09:30:00', 6, 0),
			('ep1', '`+strings.ToLower(jellyfinSecond)+`', 'ep1-uuid', 0, 0, 0, NULL, 10, 1)`,
		// State left by a deleted user, a folder and a pathless item.
		`INSERT INTO UserData (ItemId, UserId, CustomDataKey, Played, PlaybackPositionTicks) VALUES
			('ep1', '`+jellyfinDeleted+`', 'ep1-uuid', 1, 0),
			('show', '`+jellyfinOwner+`', 'show-uuid', 1, 0),
			('nopath', '`+jellyfinOwner+`', 'nopath-uuid', 1, 0)`,
		`INSERT INTO LinkedChildren (ParentId, SortOrder, ChildId) VALUES
			('heist', 0, 'heat'), ('heist', 1, 'arrival'), ('heist', 2, 'ep2'), ('heist', 3, 'show'),
			('heist', 4, 'nopath'), ('pair', 5, 'heat'), ('unnamed', 0, 'heat'), ('list', 0, 'arrival')`,
	)
	return jellyfin12DB(t, "jellyfin.db", append(stmts, extra...)...)
}

func TestReadJellyfinByUser(t *testing.T) {
	path := jellyfinUserFixture(t)
	all := ReadOptions{WatchState: true, Ratings: true, Collections: true}
	users, members, err := ReadJellyfinByUser(context.Background(), path, all)
	if err != nil {
		t.Fatal(err)
	}
	r85, r4, r6, r10 := 8.5, 4.0, 6.0, 10.0
	owner, second := "owner#"+jellyfinOwner, "second#"+jellyfinSecond
	want := map[string][]UserItem{
		owner: {
			{WatchRow: WatchRow{Path: "/media/films/Arrival.mkv"}, Rating: &r85},
			{WatchRow: WatchRow{Path: "/media/tv/S01E01.mkv", PositionSeconds: 12, DurationSeconds: 1800}, HasWatchState: true},
			{WatchRow: WatchRow{Path: "/media/tv/S01E02.mkv", Watched: true, DurationSeconds: 1800,
				LastPlayed: time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)}, HasWatchState: true, Rating: &r4},
		},
		second: {
			{WatchRow: WatchRow{Path: "/media/films/Heat.mkv", Watched: true, DurationSeconds: 9000,
				LastPlayed: time.Date(2026, 11, 20, 9, 30, 0, 0, time.UTC)}, HasWatchState: true, Rating: &r6},
			{WatchRow: WatchRow{Path: "/media/tv/S01E01.mkv"}, Rating: &r10},
		},
		"empty#" + jellyfinThird: nil,
		"#" + jellyfinDeleted: {
			{WatchRow: WatchRow{Path: "/media/tv/S01E01.mkv", Watched: true, DurationSeconds: 1800}, HasWatchState: true},
		},
	}
	if !reflect.DeepEqual(users, want) {
		t.Errorf("users = %#v\nwant    %#v", users, want)
	}
	wantMembers := []CollectionMember{
		{Path: "/media/films/Arrival.mkv", Name: "Heist Night", Position: 1},
		{Path: "/media/films/Heat.mkv", Name: "Heist Night", Position: 0},
		{Path: "/media/films/Heat.mkv", Name: "Pair", Position: 5},
		{Path: "/media/tv/S01E02.mkv", Name: "Heist Night", Position: 2},
	}
	if !reflect.DeepEqual(members, wantMembers) {
		t.Errorf("members = %#v\nwant      %#v", members, wantMembers)
	}

	t.Run("ratings only", func(t *testing.T) {
		users, members, err := ReadJellyfinByUser(context.Background(), path, ReadOptions{Ratings: true})
		if err != nil {
			t.Fatal(err)
		}
		want := map[string][]UserItem{
			owner: {
				{WatchRow: WatchRow{Path: "/media/films/Arrival.mkv"}, Rating: &r85},
				{WatchRow: WatchRow{Path: "/media/tv/S01E02.mkv"}, Rating: &r4},
			},
			second: {
				{WatchRow: WatchRow{Path: "/media/films/Heat.mkv"}, Rating: &r6},
				{WatchRow: WatchRow{Path: "/media/tv/S01E01.mkv"}, Rating: &r10},
			},
			"empty#" + jellyfinThird: nil,
		}
		if !reflect.DeepEqual(users, want) || members != nil {
			t.Errorf("users = %#v, members = %#v\nwant %#v", users, members, want)
		}
	})

	t.Run("collections only", func(t *testing.T) {
		users, members, err := ReadJellyfinByUser(context.Background(), path, ReadOptions{Collections: true})
		if err != nil {
			t.Fatal(err)
		}
		want := map[string][]UserItem{owner: nil, second: nil, "empty#" + jellyfinThird: nil}
		if !reflect.DeepEqual(users, want) || !reflect.DeepEqual(members, wantMembers) {
			t.Errorf("users = %#v, members = %#v", users, members)
		}
	})

	t.Run("watch state matches the path reader", func(t *testing.T) {
		users, _, err := ReadJellyfinByUser(context.Background(), path, ReadOptions{WatchState: true})
		if err != nil {
			t.Fatal(err)
		}
		var byUser []WatchRow
		for _, items := range users {
			for _, item := range items {
				if !item.HasWatchState || item.Rating != nil {
					t.Fatalf("watch-state-only item = %#v", item)
				}
				byUser = append(byUser, item.WatchRow)
			}
		}
		rows, err := ReadJellyfin(path)
		if err != nil {
			t.Fatal(err)
		}
		sortWatchRows(byUser)
		sortWatchRows(rows)
		if !reflect.DeepEqual(byUser, rows) {
			t.Errorf("by-user rows = %#v\npath rows    %#v", byUser, rows)
		}
	})
}

func TestReadJellyfinByUserSchemaErrors(t *testing.T) {
	for _, test := range []struct {
		name    string
		stmts   []string
		options ReadOptions
		want    string
	}{
		{name: "no Users", stmts: nil, options: ReadOptions{WatchState: true}, want: "missing table Users"},
		{
			name:    "ratings without Rating",
			stmts:   []string{`CREATE TABLE Users (Id TEXT, Username TEXT)`},
			options: ReadOptions{Ratings: true},
			want:    "missing column UserData.Rating",
		},
		{
			name:    "collections without LinkedChildren",
			stmts:   []string{`CREATE TABLE Users (Id TEXT, Username TEXT)`, `ALTER TABLE BaseItems ADD COLUMN Name TEXT`},
			options: ReadOptions{Collections: true},
			want:    "missing table LinkedChildren",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := jellyfin12DB(t, "jellyfin.db", test.stmts...)
			users, members, err := ReadJellyfinByUser(context.Background(), path, test.options)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
			if users != nil || members != nil {
				t.Fatalf("failed read returned users = %#v, members = %#v", users, members)
			}
		})
	}

	t.Run("watch state without Rating", func(t *testing.T) {
		path := jellyfin12DB(t, "jellyfin.db", `CREATE TABLE Users (Id TEXT, Username TEXT)`,
			`INSERT INTO Users VALUES ('`+jellyfinOwner+`', 'owner')`)
		users, _, err := ReadJellyfinByUser(context.Background(), path, ReadOptions{WatchState: true})
		if err != nil {
			t.Fatal(err)
		}
		if want := map[string][]UserItem{"owner#" + jellyfinOwner: nil}; !reflect.DeepEqual(users, want) {
			t.Fatalf("users = %#v, want %#v", users, want)
		}
	})

	t.Run("unreadable rows", func(t *testing.T) {
		for _, test := range []struct {
			name, stmt, want string
			options          ReadOptions
		}{
			{"rating", `UPDATE UserData SET Rating = 'x' WHERE ItemId = 'arrival'`, "scan jellyfin user state",
				ReadOptions{Ratings: true}},
			{"duration", `UPDATE BaseItems SET RunTimeTicks = 'x' WHERE Id = 'ep1'`, "RunTimeTicks",
				ReadOptions{WatchState: true}},
			{"position", `UPDATE LinkedChildren SET SortOrder = 'x' WHERE ChildId = 'arrival' AND ParentId = 'heist'`,
				"scan jellyfin collection membership", ReadOptions{Collections: true}},
		} {
			t.Run(test.name, func(t *testing.T) {
				path := jellyfinUserFixture(t, test.stmt)
				if _, _, err := ReadJellyfinByUser(context.Background(), path, test.options); err == nil ||
					!strings.Contains(err.Error(), test.want) {
					t.Fatalf("error = %v, want %q", err, test.want)
				}
			})
		}
	})

	t.Run("canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, _, err := ReadJellyfinByUser(ctx, jellyfinUserFixture(t), ReadOptions{WatchState: true}); err == nil {
			t.Fatal("canceled read succeeded")
		}
	})
}
