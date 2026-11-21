package foreignimport

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// errEmbyUnknown reports a file with neither Emby library.db layout.
var errEmbyUnknown = errors.New("not an Emby library.db")

// Emby 4.10 keeps items in MediaItems and per-user state in UserDatas joined on
// UserDataKeyId (observed in the official 4.10.2 image, CORE-2678 evidence).
// Older Emby kept items in TypedBaseItems and state in UserDatas joined on key.
var (
	emby410Columns = []tableColumns{
		{"MediaItems", []string{"Id", "type", "Path", "RunTimeTicks", "UserDataKeyId"}},
		{"UserDatas", []string{"UserDataKeyId", "userId", "played", "playbackPositionTicks", "LastPlayedDateInt"}},
	}
	embyLegacyColumns = []tableColumns{
		{"TypedBaseItems", []string{"Path", "UserDataKey", "RunTimeTicks"}},
		{"UserDatas", []string{"key", "Played", "PlaybackPositionTicks", "LastPlayedDate"}},
	}
	emby410CollectionColumns = []tableColumns{
		{"MediaItems", []string{"Name"}},
		{"ItemLinks2", []string{"ItemId", "Type", "LinkedId", "LinkOrder"}},
	}
)

// Emby 4.10 MediaItems.type values that are containers, not files: root, media
// folder, folder, collections folder, series, season and collection.
const (
	embyContainerTypes  = "1, 2, 3, 4, 6, 7, 9"
	embyCollectionType  = 9
	embyCollectionLink  = 6
	embyNoUserColumn    = "''"
	embyUserColumn      = "CAST(ud.userId AS TEXT)"
	embyLegacyQueryHead = `
		SELECT tbi.Path, `
	embyLegacyQueryTail = `,
		       COALESCE(ud.Played, 0), COALESCE(ud.PlaybackPositionTicks, 0),
		       COALESCE(ud.LastPlayedDate, ''), COALESCE(tbi.RunTimeTicks, 0)
		FROM TypedBaseItems tbi
		JOIN UserDatas ud ON ud.key = tbi.UserDataKey
		WHERE tbi.Path IS NOT NULL AND tbi.Path <> ''
		  AND (ud.Played = 1 OR ud.PlaybackPositionTicks > 0)
		ORDER BY 1 COLLATE BINARY, 2 COLLATE BINARY`
	embyLegacyQuery       = embyLegacyQueryHead + embyNoUserColumn + embyLegacyQueryTail
	embyLegacyByUserQuery = embyLegacyQueryHead + embyUserColumn + embyLegacyQueryTail
	emby410Query          = `
		SELECT mi.Path, ` + embyUserColumn + `,
		       COALESCE(ud.played, 0), COALESCE(ud.playbackPositionTicks, 0),
		       COALESCE(ud.LastPlayedDateInt, 0), COALESCE(mi.RunTimeTicks, 0)
		FROM UserDatas ud
		JOIN MediaItems mi ON mi.UserDataKeyId = ud.UserDataKeyId
		WHERE mi.Path IS NOT NULL AND mi.Path <> ''
		  AND mi.type NOT IN (` + embyContainerTypes + `)
		  AND (ud.played = 1 OR ud.playbackPositionTicks > 0)
		ORDER BY mi.Path COLLATE BINARY, ud.userId, mi.Id`
)

// embySchema is the layout found in an Emby library.db.
type embySchema struct {
	modern bool // the 4.10 MediaItems layout
	// legacyUserID reports an older UserDatas with a userId column.
	legacyUserID bool
	// collections reports the 4.10 collection tables and columns.
	collections bool
}

