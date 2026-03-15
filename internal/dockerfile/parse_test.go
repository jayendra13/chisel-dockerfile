package dockerfile_test

import (
	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/dockerfile"
)

var parseTests = []struct {
	summary  string
	input    string
	expected *dockerfile.DockerfileInfo
	err      string
}{{
	summary: "Simple single-stage Ubuntu",
	input: `FROM ubuntu:24.04
RUN apt-get update && apt-get install -y ca-certificates
COPY myapp /usr/local/bin/myapp
ENTRYPOINT ["/usr/local/bin/myapp"]
`,
	expected: &dockerfile.DockerfileInfo{
		Stages: []dockerfile.Stage{{
			BaseImage: "ubuntu:24.04",
			IsUbuntu:  true,
			UbuntuVer: "24.04",
			Lines: []dockerfile.Instruction{
				{Directive: "RUN", IsAptInstall: true, AptPackages: []string{"ca-certificates"}},
				{Directive: "COPY"},
				{Directive: "ENTRYPOINT"},
			},
		}},
	},
}, {
	summary: "Multi-stage with build alias",
	input: `FROM golang:1.22 AS build
RUN go build -o /app

FROM ubuntu:22.04
RUN apt-get update && apt-get install -y libssl3 ca-certificates
COPY --from=build /app /app
ENTRYPOINT ["/app"]
`,
	expected: &dockerfile.DockerfileInfo{
		Stages: []dockerfile.Stage{
			{
				BaseImage: "golang:1.22",
				Alias:     "build",
				Lines: []dockerfile.Instruction{
					{Directive: "RUN"},
				},
			},
			{
				BaseImage: "ubuntu:22.04",
				IsUbuntu:  true,
				UbuntuVer: "22.04",
				Lines: []dockerfile.Instruction{
					{Directive: "RUN", IsAptInstall: true, AptPackages: []string{"libssl3", "ca-certificates"}},
					{Directive: "COPY"},
					{Directive: "ENTRYPOINT"},
				},
			},
		},
	},
}, {
	summary: "Non-Ubuntu base image",
	input: `FROM alpine:3.19
RUN apk add --no-cache curl
`,
	expected: &dockerfile.DockerfileInfo{
		Stages: []dockerfile.Stage{{
			BaseImage: "alpine:3.19",
			Lines: []dockerfile.Instruction{
				{Directive: "RUN"},
			},
		}},
	},
}, {
	summary: "No FROM instruction",
	input:   `RUN echo hello`,
	err:     "instruction before FROM.*",
}, {
	summary: "Empty Dockerfile",
	input:   ``,
	err:     "no FROM instruction found",
}, {
	summary: "Multi-line continuation",
	input: `FROM ubuntu:24.04
RUN apt-get update && apt-get install -y \
    ca-certificates \
    curl
ENTRYPOINT ["/bin/sh"]
`,
	expected: &dockerfile.DockerfileInfo{
		Stages: []dockerfile.Stage{{
			BaseImage: "ubuntu:24.04",
			IsUbuntu:  true,
			UbuntuVer: "24.04",
			Lines: []dockerfile.Instruction{
				{Directive: "RUN", IsAptInstall: true, AptPackages: []string{"ca-certificates", "curl"}},
				{Directive: "ENTRYPOINT"},
			},
		}},
	},
}, {
	summary: "Version-pinned packages",
	input: `FROM ubuntu:24.04
RUN apt-get install -y libssl3t64=3.0.13-0ubuntu3
`,
	expected: &dockerfile.DockerfileInfo{
		Stages: []dockerfile.Stage{{
			BaseImage: "ubuntu:24.04",
			IsUbuntu:  true,
			UbuntuVer: "24.04",
			Lines: []dockerfile.Instruction{
				{Directive: "RUN", IsAptInstall: true, AptPackages: []string{"libssl3t64"}},
			},
		}},
	},
}}

func (s *S) TestParse(c *C) {
	for _, t := range parseTests {
		c.Logf("Summary: %s", t.summary)

		info, err := dockerfile.Parse([]byte(t.input))
		if t.err != "" {
			c.Assert(err, ErrorMatches, t.err)
			continue
		}
		c.Assert(err, IsNil)
		c.Assert(len(info.Stages), Equals, len(t.expected.Stages))

		for i, stage := range info.Stages {
			exp := t.expected.Stages[i]
			c.Check(stage.BaseImage, Equals, exp.BaseImage)
			c.Check(stage.Alias, Equals, exp.Alias)
			c.Check(stage.IsUbuntu, Equals, exp.IsUbuntu)
			c.Check(stage.UbuntuVer, Equals, exp.UbuntuVer)
			c.Assert(len(stage.Lines), Equals, len(exp.Lines))
			for j, line := range stage.Lines {
				c.Check(line.Directive, Equals, exp.Lines[j].Directive)
				c.Check(line.IsAptInstall, Equals, exp.Lines[j].IsAptInstall)
				if exp.Lines[j].AptPackages != nil {
					c.Check(line.AptPackages, DeepEquals, exp.Lines[j].AptPackages)
				}
			}
		}
	}
}
