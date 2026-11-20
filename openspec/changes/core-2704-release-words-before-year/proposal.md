# Edition, language and site words leave the title

Ticket: CORE-2704 (`queue:core:main`).
Status: proposed.

## Why

Three kinds of release text stay in a film's title and spoil the provider search:

- An edition word right before the year: `Jurassic.World.Dominion.EXTENDED.2022.2160p.UHD.BluRay.REMUX`
  parses as `Jurassic World Dominion EXTENDED`. A release word only ends a title when a release
  tag follows it, and the year does not count as one.
- Language and subtitle tags, which are in no noise list: `Color.Of.Night.Unrated.DC.VostFR.BRrip.x264`
  parses as `Color Of Night Unrated DC VostFR`.
- A torrent site's name in front of the title: `www.Torrenting.com - Anatomy Of A Fall (2023)`
  parses as `www Torrenting com - Anatomy Of A Fall`.

## What Changes

- A year ends a title after a run of edition tags (`EXTENDED`, `Unrated`, `Uncut`, `Remastered`,
  `IMAX`), language tags (`NORDiC`, `iTA`, `ENG`) and release tags: `EXTENDED.2022` drops the
  edition word, while `Charlotte's.Web.2006`, `Extended.Family.2023` and `Le.Cercle.Internal.2020`
  keep their titles. A scene tag that is also a word (`Internal`, `Proper`) is not an edition.
- French and Scandinavian language and subtitle tags (`VostFR`, `TRUEFRENCH`, `SUBFRENCH`, `VFF`,
  `VFQ`, `VF2`, `SweSub`) are release noise. `NORDiC`, `iTA`, `ENG` and `IMAX` end a title as
  `EXTENDED` does. `DC` is noise only right after an edition word (`Unrated.DC`), since it is also
  a title word (`Washington DC`); `FRENCH` and `GERMAN` stay title words (`The Good German`).
- A run is kept when dropping it would leave the title ending in a joining mark
  (`Bigger, Longer & Uncut (1999)`, `Пленка | Uncut (1997)`).
- A leading `www.site.tld - ` is dropped before parsing. A bracketed `[www.site.tld]` is dropped
  from a film title only, so it still marks a fansub episode as any leading group does.

## Impact

No SDK change. Edition words are dropped from the title, not yet read as the version label:
edition parsing belongs to CORE-0204.
