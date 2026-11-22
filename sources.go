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
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

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

// userItemsReader reads per-source-user items and library-wide collection
// memberships from a foreign database.
type userItemsReader func(
	ctx context.Context,
	dbPath string,
	options foreignimport.ReadOptions,
) (map[string][]foreignimport.UserItem, []foreignimport.CollectionMember, error)

// multiUserSource is a database source that also reads state per source user.
type multiUserSource struct {
	dbWatchSource
	// categories is the run's selection, or nil for every supported category.
	categories []string
	supported  []string
	readByUser userItemsReader
}

func (s multiUserSource) Categories() []string {
	if s.categories != nil {
		return slices.Clone(s.categories)
	}
	return slices.Clone(s.supported)
}

// ItemsBySourceUser supplies Core with account-specific playback state and ratings.
// Library-wide collection memberships are available under each source-user key.
func (s multiUserSource) ItemsBySourceUser(ctx context.Context) (map[string][]sdk.ForeignItem, error) {
	options := foreignimport.ReadOptions{
		WatchState:  s.categories == nil || slices.Contains(s.categories, sdk.CategoryWatchState),
		Ratings:     slices.Contains(s.categories, sdk.CategoryRatings),
		Collections: slices.Contains(s.categories, sdk.CategoryCollections),
	}
	accounts, memberships, err := s.readByUser(ctx, s.dbPath, options)
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

// scopeCategories keeps the supported categories a run requested, in supported
// order. A nil request means watch state.
func scopeCategories(supported, requested []string) []string {
	if requested == nil {
		requested = []string{sdk.CategoryWatchState}
	}
	categories := make([]string, 0, len(supported))
	for _, category := range supported {
		if slices.Contains(requested, category) {
			categories = append(categories, category)
		}
	}
	return categories
}

// newMultiUserSource builds a source over the given readers. When a selection
// leaves out watch state, Items and Stream return nothing.
func newMultiUserSource(
	dbPath string,
	categories, supported []string,
	read watchRowReader,
	stream watchRowStreamer,
	readByUser userItemsReader,
) multiUserSource {
	source := dbWatchSource{read: read, stream: stream, dbPath: dbPath}
	if categories != nil && !slices.Contains(categories, sdk.CategoryWatchState) {
		source.read = func(ctx context.Context, _ string) ([]foreignimport.WatchRow, error) {
			return nil, ctx.Err()
		}
		source.stream = func(ctx context.Context, _ string, _ func(foreignimport.WatchRow) error) error {
			return ctx.Err()
		}
	}
	return multiUserSource{
		dbWatchSource: source,
		categories:    categories,
		supported:     supported,
		readByUser:    readByUser,
	}
}

var (
	plexCategories     = []string{sdk.CategoryWatchState, sdk.CategoryRatings, sdk.CategoryCollections}
	jellyfinCategories = []string{sdk.CategoryWatchState, sdk.CategoryRatings, sdk.CategoryCollections}
	// Emby's API never stored a rating in the CORE-2678 observation, so Emby
	// advertises no ratings.
	embyCategories = []string{sdk.CategoryWatchState, sdk.CategoryCollections}
)

// NewPlexSource advertises the Plex importer's capabilities and reads watch
// state from the exported library database. Use NewPlexSourceForCategories
// when the resolver supplies a run's category selection.
func NewPlexSource(dbPath string) sdk.Source { //nolint:staticcheck // deliberate SDK 0.x compatibility path for pre-streaming consumers
	return newPlexSource(dbPath, nil)
}

// NewPlexSourceForCategories returns a Plex source scoped to the requested
// categories. A nil selection defaults to watch state. A non-nil empty selection
// returns no items. Ratings and collections are available through ItemsBySourceUser.
func NewPlexSourceForCategories(dbPath string, requested []string) sdk.Source { //nolint:staticcheck // deliberate SDK 0.x compatibility path for pre-streaming consumers
	return newPlexSource(dbPath, scopeCategories(plexCategories, requested))
}

func newPlexSource(dbPath string, categories []string) multiUserSource {
	return newMultiUserSource(dbPath, categories, plexCategories,
		foreignimport.ReadPlexContext, foreignimport.StreamPlexWatchRows, foreignimport.ReadPlexByAccount)
}

// NewEmbySource returns the Emby migration Source over an Emby `library.db`,
// in the 4.10 layout or the older one. Per-user state and 4.10 collections
// come through ItemsBySourceUser, with user names from the `users.db` beside
// it. (filmtv, "emby") → here.
func NewEmbySource(dbPath string) sdk.Source { //nolint:staticcheck // deliberate SDK 0.x compatibility path for pre-streaming consumers
	return newEmbySource(dbPath, nil)
}

// NewEmbySourceForCategories returns an Emby source scoped to the requested
// categories, as NewPlexSourceForCategories does.
func NewEmbySourceForCategories(dbPath string, requested []string) sdk.Source { //nolint:staticcheck // deliberate SDK 0.x compatibility path for pre-streaming consumers
	return newEmbySource(dbPath, scopeCategories(embyCategories, requested))
}

func newEmbySource(dbPath string, categories []string) multiUserSource {
	return newMultiUserSource(dbPath, categories, embyCategories,
		foreignimport.ReadEmbyContext, foreignimport.StreamEmbyWatchRows, foreignimport.ReadEmbyByUser)
}

// NewJellyfinSource returns the Jellyfin migration Source over a Jellyfin 12
// `jellyfin.db`. A pre-12 `library.db` is refused with an error that tells the
// operator to upgrade Jellyfin first. Per-user state, ratings and BoxSet
// collections come through ItemsBySourceUser. (filmtv, "jellyfin") → here.
func NewJellyfinSource(dbPath string) sdk.Source { //nolint:staticcheck // deliberate SDK 0.x compatibility path for pre-streaming consumers
	return newJellyfinSource(dbPath, nil)
}

// NewJellyfinSourceForCategories returns a Jellyfin source scoped to the
// requested categories, as NewPlexSourceForCategories does.
func NewJellyfinSourceForCategories(dbPath string, requested []string) sdk.Source { //nolint:staticcheck // deliberate SDK 0.x compatibility path for pre-streaming consumers
	return newJellyfinSource(dbPath, scopeCategories(jellyfinCategories, requested))
}

func newJellyfinSource(dbPath string, categories []string) multiUserSource {
	return newMultiUserSource(dbPath, categories, jellyfinCategories,
		foreignimport.ReadJellyfinContext, foreignimport.StreamJellyfinWatchRows, foreignimport.ReadJellyfinByUser)
}

// nfoSource adapts an NFO sidecar tree to sdk.Source. Each video file is a
// path-keyed item; when the run selected watch state and the file's NFO
// records a play (<playcount>, <watched>, <lastplayed>) the item carries that
// as watch-state, which the handler writes for the importing profile only.
// NFO provider IDs ride into identification through the scan/identify path
// (Parse Hints) and the NFO metadata through the local hydration seam
// (nfohydration.go), not through this source.
type nfoSource struct {
	root       string
	categories []string
}

// nfoCategories: watch-state from the NFO play record, plus "collections" for
// the local-artwork material the tree carries.
var nfoCategories = []string{sdk.CategoryWatchState, sdk.CategoryCollections}

// Categories are the run's selected categories the NFO tree carries.
func (s nfoSource) Categories() []string {
	return slices.Clone(s.categories)
}

// item keys a video file by path and adds the play its own NFO records when
// the run selected watch state. An NFO that exists but cannot be read fails
// the item, so a checkpoint never moves past a play it did not read.
func (s nfoSource) item(path string) (sdk.ForeignItem, error) {
	if !slices.Contains(s.categories, sdk.CategoryWatchState) {
		return sdk.ForeignItem{Path: path}, nil
	}
	state, err := nfoWatchState(path)
	if err != nil {
		return sdk.ForeignItem{}, fmt.Errorf("nfo tree: %w", err)
	}
	return sdk.ForeignItem{Path: path, WatchState: state}, nil
}

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
		item, err := s.item(it.Path)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
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
		callbackErr = pager.addFunc(func() (sdk.ForeignItem, error) { return s.item(absolute) })
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
	return p.addFunc(func() (sdk.ForeignItem, error) { return item, nil })
}

// addFunc adds the item build returns, calling build only for an item past
// the resume checkpoint, so a resumed run does not re-read what it skips.
func (p *filmTVPageEmitter) addFunc(build func() (sdk.ForeignItem, error)) error {
	if err := p.ctx.Err(); err != nil {
		return err
	}
	p.seen++
	if p.seen <= p.resume {
		return nil
	}
	item, err := build()
	if err != nil {
		return err
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
	return nfoSource{root: root, categories: slices.Clone(nfoCategories)}
}

// NewNFOSourceForCategories returns an NFO source scoped to the run's
// requested categories, as NewPlexSourceForCategories does: a nil selection
// defaults to watch state, and a selection without watch state emits items
// with none.
func NewNFOSourceForCategories(root string, requested []string) sdk.Source { //nolint:staticcheck // deliberate SDK 0.x compatibility path for pre-streaming consumers
	return nfoSource{root: root, categories: scopeCategories(nfoCategories, requested)}
}
