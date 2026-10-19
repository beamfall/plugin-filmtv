package filmtv

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	sdk "github.com/beamfall/plugin-sdk"
)

const TraktImporterID = "trakt"

// maxTraktCSVBytes bounds a single Trakt export read so a hostile/huge CSV
// export cannot exhaust memory during an operator-triggered import, while
// comfortably covering a real Trakt watched-history export (which can run
// well past 64 KB for an active account).
const maxTraktCSVBytes = 64 << 20

// errTraktImportTooLarge is returned when a Trakt export exceeds maxTraktCSVBytes.
var errTraktImportTooLarge = errors.New("import source exceeds 64 MB size limit")

type traktSource struct {
	csvPath string
}

func (traktSource) Categories() []string { return []string{sdk.CategoryWatchState} }

func (s traktSource) Items(ctx context.Context) ([]sdk.ForeignItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f, err := os.Open(s.csvPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return ParseTraktCSV(f)
}

func (s traktSource) Stream(
	ctx context.Context,
	checkpoint string,
	emit func(context.Context, []sdk.ForeignItem, string) error,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f, err := os.Open(s.csvPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() > maxTraktCSVBytes {
		return errTraktImportTooLarge
	}
	pager, err := newFilmTVPageEmitter(ctx, checkpoint, emit)
	if err != nil {
		return err
	}
	reader := csv.NewReader(io.LimitReader(f, maxTraktCSVBytes+1))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	first, err := reader.Read()
	if err == io.EOF {
		return pager.finish()
	}
	if err != nil {
		return err
	}
	columns, body := traktColumns([][]string{first})
	if len(body) > 0 {
		if err := addTraktRow(pager, body[0], columns); err != nil {
			return err
		}
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		row, err := reader.Read()
		if err == io.EOF {
			return pager.finish()
		}
		if err != nil {
			return err
		}
		if err := addTraktRow(pager, row, columns); err != nil {
			return err
		}
	}
}

func addTraktRow(pager *filmTVPageEmitter, row []string, columns traktColumnMap) error {
	ids := traktExternalIDs(row, columns)
	if len(ids) == 0 && blankCSVRow(row) {
		return nil
	}
	return pager.add(sdk.ForeignItem{
		ExternalIDs: ids,
		WatchState: sdk.WatchState{
			Present:    true,
			Watched:    true,
			OccurredAt: parseHistoryTime(csvField(row, columns.watchedAt)),
		},
	})
}

func NewTraktSource(csvPath string) sdk.Source { //nolint:staticcheck // deliberate SDK 0.x compatibility path for pre-streaming consumers
	return traktSource{csvPath: csvPath}
}

// ParseTraktCSV reads a Trakt watched-history CSV export into external-ID
// keyed watch-state rows. Supported headers are deliberately conservative:
// watched_at plus one or more of tmdb/imdb/tvdb. Rows with no external ID are
// yielded as unmatched items instead of guessed by title.
func ParseTraktCSV(r io.Reader) ([]sdk.ForeignItem, error) {
	raw, err := io.ReadAll(io.LimitReader(r, maxTraktCSVBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxTraktCSVBytes {
		return nil, errTraktImportTooLarge
	}

	reader := csv.NewReader(bytes.NewReader(raw))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []sdk.ForeignItem{}, nil
	}
	cols, body := traktColumns(rows)
	items := make([]sdk.ForeignItem, 0, len(body))
	for _, row := range body {
		ids := traktExternalIDs(row, cols)
		if len(ids) == 0 && blankCSVRow(row) {
			continue
		}
		items = append(items, sdk.ForeignItem{
			ExternalIDs: ids,
			WatchState: sdk.WatchState{
				Present:    true,
				Watched:    true,
				OccurredAt: parseHistoryTime(csvField(row, cols.watchedAt)),
			},
		})
	}
	return items, nil
}

type traktColumnMap struct {
	watchedAt, tmdb, imdb, tvdb int
}

func traktColumns(rows [][]string) (traktColumnMap, [][]string) {
	cols := traktColumnMap{watchedAt: -1, tmdb: -1, imdb: -1, tvdb: -1}
	if len(rows) == 0 {
		return cols, nil
	}
	header := rows[0]
	recognized := false
	for i, raw := range header {
		switch normalizeHeader(raw) {
		case "watchedat", "watched", "watchedon", "lastwatchedat", "lastwatched":
			cols.watchedAt, recognized = i, true
		case "tmdb", "tmdbid", "movieids.tmdb", "showids.tmdb", "episodeids.tmdb":
			cols.tmdb, recognized = i, true
		case "imdb", "imdbid", "movieids.imdb", "showids.imdb", "episodeids.imdb":
			cols.imdb, recognized = i, true
		case "tvdb", "tvdbid", "showids.tvdb", "episodeids.tvdb":
			cols.tvdb, recognized = i, true
		}
	}
	if recognized {
		return cols, rows[1:]
	}
	return traktColumnMap{watchedAt: 0, tmdb: 1, imdb: 2, tvdb: 3}, rows
}

func traktExternalIDs(row []string, cols traktColumnMap) []sdk.ExternalID {
	ids := make([]sdk.ExternalID, 0, 3)
	for _, id := range []sdk.ExternalID{
		{Scheme: SchemeTMDB, Value: csvField(row, cols.tmdb)},
		{Scheme: SchemeIMDB, Value: csvField(row, cols.imdb)},
		{Scheme: SchemeTVDB, Value: csvField(row, cols.tvdb)},
	} {
		id.Value = strings.TrimSpace(id.Value)
		if id.Value != "" && id.Value != "0" {
			ids = append(ids, id)
		}
	}
	return ids
}

func csvField(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return row[idx]
}

func normalizeHeader(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	raw = strings.ReplaceAll(raw, "_", "")
	raw = strings.ReplaceAll(raw, "-", "")
	raw = strings.ReplaceAll(raw, " ", "")
	return raw
}

func blankCSVRow(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

var historyTimeLayouts = []string{
	time.RFC3339,
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"2006-01-02",
	"02 Jan 2006 15:04",
}

func parseHistoryTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	for _, layout := range historyTimeLayouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
