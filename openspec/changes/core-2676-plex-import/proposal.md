# Import Plex account state and collections

CORE-2676 adds ratings and ordered collections to the Plex source. The Plex portion of CORE-0205 supplies account-specific playback state through Core's existing source-user method. The import reads a Plex database export using a read-only, immutable connection.

Existing single-user watch-state readers keep their streaming and batch entry points. Core owns source-user mapping under ADR-0037 §5.
