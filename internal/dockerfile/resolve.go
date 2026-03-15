package dockerfile

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// ResolveOptions configures the slice resolution.
type ResolveOptions struct {
	Release string
	Mode    string // "runtime", "tools", "auto"
	Info    *DockerfileInfo
}

// ResolveResult holds the resolved slices and any issues.
type ResolveResult struct {
	Slices   []string
	Missing  []string
	Warnings []string
	Release  string
	Mode     string
}

// buildOnlyPackages are packages that belong in the build stage only.
var buildOnlyPackages = map[string]bool{
	"build-essential": true,
	"gcc":             true,
	"g++":             true,
	"make":            true,
	"cmake":           true,
	"pkg-config":      true,
	"dpkg-dev":        true,
	"libc-dev":        true,
	"libc6-dev":       true,
}

// runtimeSliceSuffixes are the slice suffixes preferred in runtime mode.
var runtimeSliceSuffixes = []string{"_libs", "_data", "_config", "_copyright", "_core"}

// toolsSliceSuffixes adds binary slices for tools mode.
var toolsSliceSuffixes = []string{"_libs", "_bins", "_data", "_config", "_copyright", "_core"}

// Resolve maps packages found in the Dockerfile to chisel slices.
func Resolve(opts *ResolveOptions) (*ResolveResult, error) {
	if opts.Info == nil {
		return nil, fmt.Errorf("no parsed Dockerfile provided")
	}

	// Find the Ubuntu stage and its packages.
	var ubuntuVer string
	var allPackages []string
	for _, stage := range opts.Info.Stages {
		if stage.IsUbuntu {
			ubuntuVer = stage.UbuntuVer
		}
		// When --release is explicit, collect packages from all stages.
		// Otherwise, only collect from Ubuntu stages.
		if !stage.IsUbuntu && opts.Release == "" {
			continue
		}
		for _, inst := range stage.Lines {
			if inst.IsAptInstall {
				allPackages = append(allPackages, inst.AptPackages...)
			}
		}
	}

	if ubuntuVer == "" && opts.Release == "" {
		return nil, fmt.Errorf("no Ubuntu base image found and no --release specified")
	}

	release := opts.Release
	if release == "" {
		release = "ubuntu-" + ubuntuVer
	}

	mode := opts.Mode
	if mode == "" || mode == "auto" {
		mode = "runtime"
	}

	// Determine which suffix set to use.
	suffixes := runtimeSliceSuffixes
	if mode == "tools" {
		suffixes = toolsSliceSuffixes
	}

	result := &ResolveResult{
		Release: release,
		Mode:    mode,
	}

	// Always include base-files.
	sliceSet := map[string]bool{
		"base-files_base": true,
	}

	// Map packages to slices.
	for _, pkg := range allPackages {
		if buildOnlyPackages[pkg] {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("%s: build-only package, kept in build stage", pkg))
			continue
		}

		// Try to find a matching slice suffix for this package.
		// In a full implementation, this would query setup.Release
		// to find available slices. For now, use heuristic mapping.
		found := false
		for _, suffix := range suffixes {
			slice := pkg + suffix
			sliceSet[slice] = true
			found = true
			break // Take the first matching suffix.
		}

		if !found {
			result.Missing = append(result.Missing, pkg)
		}
	}

	// Collect and sort slices.
	for slice := range sliceSet {
		result.Slices = append(result.Slices, slice)
	}
	sort.Strings(result.Slices)

	return result, nil
}

// PrintMapping writes the dry-run package-to-slice mapping to w.
func PrintMapping(w io.Writer, info *DockerfileInfo, result *ResolveResult) {
	fmt.Fprintf(w, "Release:     %s\n", result.Release)
	fmt.Fprintf(w, "Mode:        %s\n", result.Mode)
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Slices:")
	for _, s := range result.Slices {
		fmt.Fprintf(w, "  %s\n", s)
	}

	if len(result.Warnings) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Warnings:")
		for _, warn := range result.Warnings {
			fmt.Fprintf(w, "  %s\n", warn)
		}
	}

	if len(result.Missing) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "Missing slices: %s\n", strings.Join(result.Missing, ", "))
	} else {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Missing slices: (none)")
	}
}
