# Remove the film-only home rail

CORE-2535: Film & TV shares one browse layout across Libraries, so the fixed Movies grid incorrectly presents movie-class unmatched files as a curated rail in TV Libraries. Remove that fixed grid. The existing All Titles view and Movies filter still provide deliberate access to loose films; owner Unmatched / fix-match remains the repair path.

Non-goals: introduce Library types, alter matching, remove catalog items, or change source filtering. Acceptance: a plugin regression rejects a movie-only home grid while retaining catalog access. Rollback: restore the section. Integration needs a new plugin tag and Core repin; this builder does not release.
