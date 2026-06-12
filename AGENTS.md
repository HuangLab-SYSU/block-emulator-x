# AGENTS.md

BlockEmulator-X (v2.0): a lightweight sharded-blockchain testbed by HuangLab @ SYSU.
Module `github.com/HuangLab-SYSU/block-emulator-x`, Go 1.25.7. Two roles: one
**supervisor** (injects txs, runs CLPA partitioning, collects metrics) and many
**consensusnodes** (PBFT consensus, block production; node 0 per shard is leader).

## Architecture (big picture)
- **Execution model**: serial, single-goroutine processing (v1.0 used goroutine-per-message, dropped to avoid races). ConsensusNode runs a 500ms ticker loop `Node.run` (`consensus/pbft/node.go`): `DrainMsgBuffer` → `curateMsg` → `step2NextStage` (preprepare→prepare→commit) → leader maybe `propose` (gated by `BlockInterval`). Supervisor runs a 1s ticker loop `Supervisor.Start` (`supervisor/supervisor.go`) plus a separate **measure subroutine** fed by `measureMsgBuf` so metrics never block the main loop.
- **Startup flow** (both roles): `flag.Parse` → `config.LoadLocalParams` → `config.LoadConfig` → optional pprof → `logger.InitLogger` → `loadnetwork.PrepareNetworkByCfg` → `network.NewConnHandler` → create node → sleep (5s consensus / 8s supervisor) → `Start()`.
- **Key packages**: `consensus/pbft` (PBFT engine + `insideop`/`outsideop`/`migration` variants), `pkg/core` (Account, Block, Transaction, TxPool), `pkg/chain` (block production + tx execution), `pkg/vm` + `pkg/contractexec` (EVM via go-ethereum), `pkg/partition` (CLPA graph partition), `pkg/broker`, `supervisor/{committee,measure,txsource}`.
- **Storage = 3 stores** (`pkg/storage`): block store (BoltDB, atomic `AddBlock`/`AddBlockHeader`, rollback via `UpdateNewestBlockHash`); account state (`vmstate` wraps go-ethereum `StateDB` — new StateDB per block, unusable after `Commit()`); account-location MPT (`trie`, `MAddKeyValuesAndCommit` commits vs `MAddKeyValuesPreview` validates only; headers carry `LocationRoot`).
- **Block types**: `TxBlock` (`Body` with normal txs) and `MigrationBlock` (`MigrationOpt` with migrated account states, CLPA). Exactly one of `Body`/`MigrationOpt` is populated.

## Dev environment tips
- Use `go list ./...` to enumerate packages and `grep -rn <symbol> --include='*.go'` (or the editor search) to locate code instead of scanning with `ls`.
- Run `go build ./...` after edits; it compiles every package and is the fastest sanity check.
- Spin up a full local experiment with `bash example_run.sh` — it runs `go mod download`, `go build ./...`, starts a 4×4 cluster + supervisor, and writes output under `./exp/`.
- Run a single node manually: `go run cmd/consensusnode/main.go -shard_id=0 -node_id=0`; supervisor uses the reserved shard id `go run cmd/supervisor/main.go -shard_id=0x7fffffff -node_id=0`. See all flags with `-h`.
- Config lives in two layers: global `config.yaml` (sections `system`/`consensus_node`/`supervisor`/`network`) plus per-process CLI flags (`-node_id`, `-shard_id`, `-config`, `-ip_table`, `-pprof-port`). Pass config to structs at construction time — do not read globals at runtime.
- When changing `shard_num`/`node_num`, also update `ip_table.json` and `SHARD_NUM`/`NODE_NUM` in `example_run.sh`, or nodes won't find each other in `direct` (gRPC) mode.
- Consensus wiring is driven by enums in `config/config_consensus.go`: `static_relay`, `static_broker`, `clpa_relay`, `clpa_broker`. Network mode (`config/config_network.go`) is `direct` or `libp2p`.

## Testing instructions
- Find the CI plan in `.github/workflows/` (`go.yml` for build/test/lint, `commitlint.yml` for commit messages).
- Run the full suite with `go test -gcflags=all='-N -l' ./...` (the `-N -l` disables inlining and matches `make test` / CI).
- Run one package with `go test ./pkg/chain`, and one test with `go test ./pkg/chain -run TestXxx`. Add `-cover` for coverage.
- CI pins `golangci-lint v2.7.0` and also enforces `go mod tidy -diff` — run `go mod tidy` before pushing or the pipeline fails.
- Lint with `golangci-lint run ./...` and auto-fix with `golangci-lint run ./... --fix` (matches `make lint-fix`). Lint skips `_test.go`.
- Fix all test, type, and lint errors until everything is green; don't loop more than ~3 times on the same lint failure.
- After moving files or changing imports, re-run `go build ./...` and the linter so `gci` import ordering (standard → default → `prefix(github.com/HuangLab-SYSU/block-emulator-x)` → blank) stays correct.
- Add or update tests for code you change, even if nobody asked. Use `t.Run` subtests with `stretchr/testify` (`assert`/`require`); don't couple to implementation details.
- `make all` runs `test` + `lint-fix` + `build-image` in one shot.

## Coding conventions
- Naming: lowercase package/dir/file names, no dashes (except `_test.go`); prefer `nodeconfig` over `node_config`.
- Errors: never ignore; wrap with `fmt.Errorf("context: %w", err)` so `errors.Is` works.
- Logging: `log/slog` only (Debug/Info/Warn/Error). No `fmt.Print` / `log.Panic` in normal flow.
- Concurrency: v2.0 processes messages serially (single goroutine) to avoid races. `Chain` public methods must be called under its `mux` lock; internal methods assume the lock is held.
- On-wire messages are protobuf `WrappedMsg{MsgType, Payload}`; `message.WrapMsg` gob-encodes the payload. Decoding is intentionally not provided — receivers gob-decode `Payload` based on `MsgType`. Send by `nodetopo.NodeInfo`, never raw IP.

## PR instructions
- Branch from `develop`, not `main`.
- Use Conventional Commits: `type(scope?): subject` (lowercase, imperative, <50 chars). Types: feat, fix, docs, style, perf, refactor, ci, test, chore.
- Always run `go build ./...`, `go test -gcflags=all='-N -l' ./...`, `go mod tidy`, and `golangci-lint run ./... --fix` before committing.
- Validate a multi-node run (≥4×4): all txs committed, no Error/Warn logs, supervisor stats correct.
