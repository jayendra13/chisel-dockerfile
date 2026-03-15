package dockerfile_test

import (
	"strings"

	. "gopkg.in/check.v1"

	"github.com/jayendra13/chisel-dockerfile/internal/dockerfile"
)

var generateTests = []struct {
	summary  string
	info     *dockerfile.DockerfileInfo
	result   *dockerfile.ResolveResult
	contains []string
	err      string
}{{
	summary: "Simple single-stage generation",
	info: &dockerfile.DockerfileInfo{
		Stages: []dockerfile.Stage{{
			BaseImage: "ubuntu:24.04",
			IsUbuntu:  true,
			UbuntuVer: "24.04",
			Lines: []dockerfile.Instruction{
				{Directive: "RUN", IsAptInstall: true},
				{Raw: "COPY myapp /usr/local/bin/myapp", Directive: "COPY"},
				{Raw: `ENTRYPOINT ["/usr/local/bin/myapp"]`, Directive: "ENTRYPOINT"},
			},
		}},
	},
	result: &dockerfile.ResolveResult{
		Release: "ubuntu-24.04",
		Slices:  []string{"base-files_base", "ca-certificates_data"},
	},
	contains: []string{
		"FROM alpine AS chisel-stage",
		"chisel cut --release ubuntu-24.04 --root /rootfs",
		"base-files_base",
		"ca-certificates_data",
		"FROM scratch",
		"COPY --from=chisel-stage /rootfs /",
		"COPY myapp /usr/local/bin/myapp",
		`ENTRYPOINT ["/usr/local/bin/myapp"]`,
	},
}, {
	summary: "Nil info returns error",
	info:    nil,
	result:  &dockerfile.ResolveResult{},
	err:     "missing parsed info.*",
}}

func (s *S) TestGenerate(c *C) {
	for _, t := range generateTests {
		c.Logf("Summary: %s", t.summary)

		output, err := dockerfile.Generate(t.info, t.result)
		if t.err != "" {
			c.Assert(err, ErrorMatches, t.err)
			continue
		}
		c.Assert(err, IsNil)
		text := string(output)
		for _, want := range t.contains {
			c.Check(strings.Contains(text, want), Equals, true,
				Commentf("output missing %q", want))
		}
	}
}
