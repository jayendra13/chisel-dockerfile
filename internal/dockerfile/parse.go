package dockerfile

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
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
	result, err := parser.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("cannot parse Dockerfile: %w", err)
	}

	// Build a map from start line to the AST node's Original text,
	// so we can recover the raw source for each instruction.
	origByLine := buildOriginalLineMap(result.AST)

	stages, _, err := instructions.Parse(result.AST, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot parse Dockerfile instructions: %w", err)
	}

	if len(stages) == 0 {
		return nil, fmt.Errorf("no FROM instruction found")
	}

	info := &DockerfileInfo{}

	for _, bkStage := range stages {
		stage := Stage{
			BaseImage: bkStage.BaseName,
			Alias:     bkStage.Name,
		}

		// Detect Ubuntu base images.
		normalized := strings.ToLower(stage.BaseImage)
		if ver, ok := ubuntuVersions[normalized]; ok {
			stage.IsUbuntu = true
			stage.UbuntuVer = ver
		}

		for _, cmd := range bkStage.Commands {
			inst := Instruction{
				Directive: strings.ToUpper(cmd.Name()),
			}

			// Reconstruct the raw line from the AST node's original text.
			inst.Raw = rawFromLocation(cmd, origByLine)

			// For RUN commands, extract apt packages.
			if runCmd, ok := cmd.(*instructions.RunCommand); ok {
				body := extractRunBody(runCmd)
				inst.AptPackages = extractAptPackages(body)
				inst.IsAptInstall = len(inst.AptPackages) > 0
			}

			stage.Lines = append(stage.Lines, inst)
		}

		info.Stages = append(info.Stages, stage)
	}

	return info, nil
}

// buildOriginalLineMap walks the AST and maps each node's start line to
// its Original text.
func buildOriginalLineMap(ast *parser.Node) map[int]string {
	m := make(map[int]string)
	for _, child := range ast.Children {
		if child.Original != "" {
			m[child.StartLine] = child.Original
		}
	}
	return m
}

// rawFromLocation retrieves the original source line for a command using
// its location information.
func rawFromLocation(cmd instructions.Command, origByLine map[int]string) string {
	locs := cmd.Location()
	if len(locs) > 0 {
		if orig, ok := origByLine[locs[0].Start.Line]; ok {
			return orig
		}
	}
	return ""
}

// extractRunBody extracts the shell command string from a RunCommand.
func extractRunBody(cmd *instructions.RunCommand) string {
	args := cmd.CmdLine
	if len(args) == 0 {
		return ""
	}
	return strings.Join(args, " ")
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
