package skillcheck

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// MarketplacePlugin is one entry in .claude-plugin/marketplace.json.
type MarketplacePlugin struct {
	Name        string `json:"name"`
	Source      string `json:"source"`
	Description string `json:"description"`
}

// MarketplaceFile is the parsed .claude-plugin/marketplace.json.
type MarketplaceFile struct {
	Name    string              `json:"name"`
	Plugins []MarketplacePlugin `json:"plugins"`
}

// ReadmeIndexes holds the skill names parsed from the two README indexes.
type ReadmeIndexes struct {
	TableRows   []string // from the Available Skills table
	TreeEntries []string // from the Repository Structure tree
}

// loadMarketplace reads and parses .claude-plugin/marketplace.json.
func loadMarketplace(root string) (*MarketplaceFile, []Failure) {
	path := filepath.Join(root, ".claude-plugin", "marketplace.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, []Failure{{Check: "D", Path: ".claude-plugin/marketplace.json",
			Msg: "cannot read: " + err.Error()}}
	}
	var mp MarketplaceFile
	if err := json.Unmarshal(data, &mp); err != nil {
		return nil, []Failure{{Check: "D", Path: ".claude-plugin/marketplace.json",
			Msg: "JSON parse: " + err.Error()}}
	}
	return &mp, nil
}

// loadReadme parses the two skill indexes from README.md.
func loadReadme(root string) (*ReadmeIndexes, []Failure) {
	path := filepath.Join(root, "README.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, []Failure{{Check: "D", Path: "README.md",
			Msg: "cannot read: " + err.Error()}}
	}
	text := string(data)
	return &ReadmeIndexes{
		TableRows:   parseReadmeTable(text),
		TreeEntries: parseReadmeTree(text),
	}, nil
}

// tableRowPattern matches `| `skill-name` | `skills/skill-name/` |`, capturing both
// the name and the path so the caller can verify they agree. Go's regexp does
// not support backreferences, so the identity check is done in Go.
var tableRowPattern = regexp.MustCompile("(?m)^\\| `([a-z0-9-]+)` \\| `skills/([a-z0-9-]+)/` \\|")

// parseReadmeTable extracts skill names from the Available Skills table.
// A row counts only when the name in the first column matches the directory
// in the second.
func parseReadmeTable(readme string) []string {
	matches := tableRowPattern.FindAllStringSubmatch(readme, -1)
	var names []string
	for _, m := range matches {
		if m[1] == m[2] {
			names = append(names, m[1])
		}
	}
	return names
}

// parseReadmeTree extracts skill names from the Repository Structure tree.
// It finds the `skills/` line and collects entries indented by exactly two
// spaces, stopping at the first non-blank, non-indented line.
func parseReadmeTree(readme string) []string {
	var entries []string
	inSkillsBlock := false
	for _, line := range strings.Split(readme, "\n") {
		trimmed := strings.TrimRight(line, "\r")
		if trimmed == "skills/" {
			inSkillsBlock = true
			continue
		}
		if !inSkillsBlock {
			continue
		}
		// Block ends at the first non-blank line that is not indented.
		if strings.TrimSpace(trimmed) != "" && !strings.HasPrefix(trimmed, "  ") {
			break
		}
		// Exactly two spaces of indent is a skill directory; deeper lines are
		// its contents (references/, SKILL.md, etc.).
		if len(trimmed) >= 3 && trimmed[0] == ' ' && trimmed[1] == ' ' && trimmed[2] != ' ' {
			rest := trimmed[2:]
			name := rest
			if idx := strings.IndexByte(name, '/'); idx >= 0 {
				name = name[:idx]
			}
			if name != "" {
				entries = append(entries, name)
			}
		}
	}
	return entries
}

