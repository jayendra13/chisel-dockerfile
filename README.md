# chisel-dockerfile

Generate optimized, chiseled Ubuntu container images from standard Dockerfiles.

[Chisel](https://github.com/canonical/chisel) extracts only the needed file slices from Ubuntu packages, producing minimal `FROM scratch` images that are 50–90% smaller than standard Ubuntu-based images.

## Examples

Three-way size comparisons — **Ubuntu** vs **Chiseled** vs **Distroless** — for two use cases:

| Use Case | What it shows |
|---|---|
| `examples/go-https/` | Static Go binary — base image overhead |
| `examples/python-api/` | Python runtime — dependency reduction |

Run the comparison (requires Docker):

```bash
./examples/compare.sh
```

Builds all 6 images, verifies each responds on `/healthz` and `/fetch` (TLS), and prints a size table.

## CLI Tool (WIP)

```bash
go build -o chisel-dockerfile ./cmd/chisel-dockerfile/
```

```bash
# Preview package → slice mapping
chisel-dockerfile --dry-run Dockerfile

# Generate optimized Dockerfile
chisel-dockerfile -o Dockerfile.chiseled Dockerfile
```

Parses `apt-get install` packages from a Dockerfile, maps them to chisel slices, and generates a multi-stage `FROM scratch` Dockerfile.

## Tests

```bash
go test ./internal/dockerfile/
```

## Status

Early development. The slice resolver currently uses heuristic mapping — integration with chisel's slice definitions is planned.
