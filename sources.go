// This file is the Film & TV plugin's compiled migration sources: it implements the sdk.Source seam for each
// foreign system the plugin declares an ImporterSpec for — Plex/Emby/Jellyfin
// (SQLite watch-state DBs), NFO (sidecar tree), and Trakt watched-history CSV
// (in trakt.go).
//
// The constructors reuse the internal parsers: internal/foreignimport reads
// the foreign SQLite DBs (path-keyed WatchRows), internal/mediaimport walks the
// NFO sidecar tree (path-keyed NFOItems), and Trakt rows carry external IDs.
// This file maps them onto sdk.ForeignItem — surfacing the path or neutral
// external IDs the source recorded so Core's migration matching can resolve each item
// to a core Media. Source-system names live here and in the plugin's
// ImporterSpec, never in core or the SDK.
package filmtv

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/beamfall/plugin-filmtv/internal/foreignimport"
	sdk "github.com/beamfall/plugin-sdk"
)

// watchRowReader is the shared shape of the three foreign-DB readers
// (foreignimport.ReadPlex/ReadEmby/ReadJellyfin): given the operator-supplied
// export-file path, it returns path-keyed watch rows. Holding it as a value lets
// the three watch-state sources share one Items implementation.
type watchRowReader func(ctx context.Context, dbPath string) ([]foreignimport.WatchRow, error)

type watchRowStreamer func(
	ctx context.Context,
	dbPath string,
	visit func(foreignimport.WatchRow) error,
) error

const filmTVImportPageItems = 256

// dbWatchSource adapts a foreign-DB watch-state reader to sdk.Source.
type dbWatchSource struct {
	read   watchRowReader
	stream watchRowStreamer
	dbPath string
}

func (dbWatchSource) Categories() []string { return []string{sdk.CategoryWatchState} }

func (s dbWatchSource) Items(ctx context.Context) ([]sdk.ForeignItem, error) {
	rows, err := s.read(ctx, s.dbPath)
	if err != nil {
		return nil, err
	}
	items := make([]sdk.ForeignItem, 0, len(rows))
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		items = append(items, watchRowForeignItem(row))
	}
	return items, nil
}

func (s dbWatchSource) Stream(
	ctx context.Context,
	checkpoint string,
	emit func(context.Context, []sdk.ForeignItem, string) error,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.stream == nil {
		return errors.New("Film/TV import source has no streaming reader")
	}
	pager, err := newFilmTVPageEmitter(ctx, checkpoint, emit)
	if err != nil {
		return err
	}
	if err := s.stream(ctx, s.dbPath, func(row foreignimport.WatchRow) error {
		return pager.add(watchRowForeignItem(row))
	}); err != nil {
		return err
	}
	return pager.finish()
}

func watchRowForeignItem(row foreignimport.WatchRow) sdk.ForeignItem {
	return sdk.ForeignItem{
		Path: row.Path,
		WatchState: sdk.WatchState{
			Present:         true,
			PositionSeconds: int(row.PositionSeconds),
			DurationSeconds: int(row.DurationSeconds),
			Watched:         row.Watched,
			OccurredAt:      row.LastPlayed,
		},
	}
}

type plexSource struct {
	dbWatchSource
	categories []string
}

func (s plexSource) Categories() []string {
	if s.categories != nil {
		return slices.Clone(s.categories)
	}
	return []string{sdk.CategoryWatchState, sdk.CategoryRatings, sdk.CategoryCollections}
}

// ItemsBySourceUser supplies Core with account-specific playback state and ratings.
// Library-wide collection memberships are available under each source-user key.
func (s plexSource) ItemsBySourceUser(ctx context.Context) (map[string][]sdk.ForeignItem, error) {
	options := foreignimport.PlexReadOptions{
		WatchState:  s.categories == nil || slices.Contains(s.categories, sdk.CategoryWatchState),
		Ratings:     slices.Contains(s.categories, sdk.CategoryRatings),
		Collections: slices.Contains(s.categories, sdk.CategoryCollections),
	}
	accounts, memberships, err := foreignimport.ReadPlexByAccount(ctx, s.dbPath, options)
	if err != nil {
		return nil, err
	}
	collectionItems := make([]sdk.ForeignItem, 0, len(memberships))
	for _, member := range memberships {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		collectionItems = append(collectionItems, sdk.ForeignItem{
			Path: member.Path,
			CollectionMember: sdk.CollectionMember{
				Present:  true,
				Name:     member.Name,
				Position: member.Position,
			},
		})
	}
	byUser := make(map[string][]sdk.ForeignItem, len(accounts))
	for account, rows := range accounts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		items := make([]sdk.ForeignItem, 0, len(rows)+len(collectionItems))
		for _, row := range rows {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			item := sdk.ForeignItem{Path: row.Path}
			if row.HasWatchState {
				item = watchRowForeignItem(row.WatchRow)
			}
			if row.Rating != nil {
				item.Rating = sdk.Rating{Present: true, Value: *row.Rating, Scale: 10}
			}
			items = append(items, item)
		}
		byUser[account] = append(items, collectionItems...)
	}
	return byUser, nil
}

