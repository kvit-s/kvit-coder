#!/bin/bash
# Amalgamate Go files into a single file for needle retrieval benchmarks
# Targets ~60k tokens by limiting to ~8000 lines (excluding test files)

set -e

OUTPUT="${1:-benchmarks/haystacks/kvit-coder.go.txt}"
MAX_LINES="${2:-8000}"  # Target ~60k tokens
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

mkdir -p "$(dirname "$OUTPUT")"

# Create temporary file for accumulation
TEMP_FILE=$(mktemp)
trap "rm -f $TEMP_FILE" EXIT

echo "// AMALGAMATED GO SOURCE - $(date -I)" > "$TEMP_FILE"
echo "// Source: kvit-coder (core files, no tests)" >> "$TEMP_FILE"
echo "// Max lines target: $MAX_LINES" >> "$TEMP_FILE"
echo "" >> "$TEMP_FILE"

current_lines=4  # Header lines

# Find Go files, excluding tests and benchmark workspaces
# Prioritize packages used in benchmarks (checkpoint, safety) then others
{
    # Priority 1: packages referenced in haystack benchmarks
    find "$ROOT/internal/checkpoint" -name "*.go" -not -name "*_test.go" 2>/dev/null | sort
    find "$ROOT/internal/safety" -name "*.go" -not -name "*_test.go" 2>/dev/null | sort
    # Priority 2: other internal packages
    find "$ROOT/internal" -name "*.go" -type f \
        -not -path "*/checkpoint/*" \
        -not -path "*/safety/*" \
        -not -path "*/testdata/*" \
        -not -name "*_test.go" 2>/dev/null | sort
    # Priority 3: cmd packages
    find "$ROOT/cmd" -name "*.go" -not -name "*_test.go" 2>/dev/null | sort
} | while read -r file; do

    # Check if we've hit the limit
    if [ "$current_lines" -ge "$MAX_LINES" ]; then
        break
    fi

    file_lines=$(wc -l < "$file")

    # Skip if adding this file would exceed limit by too much
    if [ $((current_lines + file_lines + 5)) -gt $((MAX_LINES + 500)) ]; then
        continue
    fi

    relpath="${file#$ROOT/}"
    echo "" >> "$TEMP_FILE"
    echo "// ============================================================" >> "$TEMP_FILE"
    echo "// FILE: $relpath" >> "$TEMP_FILE"
    echo "// ============================================================" >> "$TEMP_FILE"
    echo "" >> "$TEMP_FILE"
    cat "$file" >> "$TEMP_FILE"

    current_lines=$((current_lines + file_lines + 5))
done

mv "$TEMP_FILE" "$OUTPUT"
trap - EXIT

lines=$(wc -l < "$OUTPUT")
bytes=$(wc -c < "$OUTPUT")
echo "Created $OUTPUT: $lines lines, $((bytes / 1024))KB (~$((lines * 8)) estimated tokens)"
