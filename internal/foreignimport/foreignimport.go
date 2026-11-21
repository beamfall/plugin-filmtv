// Package foreignimport reads migration records, keyed by file path, from a
// predecessor media server's own SQLite database (Plex / Emby / Jellyfin) for
// the Migration Wizard. It is the Film & TV plugin's
// private SQLite reader: the plugin owns its modernc.org/sqlite dependency
// rather than forcing it onto every plugin through the SDK.
//
// It returns neutral, path-keyed rows; it imports no plugin-sdk, parse, or
// mediaimport code (a leaf). The Film & TV import sources map these rows onto
// sdk.ForeignItem — the foreign-server-name vocabulary lives here and in the
// plugin's ImporterSpec, never in core or in the SDK.
package foreignimport

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// ticksPerSecond converts .NET TimeSpan ticks (100-nanosecond units, the unit
// the whole Emby/Jellyfin family inherits) to seconds.
const ticksPerSecond = 10_000_000

// WatchRow is one path-keyed playback-history record read from a foreign server
// database. Beamfall's migration match chain keys on file path, so
// the reader surfaces the absolute path the foreign server stored, not its
// internal item id.
type WatchRow struct {
	// Path is the absolute file path the foreign server recorded for the item.
	Path string
	// Watched is the source's completion flag.
	Watched bool
	// PositionSeconds is the resume offset; 0 for completed or never-resumed.
	PositionSeconds int64
	// DurationSeconds is the item runtime when the source knew it; 0 if unknown.
	DurationSeconds int64
	// LastPlayed is the true historical instant the source last played the item;
	// zero when the source recorded none.
	LastPlayed time.Time
}

// openReadOnly opens a foreign server database read-only. The path is an
// operator-supplied export file (the user provides it per run; no live
// connection is held). mode=ro + immutable=1 guarantees the import never
// writes the original file even if it is a live copy.
func openReadOnly(path string) (*sql.DB, error) {
	// Percent-encode a literal '?' or '#' in the operator-supplied path before
	// splicing it into the DSN. Unescaped, either character would be misread
	// as the start of the DSN's query string or fragment: modernc.org/sqlite
	// splits the DSN at the *first* '?' in the whole string (see its
	// newConn), so a '?' inside the path truncates or corrupts the intended
	// "mode=ro&immutable=1" query and could silently drop the read-only
	// guarantee. The standard SQLite URI filename parser (SQLITE_OPEN_URI)
	// percent-decodes the path back to its literal bytes, so this round-trips
	// correctly; unlike url.PathEscape, it leaves '/' untouched.
	dsn := "file:" + escapeSQLiteURIPath(path) + "?mode=ro&immutable=1"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open foreign db %q: %w", path, err)
	}
	return db, nil
}

// escapeSQLiteURIPath percent-encodes the '?', '#', and '%' characters in a
// file path so it can be safely embedded in a SQLite file: DSN ahead of a
// query string. '?' and '#' are structurally significant to the DSN parser's
// query/fragment split; every other byte, including '/', passes through
// unchanged. '%' must also be encoded because the SQLite URI filename parser
// percent-decodes the path back to its literal bytes, so a literal "%23" or
// "%3F" already present in a real path would otherwise decode to '#' or '?'
// and resolve the wrong file.
func escapeSQLiteURIPath(path string) string {
	r := strings.NewReplacer("%", "%25", "?", "%3F", "#", "%23")
	return r.Replace(path)
}

// ReadPlex reads watch-state keyed by file path from a Plex library database
// (`com.plexapp.plugins.library.db`). Plex records the resume position and
// completion in `metadata_item_settings` (view_offset in MILLISECONDS,
// view_count for completion, last_viewed_at as a unix instant) keyed by the
// item's `guid`; the file path lives in `media_parts.file`, joined through
// `media_items` → `metadata_items`. An item with multiple parts/versions yields
// one row per file path — each matches independently in Beamfall's chain
// (Plex stores view state in metadata_item_settings/view_offset).
//
// This compatibility reader returns watch rows without grouping by account.
func ReadPlex(path string) ([]WatchRow, error) {
	return ReadPlexContext(context.Background(), path)
}

