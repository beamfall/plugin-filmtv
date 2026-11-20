package filmtv_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"

	filmtv "github.com/beamfall/plugin-filmtv"
)

// This file grades the parser on a second, independent corpus: the
// library-shaped release names in PTT's test fixtures (testdata/ptt, CORE-2705).
// testdata/ptt/import.py writes cases.json from a PTT checkout by a written
// rule that drops packs, adult and trash cases; the file records the PTT
// version, commit and licence it came from.
//
// Each case declares some of title, year, seasons and episodes, and only the
// declared fields are graded. A field the parser gets wrong today is marked in
// testdata/ptt/unresolved.json, which only `go test -run TestPTTCorpus
// -update-ptt` writes. A resolved field that regresses fails the test, and so
// does a marked field that now parses, until the marks are rewritten: that is
// how a parser gap ticket shows as a rising rate.

var updatePTT = flag.Bool("update-ptt", false, "rewrite testdata/ptt/unresolved.json from the parser's current output")

// The documented per-field floors: the fraction of declared fields the parser
// gets right on PTT's library-shaped set. Each is the rate observed when it was
// set, rounded down to two places (PTT 1.8.5, CORE-2705: title 92/138, year
// 92/97, season 96/134, episode 141/241), so it catches a broad slide; a single
// regression is caught by the per-case marks. Raise a floor when a parser gap
// ticket raises its rate.
const (
	pttTitleFloor   = 0.66
	pttYearFloor    = 0.94
	pttSeasonFloor  = 0.71
	pttEpisodeFloor = 0.58
)

var pttFields = []string{"title", "year", "season", "episode"}

type pttCase struct {
	Name     string  `json:"name"`
	Title    *string `json:"title"`
	Year     *int    `json:"year"`
	Seasons  *[]int  `json:"seasons"`
	Episodes *[]int  `json:"episodes"`
}

type pttFile struct {
	Version string    `json:"version"`
	Commit  string    `json:"commit"`
	License string    `json:"license"`
	Notice  string    `json:"license_text"`
	Rule    string    `json:"rule"`
	Cases   []pttCase `json:"cases"`
}

// pttPath turns a PTT release name into a file name the parser reads: a slash
// in the name is not a directory, and a name with no video extension gets one.
func pttPath(name string) string {
	path := strings.ReplaceAll(name, "/", "∕")
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mkv", ".mp4", ".avi", ".m4v":
		return path
	}
	return path + ".mkv"
}

func normalTitle(title string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.ReplaceAll(title, "/", "∕")), " "))
}

// pttGrade reports, for each field the case declares, whether the parse
// matches it. A name the parser rejects gets every declared field wrong.
func pttGrade(c pttCase, got sdk.ParsedMedia, ok bool) map[string]bool {
	episode := got.Class == "episode"
	graded := map[string]bool{}
	defer func() {
		for field := range graded {
			graded[field] = graded[field] && ok
		}
	}()
	if c.Title != nil {
		graded["title"] = normalTitle(got.Title) == normalTitle(*c.Title)
	}
	if c.Year != nil {
		graded["year"] = got.Year == *c.Year
	}
	if c.Seasons != nil {
		if len(*c.Seasons) == 0 {
			graded["season"] = !episode || got.Sequence == ""
		} else {
			graded["season"] = episode && got.Sequence == strconv.Itoa((*c.Seasons)[0])
		}
	}
	if c.Episodes != nil {
		if want := *c.Episodes; len(want) == 0 {
			graded["episode"] = !episode || got.Index == 0 && got.AbsoluteOrdinal == 0
		} else {
			first, last := want[0], want[len(want)-1]
			end := got.IndexEnd
			if end == 0 {
				end = got.Index
			}
			graded["episode"] = episode && (got.Index == first && end == last ||
				first == last && got.AbsoluteOrdinal == first && end == got.Index)
		}
	}
	return graded
}

func readPTT(t *testing.T) pttFile {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "ptt", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file pttFile
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	if file.Version == "" || file.Commit == "" || !strings.Contains(file.License, "MIT") || file.Rule == "" || len(file.Cases) == 0 {
		t.Fatalf("testdata/ptt/cases.json lacks its version, commit, licence, rule or cases")
	}
	notice, err := os.ReadFile(filepath.Join("testdata", "ptt", "LICENSE"))
	if err != nil {
		t.Fatal(err)
	}
	if file.Notice != string(notice) || !strings.Contains(file.Notice, "Copyright (c) 2024 Spoked") {
		t.Fatalf("testdata/ptt/cases.json's license_text is not PTT's notice in testdata/ptt/LICENSE")
	}
	return file
}

func TestPTTCorpus(t *testing.T) {
	file := readPTT(t)
	marksPath := filepath.Join("testdata", "ptt", "unresolved.json")
	marks := map[string][]string{}
	if !*updatePTT {
		data, err := os.ReadFile(marksPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &marks); err != nil {
			t.Fatal(err)
		}
	}

	parser := filmtv.New()
	seen := map[string]bool{}
	pass, declared := map[string]int{}, map[string]int{}
	failing := map[string][]string{}
	for _, c := range file.Cases {
		if seen[c.Name] {
			t.Fatalf("case %q appears twice", c.Name)
		}
		seen[c.Name] = true
		got, ok := parser.Parse(pttPath(c.Name), sdk.ParseProbe{})
		for field, right := range pttGrade(c, got, ok) {
			declared[field]++
			marked := slices.Contains(marks[c.Name], field)
			switch {
			case right:
				pass[field]++
				if marked && !*updatePTT {
					t.Errorf("%q %s now parses; rerun with -update-ptt to drop the mark", c.Name, field)
				}
			default:
				failing[c.Name] = append(failing[c.Name], field)
				if !marked && !*updatePTT {
					t.Errorf("%q %s regressed: got %+v (ok=%v), want %s", c.Name, field, got, ok, pttWant(c, field))
				}
			}
		}
	}
	for name := range marks {
		if !seen[name] {
			t.Errorf("unresolved.json marks %q, which is not in cases.json; rerun with -update-ptt", name)
		}
	}

	floors := map[string]float64{"title": pttTitleFloor, "year": pttYearFloor, "season": pttSeasonFloor, "episode": pttEpisodeFloor}
	for _, field := range pttFields {
		if declared[field] == 0 {
			t.Errorf("no PTT case declares %s", field)
			continue
		}
		rate := float64(pass[field]) / float64(declared[field])
		t.Logf("PTT %s: %.4f (%d/%d), floor %.2f", field, rate, pass[field], declared[field], floors[field])
		if rate < floors[field] {
			t.Errorf("PTT %s rate %.4f is below its %.2f floor", field, rate, floors[field])
		}
	}

	if *updatePTT {
		for name := range failing {
			slices.Sort(failing[name])
		}
		var out bytes.Buffer
		enc := json.NewEncoder(&out)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", " ")
		if err := enc.Encode(failing); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(marksPath, out.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func pttWant(c pttCase, field string) string {
	switch field {
	case "title":
		return fmt.Sprintf("title %q", *c.Title)
	case "year":
		return fmt.Sprintf("year %d", *c.Year)
	case "season":
		return fmt.Sprintf("seasons %v", *c.Seasons)
	default:
		return fmt.Sprintf("episodes %v", *c.Episodes)
	}
}
