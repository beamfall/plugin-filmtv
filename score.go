package filmtv

import (
	"strings"
	"unicode"

	sdk "github.com/beamfall/plugin-sdk"
)

// AutoApplyThreshold is the Film & TV plugin's match-confidence cut for
// unattended identification: a candidate scoring at or above it
// auto-applies, below it queues for manual review. The cut encodes a
// fail-closed asymmetry — "wrong auto-application is worse than no match" — so
// the model biases toward Unmatched: 0.75 demands either an exact title or a
// strong title backed by a corroborating year. A merely-similar title with no
// year to disambiguate is held below this cut (see noCorroborationPenalty) so a
// generic single candidate (e.g. "Crash", "Gladiator") routes to the Unmatched
// queue rather than silently mislabelling a file. The composition root reads
// this when it registers the scorer with identify.
const AutoApplyThreshold = 0.75

// noCorroborationPenalty is subtracted from a non-exact title score when the
// filename carried no year to corroborate it. It is sized so the strongest
// non-exact title signal — a containment match (0.85) — lands at 0.60: still
// above the corpus resolution floor (sdktest.DefaultMatchThreshold, 0.5) so the
// candidate can still rank and resolve, but below AutoApplyThreshold (0.75) so
// it will not auto-apply without a human. Fixed subtraction preserves the
// relative ordering of no-year candidates. An exact title (1.0) is exempt: a
// fully-specified unique title is safe to auto-apply alone, and a genuine
// two-identical-title ambiguity is caught by the pipeline's fail-closed tie
// rule (identify.bestCandidate refuses to auto-apply a top-score tie), not the
// scorer.
const noCorroborationPenalty = 0.25

// episodeMissingPenalty is subtracted when the parse carries season/episode
// ordinals and the candidate carries only one of the two. It is deliberately
// tiny.
const episodeMissingPenalty = 0.05

// showLevelPenalty is subtracted when the parse carries season/episode
// ordinals and the candidate carries neither (every TV search result). The
// show-level candidate makes no claim about the episode to disagree with, so
// the penalty only orders it below a candidate whose ordinals agree, and
// never takes an exact title below AutoApplyThreshold.
const showLevelPenalty = 0.01

// regionFoldScore is the title similarity of a series title that differs from
// the candidate's only by a country qualifier on one side ("The Traitors UK"
// vs "The Traitors"). It is not a unique match: a candidate carries no origin
// country, so it cannot tell the national versions sharing the base title
// apart ("The Office" UK and US), and without a year it stays below
// AutoApplyThreshold. A candidate whose own title carries the same qualifier
// scores 1.0 and outranks it.
const regionFoldScore = 0.9

// episodeMismatchPenalty is subtracted when the parse and candidate both carry
// season/episode ordinals and they disagree — a strong wrong-episode signal
// that must not auto-apply. Sized to sink a full title+year match (1.0) below
// AutoApplyThreshold (0.75) so a mislabelled episode routes to the Unmatched
// queue rather than silently attaching to the wrong episode.
const episodeMismatchPenalty = 0.4

// Scorer scores how confidently an identification candidate matches a file's
// parsed Film & TV guess. It is the match-confidence half of the plugin's two
// compiled hooks, sibling to Parser. It is keyed by
// PluginID at the composition root, which adapts it to the domain-agnostic
// identify.MatchScorer seam (identify cannot import scan, so the root maps the
// scan-side ParsedMedia onto identify's twin and feeds candidate fields here).
type Scorer struct{}

// NewScorer returns a ready Scorer.
func NewScorer() *Scorer { return &Scorer{} }

