CREATE TABLE media_parts (id INTEGER PRIMARY KEY, media_item_id INTEGER, file TEXT);
CREATE TABLE media_items (id INTEGER PRIMARY KEY, metadata_item_id INTEGER, duration INTEGER);
CREATE TABLE metadata_items (id INTEGER PRIMARY KEY, guid TEXT);
CREATE TABLE metadata_item_settings (account_id INTEGER, guid TEXT, view_offset INTEGER, view_count INTEGER, last_viewed_at INTEGER, rating REAL);
CREATE TABLE accounts (id INTEGER PRIMARY KEY, name TEXT);
CREATE TABLE tags (id INTEGER PRIMARY KEY, tag TEXT, tag_type INTEGER);
CREATE TABLE taggings (metadata_item_id INTEGER, tag_id INTEGER, "index" INTEGER);
INSERT INTO accounts VALUES (1, 'Alice'), (2, 'Bob'), (3, 'Empty');
INSERT INTO metadata_items VALUES (1, 'shared'), (2, 'rated'), (3, NULL), (4, 'collection-only');
INSERT INTO media_items VALUES (1, 1, 90000), (2, 1, 91000), (3, 2, 120000), (4, 3, 120000), (5, 4, 120000);
INSERT INTO media_parts VALUES (1, 1, '/movie.mkv'), (2, 1, '/part2.mkv'), (3, 2, '/version.mkv'),
    (4, 3, '/zero-rated.mkv'), (5, 4, '/ignore.mkv'), (6, 5, '/collection-only.mkv');
INSERT INTO metadata_item_settings VALUES (1, 'shared', 0, 2, 1700000000, 8.5),
    (2, 'shared', 12500, 0, 1700000010, NULL), (1, 'rated', NULL, 0, NULL, 0),
    (1, NULL, 89000, 1, 1700000030, 10), (99, 'shared', 18000, 0, 1700000020, 10);
INSERT INTO tags VALUES (1, 'Favorites', 2), (2, 'Weekend', 2), (3, 'Genre', 1);
INSERT INTO taggings VALUES (1, 1, 3), (1, 2, 0), (1, 3, 4), (4, 2, 7);
