// Command exportcalls turns recorded test calls into benchmark scenario files,
// so real calls can be replayed and measured later.
//
//	cd backend
//	go run ./cmd/exportcalls -scam 14,15 -legit 16              # -> ../benchmark/holdout/
//	go run ./cmd/exportcalls -scam 20 -out ../benchmark/cases   # into the tuning set instead
//	go run ./cmd/exportcalls -labelled -out ../benchmark/fresh  # every call labelled in the dashboard
//	go run ./cmd/exportcalls -labelled -from 28 -to 77 -out ../benchmark/scripted  # only those call numbers
//
// It reads the calls from the database configured in backend/.env and keeps
// only what the other side said (Chaukanna's own lines are dropped). You label
// each call yourself: the label is the ground truth the benchmark checks.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"chaukanna/internal/cases"
	"chaukanna/internal/config"
	"chaukanna/internal/database"
	"chaukanna/internal/models"
	"chaukanna/internal/scam"
)

func main() {
	scamIDs := flag.String("scam", "", "comma-separated call ids that WERE scams")
	legitIDs := flag.String("legit", "", "comma-separated call ids that were normal calls")
	labelled := flag.Bool("labelled", false, "export every call labelled in the dashboard, with its scam type")
	fromID := flag.Uint("from", 0, "with -labelled: first call number to include")
	toID := flag.Uint("to", 0, "with -labelled: last call number to include (0 = no limit)")
	out := flag.String("out", "../benchmark/holdout", "folder to write the scenario files to")
	flag.Parse()
	if *scamIDs == "" && *legitIDs == "" && !*labelled {
		log.Fatal("give at least one call id, e.g. -scam 14,15 -legit 16, or use -labelled")
	}

	cfg := config.Load()
	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}

	written := 0
	if *labelled {
		var calls []models.Call
		query := db.Where("test_label IN ? AND id >= ?", []string{"scam", "legit"}, *fromID)
		if *toID > 0 {
			query = query.Where("id <= ?", *toID)
		}
		query.Order("id").Find(&calls)
		for _, call := range calls {
			path, err := export(db, call.ID, call.TestLabel, *out)
			if err != nil {
				log.Printf("❌ call #%d: %v", call.ID, err)
				continue
			}
			fmt.Printf("✅ call #%d (%s %s) -> %s\n", call.ID, call.TestLabel, call.TestCategory, path)
			written++
		}
	}
	for _, group := range []struct{ label, ids string }{{"scam", *scamIDs}, {"legit", *legitIDs}} {
		for _, raw := range strings.Split(group.ids, ",") {
			raw = strings.TrimPrefix(strings.TrimSpace(raw), "#")
			if raw == "" {
				continue
			}
			id, err := strconv.Atoi(raw)
			if err != nil {
				log.Fatalf("bad call id %q", raw)
			}
			path, err := export(db, uint(id), group.label, *out)
			if err != nil {
				log.Printf("❌ call #%d: %v", id, err)
				continue
			}
			fmt.Printf("✅ call #%d (%s) -> %s\n", id, group.label, path)
			written++
		}
	}
	fmt.Printf("\n%d call(s) written to %s\n", written, *out)
}

// export writes one call as a scenario JSON file and returns its path.
func export(db *gorm.DB, id uint, label, dir string) (string, error) {
	var call models.Call
	err := db.Preload("Transcript", func(q *gorm.DB) *gorm.DB { return q.Order("time_sec, id") }).
		First(&call, id).Error
	if err != nil {
		return "", fmt.Errorf("not found")
	}

	var lines []scam.Line
	for _, l := range call.Transcript {
		if l.Speaker == "chaukanna" || l.Speaker == "family" {
			continue // only what was said on the call, not what we or the family typed into it
		}
		lines = append(lines, scam.Line{T: l.TimeSec, Text: l.Text})
	}
	if len(lines) == 0 {
		return "", fmt.Errorf("no transcript saved (was STORE_TRANSCRIPTS=all when the call was made?)")
	}

	title := lines[0].Text
	for _, l := range lines { // first line with some substance makes a better title
		if len([]rune(l.Text)) > 15 {
			title = l.Text
			break
		}
	}
	if r := []rune(title); len(r) > 70 {
		title = string(r[:70]) + "…"
	}

	c := cases.Case{
		ID:       fmt.Sprintf("call-%03d-%s", call.ID, label),
		Title:    fmt.Sprintf("Real %s call #%d (%s): %s", call.Provider, call.ID, call.Mode, title),
		Label:    label,
		Category: call.TestCategory,
		Language: scam.BaseLanguage(call.Language),
		Lines:    lines,
	}
	data, _ := json.MarshalIndent(c, "", "  ")
	path := filepath.Join(dir, c.ID+".json")
	return path, os.WriteFile(path, append(data, '\n'), 0o644)
}
