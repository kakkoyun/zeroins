package skillcheck

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// EvalCase is one case in evals/evals.json.
type EvalCase struct {
	ID           string   `json:"id"`
	Prompt       string   `json:"prompt"`
	Files        []string `json:"files"`
	Expectations []string `json:"expectations"`
}

// EvalsFile is the parsed evals/evals.json.
type EvalsFile struct {
	Skill string     `json:"skill"`
	Cases []EvalCase `json:"cases"`
}

// checkH validates evals/evals.json when present. The schema mirrors the
// upstream ollygarden/opentelemetry-agent-skills format: a top-level skill
// name and a cases array, each case with a unique non-empty id, a non-empty
// prompt, at least one expectation, and every files[] path resolving.
func checkH(root string, skills []Skill) []Failure {
	var fails []Failure
	for _, s := range skills {
		evalPath := filepath.Join(root, s.Dir, "evals", "evals.json")
		data, err := os.ReadFile(evalPath)
		if err != nil {
			// evals.json is optional; only not-found errors are skipped.
			// Other read errors (permissions, I/O) are reported so the gate
			// cannot be bypassed by making the file unreadable.
			if os.IsNotExist(err) {
				continue
			}
			fails = append(fails, Failure{Check: "H", Path: relPath(root, evalPath),
				Msg: "cannot read: " + err.Error()})
			continue
		}

		var evals EvalsFile
		if err := json.Unmarshal(data, &evals); err != nil {
			fails = append(fails, Failure{Check: "H", Path: relPath(root, evalPath),
				Msg: "JSON parse: " + err.Error()})
			continue
		}

		// Validate the top-level skill field matches the containing skill.
		if evals.Skill != s.Name {
			fails = append(fails, Failure{Check: "H", Path: relPath(root, evalPath),
				Msg: fmt.Sprintf("skill field %q does not match skill directory %q", evals.Skill, s.Name)})
		}

		seenIDs := make(map[string]bool)
		for i, c := range evals.Cases {
			caseLabel := fmt.Sprintf("case[%d]", i)
			if c.ID != "" {
				caseLabel = "case " + c.ID
			}
			if c.ID == "" {
				fails = append(fails, Failure{Check: "H", Path: relPath(root, evalPath),
					Msg: fmt.Sprintf("%s: id is empty", caseLabel)})
			}
			if seenIDs[c.ID] {
				fails = append(fails, Failure{Check: "H", Path: relPath(root, evalPath),
					Msg: fmt.Sprintf("duplicate case id %q", c.ID)})
			}
			seenIDs[c.ID] = true

			if c.Prompt == "" {
				fails = append(fails, Failure{Check: "H", Path: relPath(root, evalPath),
					Msg: fmt.Sprintf("%s: prompt is empty", caseLabel)})
			}
			if len(c.Expectations) == 0 {
				fails = append(fails, Failure{Check: "H", Path: relPath(root, evalPath),
					Msg: fmt.Sprintf("%s: must have at least one expectation", caseLabel)})
			}
			for _, f := range c.Files {
				filePath := filepath.Join(root, s.Dir, f)
				if _, err := os.Stat(filePath); err != nil {
					fails = append(fails, Failure{Check: "H", Path: relPath(root, evalPath),
						Msg: fmt.Sprintf("%s: files path %q does not exist", caseLabel, f)})
				}
			}
		}
	}
	return fails
}
