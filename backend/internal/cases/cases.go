// Package cases loads the scenario files in benchmark/cases/*.json.
//
// The same files are used by:
//   - the dashboard Simulator (replays a scenario as if it were a live call)
//   - the benchmark tool (measures recall, false alarms and time-to-detect)
//
// File format:
//
//	{
//	  "id": "digital-arrest-hinglish-01",
//	  "title": "Fake CBI officer, digital arrest",
//	  "label": "scam",                // or "legit"
//	  "language": "hi",
//	  "audio": "recordings/x.wav",    // optional: real phone recording (benchmark -audio)
//	  "lines": [ { "t": 0, "text": "..." }, ... ]
//	}
package cases

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"chaukanna/internal/scam"
)

// Case is one scenario.
type Case struct {
	ID       string      `json:"id"`
	Title    string      `json:"title"`
	Label    string      `json:"label"`              // scam | legit
	Category string      `json:"category,omitempty"` // true scam family, if known
	Language string      `json:"language"`
	Audio    string      `json:"audio,omitempty"`
	Lines    []scam.Line `json:"lines"`
}

// IsScam reports whether the case is labelled as a scam.
func (c Case) IsScam() bool { return strings.EqualFold(c.Label, "scam") }

// LoadDir reads every *.json file in dir, sorted by file name.
func LoadDir(dir string) ([]Case, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)

	var out []Case
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var c Case
		if err := json.Unmarshal(data, &c); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		if c.ID == "" {
			c.ID = strings.TrimSuffix(filepath.Base(f), ".json")
		}
		if c.Audio != "" && !filepath.IsAbs(c.Audio) {
			c.Audio = filepath.Join(filepath.Dir(dir), c.Audio) // relative to benchmark/
		}
		out = append(out, c)
	}
	return out, nil
}