// NewPlexSource advertises the Plex importer's capabilities and reads watch
// state from the exported library database. Use NewPlexSourceForCategories
// when the resolver supplies a run's category selection.
func NewPlexSource(dbPath string) sdk.Source { //nolint:staticcheck // deliberate SDK 0.x compatibility path for pre-streaming consumers
	return newPlexSource(dbPath, nil)
}

// NewPlexSourceForCategories returns a Plex source scoped to the requested
// categories. Ratings and collections are available through ItemsBySourceUser.
func NewPlexSourceForCategories(dbPath string, requested []string) sdk.Source { //nolint:staticcheck // deliberate SDK 0.x compatibility path for pre-streaming consumers
	categories := make([]string, 0, 3)
	for _, category := range (plexSource{}).Categories() {
		if slices.Contains(requested, category) {
			categories = append(categories, category)
		}
	}
	return newPlexSource(dbPath, categories)
}

func newPlexSource(dbPath string, categories []string) plexSource {
	source := dbWatchSource{
		read:   foreignimport.ReadPlexContext,
		stream: foreignimport.StreamPlexWatchRows,
		dbPath: dbPath,
	}
	if categories != nil && !slices.Contains(categories, sdk.CategoryWatchState) {
		source.read = func(ctx context.Context, _ string) ([]foreignimport.WatchRow, error) {
			return nil, ctx.Err()
		}
		source.stream = func(ctx context.Context, _ string, _ func(foreignimport.WatchRow) error) error {
			return ctx.Err()
		}
	}
	return plexSource{dbWatchSource: source, categories: categories}
}

// NewEmbySource returns the Emby migration Source over an Emby `library.db`.
// (filmtv, "emby") → here.
func NewEmbySource(dbPath string) sdk.Source { //nolint:staticcheck // deliberate SDK 0.x compatibility path for pre-streaming consumers
	return dbWatchSource{
		read: foreignimport.ReadEmbyContext,
		stream: func(ctx context.Context, path string, visit func(foreignimport.WatchRow) error) error {
			return streamEmbyWatchRows(ctx, path, "emby", visit)
		},
		dbPath: dbPath,
	}
}

// NewJellyfinSource returns the Jellyfin migration Source over a Jellyfin
// `library.db` (the Emby sibling dialect). (filmtv, "jellyfin")
// → here.
func NewJellyfinSource(dbPath string) sdk.Source { //nolint:staticcheck // deliberate SDK 0.x compatibility path for pre-streaming consumers
	return dbWatchSource{
		read: foreignimport.ReadJellyfinContext,
		stream: func(ctx context.Context, path string, visit func(foreignimport.WatchRow) error) error {
			return streamEmbyWatchRows(ctx, path, "jellyfin", visit)
		},
		dbPath: dbPath,
	}
}

