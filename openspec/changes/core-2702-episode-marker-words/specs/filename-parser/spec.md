## ADDED Requirements

### Requirement: Season and episode words of other languages

A season word (season, saison, temporada, temp, staffel, stagione, sezon) that starts a word,
followed by its number and an episode word (episode, épisode, episodio, capítulo, cap, folge, ep)
and its number, SHALL name a season and an episode in any folder. After `cap`, a three-digit number
whose first digit is the season SHALL be that season and the last two digits the episode.

#### Scenario: French words

- **WHEN** a file is `/downloads/Anubis saison 01 episode 38.mkv`
- **THEN** it parses to title `Anubis`, class `episode`, season 1, episode 38

#### Scenario: Packed Spanish chapter number

- **WHEN** a file is `/tv/Juego de Tronos/Season 2/Juego de Tronos - Temp.2 [Cap.209].mkv`
- **THEN** it parses to title `Juego de Tronos`, class `episode`, season 2, episode 9

#### Scenario: A word only ends with a season word

- **WHEN** a file is `/downloads/Contemp 2 Ep 3.mkv`
- **THEN** it parses to the film title `Contemp 2 Ep 3`

### Requirement: TxxEyy and dotted episode codes

A `TxxEyy` code with a two-digit season and a two- or three-digit episode, and an episode word
followed by `S.EE` (`Episode 1.22`, `Folge 2.04`), SHALL name a season and an episode in any
folder. Neither SHALL match inside a longer word or number, and a bit depth (`Episode.1.10-bit`)
SHALL NOT read as `S.EE`. When an absolute number, an `XxYY` code or an air date shares a name
with an `S.EE` code, the first SHALL win.

#### Scenario: Temporada code

- **WHEN** a file is `/downloads/My Little Pony - T02E22.mp4`
- **THEN** it parses to title `My Little Pony`, class `episode`, season 2, episode 22

#### Scenario: Dotted episode code

- **WHEN** a file is `/downloads/Desperate Housewives - Episode 1.22.avi`
- **THEN** it parses to title `Desperate Housewives`, class `episode`, season 1, episode 22

#### Scenario: A film named T2

- **WHEN** a file is `/movies/T2 (1991).mkv`
- **THEN** it parses to the film title `T2`, year 1991

### Requirement: Episode-only markers need a season folder

An episode word with a number (`Ep 14`, `Folge 12`, `Cap.105`), a bare `Eyy` and `N seriya` SHALL
name an episode only in a season folder, which SHALL give the season (0 for `Specials`). Elsewhere
they SHALL stay title text, so a film named with one of them stays a film. When an absolute
number, an `XxYY` code or an air date also appears, the first marker in the name SHALL win,
unless it is all of an absolute number's series name.

#### Scenario: Bare episode code in a season folder

- **WHEN** a file is `/tv/Mob Psycho 100/Season 2/Mob.Psycho.100.II.E10.720p.WEB.mkv`
- **THEN** it parses to title `Mob Psycho 100 II`, class `episode`, season 2, episode 10

#### Scenario: Seriya in a season folder

- **WHEN** a file is `/tv/MosGaz/Season 1/MosGaz.(08.seriya).2012.mkv`
- **THEN** it parses to title `MosGaz`, class `episode`, season 1, episode 8

#### Scenario: Film named like an episode code

- **WHEN** a file is `/downloads/E10.mkv`
- **THEN** it parses to the film title `E10`

#### Scenario: Film named with a season word

- **WHEN** a file is `/movies/Temporada (2015).mkv`
- **THEN** it parses to the film title `Temporada`, year 2015
