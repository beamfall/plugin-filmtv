package foreignimport

import (
	"context"
	"database/sql"
	"fmt"
)

// jellyfinBoxSetType is the BaseItems.Type of a Jellyfin collection, observed
// in the official 12.4 image (CORE-2678 evidence).
const jellyfinBoxSetType = "MediaBrowser.Controller.Entities.Movies.BoxSet"

// The per-user item query, with and without the rating column. The order
// groups each entry's copies together and is total.
const (
	jellyfinUserItemsHead = `
		SELECT COALESCE(us.Username, '') || '#' || COALESCE(us.Id, u.UserId),
		       b.Path, u.UserId, u.ItemId,
		       COALESCE(u.Played, 0), COALESCE(u.PlaybackPositionTicks, 0),
		       COALESCE(u.LastPlayedDate, ''), CAST(COALESCE(b.RunTimeTicks, 0) AS TEXT), `
	jellyfinUserItemsTail = `
		FROM UserData u
		JOIN BaseItems b ON b.Id = u.ItemId
		LEFT JOIN Users us ON us.Id = u.UserId COLLATE NOCASE
		WHERE b.Path IS NOT NULL AND b.Path <> '' AND COALESCE(b.IsFolder, 0) = 0
		ORDER BY u.UserId COLLATE BINARY, b.Path COLLATE BINARY,
		         u.ItemId COLLATE BINARY, u.CustomDataKey COLLATE BINARY`
	jellyfinUserItemsQuery      = jellyfinUserItemsHead + "NULL" + jellyfinUserItemsTail
	jellyfinRatedUserItemsQuery = jellyfinUserItemsHead + "u.Rating" + jellyfinUserItemsTail
)

// ReadJellyfinByUser returns Jellyfin 12 user state keyed by source user
// (`Username#Id`) and the BoxSet memberships shared by every user. Every user
// in the Users table is a key, even one with no state.
//
// Watch state is chosen per entry as StreamJellyfinWatchRows chooses it. A
// rating (0 to 10) comes from the best copy that has one, ranked the same way,
// so a rated-only item carries a rating and no watch state. Favourites are not
// read (CORE-2895).
func ReadJellyfinByUser(ctx context.Context, path string, options ReadOptions) (map[string][]UserItem, []CollectionMember, error) {
	db, err := openReadOnly(path)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = db.Close() }()
	extra := [][]tableColumns{jellyfinUserColumns}
	if options.Ratings {
		extra = append(extra, jellyfinRatingColumns)
	}
	if options.Collections {
		extra = append(extra, jellyfinCollectionColumns)
	}
	if err := checkJellyfinSchema(ctx, db, extra...); err != nil {
		return nil, nil, err
	}
	users, err := readJellyfinUserItems(ctx, db, options)
	if err != nil {
		return nil, nil, err
	}
	if !options.Collections {
		return users, nil, nil
	}
	members, err := readJellyfinCollections(ctx, db)
	if err != nil {
		return nil, nil, err
	}
	return users, members, nil
}

func readJellyfinUserKeys(ctx context.Context, db *sql.DB) (map[string][]UserItem, error) {
	rows, err := db.QueryContext(ctx, `SELECT COALESCE(Username, '') || '#' || Id FROM Users`)
	if err != nil {
		return nil, fmt.Errorf("query jellyfin users: %w", err)
	}
	defer func() { _ = rows.Close() }()
	users := make(map[string][]UserItem)
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("scan jellyfin user: %w", err)
		}
		users[key] = nil
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jellyfin users: %w", err)
	}
	return users, nil
}

func readJellyfinUserItems(ctx context.Context, db *sql.DB, options ReadOptions) (map[string][]UserItem, error) {
	users, err := readJellyfinUserKeys(ctx, db)
	if err != nil || (!options.WatchState && !options.Ratings) {
		return users, err
	}
	query := jellyfinUserItemsQuery
	if options.Ratings {
		query = jellyfinRatedUserItemsQuery
	}
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query jellyfin user state: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var (
		key, bestKey string
		best, rated  jellyfinCopy
		rating       *float64
		pending      bool
	)
	emit := func() error {
		if !pending {
			return nil
		}
		item := UserItem{WatchRow: WatchRow{Path: best.path}, Rating: rating}
		if options.WatchState && best.hasWatchState() {
			row, err := best.watchRow()
			if err != nil {
				return err
			}
			item.WatchRow, item.HasWatchState = row, true
		}
		if item.HasWatchState || item.Rating != nil {
			users[bestKey] = append(users[bestKey], item)
		}
		return nil
	}
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var (
			c          jellyfinCopy
			played     int64
			lastPlayed string
			copyRating *float64
		)
		if err := rows.Scan(&key, &c.path, &c.userID, &c.itemID, &played, &c.positionTicks,
			&lastPlayed, &c.runTimeTicks, &copyRating); err != nil {
			return nil, fmt.Errorf("scan jellyfin user state: %w", err)
		}
		c.setScanned(played, lastPlayed)
		if pending && c.sameEntry(best) {
			if c.beats(best) {
				best = c
			}
			if copyRating != nil && (rating == nil || c.beats(rated)) {
				rated, rating = c, copyRating
			}
			continue
		}
		if err := emit(); err != nil {
			return nil, err
		}
		best, bestKey, pending = c, key, true
		rated, rating = c, copyRating
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jellyfin user state: %w", err)
	}
	if err := emit(); err != nil {
		return nil, err
	}
	return users, nil
}

// readJellyfinCollections returns BoxSet memberships in file path order. The
// position is the member's LinkedChildren.SortOrder. Folder and pathless
// members are skipped.
func readJellyfinCollections(ctx context.Context, db *sql.DB) ([]CollectionMember, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT b.Path, bs.Name, lc.SortOrder
		FROM LinkedChildren lc
		JOIN BaseItems bs ON bs.Id = lc.ParentId
		JOIN BaseItems b ON b.Id = lc.ChildId
		WHERE bs.Type = ? AND bs.Name IS NOT NULL AND bs.Name <> ''
		  AND b.Path IS NOT NULL AND b.Path <> '' AND COALESCE(b.IsFolder, 0) = 0
		ORDER BY b.Path COLLATE BINARY, bs.Id COLLATE BINARY, lc.SortOrder`, jellyfinBoxSetType)
	if err != nil {
		return nil, fmt.Errorf("query jellyfin collections: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var members []CollectionMember
	for rows.Next() {
		var member CollectionMember
		if err := rows.Scan(&member.Path, &member.Name, &member.Position); err != nil {
			return nil, fmt.Errorf("scan jellyfin collection membership: %w", err)
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jellyfin collection memberships: %w", err)
	}
	return members, nil
}
