# Use TheTVDB as its v4 API documents, and present the subscriber PIN

Tickets: CORE-1127 and CORE-1128 (`queue:core:main`).

## Why

The owner asked for TheTVDB to be used as its v4 API documents. Every endpoint below was confirmed
against the official v4 swagger (`docs/swagger.yml` in `thetvdb/v4-api`, version 4.8.0, sha256
`ff4a162b2b9192f83abdda672fa64c4f88fe5c1413c6c9eccc7b46a752c633f3`) and its README.
`tvdb-series` sent only `query` and `type` to `/search`, read the base record (original-language
name and overview, poster only), and looked up only TheTVDB ids. The PIN login itself came with
CORE-1129 (the PIN is the AuthRef, the project key a second login secret); that decision
supersedes CORE-1127's optional-PIN shape, so CORE-1127 here is presentation only.

## What Changes

- **PIN presentation (CORE-1127).** `CredentialHelp` labels the AuthRef field "TheTVDB subscriber
  PIN" and links to `https://www.thetvdb.com/subscribe` ("Subscribe at thetvdb.com").
- **Year.** `SearchTemplate` is `/search?query={query}&type=series&year={year}`; the host drops
  `year` when the file has none.
- **Language.** A bundle config field `metadata-language` (ISO 639-2, default `eng`). The lookup is
  `/series/{id}/extended?meta=translations`; title and summary read the translation in that language
  and fall back to the original-language base value.
- **Remote IDs.** `LookupByIDTemplate` maps `imdb` and `tmdb` to `/search/remoteid/{id}`; field
  paths fall back to its `data[0].series` shape.
- **Artwork.** Backdrop, banner and logo map from the extended record's artworks by type id, read
  from `/artwork/types` through the `artwork-types` reference document (cached for a week by the
  host) rather than hard-coded. The poster stays the base-record image, which the README names the
  best poster.

## Out of scope (owned elsewhere)

- Episode lookup by season type (`/series/{id}/episodes/{season-type}`) stays with CORE-0362.
- Wiring NFO and filename id hints into identification stays with CORE-0360.
- Other providers' languages stay with CORE-0363.

## Impact

- Needs plugin-sdk v0.3.6 (`CredentialHelp`, `ReferenceDocuments`) and a core build with the
  CORE-1128 engine (`{year}`, multi-condition and `||` paths, `{ref.<name>:<path>}`). Core must land
  that engine before it repins to the filmtv release carrying this change.
- `go.mod` names plugin-sdk v0.3.6, which is not tagged yet; `go.sum` gains its hash at the repin.
