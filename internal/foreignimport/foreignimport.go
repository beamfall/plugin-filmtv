// Package foreignimport reads watch-state, keyed by file path, from a
// predecessor media server's own SQLite database (Plex / Emby / Jellyfin) for
// the Migration Wizard. It is the Film & TV plugin's
// private SQLite reader: the plugin owns its modernc.org/sqlite dependency
// rather than forcing it onto every plugin through the SDK.
//
// It returns neutral, path-keyed WatchRows; it imports no plugin-sdk, parse, or
// mediaimport code (a leaf). The Film & TV import sources map these rows onto
// sdk.ForeignItem — the foreign-server-name vocabulary lives here and in the
// plugin's ImporterSpec, never in core or in the SDK.
package foreignimport

import (
	"context"
	"database/sql"
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
// Plex's metadata_item_settings watch-state is server-global, not per-Plex-user
// — the importer maps it to the operator's chosen Beamfall Profile; per-user
// Plex history is not imported yet.
func ReadPlex(path string) ([]WatchRow, error) {
	return ReadPlexContext(context.Background(), path)
}

// ReadPlexContext is ReadPlex with cancellation propagated to SQLite.
func ReadPlexContext(ctx context.Context, path string) ([]WatchRow, error) {
	db, err := openReadOnly(path)
	if err != nil {
		return nil, err
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
		  AND (COALESCE(mis.view_offset, 0) > 0 OR COALESCE(mis.view_count, 0) > 0)`)
	if err != nil {
		return nil, fmt.Errorf("query plex watch-state: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []WatchRow
	for rows.Next() {
		var (
			file                string
			offsetMS, viewCount int64
			lastViewedAt, durMS int64
		)
		if err := rows.Scan(&file, &offsetMS, &viewCount, &lastViewedAt, &durMS); err != nil {
			return nil, fmt.Errorf("scan plex row: %w", err)
		}
		row := WatchRow{
			Path:            file,
			Watched:         viewCount > 0,
			PositionSeconds: offsetMS / 1000,
			DurationSeconds: durMS / 1000,
		}
		if lastViewedAt > 0 {
			row.LastPlayed = time.Unix(lastViewedAt, 0).UTC()
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plex rows: %w", err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// ReadJellyfin reads watch-state keyed by file path from a Jellyfin
// `library.db`. Jellyfin keeps the file path on `TypedBaseItems.Path` and per-
// user playback state on `UserDatas` (`PlaybackPositionTicks`, `Played`,
// `LastPlayedDate`) joined on `UserDataKey`. The importer reads the
// aggregate (any user's) state and maps it to the operator's Profile, matching
// the Plex behaviour; per-user selection is not supported yet.
func ReadJellyfin(path string) ([]WatchRow, error) {
	return ReadJellyfinContext(context.Background(), path)
}

// ReadJellyfinContext is ReadJellyfin with cancellation propagated to SQLite.
func ReadJellyfinContext(ctx context.Context, path string) ([]WatchRow, error) {
	return readEmbyFamilyContext(ctx, path, "jellyfin")
}

// ReadEmby reads the same shape from an Emby `library.db`. Jellyfin is a 2018
// fork of Emby, so the schema is a sibling dialect — drifted but
// close enough to share one reader. Both descend from the same Emby schema
// (TypedBaseItems.Path + UserDatas keyed by UserDataKey), so the query is
// identical today; the dialect argument is carried for error context and any
// future per-dialect divergence.
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
