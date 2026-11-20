# Season and episode markers of other languages

Ticket: CORE-2702 (`queue:core:main`).
Status: proposed.

## Why

The parser only knows English episode markers, so French, Spanish, German, Italian, Polish and
Russian releases parse as titles with no episode (`Anubis saison 01 episode 38`,
`Juego de Tronos - Temp.2 [Cap.209]`, `My Little Pony - T02E22`, `MosGaz.(08.seriya).2012`), and
so do `Desperate Housewives - Episode 1.22` and `Mob.Psycho.100.II.E10`.

## What Changes

- A season word (season, saison, temporada, temp, staffel, stagione, sezon) followed by an episode
  word (episode, épisode, episodio, capítulo, cap, folge, ep) and their numbers is a season and
  episode anywhere. After `cap` a three-digit number whose first digit is the season packs both
  (`Cap.209`: episode 9).
- `TxxEyy` with both numbers padded, and `Episode S.EE`, are a season and episode anywhere.
- A lone episode word with a number, a bare `Eyy`, or `N seriya` is an episode only in a season
  folder, which gives the season (0 for `Specials`).
- A film named with one of the words (`Temporada (2015)`, `T2 (1991)`, `E10`) stays a film.

## Impact

No SDK change. The vocabulary is bounded to the words above; season folders are still recognized
only by their English names. No PTT code or fixtures are used.
