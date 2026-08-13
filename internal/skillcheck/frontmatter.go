package skillcheck

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// allowedFrontmatterKeys is the closed set of frontmatter keys a skill may use.
var allowedFrontmatterKeys = map[string]bool{
	"name":          true,
	"description":   true,
	"license":       true,
	"compatibility": true,
	"metadata":      true,
}

var namePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// parseFrontmatter extracts and parses the YAML frontmatter delimited by
// leading and trailing `---` lines. Returns the decoded map, the raw text
// between the fences, and an error if the frontmatter is malformed.
func parseFrontmatter(content []byte) (map[string]interface{}, string, error) {
	text := string(content)
	if !strings.HasPrefix(text, "---\n") && text != "---" && !strings.HasPrefix(text, "---\r\n") {
		return nil, "", fmt.Errorf("missing leading --- fence")
	}

	// Find the closing fence. The opening fence is line 1; the closing fence
	// is the next line that is exactly "---" (optionally with a trailing
	// carriage return).
	lines := strings.Split(text, "\n")
	if len(lines) < 2 {
		return nil, "", fmt.Errorf("frontmatter has no closing fence")
	}

	closeIdx := -1
	for i := 1; i < len(lines); i++ {
		trimmed := strings.TrimRight(lines[i], "\r")
		if trimmed == "---" {
			closeIdx = i
			break
		}
	}
	if closeIdx == -1 {
		return nil, "", fmt.Errorf("frontmatter has no closing --- fence")
	}

	fmText := strings.Join(lines[1:closeIdx], "\n")

	var raw map[string]interface{}
	if err := yaml.Unmarshal([]byte(fmText), &raw); err != nil {
		return nil, fmText, fmt.Errorf("YAML parse: %w", err)
	}
	if raw == nil {
		raw = map[string]interface{}{}
	}
	return raw, fmText, nil
}

// frontmatterString extracts a scalar string value from the frontmatter map.
// Handles yaml.v3's decoding of block scalars (| and >) and quoted strings.
func frontmatterString(raw map[string]interface{}, key string) (string, bool) {
	val, ok := raw[key]
	if !ok {
		return "", false
	}
	switch v := val.(type) {
	case string:
		return strings.TrimSpace(v), true
	case int:
		return fmt.Sprintf("%d", v), true
	case bool:
		return fmt.Sprintf("%t", v), true
	default:
		// yaml.v3 decodes block scalars as strings, but fall back to the
		// raw text if the type is unexpected.
		return strings.TrimSpace(fmt.Sprintf("%v", v)), true
	}
}

// checkB validates frontmatter structure and field constraints.
func checkB(skills []Skill) []Failure {
	var fails []Failure
	for _, s := range skills {
		if s.Frontmatter == nil {
			// parseFrontmatter already reported the parse error during discovery.
			continue
		}

		// Key restriction.
		for key := range s.Frontmatter {
			if !allowedFrontmatterKeys[key] {
				fails = append(fails, Failure{Check: "B", Path: s.SkillPath,
					Msg: fmt.Sprintf("frontmatter key %q is not allowed (permitted: name, description, license, compatibility, metadata)", key)})
			}
		}

		// name
		name, hasName := frontmatterString(s.Frontmatter, "name")
		if !hasName || name == "" {
			fails = append(fails, Failure{Check: "B", Path: s.SkillPath, Msg: "frontmatter: name is required"})
		} else {
			if name != s.Name {
				fails = append(fails, Failure{Check: "B", Path: s.SkillPath,
					Msg: fmt.Sprintf("frontmatter name %q does not match directory %q", name, s.Name)})
			}
			if len(name) > 64 {
				fails = append(fails, Failure{Check: "B", Path: s.SkillPath,
					Msg: fmt.Sprintf("frontmatter name is %d chars, must be <= 64", len(name))})
			}
			if !namePattern.MatchString(name) {
				fails = append(fails, Failure{Check: "B", Path: s.SkillPath,
					Msg: fmt.Sprintf("frontmatter name %q must match ^[a-z0-9]+(-[a-z0-9]+)*$", name)})
			}
		}

		// description
		desc, hasDesc := frontmatterString(s.Frontmatter, "description")
		if !hasDesc || desc == "" {
			fails = append(fails, Failure{Check: "B", Path: s.SkillPath, Msg: "frontmatter: description is required and must be non-empty"})
		} else if len(desc) > 1024 {
			fails = append(fails, Failure{Check: "B", Path: s.SkillPath,
				Msg: fmt.Sprintf("frontmatter description is %d chars, must be <= 1024", len(desc))})
		}

		// compatibility
		if compat, ok := frontmatterString(s.Frontmatter, "compatibility"); ok && len(compat) > 500 {
			fails = append(fails, Failure{Check: "B", Path: s.SkillPath,
				Msg: fmt.Sprintf("frontmatter compatibility is %d chars, must be <= 500", len(compat))})
		}
	}
	return fails
}

// checkC enforces the under-500-line cap on SKILL.md.
func checkC(skills []Skill) []Failure {
	var fails []Failure
	for _, s := range skills {
		if s.LineCount >= 500 {
			fails = append(fails, Failure{Check: "C", Path: s.SkillPath,
				Msg: fmt.Sprintf("SKILL.md is %d lines, must be under 500", s.LineCount)})
		}
	}
	return fails
}
