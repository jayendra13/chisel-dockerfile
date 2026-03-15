# Plan: `chisel-dockerfile` — Automated Chiseled Image Generation

## Context

https://github.com/canonical/chisel

Given a standard Dockerfile that uses `apt-get install` to install Ubuntu packages, manually converting it to a multi-stage chiseled build is tedious: you need to identify all packages, look up their available slices, resolve dependencies, and write the multi-stage Dockerfile. This tool automates that entire workflow.

The chiseled pattern produces images 50-65% smaller than standard Ubuntu images (e.g., 5-16 MB vs 70+ MB) by extracting only the needed file slices from packages rather than installing full packages.

**Goal**: A standalone CLI tool (`chisel-dockerfile`) that parses a Dockerfile, maps installed packages to chisel slices, and generates an optimized multi-stage Dockerfile using `FROM scratch`.

### Why Standalone (Not a `chisel` Subcommand)

Per GitHub discussions (#203, #223), Canonical's maintainers have clarified that "Chisel isn't a container tool" — it creates Ubuntu filesystems for any format. Adding Docker-specific features as a subcommand would face philosophical friction upstream. A standalone companion tool:
- Can iterate faster without upstream approval cycles
- Can freely integrate Docker-specific patterns (TARGETARCH, cache mounts, etc.)
- Can still import chisel's `internal/setup` package for slice resolution
- Aligns with the community's request for better container UX (#157, #203, #220) without burdening chisel's core scope

### Related GitHub Issues
- **#203**: Official chisel container image — extensive Dockerfile patterns from community
- **#157**: Simplify adopting chisel distroless base images
- **#223**: Multi-arch Dockerfile + TARGETARCH mapping
- **#220**: Reading slice lists from file to reduce Dockerfile verbosity

---

## Architecture

### New Files

```
cmd/chisel-dockerfile/main.go             # Standalone CLI entry point
internal/dockerfile/parse.go              # Lightweight Dockerfile parser
internal/dockerfile/resolve.go            # Package → slice resolution
internal/dockerfile/generate.go           # Output Dockerfile generation
internal/dockerfile/parse_test.go         # Parser tests
internal/dockerfile/resolve_test.go       # Resolution tests
internal/dockerfile/generate_test.go      # Generation tests
internal/dockerfile/suite_test.go         # check.v1 test suite setup
```

**No new external dependencies.** Uses only stdlib + existing internal packages (`internal/setup`).

---

## Step 1: CLI Entry Point (`cmd/chisel-dockerfile/main.go`)

Standalone binary using `go-flags` (same library as chisel):

```
chisel-dockerfile [OPTIONS] <dockerfile>

Options:
  --release <branch|dir>   Chisel release name or directory (e.g. ubuntu-22.04)
  --output, -o <path>      Write to file instead of stdout
  --mode <mode>            runtime | tools | auto (default: auto)
  --dry-run                Show package→slice mapping without generating
```

Uses `setup.FetchRelease()` / `setup.ReadRelease()` directly (same logic as chisel's `obtainRelease()`).

**Execute flow:**
1. Read & parse input Dockerfile → `DockerfileInfo`
2. Infer release from `FROM` line (or use `--release`)
3. `setup.FetchRelease()` to fetch slice definitions
4. Resolve packages → slices
5. Generate multi-stage Dockerfile
6. Write to stdout or `--output` file

---

## Step 2: Dockerfile Parser (`internal/dockerfile/parse.go`)

**Lightweight, purpose-built parser** (no `moby/buildkit` dependency — too heavy for this project's philosophy).

Extracts:
- `FROM` directives (base image, `AS` alias, Ubuntu version detection)
- `RUN apt-get install` / `RUN apt install` commands → package list
- All other instructions preserved verbatim

Key types:
```go
type DockerfileInfo struct {
    Stages []Stage
}

type Stage struct {
    BaseImage string           // "ubuntu:22.04"
    Alias     string           // "builder"
    IsUbuntu  bool
    UbuntuVer string           // "22.04"
    Lines     []Instruction
}

type Instruction struct {
    Raw         string         // original line(s) verbatim
    Directive   string         // FROM, RUN, COPY, etc.
    IsAptInstall bool
    AptPackages []string       // extracted package names
}
```

Parsing logic:
1. Join backslash-continuation lines
2. For `RUN` instructions, split on `&&` / `;` to isolate commands
3. Match `apt-get install` or `apt install` patterns
4. Extract package names (skip flags starting with `-`, strip `=version` pinning)
5. Map base images to Ubuntu versions via a lookup table (`ubuntu:jammy` → `22.04`, etc.)

---

## Step 3: Slice Resolution (`internal/dockerfile/resolve.go`)

Given packages + a `*setup.Release`, determine optimal slices:

```go
type ResolveResult struct {
    Slices   []setup.SliceKey     // ordered list of slices to cut
    Missing  []string             // packages with no slice definitions
    Warnings []string
}
```

**Algorithm:**
1. For each package, look up `release.Packages[pkg]`
2. Select slices based on `--mode`:
   - **runtime**: prefer `_libs`, `_data`, `_config`, `_copyright`
   - **tools**: include `_libs` + `_bins`
   - **auto**: inspect `CMD`/`ENTRYPOINT` to decide
3. Follow `Essential` dependencies transitively (each slice declares its deps)
4. Always include `base-files_base` as a foundation
5. Filter out build-only packages (`build-essential`, `gcc`, `make`, etc.) — these stay in the build stage only
6. Deduplicate and sort

---

## Step 4: Dockerfile Generation (`internal/dockerfile/generate.go`)

Produces a multi-stage Dockerfile using `text/template`:

```dockerfile
# Generated by: chisel-dockerfile
# Original base: ubuntu:22.04

# Stage 1: Chisel — extract minimal filesystem
# Uses alpine to download chisel binary (lightweight, no Go build needed)
FROM alpine AS chisel-stage
ARG CHISEL_VERSION=1.1.0
ARG TARGETARCH
RUN wget -qO - "https://github.com/canonical/chisel/releases/download/v${CHISEL_VERSION}/chisel_v${CHISEL_VERSION}_linux_${TARGETARCH}.tar.gz" \
    | tar -xz --no-same-owner -C /usr/local/bin chisel
WORKDIR /rootfs
RUN chisel cut --release ubuntu-22.04 --root /rootfs \
    base-files_base \
    libc6_libs \
    libssl3_libs \
    ca-certificates_data

# Stage 2: Application build (from original Dockerfile)
FROM ubuntu:22.04 AS app-build
COPY . /app
RUN apt-get update && apt-get install -y build-essential
RUN make -C /app

# Stage 3: Final chiseled image
FROM scratch
COPY --from=chisel-stage /rootfs /
COPY --from=app-build /app/mybin /usr/local/bin/mybin
ENV ...
WORKDIR ...
ENTRYPOINT ...
CMD ...
```

This pattern (from GitHub #203 community discussion) avoids installing Go in the build
stage by downloading the pre-built chisel binary from GitHub releases.

**Rules:**
- Chisel stage: installs chisel, runs `chisel cut` with resolved slices
- Build stage: preserves original build instructions (compile, download, etc.)
- Final stage: `FROM scratch`, copies chiseled rootfs + build artifacts, preserves `ENV`/`WORKDIR`/`ENTRYPOINT`/`CMD`/`EXPOSE`/`USER`/`LABEL`
- Build-only packages stay in build stage; runtime packages become slices in chisel stage

---

## Step 5: Tests

Follow project conventions: `gopkg.in/check.v1`, table-driven, `Suite` registration.

- **Parse tests**: various Dockerfile patterns (simple, multi-line, multi-stage, non-Ubuntu → error)
- **Resolve tests**: use synthetic `setup.Release` objects, verify slice selection per mode
- **Generate tests**: golden-output comparison of generated Dockerfiles

---

## Usage

### Install

```bash
go install github.com/canonical/chisel/cmd/chisel-dockerfile@latest
```

### Basic: Optimize a Dockerfile

Given a standard Dockerfile:
```dockerfile
FROM ubuntu:24.04
RUN apt-get update && apt-get install -y \
    libssl3t64 \
    ca-certificates \
    curl
COPY myapp /usr/local/bin/myapp
ENTRYPOINT ["/usr/local/bin/myapp"]
```

Run:
```bash
chisel-dockerfile Dockerfile
```

Outputs an optimized multi-stage Dockerfile to stdout that replaces the `apt-get install` with chisel slices, producing a ~10 MB image instead of ~80 MB.

### Write to file

```bash
chisel-dockerfile -o Dockerfile.chiseled Dockerfile
```

### Dry run (inspect what slices would be selected)

```bash
chisel-dockerfile --dry-run Dockerfile
```

Output:
```
Base image:  ubuntu:24.04
Release:     ubuntu-24.04
Mode:        auto (runtime)

Package → Slices:
  libssl3t64     → libssl3t64_libs
  ca-certificates → ca-certificates_data
  curl           → curl_bins (build-only, kept in build stage)

Auto-included:
  base-files_base
  libc6_libs

Missing slices: (none)
```

### Override release or mode

```bash
# Force a specific release
chisel-dockerfile --release ubuntu-22.04 Dockerfile

# Include binaries (not just libs) for all packages
chisel-dockerfile --mode tools Dockerfile

# Only runtime libs (skip bins even if CMD references them)
chisel-dockerfile --mode runtime Dockerfile
```

### Pipe directly into docker build

```bash
chisel-dockerfile Dockerfile | docker build -f - -t myapp:chiseled .
```

### Compare image sizes

```bash
docker build -t myapp:original .
chisel-dockerfile Dockerfile | docker build -f - -t myapp:chiseled .
docker images myapp
# REPOSITORY   TAG        SIZE
# myapp        original   78MB
# myapp        chiseled   12MB
```

---

## Verification

1. `go build ./cmd/chisel-dockerfile` — compiles
2. `go test ./internal/dockerfile/` — unit tests pass
3. `chisel-dockerfile --dry-run examples/Dockerfile` — shows package→slice mapping
4. `chisel-dockerfile examples/Dockerfile | docker build -f - .` — builds and runs
5. Compare image sizes: `docker images` before/after

---

## Edge Cases

- **Non-Ubuntu base**: clear error message
- **Unknown packages** (no slices): warn but don't fail; comment in generated Dockerfile
- **Version-pinned packages**: strip version for lookup, note in comment
- **Already multi-stage Dockerfiles**: preserve all stages, optimize only the final runtime stage
- **No apt-get install found**: warn that there's nothing to optimize
