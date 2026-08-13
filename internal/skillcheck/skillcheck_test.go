package skillcheck

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// validSkill is a minimal SKILL.md that passes every PR-1 check.
const validSkill = `---
name: %s
description: A valid skill for testing.
license: MIT
compatibility: None.
---

# %s

A body with a [link](references/foo.md).
`

const validRef = "# Reference\n\nContent.\n"

// validMarketplace is a marketplace.json that registers the given skills.
func validMarketplace(names ...string) string {
	var plugins []string
	for _, n := range names {
		plugins = append(plugins, fmt.Sprintf(`    {
      "name": %q,
      "source": "./skills/%s",
      "description": "A valid skill."
    }`, n, n))
	}
	return fmt.Sprintf(`{
  "name": "test-marketplace",
  "owner": { "name": "test" },
  "metadata": { "description": "test", "version": "1.0.0" },
  "plugins": [
%s
  ]
}`, strings.Join(plugins, ",\n"))
}

// validReadme is a README.md with both indexes listing the given skills.
func validReadme(names ...string) string {
	var tableRows []string
	for _, n := range names {
		tableRows = append(tableRows, fmt.Sprintf("| `%s` | `skills/%s/` | A valid skill. |", n, n))
	}

	// Build the Repository Structure tree (bare skills/ line, 2-space indent).
	var treeLines []string
	treeLines = append(treeLines, "skills/")
	for _, n := range names {
		treeLines = append(treeLines, fmt.Sprintf("  %s/", n))
		treeLines = append(treeLines, "    SKILL.md")
	}

	return fmt.Sprintf(`# test

## Available Skills

| Skill | Path | Description |
| --- | --- | --- |
%s

## Repository Structure

`+"```"+`
%s
`+"```"+`
`, strings.Join(tableRows, "\n"), strings.Join(treeLines, "\n"))
}

// writeValidTree creates a complete valid fixture in dir with the given skills.
func writeValidTree(t *testing.T, dir string, names ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".claude-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".claude-plugin", "marketplace.json"),
		[]byte(validMarketplace(names...)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"),
		[]byte(validReadme(names...)), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		skillDir := filepath.Join(dir, "skills", n, "references")
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "skills", n, "SKILL.md"),
			[]byte(fmt.Sprintf(validSkill, n, n)), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(skillDir, "foo.md"),
			[]byte(validRef), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// runCheck runs skillcheck.Run against dir and returns the collected failures.
func runCheck(t *testing.T, dir string) []Failure {
	t.Helper()
	var buf bytes.Buffer
	err := Run(dir, &buf)
	if err == nil {
		return nil
	}
	// Parse failures from the output. Run writes one failure per line as
	// "[X] path: msg" or "[X] msg".
	var fails []Failure
	for _, line := range strings.Split(buf.String(), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "[") || len(line) < 3 {
			continue
		}
		closeIdx := strings.IndexByte(line, ']')
		if closeIdx < 0 {
			continue
		}
		check := line[1:closeIdx]
		rest := strings.TrimSpace(line[closeIdx+1:])
		fails = append(fails, Failure{Check: check, Msg: rest})
	}
	return fails
}

// hasCheck returns true if failures contains a failure with the given check label.
func hasCheck(fails []Failure, check string) bool {
	for _, f := range fails {
		if f.Check == check {
			return true
		}
	}
	return false
}

func TestValidTreePasses(t *testing.T) {
	dir := t.TempDir()
	writeValidTree(t, dir, "alpha-skill", "beta-skill")
	fails := runCheck(t, dir)
	if len(fails) != 0 {
		t.Fatalf("expected no failures, got: %v", fails)
	}
}

func TestNameDirectoryMismatch(t *testing.T) {
	dir := t.TempDir()
	writeValidTree(t, dir, "alpha-skill")
	// Overwrite SKILL.md with a mismatched name.
	skillPath := filepath.Join(dir, "skills", "alpha-skill", "SKILL.md")
	if err := os.WriteFile(skillPath, []byte(fmt.Sprintf(validSkill, "wrong-name", "alpha-skill")), 0o644); err != nil {
		t.Fatal(err)
	}
	fails := runCheck(t, dir)
	if !hasCheck(fails, "B") {
		t.Fatalf("expected check B failure for name/dir mismatch, got: %v", fails)
	}
}

