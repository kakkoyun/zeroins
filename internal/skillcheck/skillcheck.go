// Package skillcheck validates the skills/ directory against the repository's
// skill conventions. It is offline: no network, no cluster, no Helm.
//
// The checks are labelled A through H to match the plan that introduced them.
// PR 1 ships checks A–E and H; checks F (confirmation-gate identity) and
// G (version-pin agreement) are added once the content that makes them pass
// also lands.
package skillcheck

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Failure is a single check violation.
type Failure struct {
	Check string // check label, e.g. "A", "B"
	Path  string // repo-relative path, may be empty
	Msg   string // human-readable detail
}

func (f Failure) String() string {
	if f.Path != "" {
		return fmt.Sprintf("[%s] %s: %s", f.Check, f.Path, f.Msg)
	}
	return fmt.Sprintf("[%s] %s", f.Check, f.Msg)
}

// Skill holds the parsed state for one skill directory.
type Skill struct {
	Name        string                 // directory name
	Dir         string                 // repo-relative, e.g. skills/foo
	SkillPath   string                 // repo-relative, e.g. skills/foo/SKILL.md
	Content     []byte                 // raw SKILL.md bytes
	Frontmatter map[string]interface{} // parsed YAML frontmatter, nil if none
	FmText      string                 // raw frontmatter text between the fences
	LineCount   int                    // logical line count
}

// Run executes every enabled check against root and writes a summary to w.
// Returns an error when one or more checks fail.
func Run(root string, w io.Writer) error {
	skills, discoverFails := discoverSkills(root)
	marketplace, mpFails := loadMarketplace(root)
	readme, readmeFails := loadReadme(root)

	var fails []Failure
	fails = append(fails, discoverFails...)
	fails = append(fails, checkB(skills)...)
	fails = append(fails, checkC(skills)...)
	fails = append(fails, checkD(skills, marketplace, readme)...)
	fails = append(fails, mpFails...)
	fails = append(fails, readmeFails...)
	fails = append(fails, checkE(root, skills)...)
	fails = append(fails, checkH(root, skills)...)

	if len(fails) == 0 {
		fmt.Fprintf(w, "skillcheck: %d skill(s) validated.\n", len(skills))
		return nil
	}

	fmt.Fprintf(w, "skillcheck: %d problem(s).\n", len(fails))
	for _, f := range fails {
		fmt.Fprintln(w, f.String())
	}
	return fmt.Errorf("skillcheck: %d problem(s)", len(fails))
}

// discoverSkills finds every skills/*/SKILL.md and parses its frontmatter.
// Check A (file exists, uppercase filename) is applied here because it is the
// discovery gate: a directory without SKILL.md cannot be checked further.
func discoverSkills(root string) ([]Skill, []Failure) {
	skillsDir := filepath.Join(root, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil, []Failure{{Check: "A", Path: "skills/", Msg: "cannot read skills directory: " + err.Error()}}
	}

	var skills []Skill
	var fails []Failure
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		dir := filepath.Join("skills", name)
		skillPath := filepath.Join(dir, "SKILL.md")

		full := filepath.Join(root, skillPath)
		content, err := os.ReadFile(full)
		if err != nil {
			fails = append(fails, Failure{Check: "A", Path: skillPath, Msg: "SKILL.md not found (must be uppercase)"})
			continue
		}

		s := Skill{
			Name:      name,
			Dir:       dir,
			SkillPath: skillPath,
			Content:   content,
			LineCount: lineCount(content),
		}
		fm, fmText, fmErr := parseFrontmatter(content)
		if fmErr != nil {
			fails = append(fails, Failure{Check: "B", Path: skillPath, Msg: "frontmatter: " + fmErr.Error()})
		} else {
			s.Frontmatter = fm
			s.FmText = fmText
		}
		skills = append(skills, s)
	}

	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skills, fails
}

// lineCount counts logical lines the way awk 'END { print NR }' does: a
// final line without a trailing newline still counts. wc -l would report one
// fewer for such a file, letting a 500-line SKILL.md slip past the cap.
func lineCount(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	count := strings.Count(string(data), "\n")
	if !strings.HasSuffix(string(data), "\n") {
		count++
	}
	return count
}
