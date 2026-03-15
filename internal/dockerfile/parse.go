package dockerfile

import (
	"fmt"
	"strings"
)

// DockerfileInfo holds the parsed representation of a Dockerfile.
type DockerfileInfo struct {
	Stages []Stage
}

// Stage represents a single FROM ... block in a Dockerfile.
type Stage struct {
	BaseImage string
	Alias     string
	IsUbuntu  bool
	UbuntuVer string
	Lines     []Instruction
}

// Instruction represents a single Dockerfile instruction.
type Instruction struct {
	Raw          string
	Directive    string
	IsAptInstall bool
	AptPackages  []string
}

// ubuntuVersions maps known base image tags to Ubuntu version numbers.
var ubuntuVersions = map[string]string{
	"ubuntu:24.04":  "24.04",
	"ubuntu:noble":  "24.04",
	"ubuntu:22.04":  "22.04",
	"ubuntu:jammy":  "22.04",
	"ubuntu:20.04":  "20.04",
	"ubuntu:focal":  "20.04",
	"ubuntu:latest": "24.04",
}

// Parse parses raw Dockerfile content into a DockerfileInfo.
func Parse(data []byte) (*DockerfileInfo, error) {
	lines := joinContinuationLines(string(data))
	info := &DockerfileInfo{}
	var current *Stage

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		directive := strings.ToUpper(strings.Fields(trimmed)[0])

		if directive == "FROM" {
			stage := parseFrom(trimmed)
			info.Stages = append(info.Stages, stage)
			current = &info.Stages[len(info.Stages)-1]
			continue
		}

		if current == nil {
			return nil, fmt.Errorf("instruction before FROM: %s", trimmed)
		}

		inst := Instruction{
			Raw:       line,
			Directive: directive,
		}

		if directive == "RUN" {
			body := trimmed[len("RUN"):]
			inst.AptPackages = extractAptPackages(body)
			inst.IsAptInstall = len(inst.AptPackages) > 0
		}

		current.Lines = append(current.Lines, inst)
	}

	if len(info.Stages) == 0 {
		return nil, fmt.Errorf("no FROM instruction found")
	}

	return info, nil
}

// parseFrom parses a FROM line into a Stage.
func parseFrom(line string) Stage {
	fields := strings.Fields(line)
	s := Stage{
		BaseImage: fields[1],
	}
	for i, f := range fields {
		if strings.EqualFold(f, "AS") && i+1 < len(fields) {
			s.Alias = fields[i+1]
		}
	}
	// Detect Ubuntu base images.
	normalized := strings.ToLower(s.BaseImage)
	if ver, ok := ubuntuVersions[normalized]; ok {
		s.IsUbuntu = true
		s.UbuntuVer = ver
	}
	return s
}

// joinContinuationLines joins backslash-continuation lines.
func joinContinuationLines(content string) []string {
	var result []string
	var buf strings.Builder
	for _, raw := range strings.Split(content, "\n") {
		trimmed := strings.TrimRight(raw, " \t")
		if strings.HasSuffix(trimmed, "\\") {
			buf.WriteString(strings.TrimSuffix(trimmed, "\\"))
			buf.WriteString(" ")
			continue
		}
		buf.WriteString(raw)
		result = append(result, buf.String())
		buf.Reset()
	}
	if buf.Len() > 0 {
		result = append(result, buf.String())
	}
	return result
}

// extractAptPackages extracts package names from a RUN command body
// containing apt-get install or apt install.
func extractAptPackages(body string) []string {
	var packages []string
	// Split on && and ; to isolate individual commands.
	cmds := splitCommands(body)
	for _, cmd := range cmds {
		cmd = strings.TrimSpace(cmd)
		if !isAptInstall(cmd) {
			continue
		}
		// Extract tokens after "install".
		idx := strings.Index(strings.ToLower(cmd), "install")
		if idx < 0 {
			continue
		}
		rest := cmd[idx+len("install"):]
		for _, token := range strings.Fields(rest) {
			if strings.HasPrefix(token, "-") {
				continue
			}
			// Strip version pinning (e.g. "pkg=1.0").
			if i := strings.Index(token, "="); i >= 0 {
				token = token[:i]
			}
			if token != "" {
				packages = append(packages, token)
			}
		}
	}
	return packages
}

// splitCommands splits a shell command on && and ;.
func splitCommands(s string) []string {
	var parts []string
	s = strings.ReplaceAll(s, "&&", "\x00")
	s = strings.ReplaceAll(s, ";", "\x00")
	for _, p := range strings.Split(s, "\x00") {
		parts = append(parts, strings.TrimSpace(p))
	}
	return parts
}

// isAptInstall reports whether a command is an apt-get/apt install.
func isAptInstall(cmd string) bool {
	lower := strings.ToLower(cmd)
	return strings.Contains(lower, "apt-get install") ||
		strings.Contains(lower, "apt install")
}
