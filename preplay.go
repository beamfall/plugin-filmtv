package filmtv

import (
	"context"
	"errors"
	"math/rand/v2"
	"slices"
	"strings"
)

// PrePlayMaxTrailers caps how many trailers one plan holds, whatever the
// request asks for.
const PrePlayMaxTrailers = 10

// PrePlayLabel is the display label Core shows on a trailer the plan picked.
const PrePlayLabel = "Trailer"

const filmClass = "movie"

// prePlayExtraFolders are the extras folder names Core's scanner recognises.
// The nearest one above a file decides what kind of Extra it is.
var prePlayExtraFolders = map[string]bool{
	"extras":      true,
	"featurettes": true,
	"trailers":    true,
	"interviews":  true,
	"shorts":      true,
}

// PrePlayExtra is one Extra Core found in the feature's Library: the Extra's
// Media ID, the Media it is an Extra of and the Extra's file path.
type PrePlayExtra struct {
	MediaID  string
	ParentID string
	Path     string
}

// PrePlayReader is the store view Core implements for the pre-play policy.
// Every read honours the requesting Profile's Library visibility: hidden
// Media are left out of the results.
type PrePlayReader interface {
	// MediaClasses returns the `class` field of each visible Media in ids.
	// Media that are hidden or have no class are absent from the map.
	MediaClasses(ctx context.Context, ids []string) (map[string]string, error)
	// Extras returns the visible Extras in the feature's Library.
	Extras(ctx context.Context, featureID string) ([]PrePlayExtra, error)
	// Watched reports which of ids the Profile has finished.
	Watched(ctx context.Context, ids []string) (map[string]bool, error)
}

// PrePlayRequest asks for a plan before FeatureID plays. Count is how many
// trailers the Profile wants; Seed fixes the shuffle.
type PrePlayRequest struct {
	FeatureID string
	Count     int
	Seed      uint64
}

// PrePlayPlan is the policy's answer. Eligible reports whether the feature
// gets pre-play at all (Core's clips included); MediaIDs are the trailers to
// play, in order, each shown with Label.
type PrePlayPlan struct {
	Eligible bool
	MediaIDs []string
	Label    string
}

// PlanPrePlay decides whether a feature gets cinema-mode pre-play and which
// trailers play before it. Only a film is eligible. Trailers come from the
// feature's Library, one per other film, unwatched films first, in an order
// seeded by the request.
func PlanPrePlay(ctx context.Context, reader PrePlayReader, req PrePlayRequest) (PrePlayPlan, error) {
	if reader == nil {
		return PrePlayPlan{}, errors.New("filmtv: pre-play reader is nil")
	}
	feature := strings.TrimSpace(req.FeatureID)
	if feature == "" {
		return PrePlayPlan{}, errors.New("filmtv: pre-play feature id is empty")
	}
	classes, err := reader.MediaClasses(ctx, []string{feature})
	if err != nil {
		return PrePlayPlan{}, err
	}
	if classes[feature] != filmClass {
		return PrePlayPlan{}, nil
	}
	plan := PrePlayPlan{Eligible: true, Label: PrePlayLabel}
	count := min(max(req.Count, 0), PrePlayMaxTrailers)
	if count == 0 {
		return plan, nil
	}
	byParent, err := trailersByParent(ctx, reader, feature)
	if err != nil {
		return PrePlayPlan{}, err
	}
	if len(byParent) == 0 {
		return plan, nil
	}
	parents := make([]string, 0, len(byParent))
	for parent := range byParent {
		parents = append(parents, parent)
	}
	slices.Sort(parents)
	watched, err := reader.Watched(ctx, parents)
	if err != nil {
		return PrePlayPlan{}, err
	}
	var fresh, seen []string
	for _, parent := range parents {
		if watched[parent] {
			seen = append(seen, parent)
		} else {
			fresh = append(fresh, parent)
		}
	}
	rng := rand.New(rand.NewPCG(req.Seed, req.Seed^0x9e3779b97f4a7c15))
	shuffle(rng, fresh)
	shuffle(rng, seen)
	for _, parent := range append(fresh, seen...) {
		if len(plan.MediaIDs) == count {
			break
		}
		plan.MediaIDs = append(plan.MediaIDs, byParent[parent])
	}
	return plan, nil
}

// trailersByParent maps each other film in the feature's Library to one of
// its trailers, the lowest Media ID when it has several.
func trailersByParent(ctx context.Context, reader PrePlayReader, feature string) (map[string]string, error) {
	extras, err := reader.Extras(ctx, feature)
	if err != nil {
		return nil, err
	}
	candidates := map[string]string{}
	var parentIDs []string
	for _, extra := range extras {
		if extra.MediaID == "" || extra.ParentID == "" || extra.ParentID == feature ||
			extra.MediaID == feature || !IsTrailerPath(extra.Path) {
			continue
		}
		current, ok := candidates[extra.ParentID]
		if !ok {
			parentIDs = append(parentIDs, extra.ParentID)
		}
		if !ok || extra.MediaID < current {
			candidates[extra.ParentID] = extra.MediaID
		}
	}
	if len(parentIDs) == 0 {
		return nil, nil
	}
	slices.Sort(parentIDs)
	classes, err := reader.MediaClasses(ctx, parentIDs)
	if err != nil {
		return nil, err
	}
	for parent := range candidates {
		if classes[parent] != filmClass {
			delete(candidates, parent)
		}
	}
	return candidates, nil
}

// IsTrailerPath reports whether an Extra's path marks it as a trailer: its
// filename stem ends in "-trailer", or its nearest extras folder is named
// "trailers". Both checks ignore case and accept either path separator.
func IsTrailerPath(path string) bool {
	parts := strings.Split(strings.ReplaceAll(strings.ToLower(path), `\`, "/"), "/")
	name := parts[len(parts)-1]
	if dot := strings.LastIndexByte(name, '.'); dot > 0 {
		name = name[:dot]
	}
	if strings.HasSuffix(name, "-trailer") {
		return true
	}
	for i := len(parts) - 2; i >= 0; i-- {
		if prePlayExtraFolders[parts[i]] {
			return parts[i] == "trailers"
		}
	}
	return false
}

func shuffle(rng *rand.Rand, ids []string) {
	rng.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
}
