## Implementation

- [x] Observe the Jellyfin 12.4 `jellyfin.db` schema from a pinned official image driven through its public API.
- [x] Read Jellyfin 12 watch state, collapse the data-key copies, and parse the stored `LastPlayedDate` form.
- [x] Refuse a legacy Jellyfin `library.db` and unknown files with errors an operator can act on.
- [x] Prove with fixtures built from the observed schema that Jellyfin 12 returns the same records as the legacy reader for the same library.
- [x] Name the database file for Jellyfin 12 and Emby in the README.
- [ ] Archive after landing.
