package dockerfile_test

import (
	. "gopkg.in/check.v1"

	"github.com/jayendra13/chisel-dockerfile/internal/dockerfile"
)

var resolveTests = []struct {
	summary  string
	info     *dockerfile.DockerfileInfo
	opts     *dockerfile.ResolveOptions
	slices   []string
	missing  []string
	warnings []string
	err      string
}{{
	summary: "Simple runtime resolution",
	info: &dockerfile.DockerfileInfo{
		Stages: []dockerfile.Stage{{
			BaseImage: "ubuntu:24.04",
			IsUbuntu:  true,
			UbuntuVer: "24.04",
			Lines: []dockerfile.Instruction{
				{Directive: "RUN", IsAptInstall: true, AptPackages: []string{"ca-certificates"}},
			},
		}},
	},
	opts: &dockerfile.ResolveOptions{Mode: "runtime"},
	slices: []string{
		"base-files_base",
		"ca-certificates_libs",
	},
}, {
	summary: "Build-only packages produce warnings",
	info: &dockerfile.DockerfileInfo{
		Stages: []dockerfile.Stage{{
			BaseImage: "ubuntu:24.04",
			IsUbuntu:  true,
			UbuntuVer: "24.04",
			Lines: []dockerfile.Instruction{
				{Directive: "RUN", IsAptInstall: true, AptPackages: []string{"build-essential", "ca-certificates"}},
			},
		}},
	},
	opts: &dockerfile.ResolveOptions{Mode: "runtime"},
	slices: []string{
		"base-files_base",
		"ca-certificates_libs",
	},
	warnings: []string{"build-essential: build-only package, kept in build stage"},
}, {
	summary: "No Ubuntu base and no release flag",
	info: &dockerfile.DockerfileInfo{
		Stages: []dockerfile.Stage{{
			BaseImage: "alpine:3.19",
		}},
	},
	opts: &dockerfile.ResolveOptions{Mode: "runtime"},
	err:  "no Ubuntu base image found.*",
}, {
	summary: "Explicit release overrides base image",
	info: &dockerfile.DockerfileInfo{
		Stages: []dockerfile.Stage{{
			BaseImage: "alpine:3.19",
			Lines: []dockerfile.Instruction{
				{Directive: "RUN", IsAptInstall: true, AptPackages: []string{"curl"}},
			},
		}},
	},
	opts: &dockerfile.ResolveOptions{
		Mode:    "runtime",
		Release: "ubuntu-24.04",
	},
	slices: []string{
		"base-files_base",
		"curl_libs",
	},
}}

func (s *S) TestResolve(c *C) {
	for _, t := range resolveTests {
		c.Logf("Summary: %s", t.summary)

		opts := t.opts
		opts.Info = t.info
		result, err := dockerfile.Resolve(opts)
		if t.err != "" {
			c.Assert(err, ErrorMatches, t.err)
			continue
		}
		c.Assert(err, IsNil)
		c.Check(result.Slices, DeepEquals, t.slices)
		if t.warnings != nil {
			c.Check(result.Warnings, DeepEquals, t.warnings)
		}
		if t.missing != nil {
			c.Check(result.Missing, DeepEquals, t.missing)
		}
	}
}
