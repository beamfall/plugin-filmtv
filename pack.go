package filmtv

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Offline pack target kinds for the Film & TV plugin. Core registers them into
// its offline pack service through an adapter, never by name.
const (
	// PackKindSeason packs one season's episodes in episode order. The ref is
	// the season Entity ID.
	PackKindSeason = "filmtv-season"
	// PackKindSeries packs every season of a show: numbered seasons in order,
	// then specials. The ref is the show Entity ID.
	PackKindSeries = "filmtv-series"
	// PackKindNextUnwatched packs the next N unwatched episodes of a show for
	// the requesting Profile. The ref is "<show Entity ID>:<N>".
	PackKindNextUnwatched = "filmtv-next-unwatched"
)

// MaxNextUnwatched bounds N in a PackKindNextUnwatched ref.
const MaxNextUnwatched = 50

const (
	seasonKind = "season"
	// seasonNumberField and seasonSpecialsField are the season fields declared
	// in bundle.go.
	seasonNumberField   = "number"
	seasonSpecialsField = "specials"
)

// PackResolved is a resolver's output: the ordered Media IDs that belong in the
// pack and a display label. Core re-checks visibility before freezing them.
type PackResolved struct {
	MediaIDs []string
	Label    string
}

// PackEntity is one Entity as the pack reader returns it: its ID plus its
// number and flag fields by declared field name.
type PackEntity struct {
	ID      string
	Numbers map[string]float64
	Flags   map[string]bool
}

// packGroup is a season read from a PackEntity.
type packGroup struct {
	id        string
	number    float64
	hasNumber bool
	specials  bool
}

func groupOf(entity PackEntity) packGroup {
	group := packGroup{id: entity.ID, specials: entity.Flags[seasonSpecialsField]}
	group.number, group.hasNumber = entity.Numbers[seasonNumberField]
	return group
}

// PackMediaReader is the store view the resolvers read. Core builds one per
// resolve, bound to the caller's Library filter and Profile, so every read is
// already visibility-filtered.
type PackMediaReader interface {
	// GroupMembers returns the visible Media of a visible container Entity in
	// its stored order.
	GroupMembers(ctx context.Context, entityRef string) ([]string, error)
	// EntitiesOf returns the visible Entities of kind that point at parentRef
	// through relation. parentRef itself must be visible, or EntitiesOf errors.
	EntitiesOf(ctx context.Context, parentRef, kind, relation string) ([]PackEntity, error)
	// Watched reports which of ids the requesting Profile has finished.
	Watched(ctx context.Context, ids []string) (map[string]bool, error)
}

// PackResolver translates a target ref into the Media that belong in a pack.
type PackResolver interface {
	Resolve(ctx context.Context, targetRef string, r PackMediaReader) (PackResolved, error)
}

// PackResolvers returns the resolver for each Film & TV pack kind.
func PackResolvers() map[string]PackResolver {
	return map[string]PackResolver{
		PackKindSeason:        seasonPackResolver{},
		PackKindSeries:        seriesPackResolver{},
		PackKindNextUnwatched: nextUnwatchedPackResolver{},
	}
}

type seasonPackResolver struct{}

func (seasonPackResolver) Resolve(ctx context.Context, ref string, r PackMediaReader) (PackResolved, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return PackResolved{}, errors.New("filmtv: season pack needs a season ref")
	}
	ids, err := r.GroupMembers(ctx, ref)
	if err != nil {
		return PackResolved{}, err
	}
	return PackResolved{MediaIDs: dedupe(ids), Label: "Season"}, nil
}

type seriesPackResolver struct{}

func (seriesPackResolver) Resolve(ctx context.Context, ref string, r PackMediaReader) (PackResolved, error) {
	ids, err := seriesMedia(ctx, strings.TrimSpace(ref), r, true)
	if err != nil {
		return PackResolved{}, err
	}
	return PackResolved{MediaIDs: ids, Label: "Series"}, nil
}

type nextUnwatchedPackResolver struct{}

func (nextUnwatchedPackResolver) Resolve(ctx context.Context, ref string, r PackMediaReader) (PackResolved, error) {
	showRef, count, err := parseNextUnwatchedRef(ref)
	if err != nil {
		return PackResolved{}, err
	}
	ids, err := seriesMedia(ctx, showRef, r, false)
	if err != nil {
		return PackResolved{}, err
	}
	watched, err := r.Watched(ctx, ids)
	if err != nil {
		return PackResolved{}, err
	}
	return PackResolved{MediaIDs: nextUnwatched(ids, watched, count), Label: "Up Next"}, nil
}

// seriesMedia lists a show's episodes in viewing order: numbered seasons
// ascending, then unnumbered seasons, then specials when includeSpecials.
func seriesMedia(ctx context.Context, showRef string, r PackMediaReader, includeSpecials bool) ([]string, error) {
	if showRef == "" {
		return nil, errors.New("filmtv: series pack needs a show ref")
	}
	entities, err := r.EntitiesOf(ctx, showRef, seasonKind, SeasonOfRelation)
	if err != nil {
		return nil, err
	}
	groups := make([]packGroup, len(entities))
	for i, entity := range entities {
		groups[i] = groupOf(entity)
	}
	sortGroups(groups)
	var ids []string
	for _, group := range groups {
		if group.isSpecials() && !includeSpecials {
			continue
		}
		members, err := r.GroupMembers(ctx, group.id)
		if err != nil {
			return nil, err
		}
		ids = append(ids, members...)
	}
	return dedupe(ids), nil
}

func (g packGroup) isSpecials() bool {
	return g.specials || (g.hasNumber && g.number == 0)
}

// sortGroups orders seasons for viewing, keeping the reader's order among
// equals so the result is deterministic.
func sortGroups(groups []packGroup) {
	rank := func(g packGroup) int {
		switch {
		case g.isSpecials():
			return 2
		case g.hasNumber:
			return 0
		default:
			return 1
		}
	}
	sort.SliceStable(groups, func(i, j int) bool {
		ri, rj := rank(groups[i]), rank(groups[j])
		if ri != rj {
			return ri < rj
		}
		return ri == 0 && groups[i].number < groups[j].number
	})
}

// nextUnwatched returns up to count episodes after the last finished one,
// skipping any finished later. A partly watched episode counts as unwatched.
func nextUnwatched(ids []string, watched map[string]bool, count int) []string {
	start := 0
	for i, id := range ids {
		if watched[id] {
			start = i + 1
		}
	}
	out := make([]string, 0, count)
	for _, id := range ids[start:] {
		if len(out) == count {
			break
		}
		if !watched[id] {
			out = append(out, id)
		}
	}
	return out
}

func parseNextUnwatchedRef(ref string) (string, int, error) {
	ref = strings.TrimSpace(ref)
	cut := strings.LastIndexByte(ref, ':')
	if cut <= 0 {
		return "", 0, errors.New("filmtv: next-unwatched pack ref must be <show>:<count>")
	}
	count, err := strconv.Atoi(ref[cut+1:])
	if err != nil || count < 1 || count > MaxNextUnwatched {
		return "", 0, fmt.Errorf("filmtv: next-unwatched count must be 1 to %d", MaxNextUnwatched)
	}
	return ref[:cut], count, nil
}

func dedupe(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