// Score weighs one candidate against the parsed guess and returns a confidence
// in [0,1]. The model is deliberately simple for v1:
//
//   - Title similarity (weight 0.7) dominates — an exact normalized match scores
//     1.0, an episode's series title equal but for a country qualifier on one
//     side regionFoldScore, a containment (one title inside the other) 0.85, otherwise a
//     token-overlap (Jaccard) ratio. A blank parsed or candidate title is a
//     hard zero — nothing to match on.
//   - Year agreement (weight 0.3) corroborates: an exact year match adds the
//     full weight, ±1 (release-vs-air-year drift) adds half, a conflicting year
//     adds nothing.
//   - Season/episode agreement breaks same-title episode ambiguity: a matching
//     pair keeps the title/year score; a show-level candidate that carries
//     no ordinals takes the tiny showLevelPenalty, a candidate carrying only
//     one the small episodeMissingPenalty, and a disagreeing pair the large
//     episodeMismatchPenalty fail-closed hit.
//   - TMDB popularity is not part of this scalar score. Core's ranking layer
//     sees the candidate set and uses popularity only to break exact score ties.
//
// When the parse carried no year there is nothing to corroborate the title, so
// the fail-closed asymmetry applies: an *exact* title still scores
// 1.0 (safe to auto-apply alone), but a *non-exact* title, a country-folded
// one included, is docked
// noCorroborationPenalty so it cannot clear AutoApplyThreshold on title
// similarity alone — a mediocre single candidate with no year disambiguation
// routes to Unmatched rather than auto-applying. A genuine two-identical-title
// ambiguity is caught upstream by identify.bestCandidate's tie rule.
//
// Candidate.Year/Season/Episode are 0 when the candidate carried none, matching
// the sdk convention.
func (Scorer) Score(candidate sdk.Candidate, parsed sdk.ParsedMedia) float64 {
	title, unique := titleScore(parsed, candidate.Title)
	if title == 0 {
		return 0
	}
	base := baseScore(title, unique, parsed, candidate)
	penalized := base - episodePenalty(parsed, candidate)
	return clampUnit(penalized)
}

// baseScore is the title+year confidence before the season/episode and
// popularity adjustments: the no-year fail-closed verdict when the parse
// carried no year, else the weighted title+year blend.
func baseScore(title float64, unique bool, parsed sdk.ParsedMedia, candidate sdk.Candidate) float64 {
	if parsed.Year == 0 {
		return noYearScore(title, unique)
	}
	const titleWeight, yearWeight = 0.7, 0.3
	return titleWeight*title + yearWeight*yearScore(parsed.Year, candidate.Year)
}

// noYearScore is the fail-closed verdict when the filename carried no year: an
// exact title (1.0), or a series title whose country qualifier agrees,
// passes through — a unique title needs no corroboration — while any weaker
// title is docked noCorroborationPenalty (floored at 0) so it falls below
// AutoApplyThreshold and defers to manual review.
func noYearScore(title float64, unique bool) float64 {
	if title == 1 || unique {
		return title
	}
	if docked := title - noCorroborationPenalty; docked > 0 {
		return docked
	}
	return 0
}

// episodePenalty returns the ordinal penalty for the parsed/candidate pair.
// The parse carries season in Sequence (decimal-compatible text) and
// episode in Index; the candidate carries them as integers.
func episodePenalty(parsed sdk.ParsedMedia, candidate sdk.Candidate) float64 {
	parsedSeason := atoiSequence(parsed.Sequence)
	if parsedSeason == 0 || parsed.Index == 0 {
		return 0
	}
	carriesSeason := candidate.Sequence != 0 || candidate.SequencePresent
	carriesEpisode := candidate.Index != 0 || candidate.IndexPresent
	if !carriesSeason && !carriesEpisode {
		return showLevelPenalty
	}
	if candidate.Sequence == 0 || candidate.Index == 0 {
		return episodeMissingPenalty
	}
	if parsedSeason == candidate.Sequence && parsed.Index == candidate.Index {
		return 0
	}
	return episodeMismatchPenalty
}

