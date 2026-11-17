# Declare the film and TV Media subtitle

Ticket: CORE-1226 (`queue:core:main`), consumed by CORE-2386.
Status: proposed.

## Why

In a season folder every episode card shows the show's poster, title and year, so twelve
episodes look identical. plugin-sdk adds `Bundle.MediaSubtitle` (CORE-1226) so the plugin that
owns the vocabulary says how its items differ; core renders it without knowing the words.

## What Changes

- The bundle declares `MediaSubtitle` as `S{sequence}`, `E{index}` and `{episode-title}`. A movie
  has none of these values, so it gets no subtitle.
- The `tmdb-tv` per-episode lookup also maps the episode's `name` onto `media.episode-title`, so an
  episode whose filename carries no title still gets one.
- `go.mod` pins the plugin-sdk branch commit that adds the field until plugin-sdk is tagged.

## Non-goals

- No parser or local mapper change. No new Media field.

## Failure modes

- An episode missing its ordinals or title drops those segments; a movie keeps its current card.
- A failed episode lookup leaves the parsed title in place, as today.

## Acceptance evidence

Bundle tests pin the template and the item field map, the bundle validates, and the rendered
subtitle for a parsed episode is `S5 · E3 · <title>`.

## Rollback

Revert the commit; clients fall back to the card without a subtitle.