// checkD validates the registration triple: every skill must appear in the
// directory, the marketplace plugins array, and both README indexes. Diffed
// in both directions so a stale entry for a deleted skill also fails.
func checkD(skills []Skill, mp *MarketplaceFile, readme *ReadmeIndexes) []Failure {
	var fails []Failure

	skillNames := make(map[string]bool, len(skills))
	for _, s := range skills {
		skillNames[s.Name] = true
	}

	// Marketplace checks.
	if mp != nil {
		pluginNames := make(map[string]bool)
		for _, p := range mp.Plugins {
			if p.Name == "" {
				fails = append(fails, Failure{Check: "D", Path: ".claude-plugin/marketplace.json",
					Msg: "plugin with empty name"})
			}
			if pluginNames[p.Name] {
				fails = append(fails, Failure{Check: "D", Path: ".claude-plugin/marketplace.json",
					Msg: fmt.Sprintf("duplicate plugin name %q", p.Name)})
			}
			pluginNames[p.Name] = true

			if skillNames[p.Name] && p.Source != "./skills/"+p.Name {
				fails = append(fails, Failure{Check: "D", Path: ".claude-plugin/marketplace.json",
					Msg: fmt.Sprintf("plugin %q has source %q; expected %q", p.Name, p.Source, "./skills/"+p.Name)})
			}
			if p.Description == "" {
				fails = append(fails, Failure{Check: "D", Path: ".claude-plugin/marketplace.json",
					Msg: fmt.Sprintf("plugin %q has no description", p.Name)})
			}
		}

		// Direction 1: skill with no marketplace entry.
		for name := range skillNames {
			if !pluginNames[name] {
				fails = append(fails, Failure{Check: "D", Path: ".claude-plugin/marketplace.json",
					Msg: fmt.Sprintf("missing plugin entry for %q", name)})
			}
		}
		// Direction 2: marketplace entry with no skill.
		for name := range pluginNames {
			if !skillNames[name] {
				fails = append(fails, Failure{Check: "D", Path: ".claude-plugin/marketplace.json",
					Msg: fmt.Sprintf("plugin %q has no skill directory", name)})
			}
		}
	}

	// README index checks (both indexes, both directions).
	if readme != nil {
		for _, label := range []struct {
			name string
			rows []string
		}{
			{"Available Skills table", readme.TableRows},
			{"Repository Structure tree", readme.TreeEntries},
		} {
			seen := make(map[string]bool)
			var dups []string
			for _, name := range label.rows {
				if seen[name] {
					dups = append(dups, name)
				}
				seen[name] = true
			}
			if len(dups) > 0 {
				fails = append(fails, Failure{Check: "D", Path: "README.md",
					Msg: fmt.Sprintf("%s lists %v more than once", label.name, dups)})
			}
			for name := range skillNames {
				if !seen[name] {
					fails = append(fails, Failure{Check: "D", Path: "README.md",
						Msg: fmt.Sprintf("missing %s entry for %q", label.name, name)})
				}
			}
			for name := range seen {
				if !skillNames[name] {
					fails = append(fails, Failure{Check: "D", Path: "README.md",
						Msg: fmt.Sprintf("%s lists %q, which is not a directory under skills/", label.name, name)})
				}
			}
		}
	}

	return fails
}

// linkPattern matches inline Markdown links: [text](destination).
var linkPattern = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)

// checkE verifies that every relative Markdown link under skills/** resolves
// to an existing file. External links (http://, https://, mailto:, etc.) and
// pure-anchor links (#section) are skipped.
func checkE(root string, skills []Skill) []Failure {
	var fails []Failure
	for _, s := range skills {
		// Walk every .md file in the skill directory.
		err := filepath.WalkDir(filepath.Join(root, s.Dir), func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || !strings.HasSuffix(path, ".md") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				fails = append(fails, Failure{Check: "E", Path: relPath(root, path), Msg: "cannot read: " + err.Error()})
				return nil
			}
			for _, m := range linkPattern.FindAllStringSubmatch(string(data), -1) {
				dest := strings.TrimSpace(m[2])
				if isExternal(dest) || isAnchorOnly(dest) {
					continue
				}
				// Strip any anchor suffix.
				if idx := strings.IndexByte(dest, '#'); idx >= 0 {
					dest = dest[:idx]
				}
				if dest == "" {
					continue
				}
				resolved := filepath.Join(filepath.Dir(path), dest)
				resolved = filepath.Clean(resolved)
				if _, err := os.Stat(resolved); err != nil {
					rel := relPath(root, path)
					fails = append(fails, Failure{Check: "E", Path: rel,
						Msg: fmt.Sprintf("broken link: %q (from %q)", m[2], rel)})
				}
			}
			return nil
		})
		if err != nil {
			fails = append(fails, Failure{Check: "E", Path: s.Dir, Msg: "walk error: " + err.Error()})
		}
	}
	sort.Slice(fails, func(i, j int) bool {
		return fails[i].Path+fails[i].Msg < fails[j].Path+fails[j].Msg
	})
	return fails
}

func isExternal(dest string) bool {
	return strings.HasPrefix(dest, "http://") ||
		strings.HasPrefix(dest, "https://") ||
		strings.HasPrefix(dest, "mailto:") ||
		strings.HasPrefix(dest, "ftp://") ||
		strings.HasPrefix(dest, "slack://")
}

func isAnchorOnly(dest string) bool {
	return strings.HasPrefix(dest, "#")
}

func relPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return rel
}