// ReadPlexContext is ReadPlex with cancellation propagated to SQLite.
func ReadPlexContext(ctx context.Context, path string) ([]WatchRow, error) {
	var out []WatchRow
	err := StreamPlexWatchRows(ctx, path, func(row WatchRow) error {
		out = append(out, row)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// StreamPlexWatchRows visits the compatibility watch rows in file path order.
func StreamPlexWatchRows(ctx context.Context, path string, visit func(WatchRow) error) error {
	db, err := openReadOnly(path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	rows, err := db.QueryContext(ctx, `
		SELECT mp.file,
		       COALESCE(mis.view_offset, 0),
		       COALESCE(mis.view_count, 0),
		       COALESCE(mis.last_viewed_at, 0),
		       COALESCE(mi.duration, 0)
		FROM media_parts mp
		JOIN media_items mi ON mi.id = mp.media_item_id
		JOIN metadata_items md ON md.id = mi.metadata_item_id
		LEFT JOIN metadata_item_settings mis ON mis.guid = md.guid
		WHERE mp.file IS NOT NULL AND mp.file <> ''
		  AND (COALESCE(mis.view_offset, 0) > 0 OR COALESCE(mis.view_count, 0) > 0)
		ORDER BY mp.file COLLATE BINARY`)
	if err != nil {
		return fmt.Errorf("query plex watch-state: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return err
		}
		var (
			file                string
			offsetMS, viewCount int64
			lastViewedAt, durMS int64
		)
		if err := rows.Scan(&file, &offsetMS, &viewCount, &lastViewedAt, &durMS); err != nil {
			return fmt.Errorf("scan plex row: %w", err)
		}
		if err := visit(plexWatchRow(file, offsetMS, viewCount, lastViewedAt, durMS)); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate plex rows: %w", err)
	}
	return nil
}

func plexWatchRow(file string, offsetMS, viewCount, lastViewedAt, durationMS int64) WatchRow {
	row := WatchRow{
		Path:            file,
		Watched:         viewCount > 0,
		PositionSeconds: offsetMS / 1000,
		DurationSeconds: durationMS / 1000,
	}
	if lastViewedAt > 0 {
		row.LastPlayed = time.Unix(lastViewedAt, 0).UTC()
	}
	return row
}

// PlexAccountItem holds one account's settings for a file. A nil Rating means unrated.
type PlexAccountItem struct {
	WatchRow
	HasWatchState bool
	Rating        *float64
}

// PlexCollectionMember is a file's position in a library-wide collection.
type PlexCollectionMember struct {
	Path     string
	Name     string
	Position int
}

// PlexReadOptions selects the account state and collection data to read.
type PlexReadOptions struct {
	WatchState  bool
	Ratings     bool
	Collections bool
}

// ReadPlexByAccount returns account settings keyed by source-user ID and the
// collections shared by those accounts. Accounts with no settings are included.
func ReadPlexByAccount(ctx context.Context, path string, options PlexReadOptions) (map[string][]PlexAccountItem, []PlexCollectionMember, error) {
	db, err := openReadOnly(path)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = db.Close() }()
	accounts, err := readPlexAccountItems(ctx, db, options)
	if err != nil {
		return nil, nil, err
	}
	if !options.Collections {
		return accounts, nil, nil
	}
	collections, err := readPlexCollectionMembers(ctx, db)
	if err != nil {
		return nil, nil, err
	}
	return accounts, collections, nil
}

func readPlexAccountItems(ctx context.Context, db *sql.DB, options PlexReadOptions) (map[string][]PlexAccountItem, error) {
	if !options.WatchState && !options.Ratings {
		return readPlexAccountKeys(ctx, db)
	}
	rows, err := db.QueryContext(ctx, `
		WITH account_files AS (
			SELECT mis.account_id, mp.file,
			       CASE WHEN ?1 THEN mis.view_offset END AS view_offset,
			       CASE WHEN ?1 THEN mis.view_count END AS view_count,
			       CASE WHEN ?1 THEN mis.last_viewed_at END AS last_viewed_at,
			       CASE WHEN ?1 THEN mi.duration END AS duration,
			       CASE WHEN ?2 THEN mis.rating END AS rating
			FROM metadata_item_settings mis
			JOIN metadata_items md ON md.guid = mis.guid
			JOIN media_items mi ON mi.metadata_item_id = md.id
			JOIN media_parts mp ON mp.media_item_id = mi.id
			WHERE mp.file <> ''
			  AND ((?1 AND (mis.view_offset > 0 OR mis.view_count > 0)) OR (?2 AND mis.rating IS NOT NULL))
		)
		SELECT COALESCE(a.name, '') || '#' || a.id, COALESCE(f.file, ''),
		       COALESCE(f.view_offset, 0), COALESCE(f.view_count, 0),
		       COALESCE(f.last_viewed_at, 0), COALESCE(f.duration, 0), f.rating
		FROM accounts a
		LEFT JOIN account_files f ON f.account_id = a.id
		ORDER BY a.id, f.file COLLATE BINARY`, options.WatchState, options.Ratings)
	if err != nil {
		return nil, fmt.Errorf("query plex account settings: %w", err)
	}
	defer func() { _ = rows.Close() }()
	accounts := make(map[string][]PlexAccountItem)
	for rows.Next() {
		var account, file string
		var offsetMS, viewCount, lastViewedAt, durationMS int64
		var rating *float64
		if err := rows.Scan(&account, &file, &offsetMS, &viewCount, &lastViewedAt, &durationMS, &rating); err != nil {
			return nil, fmt.Errorf("scan plex account settings: %w", err)
		}
		if file == "" {
			accounts[account] = nil
			continue
		}
		accounts[account] = append(accounts[account], PlexAccountItem{
			WatchRow:      plexWatchRow(file, offsetMS, viewCount, lastViewedAt, durationMS),
			HasWatchState: offsetMS > 0 || viewCount > 0,
			Rating:        rating,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plex account settings: %w", err)
	}
	return accounts, nil
}

func readPlexAccountKeys(ctx context.Context, db *sql.DB) (map[string][]PlexAccountItem, error) {
	rows, err := db.QueryContext(ctx, `SELECT COALESCE(name, '') || '#' || id FROM accounts`)
	if err != nil {
		return nil, fmt.Errorf("query plex account keys: %w", err)
	}
	defer func() { _ = rows.Close() }()
	accounts := make(map[string][]PlexAccountItem)
	for rows.Next() {
		var account string
		if err := rows.Scan(&account); err != nil {
			return nil, fmt.Errorf("scan plex account key: %w", err)
		}
		accounts[account] = nil
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plex account keys: %w", err)
	}
	return accounts, nil
}

func readPlexCollectionMembers(ctx context.Context, db *sql.DB) ([]PlexCollectionMember, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT mp.file, tags.tag, COALESCE(taggings."index", 0)
		FROM tags
		JOIN taggings ON taggings.tag_id = tags.id
		JOIN metadata_items md ON md.id = taggings.metadata_item_id
		JOIN media_items mi ON mi.metadata_item_id = md.id
		JOIN media_parts mp ON mp.media_item_id = mi.id
		WHERE tags.tag_type = 2 AND tags.tag <> '' AND mp.file <> ''
		ORDER BY mp.file COLLATE BINARY, tags.id, taggings."index"`)
	if err != nil {
		return nil, fmt.Errorf("query plex collections: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var members []PlexCollectionMember
	for rows.Next() {
		var member PlexCollectionMember
		if err := rows.Scan(&member.Path, &member.Name, &member.Position); err != nil {
			return nil, fmt.Errorf("scan plex collection membership: %w", err)
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plex collection memberships: %w", err)
	}
	return members, nil
}

// ReadJellyfin reads watch-state keyed by file path from a Jellyfin 12
// `jellyfin.db`. Jellyfin 12 keeps the file path on `BaseItems.Path` and
// per-user playback state on `UserData` (`Played`, `PlaybackPositionTicks`,
// `LastPlayedDate`) keyed by item, user and data key. The importer reads every
// source user's state and maps it to the operator's Profile.
//
// Only Jellyfin 12 is read. A pre-12 `library.db` is refused with an error that
// tells the operator to upgrade Jellyfin first (owner decision 2026-11-21).
// The layout was observed by inspecting the database of the official 12.4
// image (CORE-2677), not from Jellyfin server source.
func ReadJellyfin(path string) ([]WatchRow, error) {
	return ReadJellyfinContext(context.Background(), path)
}

// ReadJellyfinContext is ReadJellyfin with cancellation propagated to SQLite.
func ReadJellyfinContext(ctx context.Context, path string) ([]WatchRow, error) {
	var out []WatchRow
	err := StreamJellyfinWatchRows(ctx, path, func(row WatchRow) error {
		out = append(out, row)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// errJellyfinLegacy reports a pre-12 Jellyfin `library.db`.
var errJellyfinLegacy = errors.New("this is a Jellyfin library.db from before Jellyfin 12: " +
	"upgrade Jellyfin to 12, then import jellyfin.db from its data folder")

// errJellyfinUnknown reports a file without the Jellyfin 12 layout.
var errJellyfinUnknown = errors.New("not a Jellyfin 12 jellyfin.db")

// jellyfinColumns lists, in check order, the Jellyfin 12 tables and columns the
// reader uses.
var jellyfinColumns = []struct {
	table   string
	columns []string
}{
	{"BaseItems", []string{"Id", "Path", "RunTimeTicks", "IsFolder"}},
	{"UserData", []string{"ItemId", "UserId", "CustomDataKey", "Played", "PlaybackPositionTicks", "LastPlayedDate"}},
}

// checkJellyfinSchema refuses anything but the Jellyfin 12 layout. It checks
// shape, not a version number, so a later 12.x that keeps these columns reads.
func checkJellyfinSchema(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `
		SELECT m.name, p.name
		FROM sqlite_master m JOIN pragma_table_info(m.name) p
		WHERE m.type = 'table'`)
	if err != nil {
		return fmt.Errorf("read jellyfin schema: %w", err)
	}
	defer func() { _ = rows.Close() }()
	tables := map[string]map[string]bool{}
	for rows.Next() {
		var table, column string
		if err := rows.Scan(&table, &column); err != nil {
			return fmt.Errorf("scan jellyfin schema: %w", err)
		}
		if tables[table] == nil {
			tables[table] = map[string]bool{}
		}
		tables[table][column] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read jellyfin schema: %w", err)
	}
	if tables["BaseItems"] == nil && tables["TypedBaseItems"] != nil {
		return errJellyfinLegacy
	}
	for _, want := range jellyfinColumns {
		have := tables[want.table]
		if have == nil {
			return fmt.Errorf("%w: missing table %s", errJellyfinUnknown, want.table)
		}
		for _, column := range want.columns {
			if !have[column] {
				return fmt.Errorf("%w: missing column %s.%s", errJellyfinUnknown, want.table, column)
			}
		}
	}
	return nil
}

// jellyfinCopy is one Jellyfin 12 `UserData` row joined to its item.
type jellyfinCopy struct {
	path, userID, itemID string
	played               bool
	positionTicks        int64
	lastPlayed           time.Time
	runTimeTicks         int64
}

func (c jellyfinCopy) sameEntry(o jellyfinCopy) bool {
	return c.path == o.path && c.userID == o.userID && c.itemID == o.itemID
}

// beats reports whether c is a better copy of the same entry than o: a later
// last-played instant (an absent or unreadable time ranks last), then played,
// then the larger resume position. On a full tie the earlier copy, the lower
// data key, stays.
func (c jellyfinCopy) beats(o jellyfinCopy) bool {
	if !c.lastPlayed.Equal(o.lastPlayed) {
		return c.lastPlayed.After(o.lastPlayed)
	}
	if c.played != o.played {
		return c.played
	}
	return c.positionTicks > o.positionTicks
}

// StreamJellyfinWatchRows visits Jellyfin 12 watch rows in file path order, one
// per item and source user.
//
// Jellyfin 12 writes one user's state for one item as several identical
// `UserData` rows, one per data key (provider IDs and internal keys). The
// reader keeps one: the latest last-played instant, then a played copy, then
// the larger resume position, then the lowest data key. Instants are compared
// after parsing, so two spellings of one time tie. It picks before it filters,
// so a newer unplayed copy wins over an older played one.
func StreamJellyfinWatchRows(ctx context.Context, path string, visit func(WatchRow) error) error {
	db, err := openReadOnly(path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	if err := checkJellyfinSchema(ctx, db); err != nil {
		return err
	}

	// The order groups each entry's copies together and is total, so a resumed
	// import counts the same rows in the same order.
	rows, err := db.QueryContext(ctx, `
		SELECT b.Path, u.UserId, u.ItemId,
		       COALESCE(u.Played, 0), COALESCE(u.PlaybackPositionTicks, 0),
		       COALESCE(u.LastPlayedDate, ''), COALESCE(b.RunTimeTicks, 0)
		FROM UserData u
		JOIN BaseItems b ON b.Id = u.ItemId
		WHERE b.Path IS NOT NULL AND b.Path <> '' AND COALESCE(b.IsFolder, 0) = 0
		ORDER BY b.Path COLLATE BINARY, u.UserId COLLATE BINARY,
		         u.ItemId COLLATE BINARY, u.CustomDataKey COLLATE BINARY`)
	if err != nil {
		return fmt.Errorf("query jellyfin watch-state: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var best jellyfinCopy
	pending := false
	emit := func() error {
		if !pending || (!best.played && best.positionTicks <= 0) {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		return visit(WatchRow{
			Path:            best.path,
			Watched:         best.played,
			PositionSeconds: best.positionTicks / ticksPerSecond,
			DurationSeconds: best.runTimeTicks / ticksPerSecond,
			LastPlayed:      best.lastPlayed,
		})
	}
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return err
		}
		var (
			c          jellyfinCopy
			played     int64
			lastPlayed string
		)
		if err := rows.Scan(&c.path, &c.userID, &c.itemID, &played, &c.positionTicks, &lastPlayed, &c.runTimeTicks); err != nil {
			return fmt.Errorf("scan jellyfin row: %w", err)
		}
		c.played = played == 1
		// A malformed LastPlayedDate leaves the time zero rather than ending
		// the import.
		if parsed, err := parseJellyfinTime(lastPlayed); err == nil {
			c.lastPlayed = parsed
		}
		if pending && c.sameEntry(best) {
			if c.beats(best) {
				best = c
			}
			continue
		}
		if err := emit(); err != nil {
			return err
		}
		best, pending = c, true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate jellyfin rows: %w", err)
	}
	return emit()
}

// jellyfinTimeLayout is the form Jellyfin 12 stores LastPlayedDate in: UTC,
// a space before the time, up to seven fractional digits, no zone suffix.
const jellyfinTimeLayout = "2006-01-02 15:04:05.9999999"

// parseJellyfinTime reads Jellyfin 12's stored LastPlayedDate, falling back to
// RFC 3339 for a value written with a zone.
func parseJellyfinTime(value string) (time.Time, error) {
	if parsed, err := time.ParseInLocation(jellyfinTimeLayout, value, time.UTC); err == nil {
		return parsed, nil
	}
	return parseTime(value)
}

// ReadEmby reads watch-state keyed by file path from an Emby `library.db`.
// Emby keeps the file path on `TypedBaseItems.Path` and per-user playback state
// on `UserDatas` (`PlaybackPositionTicks`, `Played`, `LastPlayedDate`) joined on
// `UserDataKey`. Jellyfin before 12 used the same layout; the Jellyfin source
// now refuses it (see ReadJellyfin).
func ReadEmby(path string) ([]WatchRow, error) {
	return ReadEmbyContext(context.Background(), path)
}

// ReadEmbyContext is ReadEmby with cancellation propagated to SQLite.
func ReadEmbyContext(ctx context.Context, path string) ([]WatchRow, error) {
	return readEmbyFamilyContext(ctx, path, "emby")
}

func readEmbyFamilyContext(ctx context.Context, path, dialect string) ([]WatchRow, error) {
	db, err := openReadOnly(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()

	rows, err := db.QueryContext(ctx, `
		SELECT tbi.Path,
		       COALESCE(ud.Played, 0),
		       COALESCE(ud.PlaybackPositionTicks, 0),
		       COALESCE(ud.LastPlayedDate, ''),
		       COALESCE(tbi.RunTimeTicks, 0)
		FROM TypedBaseItems tbi
		JOIN UserDatas ud ON ud.key = tbi.UserDataKey
		WHERE tbi.Path IS NOT NULL AND tbi.Path <> ''
		  AND (ud.Played = 1 OR ud.PlaybackPositionTicks > 0)`)
	if err != nil {
		return nil, fmt.Errorf("query %s watch-state: %w", dialect, err)
	}
	defer func() { _ = rows.Close() }()

	var out []WatchRow
	for rows.Next() {
		var (
			file          string
			played        int64
			positionTicks int64
			lastPlayed    string
			runTimeTicks  int64
		)
		if err := rows.Scan(&file, &played, &positionTicks, &lastPlayed, &runTimeTicks); err != nil {
			return nil, fmt.Errorf("scan %s row: %w", dialect, err)
		}
		row := WatchRow{
			Path:            file,
			Watched:         played == 1,
			PositionSeconds: positionTicks / ticksPerSecond,
			DurationSeconds: runTimeTicks / ticksPerSecond,
		}
		if lastPlayed != "" {
			// A malformed LastPlayedDate degrades this one row to a zero
			// LastPlayed rather than aborting the whole import, matching the
			// Plex path's graceful handling of an absent/zero timestamp.
			if parsed, err := parseTime(lastPlayed); err == nil {
				row.LastPlayed = parsed
			}
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s rows: %w", dialect, err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// parseTime parses the RFC3339(.NET) timestamp Emby/Jellyfin write for
// LastPlayedDate. An empty string is handled by the caller; a non-empty value
// that does not parse is an error so a schema drift surfaces rather than
// silently dropping the historical instant.
func parseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time %q: %w", value, err)
	}
	return parsed.UTC(), nil
}
