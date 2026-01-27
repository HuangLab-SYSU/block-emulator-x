#!/usr/bin/env bash
set -euo pipefail

# 脚本功能：
# 调用 cmd/fillcontractaddr/main.go，将部署交易计算出的合约地址回填到 CSV 的 toCreate 字段
# nonce 规则：同一 from 地址在 CSV 中第 1 次部署 nonce=0，之后每次部署 +1（由 Go 代码内部处理）
#
# 用法：
#   原地修改:
#     bash scripts/fill_contract_addr.sh -i /path/tx.csv --in-place
#
#   输出新文件:
#     bash scripts/fill_contract_addr.sh -i /path/tx.csv -o /path/tx_out.csv
#
# 可选：
#   --no-overwrite   如果 toCreate 已有值则不覆盖
#   -h, --help       查看帮助

#ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
#cd "$ROOT_DIR"

IN="./supervisor/txsource/csvsource/contracttestdata.csv"
OUT=""
IN_PLACE="true"
OVERWRITE="false"

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
    bash scripts/fill_contract_addr.sh -i <input.csv> --in-place [--no-overwrite]

  Output to new file:
    bash scripts/fill_contract_addr.sh -i <input.csv> -o <output.csv> [--no-overwrite]
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

CMD=(go run ./cmd/getcontractaddr/main.go -in "$IN" -overwrite "$OVERWRITE")

if [[ "$IN_PLACE" == "true" ]]; then
  CMD+=(--in-place)
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