// atoiSequence parses a ParsedMedia.Sequence season token to its integer part,
// returning 0 for empty or unparseable text (the non-signal case).
func atoiSequence(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// clampUnit pins a score to the [0,1] contract range.
func clampUnit(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}

// titleScore is the normalized title similarity in [0,1]: exact match 1.0,
// an episode's series title equal but for a country qualifier
// regionFoldScore (1.0 when both carry the same one), containment 0.85, else
// Jaccard token overlap. Either title blank yields 0. unique reports a match
// that needs no year to auto-apply.
func titleScore(parsed sdk.ParsedMedia, candTitle string) (score float64, unique bool) {
	a := normalizeTitle(parsed.Title)
	b := normalizeTitle(candTitle)
	if a == "" || b == "" {
		return 0, false
	}
	if a == b {
		return 1, true
	}
	if parsed.Class == "episode" {
		aBase, aRegion := splitRegion(parsed.Title, true)
		bBase, bRegion := splitRegion(candTitle, false)
		if aBase == bBase {
			switch {
			case aRegion == bRegion:
				return 1, true
			case aRegion == "" || bRegion == "":
				return regionFoldScore, false
			}
		}
	}
	if strings.Contains(a, b) || strings.Contains(b, a) {
		return 0.85, false
	}
	return jaccard(strings.Fields(a), strings.Fields(b)), false
}

// regionAliases maps the trailing country qualifiers a series title carries
// to tell national versions apart ("The Office US", "The Traitors (UK)") to
// one spelling.
var regionAliases = map[string]string{
	"us": "us", "usa": "us", "uk": "uk", "gb": "uk", "au": "au", "nz": "nz", "ca": "ca", "ie": "ie",
}

// splitRegion splits a title into its normalized base and a trailing country
// qualifier, or returns the normalized title and "" when it carries none. A
// qualifier is a parenthesised alias ("The Office (US)"); with bareCaps, as a
// filename writes it, also a bare all-capitals one ("The.Traitors.UK"). A
// lower-case word is never one: "This Is Us" ends on a word, not a country.
// A title that is only the qualifier keeps it.
func splitRegion(title string, bareCaps bool) (string, string) {
	title = strings.TrimSpace(title)
	cut, token := -1, ""
	if strings.HasSuffix(title, ")") {
		if open := strings.LastIndexByte(title, '('); open >= 0 {
			cut, token = open, strings.TrimSpace(title[open+1:len(title)-1])
		}
	} else if bareCaps {
		if sep := strings.LastIndexAny(title, " ._-"); sep >= 0 && title[sep+1:] == strings.ToUpper(title[sep+1:]) {
			cut, token = sep, title[sep+1:]
		}
	}
	region, ok := regionAliases[strings.ToLower(token)]
	base := normalizeTitle(title[:max(cut, 0)])
	if !ok || base == "" {
		return normalizeTitle(title), ""
	}
	return base, region
}

// yearScore corroborates the title: exact 1.0, ±1 year (release-vs-air drift)
// 0.5, otherwise 0. A zero candidate year (none parsed) is a non-signal, not a
// conflict — it scores 0.5 so a strong title still clears with a missing year.
func yearScore(parsedYear, candYear int) float64 {
	if candYear == 0 {
		return 0.5
	}
	switch diff := parsedYear - candYear; diff {
	case 0:
		return 1
	case 1, -1:
		return 0.5
	default:
		return 0
	}
}

// normalizeTitle lower-cases and drops apostrophes, strips other punctuation to
// spaces, spells "&" as "and", joins a lone "s" to the word before it, then
// collapses whitespace — so "The Matrix!" and "the matrix" compare equal.
func normalizeTitle(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToLower(s) {
		switch {
		case r == '&':
			// "Key & Peele" and "Key and Peele" are one title.
			b.WriteString(" and ")
		case isApostrophe(r):
			// An apostrophe joins its word: release names drop it ("A Mothers
			// Son") where the provider keeps it ("A Mother's Son"). Checked
			// first: the modifier letter apostrophe counts as a letter.
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			b.WriteRune(' ')
		}
	}
	// A release name that turned the apostrophe into a separator leaves a
	// lone "s" ("Ocean_s_Eleven"): it joins the word before it.
	words := strings.Fields(b.String())
	joined := words[:0]
	for _, w := range words {
		if w == "s" && len(joined) > 0 {
			joined[len(joined)-1] += w
			continue
		}
		joined = append(joined, w)
	}
	return strings.Join(joined, " ")
}

// isApostrophe reports whether r is one of the marks titles use as an
// apostrophe: the typewriter and typographic quotes, the modifier letter and
// the backtick.
func isApostrophe(r rune) bool {
	switch r {
	case '\'', '\u2019', '\u2018', '\u02bc', '`':
		return true
	}
	return false
}

// jaccard is the token-set overlap ratio |A∩B| / |A∪B| in [0,1].
func jaccard(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	set := make(map[string]bool, len(a))
	for _, t := range a {
		set[t] = true
	}
	union := make(map[string]bool, len(a)+len(b))
	for _, t := range a {
		union[t] = true
	}
	intersect := 0
	for _, t := range b {
		if set[t] {
			intersect++
			delete(set, t) // count each shared token once
		}
		union[t] = true
	}
	return float64(intersect) / float64(len(union))
}
