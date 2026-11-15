## 1. Build

- [x] 1.1 Parse an air date as the episode token of a name with no season/episode token
- [x] 1.2 Emit `air-date` (date) and `episode-title` (text) through `ParseTypedMediaFields`
- [x] 1.3 `MediaTitle` returns the episode code (air date, `SxxEyy` or `SxxEyy-zz`) and the episode title
- [x] 1.4 `LocalHydration` groups a dated episode by air year and orders it by month and day
- [x] 1.5 Stop the `tmdb-tv` and `tvmaze-show` scrapers mapping the series premiere onto `media.air-date`

## 2. Prove

- [x] 2.1 Synthetic corpus test: two dated files get distinct air-date titles that sort by date, the
  range file shows its range, and every episode display title carries `SxxEyy` or the date
- [x] 2.2 Bundle test: no scraper maps `media.air-date`
- [x] 2.3 `openspec validate --all --strict` passes
- [ ] 2.4 `script/gate.sh` passes on the lane head

## 3. Close

- [ ] 3.1 Archive this change after landing, before completing the ticket
- [ ] 3.2 Core repins the plugin-filmtv release and adds a scan test over the synthetic corpus
