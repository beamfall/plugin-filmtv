# Import watch state from Jellyfin 12

CORE-2677. The Jellyfin source reads the legacy `library.db` tables `TypedBaseItems` and `UserDatas`. Current Jellyfin keeps its library in `jellyfin.db`, so importing from a current install yields nothing.

Owner decision, 2026-11-21: support Jellyfin 12 only. Operators on an older Jellyfin upgrade it first, then import. The Jellyfin source reads a Jellyfin 12 `jellyfin.db` and refuses a legacy `library.db` with an error that says to upgrade. Emby keeps the legacy `library.db` reader unchanged.

## How the schema was observed

The schema was observed by inspecting a database, never by reading Jellyfin server source. `jellyfin/jellyfin:12.4@sha256:fea863d0cd9ec445985d9c75fc6ed64e38a1d2766cf0c250a33d553f4fc1b771` ran on four synthetic files: two films and two episodes. Setup, libraries, a second user and watch state were driven only through the public HTTP API. The server was then stopped, and `jellyfin.db` was read with `sqlite3`. The run script and schema dump are kept with the CORE-2677 release evidence.

What the inspection showed:

- `BaseItems` holds `Id`, `Path`, `RunTimeTicks` and `IsFolder`.
- `UserData` is keyed by `(ItemId, UserId, CustomDataKey)` and holds `Played`, `PlaybackPositionTicks` and `LastPlayedDate`.
- One user's state for one item is written as several identical rows, one per data key (provider IDs and internal keys).
- `LastPlayedDate` is UTC text in the form `2026-11-21 13:40:27.0279689`, which is not RFC 3339.
