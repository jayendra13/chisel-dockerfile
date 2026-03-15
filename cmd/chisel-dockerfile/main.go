package main

import (
	"fmt"
	"log"
	"os"

	"github.com/jessevdk/go-flags"

	"github.com/canonical/chisel/internal/dockerfile"
)

type Options struct {
	Release string `long:"release" description:"Chisel release name or directory (e.g. ubuntu-24.04)"`
	Output  string `long:"output" short:"o" description:"Write to file instead of stdout"`
	Mode    string `long:"mode" default:"auto" description:"Slice selection mode: runtime, tools, auto"`
	DryRun  bool   `long:"dry-run" description:"Show package-to-slice mapping without generating"`
}

var (
	Stdin  = os.Stdin
	Stdout = os.Stdout
	Stderr = os.Stderr
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var opts Options
	args, err := flags.Parse(&opts)
	if err != nil {
		// go-flags already printed the error
		os.Exit(1)
	}

	if len(args) != 1 {
		return fmt.Errorf("usage: chisel-dockerfile [OPTIONS] <dockerfile>")
	}
	dockerfilePath := args[0]

	// Step 1: Parse input Dockerfile.
	data, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", dockerfilePath, err)
	}
	info, err := dockerfile.Parse(data)
	if err != nil {
		return fmt.Errorf("cannot parse %s: %w", dockerfilePath, err)
	}

	// Step 2: Resolve packages to slices.
	resolveOpts := &dockerfile.ResolveOptions{
		Release: opts.Release,
		Mode:    opts.Mode,
		Info:    info,
	}
	result, err := dockerfile.Resolve(resolveOpts)
	if err != nil {
		return fmt.Errorf("cannot resolve slices: %w", err)
	}

	// Step 3: Dry-run prints mapping and exits.
	if opts.DryRun {
		dockerfile.PrintMapping(Stdout, info, result)
		return nil
	}

	// Step 4: Generate optimized Dockerfile.
	output, err := dockerfile.Generate(info, result)
	if err != nil {
		return fmt.Errorf("cannot generate dockerfile: %w", err)
	}

	// Step 5: Write output.
	if opts.Output != "" {
		if err := os.WriteFile(opts.Output, output, 0644); err != nil {
			return fmt.Errorf("cannot write %s: %w", opts.Output, err)
		}
		log.Printf("wrote %s", opts.Output)
	} else {
		Stdout.Write(output)
	}

	return nil
}