func readEmbySchema(ctx context.Context, db *sql.DB) (embySchema, error) {
	tables, err := readTableColumns(ctx, db, "emby", "MediaItems", "UserDatas", "TypedBaseItems", "ItemLinks2")
	if err != nil {
		return embySchema{}, err
	}
	switch {
	case tables["MediaItems"] != nil:
		if err := requireColumns(tables, errEmbyUnknown, emby410Columns); err != nil {
			return embySchema{}, err
		}
		return embySchema{
			modern:      true,
			collections: requireColumns(tables, errEmbyUnknown, emby410CollectionColumns) == nil,
		}, nil
	case tables["TypedBaseItems"] != nil:
		if err := requireColumns(tables, errEmbyUnknown, embyLegacyColumns); err != nil {
			return embySchema{}, err
		}
		return embySchema{legacyUserID: tables["UserDatas"]["userid"]}, nil
	default:
		return embySchema{}, fmt.Errorf("%w: missing table MediaItems", errEmbyUnknown)
	}
}

// ReadEmby reads watch-state keyed by file path from an Emby `library.db`.
// It reads the Emby 4.10 layout (MediaItems and UserDatas joined on
// UserDataKeyId) and the older one (TypedBaseItems and UserDatas joined on
// key). Rows come in file path order.
func ReadEmby(path string) ([]WatchRow, error) {
	return ReadEmbyContext(context.Background(), path)
}

