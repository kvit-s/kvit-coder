#!/bin/bash
# Amalgamate all Go files into a single file for needle retrieval benchmarks

set -e

OUTPUT="${1:-benchmarks/haystacks/kvit-coder.go.txt}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

mkdir -p "$(dirname "$OUTPUT")"

echo "// AMALGAMATED GO SOURCE - $(date -I)" > "$OUTPUT"
echo "// Source: kvit-coder" >> "$OUTPUT"
FIND_CMD="find $ROOT -name '*.go' -type f -not -path '*/benchmarks/.kvit-coder-benchmark/*' -not -path '*/_test_workspace/*' -not -path '*/testdata/*'"
echo "// Files: $(eval $FIND_CMD | wc -l)" >> "$OUTPUT"
echo "// Lines: $(eval $FIND_CMD | xargs wc -l | tail -1 | awk '{print \$1}')" >> "$OUTPUT"
echo "" >> "$OUTPUT"

# Find all Go files, excluding benchmarks/test workspaces, sort for consistency
find "$ROOT" -name "*.go" -type f \
    -not -path "*/benchmarks/.kvit-coder-benchmark/*" \
    -not -path "*/_test_workspace/*" \
    -not -path "*/testdata/*" \
    | sort | while read -r file; do
    relpath="${file#$ROOT/}"
    echo "" >> "$OUTPUT"
    echo "// ============================================================" >> "$OUTPUT"
    echo "// FILE: $relpath" >> "$OUTPUT"
    echo "// ============================================================" >> "$OUTPUT"
    echo "" >> "$OUTPUT"
    cat "$file" >> "$OUTPUT"
done

lines=$(wc -l < "$OUTPUT")
bytes=$(wc -c < "$OUTPUT")
echo "Created $OUTPUT: $lines lines, $((bytes / 1024))KB"
