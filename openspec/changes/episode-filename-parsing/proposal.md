# Parse episode filenames into a clean series title and an episode title

Ticket: CORE-1148 (`queue:core:main`). Status: proposed.

## Why

A local file named `Pioneer One - S01E01 - Earthfall.mkv` is titled "Pioneer One -". The parser
cuts the name at the episode token, keeps the separator in front of it and throws away everything
after it. The dangling separator shows on every card, is sent to metadata providers as the search
query, and the episode's own name is lost, so four episodes of one series look identical.

## What Changes

- The parsed title of an episode is the series name with no leading or trailing separators and no
  leading release-group bracket.
- The text after the episode token, up to the first release tag or bracket, is the episode title.
  The parser emits it as the new plugin-declared Media field `episode-title` (searchable text)
  through the SDK's optional media-field parser hook.
- Multi-episode names report the first and the last episode of the chain (`S01E01E02`,
  `S01E01-E02-E03`, `S01E01-S01E02`, `1x01-1x02`). `S01.E01` and `S01 E01` are accepted.
- An episode file that carries no series name of its own (`S01E01 - Earthfall.mkv`) takes the
  series name, and a parenthesised year, from the folder above its season folder.
- Release-tag detection covers any `NNNp`/`NNNi` resolution and the common source, codec and audio
  tags. A tag that is also an ordinary word (`Internal`, `Extended`, `Proper`, `Web`) ends a title
  only when a release tag follows it or it is the last word.

## Non-goals

- Show and season containers for unidentified files. That needs an SDK and Core change and is
  reported on the ticket.
- Composing the card's second line ("S1 · E1 · Earthfall"). Core and the clients render cards; this
  change only supplies the fields.
- Date-stamped daily shows, absolute-numbered anime and episode-only names (`05 - Title.mkv`).
  They stay pinned as documented gaps in the parser corpus.

## Failure modes

- A hyphenated release group with no release tag in front of it (`Show.S01E01.Title-GRP.mkv`)
  cannot be told from a hyphenated title and stays in the episode title.
- A file placed directly in a library root with no series name is still refused: only a folder
  above a recognised season folder is trusted as a series name.

## Acceptance evidence

`go test ./...` (parser corpus, episode-title vectors, bundle and SDK conformance), the bounded
`FuzzParseFilename` run and `script/gate.sh`.

## Rollback

Revert the code commit. `episode-title` values already written to a library are plain Media fields
and are ignored by a bundle that no longer declares the field.
