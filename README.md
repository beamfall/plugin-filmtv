<p align="center">
  <img src=".github/brand/beamfall-logo.png" alt="Beamfall" width="96" height="96">
</p>

# Beamfall Film & TV

**Movies and TV shows, recognized, matched, and organized in your own Beamfall server.**

This is the first-party Film & TV plugin for [Beamfall Core](https://github.com/beamfall/core), the
self-hosted media server for the library you already own. Point a Beamfall Library at a folder of
films, episodes, or both, and this plugin works out what each file is, matches it against TMDB or
TVDB, and gives the Library a home screen built for watching: continue watching, next up, new
arrivals, and browse grids for movies and episodes. It also brings your watch history with you
from Plex, Emby, Jellyfin, Trakt, or an NFO-tagged library.

It is free and open source under [AGPL-3.0](LICENSE). Like the rest of Beamfall, it needs no
account and has no telemetry.

## What you get

- **One plugin for mixed libraries.** Real folders mix films and shows, so every file is classified
  as a movie or an episode on its own. A Library can also be pinned to Films or TV Shows when you
  know what it holds; Mixed is the default.
- **A filename parser that handles scene names.** It reads `S01E02`, `1x02`, and
  `Season 1 Episode 2` forms, multi-episode ranges, and years, and strips release noise such as
  resolution, codec, and source tags. Provider ID tags in the path (`{tmdb-…}`, `[imdbid-tt…]`,
  `{tvdb-…}`, `{anidb-…}`) and IDs from a local `.nfo` go straight to identification.
- **Matching that would rather ask than guess.** Each candidate gets a confidence score from title,
  year, and season/episode agreement. Only strong matches apply automatically; a vague title with
  no year to back it up, or a tie between look-alikes, goes to the Unmatched queue for you to
  decide.
- **Rich metadata from TMDB and TVDB.** TMDB is declared for both movies and TV, and TVDB for
  series. Identification fills in original title, tagline, release or air date, production
  status, and network, and links cast and crew, studios, and genres as browsable
  entries. You register your own API keys; none are embedded.
- **A model that fits film and TV.** Shows hold ordered Seasons, Seasons hold Episodes, Season 0
  is Specials, and TMDB collections become Film Series. Guest stars attach to the episode they
  appear in.
- **A home screen and detail pages ready to use.** Most Played, Continue Watching, Next Up, New
  Movies & Episodes, a genre shelf, Movies and Episodes grids, a filter bar by type and network,
  and detail pages for movies, shows, and seasons. Labels ship in English and French.
- **Parental ratings.** Titles have a certification field, and the US MPA scale (G to NC-17) is
  declared in order so Core can enforce Profile rating ceilings.
- **Bring your history.** Import watched state, resume position, and last-played time from a Plex,
  Emby, or Jellyfin database export or a Trakt watched-history CSV. An NFO sidecar library
  contributes its TMDB/IMDB IDs and local artwork. Imports are additive and safe to re-run; the
  foreign databases are opened read-only.
- **Trakt scrobbling.** When you finish something, Beamfall can record it in your Trakt history
  using the title's TMDB, IMDB, or TVDB ID. TV episodes are currently submitted as movie entries;
  episode-aware submission is planned.

## How it fits into Beamfall

The Film & TV plugin ships with [Beamfall Core](https://github.com/beamfall/core) and is compiled
into the server; you do not install it separately. It is built on the
[Beamfall plugin SDK](https://github.com/beamfall/plugin-sdk): Core depends on the SDK, plugins
depend on the SDK, and Core never depends on a plugin. All Film & TV vocabulary (movies, episodes,
seasons, networks) lives here, not in Core.

| File | What it does |
| --- | --- |
| `bundle.go` | The declarative bundle: entity kinds, fields, layouts, scrapers, family scopes, and importers. |
| `parse.go` | The filename parser: movie-vs-episode classification, year, release-name cleanup, provider ID hints. |
| `score.go` | The match scorer and its fail-closed auto-apply threshold. |
| `sources.go`, `nfotree.go` | Migration sources for Plex, Emby, Jellyfin, and NFO sidecar trees. |
| `trakt.go` | Migration source for Trakt watched-history CSV exports. |
| `trakt_scrobble.go` | The Trakt scrobble adapter. |
| `internal/foreignimport` | Read-only readers for Plex, Emby, and Jellyfin SQLite databases. |
| `internal/mediaimport` | Parsers for NFO XML, external IDs, artwork, ordered lists, and watch-state JSON. |

## Develop

You need the Go toolchain named in [`go.mod`](go.mod), Python 3, and `golangci-lint` on your
`PATH`.

```sh
go test ./...        # unit, corpus, and conformance tests
script/gate.sh       # full local gate: gofmt, vet, tests with coverage floors, golangci-lint, govulncheck
script/gate.sh race  # tests under the race detector
```

The tests run the SDK's `sdktest` conformance harness against this plugin's parser and scorer
(`conformance_test.go`), plus an adversarial filename corpus (`parsecorpus_test.go`) and a
wrong-match regression corpus (`matchcorpus_test.go`). CI runs the same gate on every push and pull
request, and runs bounded fuzzing of the parsers on a schedule
([`conformance.yml`](.github/workflows/conformance.yml)).

## Contributing

Contributions require a signed Contributor License Agreement. Read
[CONTRIBUTING.md](CONTRIBUTING.md) and [CLA.md](CLA.md) before you open a pull request.

## Security

Please report vulnerabilities privately as described in [SECURITY.md](SECURITY.md). Do not open a
public issue.

## License

Beamfall Film & TV is licensed under the GNU Affero General Public License v3.0. See
[LICENSE](LICENSE).
