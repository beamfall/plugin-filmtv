## 1. Build

- [ ] 1.1 Declare the `film-series-film` single-holder relation on `film-series`
- [ ] 1.2 Add the `belongs_to_collection` series and container maps to the `tmdb-movie` scraper

## 2. Prove

- [ ] 2.1 Bundle tests pin both maps and the relation, and the bundle validates
- [ ] 2.2 A scraper trace vector over a recorded movie document freezes the series container output
- [ ] 2.3 A recorded movie document without a collection emits no series container
- [ ] 2.4 Cross-repo: core identifies two recorded films of one collection into one series in release order, and relinks a moved film
- [ ] 2.5 `openspec validate --all --strict` passes

## 3. Release

- [ ] 3.1 Tag the plugin after core's CORE-3152 seam lands, then repin core
