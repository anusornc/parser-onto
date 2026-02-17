# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
# Go is installed at /home/nodeadmin/go-sdk/go/bin — add to PATH if needed
export PATH=/home/nodeadmin/go-sdk/go/bin:$PATH

# Build
go build -o chebi-parser .

# Run
./chebi-parser -input <file.obo|file.owl> [-output out.json] [-format auto|obo|owl] [-pretty]

# Vet
go vet ./...
```

No external dependencies — stdlib only. No test suite yet.

## Architecture

The parser is a CLI tool that reads ChEBI ontology files (OBO or OWL format) and outputs JSON. Format is auto-detected from file extension.

- **`main.go`** — CLI entry point. Handles flags, format detection, orchestrates parse→write pipeline, reports timing to stderr.
- **`ontology/model.go`** — Shared data model: `Ontology` (top-level) → `[]Term` → `Synonym`, `Relationship`, properties map. All structs have JSON tags.
- **`ontology/obo_parser.go`** — `ParseOBO(io.Reader)` — streaming line-by-line parser using `bufio.Scanner` with 1MB buffer. Uses string interning (`internPool`) for repeated values. Pre-allocates 200k term capacity.
- **`ontology/owl_parser.go`** — `ParseOWL(io.Reader)` — streaming XML token parser using `encoding/xml.Decoder`. Converts OBO-style URIs (`obo/CHEBI_12345`) to `CHEBI:12345` IDs via `oboIDFromURI`.
- **`ontology/writer.go`** — `WriteJSON`/`WriteJSONPretty`/`WriteJSONFile` — buffered (256KB) JSON encoding directly to writer, no intermediate `[]byte`.

## Performance Notes

Both parsers are single-pass streaming (no DOM, no backtracking). The OBO parser handles ~128 MB/s on the full 248MB chebi.obo (~205k terms in ~2s). The OWL parser is inherently slower due to XML overhead (~774MB chebi.owl, ~224k terms in ~30s). Key techniques: pre-allocated slices, string interning for namespaces/relationship types, `strings.Cut` over regex, large I/O buffers.

## Test Data

- `testdata/sample.obo` / `sample.owl` — small 4-term samples for quick validation
- `testdata/chebi.obo` / `chebi.owl` — full ChEBI downloads (248MB / 774MB), not in version control
