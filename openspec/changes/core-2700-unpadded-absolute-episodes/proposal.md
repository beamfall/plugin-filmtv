# Read unpadded absolute episode numbers after ` - `

Ticket: CORE-2700 (`queue:core:main`).
Status: proposed.

## Why

Anime releases number episodes absolutely, and most do not zero-pad to three digits:
`Mob Psycho 100 - 09`, `Naruto Shippuden - 107 - Title`, `One Piece - 1071`,
`[Erai-raws] Show - 02 [1080p]`. The parser read the dashed form only as exactly three digits
starting with `0` (`- 001`), so these names fell through as films titled `Show - 09`, and an episode
library could not match them. The corpus pinned `[HorribleSubs] One Piece - 1015` as an unusable
parse.

## What Changes

- The dashed absolute form accepts two to four digits after ` - `, with an optional release
  revision (`- 12v2`), at the end of the name, before release noise, or before an episode title.
- The form still needs episode context, as before: a season folder, a tv or anime library folder, a
  checksum tag, a leading group bracket, or a show folder named like the series.
- A single digit after ` - ` is never read as an episode: it numbers film sequels (`Rocky - 2`).
- Films in a film library and names with no episode context keep their number in the title.

## Impact

No SDK change. Two existing cases change from a film or untyped title to an absolute episode
(`Show - 01 [4E01A2B3]` under `/media/Anime`, and the HorribleSubs corpus case). The corpus
auto-match rate rises.
