# Carry ratings and collections from Jellyfin 12 and Emby 4.10

CORE-2678. The Jellyfin and Emby sources import watch state only, merged across every source user into the run's one Profile. Plex already reports per-account state, ratings and collections through `ItemsBySourceUser`, so Core can map each source user to a Profile and skip unmapped users (ADR-0037 §5).

This change gives Jellyfin and Emby the same shape:

- Jellyfin 12 reports each source user's watch state and ratings, plus collection (BoxSet) memberships, and advertises the ratings and collections categories.
- Emby reads the Emby 4.10 `library.db` layout (`MediaItems`, `UserDatas` keyed by `UserDataKeyId` and `userId`, `ItemLinks2`) as well as the older layout it reads today. It reports each source user's watch state plus collection memberships and advertises the collections category. The reader before this change cannot read Emby 4.10 at all.
- Emby does not advertise ratings: the Emby API never stored a numeric rating during observation, so the scale is unknown.
- Favourites are left out. plugin-sdk has no file-keyed favourite, and `ItemState` is keyed by feed, so a favourite has no correct place yet. CORE-2895 carries the SDK field, the Core apply and the filmtv emit.

Core does not yet apply imported ratings or static collection members (CORE-2676 tracks that). Nothing changes what an App Store binary renders or names.

## How the schemas were observed

Clean-room, as in CORE-2677. `jellyfin/jellyfin:12.4@sha256:fea863d0cd9ec445985d9c75fc6ed64e38a1d2766cf0c250a33d553f4fc1b771` and `emby/embyserver@sha256:7082c63b10a2fcb3805a7eaebba407d5723bc57c085c9f0b29b7d0a702765cc1` (server 4.10.2.0) ran on four synthetic files. Users, libraries, ratings, likes, favourites, watch state and one collection were set only through each server's public HTTP API. The databases were then read with `sqlite3`. No server source was read. The scripts and schema dumps are kept with the CORE-2678 release evidence.

What the inspection showed:

- Jellyfin `UserData.Rating` is a 0 to 10 real, copied to every data key of the item; a like stores 10. `UserData.IsFavorite` is an integer flag.
- A Jellyfin collection is a `BaseItems` row of type `MediaBrowser.Controller.Entities.Movies.BoxSet`. Its members are `LinkedChildren` rows whose `SortOrder` follows the order members were added. `Users` holds `Id` and `Username`.
- Emby 4.10 keeps items in `MediaItems` (no `IsFolder`; `type` 1 to 4 and 6, 7 and 9 are containers), user state in `UserDatas` (`LastPlayedDateInt` in Unix seconds), collection members in `ItemLinks2` rows of `Type` 6 (`ItemId` member, `LinkedId` collection, `LinkOrder` 0), and users in the separate `users.db` (`LocalUsersv2.data` JSON `Name`).