// ReadEmbyContext is ReadEmby with cancellation propagated to SQLite.
func ReadEmbyContext(ctx context.Context, path string) ([]WatchRow, error) {
	var out []WatchRow
	err := StreamEmbyWatchRows(ctx, path, func(row WatchRow) error {
		out = append(out, row)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// StreamEmbyWatchRows visits Emby watch rows in file path order, one per item
// and user.
func StreamEmbyWatchRows(ctx context.Context, path string, visit func(WatchRow) error) error {
	db, err := openReadOnly(path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	schema, err := readEmbySchema(ctx, db)
	if err != nil {
		return err
	}
	return streamEmbyRows(ctx, db, schema, func(_ string, row WatchRow) error { return visit(row) })
}

// streamEmbyRows visits each watch row with its user ID, which is empty for an
// older library without a userId column.
func streamEmbyRows(ctx context.Context, db *sql.DB, schema embySchema, visit func(userID string, row WatchRow) error) error {
	query := embyLegacyQuery
	switch {
	case schema.modern:
		query = emby410Query
	case schema.legacyUserID:
		query = embyLegacyByUserQuery
	}
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("query emby watch-state: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return err
		}
		var (
			row                         WatchRow
			userID                      string
			played, positionTicks, runs int64
			lastPlayed                  any
		)
		if err := rows.Scan(&row.Path, &userID, &played, &positionTicks, &lastPlayed, &runs); err != nil {
			return fmt.Errorf("scan emby row: %w", err)
		}
		row.Watched = played == 1
		row.PositionSeconds = positionTicks / ticksPerSecond
		row.DurationSeconds = runs / ticksPerSecond
		row.LastPlayed = embyLastPlayed(lastPlayed)
		if err := visit(userID, row); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate emby rows: %w", err)
	}
	return nil
}

// embyLastPlayed reads LastPlayedDateInt (4.10, Unix seconds UTC, 0 or null
// for none) or LastPlayedDate (older, RFC 3339). A malformed value degrades the
// row to no time rather than ending the import.
func embyLastPlayed(value any) time.Time {
	switch v := value.(type) {
	case int64:
		if v > 0 {
			return time.Unix(v, 0).UTC()
		}
	case string:
		if parsed, err := parseTime(v); err == nil {
			return parsed
		}
	case []byte:
		if parsed, err := parseTime(string(v)); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

// ReadEmbyByUser returns Emby watch state keyed by source user and the
// collection memberships shared by every user.
//
// On the 4.10 layout the key is `Name#Id`, with names from the users.db beside
// library.db, and every user there is a key even with no state. Without
// users.db the key is `#Id`. On the older layout the key is `#<userId>`, or `#`
// for everything when UserDatas has no userId column, and there are no
// collections. Ratings and favourites are not read: the Emby API observed for
// CORE-2678 never stored a rating, and favourites wait for CORE-2895.
func ReadEmbyByUser(ctx context.Context, path string, options ReadOptions) (map[string][]UserItem, []CollectionMember, error) {
	db, err := openReadOnly(path)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = db.Close() }()
	schema, err := readEmbySchema(ctx, db)
	if err != nil {
		return nil, nil, err
	}
	if options.Collections && schema.modern && !schema.collections {
		tables, err := readTableColumns(ctx, db, "emby", "MediaItems", "ItemLinks2")
		if err != nil {
			return nil, nil, err
		}
		return nil, nil, requireColumns(tables, errEmbyUnknown, emby410CollectionColumns)
	}
	names := map[string]string{}
	if schema.modern {
		if names, err = readEmbyUserNames(ctx, filepath.Join(filepath.Dir(path), "users.db")); err != nil {
			return nil, nil, err
		}
	}
	users := make(map[string][]UserItem, len(names))
	for id, name := range names {
		users[name+"#"+id] = nil
	}
	if options.WatchState {
		err := streamEmbyRows(ctx, db, schema, func(userID string, row WatchRow) error {
			key := names[userID] + "#" + userID
			users[key] = append(users[key], UserItem{WatchRow: row, HasWatchState: true})
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
	}
	if !options.Collections || !schema.modern {
		return users, nil, nil
	}
	members, err := readEmbyCollections(ctx, db)
	if err != nil {
		return nil, nil, err
	}
	return users, members, nil
}

// readEmbyUserNames reads Emby 4.10 user names keyed by LocalUsersv2.Id, which
// equals UserDatas.userId. A missing users.db or table gives no names.
func readEmbyUserNames(ctx context.Context, path string) (map[string]string, error) {
	names := map[string]string{}
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return names, nil
	} else if err != nil {
		return nil, fmt.Errorf("open emby users.db: %w", err)
	}
	db, err := openReadOnly(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	tables, err := readTableColumns(ctx, db, "emby users", "LocalUsersv2")
	if err != nil {
		return nil, err
	}
	if requireColumns(tables, errEmbyUnknown, []tableColumns{{"LocalUsersv2", []string{"Id", "data"}}}) != nil {
		return names, nil
	}
	rows, err := db.QueryContext(ctx, `
		SELECT CAST(Id AS TEXT),
		       COALESCE(CASE WHEN json_valid(data) THEN CAST(json_extract(data, '$.Name') AS TEXT) END, '')
		FROM LocalUsersv2`)
	if err != nil {
		return nil, fmt.Errorf("query emby users: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, fmt.Errorf("scan emby user: %w", err)
		}
		names[id] = name
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate emby users: %w", err)
	}
	return names, nil
}

// readEmbyCollections returns Emby 4.10 collection memberships in file path
// order. ItemLinks2 rows of type 6 link a member (ItemId) to a collection
// (LinkedId). Positions follow LinkOrder, then member path, counted from 0
// within each collection over its file members.
func readEmbyCollections(ctx context.Context, db *sql.DB) ([]CollectionMember, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT m.Path, c.Name, c.Id
		FROM ItemLinks2 il
		JOIN MediaItems c ON c.Id = il.LinkedId
		JOIN MediaItems m ON m.Id = il.ItemId
		WHERE il.Type = ? AND c.type = ? AND c.Name IS NOT NULL AND c.Name <> ''
		  AND m.Path IS NOT NULL AND m.Path <> ''
		  AND m.type NOT IN (`+embyContainerTypes+`)
		ORDER BY c.Id, il.LinkOrder, m.Path COLLATE BINARY`, embyCollectionLink, embyCollectionType)
	if err != nil {
		return nil, fmt.Errorf("query emby collections: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var (
		members    []CollectionMember
		collection int64
		position   int
	)
	for rows.Next() {
		var member CollectionMember
		var id int64
		if err := rows.Scan(&member.Path, &member.Name, &id); err != nil {
			return nil, fmt.Errorf("scan emby collection membership: %w", err)
		}
		if len(members) == 0 || id != collection {
			collection, position = id, 0
		}
		member.Position = position
		position++
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate emby collection memberships: %w", err)
	}
	sort.SliceStable(members, func(i, j int) bool { return members[i].Path < members[j].Path })
	return members, nil
}
