#!/usr/bin/env bash
set -euo pipefail

# Script purpose:
# Call cmd/getcontractaddr/main.go to compute contract addresses for deploy txs
# and write them back to CSV (toCreate for deploy txs, and to for related call txs).
#
# Usage:
#   In-place update:
#     bash scripts/contractaddr_get.sh -i /path/tx.csv --in-place
#
#   Write to a new file:
#     bash scripts/contractaddr_get.sh -i /path/tx.csv -o /path/tx_out.csv
#
# Optional:
#   --no-overwrite   Do not overwrite existing toCreate/to values
#   -h, --help       Show help

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"

IN="./supervisor/txsource/csvsource/contracttestdata2.csv"
OUT=""
IN_PLACE="true"
OVERWRITE="true"

while [[ $# -gt 0 ]]; do
  case "$1" in
    -i|--in)
      IN="${2:-}"
      shift 2
      ;;
    -o|--out)
      OUT="${2:-}"
      shift 2
      ;;
    --in-place)
      IN_PLACE="true"
      shift
      ;;
    --no-overwrite)
      OVERWRITE="false"
      shift
      ;;
    -h|--help)
      cat <<'EOF'
Usage:
  In-place:
    bash scripts/contractaddr_get.sh -i <input.csv> --in-place [--no-overwrite]

  Output to new file:
    bash scripts/contractaddr_get.sh -i <input.csv> -o <output.csv> [--no-overwrite]
EOF
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

if [[ -z "$IN" ]]; then
  echo "Error: -i/--in is required." >&2
  exit 1
fi

if [[ ! -f "$IN" ]]; then
  echo "Error: input file not found: $IN" >&2
  exit 1
fi

if [[ "$IN_PLACE" == "true" && -n "$OUT" ]]; then
  echo "Error: do not use -o with --in-place." >&2
  exit 1
fi

if [[ "$IN_PLACE" == "false" && -z "$OUT" ]]; then
  echo "Error: -o/--out is required when not using --in-place." >&2
  exit 1
fi

CMD=(go run ./cmd/getcontractaddr/main.go -in "$IN" -overwrite="$OVERWRITE")

if [[ "$IN_PLACE" == "true" ]]; then
  CMD+=(-in-place=true)
else
  CMD+=(-out "$OUT")
fi

echo "Running: ${CMD[*]}"
"${CMD[@]}"

if [[ "$IN_PLACE" == "true" ]]; then
  echo "Done. Updated in place: $IN"
else
  echo "Done. Output: $OUT"
fi
