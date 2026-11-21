## Implementation

- [x] Observe Jellyfin 12.4 and Emby 4.10.2 ratings, favourites and collections from pinned official images driven through their public APIs.
- [x] Report Jellyfin watch state and ratings per source user, and BoxSet memberships, through `ItemsBySourceUser`.
- [x] Read the Emby 4.10 layout for watch state, per source user, and collection memberships, keeping the older layout readable.
- [x] Advertise ratings and collections for Jellyfin and collections for Emby, with category-scoped constructors.
- [x] Fixture tests on both schemas: favourite-only, rated-only and collection members.
- [x] File the favourites follow-up (CORE-2895).
- [ ] Archive after landing.