func streamEmbyWatchRows(
	ctx context.Context,
	path string,
	dialect string,
	visit func(foreignimport.WatchRow) error,
) error {
	db, err := openFilmTVReadOnly(path)
	if err != nil {
		return err
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
		  AND (ud.Played = 1 OR ud.PlaybackPositionTicks > 0)
		ORDER BY tbi.Path`)
	if err != nil {
		return fmt.Errorf("query %s watch-state: %w", dialect, err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return err
		}
		var file, lastPlayed string
		var played, positionTicks, runTimeTicks int64
		if err := rows.Scan(&file, &played, &positionTicks, &lastPlayed, &runTimeTicks); err != nil {
			return fmt.Errorf("scan %s row: %w", dialect, err)
		}
		row := foreignimport.WatchRow{
			Path:            file,
			Watched:         played == 1,
			PositionSeconds: positionTicks / 10_000_000,
			DurationSeconds: runTimeTicks / 10_000_000,
		}
		if parsed, err := time.Parse(time.RFC3339Nano, lastPlayed); err == nil && lastPlayed != "" {
			row.LastPlayed = parsed.UTC()
		}
		if err := visit(row); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate %s rows: %w", dialect, err)
	}
	return nil
}

func openFilmTVReadOnly(path string) (*sql.DB, error) {
	escaped := strings.NewReplacer("%", "%25", "?", "%3F", "#", "%23").Replace(path)
	db, err := sql.Open("sqlite", "file:"+escaped+"?mode=ro&immutable=1")
	if err != nil {
		return nil, fmt.Errorf("open foreign db %q: %w", path, err)
	}
	return db, nil
}

// nfoSource adapts an NFO sidecar tree to sdk.Source. An NFO library carries no
// watch-state (NFO = parse-hint ids + local artwork), so its
// declared category is "ratings"/"collections" parse-hint material rather than
// watch-state — but the v1 handler's apply path is watch-state only (the
// ratings/collections store projection is later work in Core).
// The NFO source therefore yields path-keyed items with no WatchState: the match
// chain still resolves and pins each path so the operator sees the match rate,
// and the parse-hint IDs ride into identification through the existing scan/
// identify path (NFO ids enter as Parse Hints), not through the
// migrate engagement write.
type nfoSource struct {
	root string
}

// Categories: the NFO source declares "collections" as a placeholder for the
// local-artwork material it carries — it is a real ImportCategories member
// (so the wizard validates it). The handler applies no
// engagement for it (watch-state only in v1), so an NFO run reports the match
// rate and pins matched files without writing playback history.
func (nfoSource) Categories() []string { return []string{sdk.CategoryCollections} }

func (s nfoSource) Items(ctx context.Context) ([]sdk.ForeignItem, error) {
	found, err := walkNFOTreeContext(ctx, s.root)
	if err != nil {
		return nil, err
	}
	items := make([]sdk.ForeignItem, 0, len(found))
	for _, it := range found {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		// Path-keyed only: NFO provider IDs are read by the filename parser
		// during identification, not carried through the migration source.
		items = append(items, sdk.ForeignItem{Path: it.Path})
	}
	return items, nil
}

func (s nfoSource) Stream(
	ctx context.Context,
	checkpoint string,
	emit func(context.Context, []sdk.ForeignItem, string) error,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	pager, err := newFilmTVPageEmitter(ctx, checkpoint, emit)
	if err != nil {
		return err
	}
	var callbackErr error
	err = filepath.WalkDir(s.root, func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErrOverride != nil {
			if injected := walkErrOverride(path); injected != nil {
				walkErr = injected
			}
		}
		if walkErr != nil {
			if path == s.root {
				return walkErr
			}
			return nil
		}
		if entry.IsDir() || !videoExts[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("nfo tree: resolve %q: %w", path, err)
		}
		callbackErr = pager.add(sdk.ForeignItem{Path: absolute})
		return callbackErr
	})
	if callbackErr != nil {
		return callbackErr
	}
	if err != nil {
		return fmt.Errorf("nfo tree walk %q: %w", s.root, err)
	}
	return pager.finish()
}

type filmTVPageEmitter struct {
	ctx    context.Context
	emit   func(context.Context, []sdk.ForeignItem, string) error
	resume int
	seen   int
	page   []sdk.ForeignItem
}

func newFilmTVPageEmitter(
	ctx context.Context,
	checkpoint string,
	emit func(context.Context, []sdk.ForeignItem, string) error,
) (*filmTVPageEmitter, error) {
	resume, err := parseImportCheckpoint(checkpoint)
	if err != nil {
		return nil, err
	}
	return &filmTVPageEmitter{
		ctx:    ctx,
		emit:   emit,
		resume: resume,
		page:   make([]sdk.ForeignItem, 0, filmTVImportPageItems),
	}, nil
}

func (p *filmTVPageEmitter) add(item sdk.ForeignItem) error {
	if err := p.ctx.Err(); err != nil {
		return err
	}
	p.seen++
	if p.seen <= p.resume {
		return nil
	}
	p.page = append(p.page, item)
	if len(p.page) < filmTVImportPageItems {
		return nil
	}
	return p.flush()
}

func (p *filmTVPageEmitter) finish() error {
	if err := p.ctx.Err(); err != nil {
		return err
	}
	if p.seen < p.resume {
		return fmt.Errorf("invalid Film/TV import checkpoint %d: beyond source", p.resume)
	}
	return p.flush()
}

func (p *filmTVPageEmitter) flush() error {
	if len(p.page) == 0 {
		return nil
	}
	if err := p.emit(p.ctx, p.page, strconv.Itoa(p.seen)); err != nil {
		return err
	}
	if err := p.ctx.Err(); err != nil {
		return err
	}
	p.page = make([]sdk.ForeignItem, 0, filmTVImportPageItems)
	return nil
}

func parseImportCheckpoint(checkpoint string) (int, error) {
	if checkpoint == "" {
		return 0, nil
	}
	index, err := strconv.Atoi(checkpoint)
	if err != nil || index < 0 {
		return 0, fmt.Errorf("invalid Film/TV import checkpoint %q", checkpoint)
	}
	return index, nil
}

// NewNFOSource returns the NFO migration Source over a sidecar library rooted at
// root. (filmtv, "nfo") → here.
func NewNFOSource(root string) sdk.Source { //nolint:staticcheck // deliberate SDK 0.x compatibility path for pre-streaming consumers
	return nfoSource{root: root}
}