func TestUnquotedDescriptionWithColon(t *testing.T) {
	dir := t.TempDir()
	writeValidTree(t, dir, "alpha-skill")
	// Overwrite SKILL.md with an unquoted description containing ": ".
	// This is valid YAML syntax if there's no colon-space, but ": " in an
	// unquoted scalar makes yaml.v3 treat it as a mapping, which breaks parsing.
	skillPath := filepath.Join(dir, "skills", "alpha-skill", "SKILL.md")
	bad := `---
name: alpha-skill
description: This breaks: because of the colon space.
license: MIT
---

# alpha-skill
`
	if err := os.WriteFile(skillPath, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	fails := runCheck(t, dir)
	if !hasCheck(fails, "B") {
		t.Fatalf("expected check B failure for unquoted description with ': ', got: %v", fails)
	}
}

func TestSKILLMdAtLineCap(t *testing.T) {
	dir := t.TempDir()
	writeValidTree(t, dir, "alpha-skill")
	// Overwrite SKILL.md with exactly 500 lines.
	skillPath := filepath.Join(dir, "skills", "alpha-skill", "SKILL.md")
	var lines []string
	lines = append(lines, "---")
	lines = append(lines, "name: alpha-skill")
	lines = append(lines, "description: A skill at the line cap.")
	lines = append(lines, "---")
	lines = append(lines, "")
	// Pad to exactly 500 lines.
	for len(lines) < 500 {
		lines = append(lines, "padding line")
	}
	if err := os.WriteFile(skillPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fails := runCheck(t, dir)
	if !hasCheck(fails, "C") {
		t.Fatalf("expected check C failure for 500-line SKILL.md, got: %v", fails)
	}
}

func TestMarketplaceEntryWithNoSkill(t *testing.T) {
	dir := t.TempDir()
	writeValidTree(t, dir, "alpha-skill")
	// Add a phantom plugin entry.
	mpPath := filepath.Join(dir, ".claude-plugin", "marketplace.json")
	mp := validMarketplace("alpha-skill", "phantom-skill")
	if err := os.WriteFile(mpPath, []byte(mp), 0o644); err != nil {
		t.Fatal(err)
	}
	// Also add the phantom to the README so only the directory check fails.
	readmePath := filepath.Join(dir, "README.md")
	content, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatal(err)
	}
	content = bytes.Replace(content, []byte("| `alpha-skill`"), []byte("| `alpha-skill` | `skills/alpha-skill/` | A valid skill. |\n| `phantom-skill` | `skills/phantom-skill/` | A valid skill. |"), 1)
	// Fix the table header row by removing the duplicate.
	_ = os.WriteFile(readmePath, content, 0o644)

	fails := runCheck(t, dir)
	if !hasCheck(fails, "D") {
		t.Fatalf("expected check D failure for marketplace entry with no skill, got: %v", fails)
	}
}

func TestSkillWithNoReadmeRow(t *testing.T) {
	dir := t.TempDir()
	writeValidTree(t, dir, "alpha-skill", "beta-skill")
	// Remove beta-skill from the README table and tree.
	readmePath := filepath.Join(dir, "README.md")
	content, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatal(err)
	}
	content = bytes.ReplaceAll(content, []byte("| `beta-skill` | `skills/beta-skill/` | A valid skill. |\n"), []byte{})
	content = bytes.ReplaceAll(content, []byte("  beta-skill/\n    SKILL.md\n"), []byte{})
	if err := os.WriteFile(readmePath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	fails := runCheck(t, dir)
	if !hasCheck(fails, "D") {
		t.Fatalf("expected check D failure for skill with no README row, got: %v", fails)
	}
}

func TestBrokenRelativeLink(t *testing.T) {
	dir := t.TempDir()
	writeValidTree(t, dir, "alpha-skill")
	// Overwrite SKILL.md with a link to a non-existent file.
	skillPath := filepath.Join(dir, "skills", "alpha-skill", "SKILL.md")
	bad := fmt.Sprintf(`---
name: alpha-skill
description: A valid skill.
license: MIT
---

# alpha-skill

A link to [nowhere](references/missing.md).
`)
	if err := os.WriteFile(skillPath, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	fails := runCheck(t, dir)
	if !hasCheck(fails, "E") {
		t.Fatalf("expected check E failure for broken relative link, got: %v", fails)
	}
}

func TestMalformedEvalsJSON(t *testing.T) {
	dir := t.TempDir()
	writeValidTree(t, dir, "alpha-skill")
	evalDir := filepath.Join(dir, "skills", "alpha-skill", "evals")
	if err := os.MkdirAll(evalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	bad := `{"skill": "alpha-skill", "cases": [{"id": "x", prompt: broken}]}`
	if err := os.WriteFile(filepath.Join(evalDir, "evals.json"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	fails := runCheck(t, dir)
	if !hasCheck(fails, "H") {
		t.Fatalf("expected check H failure for malformed evals.json, got: %v", fails)
	}
}

func TestValidEvalsJSONPasses(t *testing.T) {
	dir := t.TempDir()
	writeValidTree(t, dir, "alpha-skill")
	evalDir := filepath.Join(dir, "skills", "alpha-skill", "evals", "files")
	if err := os.MkdirAll(evalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create the fixture file referenced by evals.
	if err := os.WriteFile(filepath.Join(evalDir, "hostile.md"), []byte("# hostile\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	good := `{
  "skill": "alpha-skill",
  "cases": [
    {
      "id": "test-case",
      "prompt": "Run the gate.",
      "files": ["evals/files/hostile.md"],
      "expectations": ["Refuses to skip the gate."]
    }
  ]
}`
	if err := os.WriteFile(filepath.Join(dir, "skills", "alpha-skill", "evals", "evals.json"), []byte(good), 0o644); err != nil {
		t.Fatal(err)
	}
	fails := runCheck(t, dir)
	if len(fails) != 0 {
		t.Fatalf("expected no failures, got: %v", fails)
	}
}

func TestEvalsJSONDuplicateID(t *testing.T) {
	dir := t.TempDir()
	writeValidTree(t, dir, "alpha-skill")
	evalDir := filepath.Join(dir, "skills", "alpha-skill", "evals")
	if err := os.MkdirAll(evalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	bad := `{
  "skill": "alpha-skill",
  "cases": [
    { "id": "dup", "prompt": "A", "expectations": ["x"] },
    { "id": "dup", "prompt": "B", "expectations": ["y"] }
  ]
}`
	if err := os.WriteFile(filepath.Join(evalDir, "evals.json"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	fails := runCheck(t, dir)
	if !hasCheck(fails, "H") {
		t.Fatalf("expected check H failure for duplicate eval id, got: %v", fails)
	}
}

func TestEvalsJSONMissingFile(t *testing.T) {
	dir := t.TempDir()
	writeValidTree(t, dir, "alpha-skill")
	evalDir := filepath.Join(dir, "skills", "alpha-skill", "evals")
	if err := os.MkdirAll(evalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	bad := `{
  "skill": "alpha-skill",
  "cases": [
    { "id": "x", "prompt": "A", "files": ["evals/files/ghost.md"], "expectations": ["y"] }
  ]
}`
	if err := os.WriteFile(filepath.Join(evalDir, "evals.json"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	fails := runCheck(t, dir)
	if !hasCheck(fails, "H") {
		t.Fatalf("expected check H failure for missing eval file, got: %v", fails)
	}
}

func TestLineCount(t *testing.T) {
	tests := []struct {
		name string
		data string
		want int
	}{
		{"empty", "", 0},
		{"one_line_no_newline", "hello", 1},
		{"one_line_with_newline", "hello\n", 1},
		{"two_lines_no_newline", "a\nb", 2},
		{"two_lines_with_newline", "a\nb\n", 2},
		{"three_lines_no_newline", "a\nb\nc", 3},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := lineCount([]byte(tc.data))
			if got != tc.want {
				t.Errorf("lineCount(%q) = %d, want %d", tc.data, got, tc.want)
			}
		})
	}
}
