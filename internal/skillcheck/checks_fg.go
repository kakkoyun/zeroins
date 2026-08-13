package skillcheck

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// gateBearingSkills is the constant set of skills that can mutate a cluster
// and therefore must carry the confirmation gate. Check F fails when a listed
// skill lacks the file and when an unlisted skill has one — otherwise deleting
// the file would be a way to pass.
var gateBearingSkills = []string{
	"collect-go-telemetry",
	"zeroins-obi-attach",
	"zeroins-ebpf-profiler-attach",
	"profile-go-with-perfgo",
}

// canonicalGatePath is the designated canonical copy of the confirmation gate.
const canonicalGateRel = "skills/collect-go-telemetry/references/confirmation-gate.md"

// checkF validates the confirmation gate:
//   - All references/confirmation-gate.md copies are byte-identical to the
//     canonical one (skills/collect-go-telemetry/references/confirmation-gate.md).
//   - Every gate-bearing SKILL.md links to it.
//   - That link precedes the first fenced block containing attach or detach.
//   - A non-gate-bearing skill must NOT have the file.
func checkF(root string, skills []Skill) []Failure {
	gateSet := make(map[string]bool, len(gateBearingSkills))
	for _, n := range gateBearingSkills {
		gateSet[n] = true
	}

	// Determine whether any gate-bearing skills are present.
	hasGateBearing := false
	for _, s := range skills {
		if gateSet[s.Name] {
			hasGateBearing = true
			break
		}
	}

	var fails []Failure

	// The canonical gate must exist when any gate-bearing skill is present.
	var canon []byte
	if hasGateBearing {
		canonPath := filepath.Join(root, filepath.FromSlash(canonicalGateRel))
		var err error
		canon, err = os.ReadFile(canonPath)
		if err != nil {
			return []Failure{{Check: "F", Path: canonicalGateRel,
				Msg: "canonical confirmation-gate.md not found: " + err.Error()}}
		}
	}

	for _, s := range skills {
		gateRel := filepath.Join(s.Dir, "references", "confirmation-gate.md")
		gateAbs := filepath.Join(root, gateRel)
		isGateBearing := gateSet[s.Name]

		if isGateBearing {
			content, err := os.ReadFile(gateAbs)
			if err != nil {
				fails = append(fails, Failure{Check: "F", Path: gateRel,
					Msg: "gate-bearing skill is missing references/confirmation-gate.md"})
				continue
			}
			if !bytes.Equal(content, canon) {
				fails = append(fails, Failure{Check: "F", Path: gateRel,
					Msg: "confirmation-gate.md differs from the canonical copy"})
			}
			if orderErr := checkGateLinkOrder(s); orderErr != nil {
				fails = append(fails, Failure{Check: "F", Path: s.SkillPath,
					Msg: orderErr.Error()})
			}
		} else {
			// Non-gate-bearing skills must NOT have the file. This check
			// runs regardless of whether gate-bearing skills exist.
			if _, err := os.Stat(gateAbs); err == nil {
				fails = append(fails, Failure{Check: "F", Path: gateRel,
					Msg: "non-gate-bearing skill has references/confirmation-gate.md"})
			}
		}
	}
	return fails
}

// checkGateLinkOrder verifies that the SKILL.md links to
// references/confirmation-gate.md before the first fenced code block containing
// attach or detach.
func checkGateLinkOrder(s Skill) error {
	content := string(s.Content)

	gateLinkIdx := strings.Index(content, "references/confirmation-gate.md")
	if gateLinkIdx < 0 {
		return fmt.Errorf("SKILL.md does not link to references/confirmation-gate.md")
	}

	lines := strings.Split(content, "\n")
	inFence := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if inFence && (strings.Contains(line, "attach") || strings.Contains(line, "detach")) {
			// Compute the character offset of this line.
			charOffset := 0
			for j := 0; j < i; j++ {
				charOffset += len(lines[j]) + 1 // +1 for \n
			}
			if gateLinkIdx >= charOffset {
				return fmt.Errorf("link to references/confirmation-gate.md must precede the first fenced block containing attach or detach")
			}
			return nil
		}
	}
	return nil
}

// versionPattern matches version-like strings: v1.0.1, 0.166.0, v0.0.202632.
var versionPattern = regexp.MustCompile(`v?\d+\.\d+\.\d+`)

// checkG validates that version pins quoted anywhere under skills/** agree
// with the README "Version pins" table.
func checkG(root string, skills []Skill) []Failure {
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		return nil // README read failure is caught by check D
	}
	knownPins := extractReadmePins(string(readme))
	if len(knownPins) == 0 {
		return nil // no pins table found; nothing to check
	}

	var fails []Failure
	for _, s := range skills {
		err := filepath.WalkDir(filepath.Join(root, s.Dir), func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || !strings.HasSuffix(path, ".md") {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			seen := make(map[string]bool)
			for _, m := range versionPattern.FindAllString(string(content), -1) {
				if seen[m] {
					continue
				}
				seen[m] = true
				normalized := strings.TrimPrefix(m, "v")
				if !knownPins[normalized] {
					fails = append(fails, Failure{Check: "G", Path: relPath(root, path),
						Msg: fmt.Sprintf("version %q does not match any README pin", m)})
				}
			}
			return nil
		})
		if err != nil {
			fails = append(fails, Failure{Check: "G", Path: s.Dir, Msg: "walk error: " + err.Error()})
		}
	}
	return fails
}

// extractReadmePins parses the README "Version pins" table and returns the
// set of normalized (v-prefix stripped) pin values.
func extractReadmePins(readme string) map[string]bool {
	pins := make(map[string]bool)
	inPinsSection := false

	for _, line := range strings.Split(readme, "\n") {
		if strings.HasPrefix(line, "## Version pins") {
			inPinsSection = true
			continue
		}
		if inPinsSection && strings.HasPrefix(line, "## ") {
			break
		}
		if !inPinsSection {
			continue
		}
		// Skip header and separator rows.
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "| ---") || strings.HasPrefix(trimmed, "| Contract") {
			continue
		}
		if !strings.HasPrefix(trimmed, "|") {
			continue
		}
		// Extract the right column (last |...| before trailing whitespace).
		parts := strings.Split(trimmed, "|")
		if len(parts) < 3 {
			continue
		}
		pinCol := parts[len(parts)-2]
		for _, m := range versionPattern.FindAllString(pinCol, -1) {
			pins[strings.TrimPrefix(m, "v")] = true
		}
	}
	return pins
}
