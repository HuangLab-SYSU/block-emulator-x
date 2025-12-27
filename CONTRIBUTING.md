# Contributing to BlockEmulator

Welcome to BlockEmulator!
This document describes how to contribute code, documentation, tests, and other improvements.

## Background

### Who is this document for?

- People who want to submit code, documentation, tests, or other improvements to the project.
- New developers who want to understand how to participate, open issues, submit PRs, follow coding conventions, etc.
- Project maintainers, who may also use it to keep contribution processes consistent.

### Why does this document matter?

1. Guiding contributors
    - Explains how the project operates and what kinds of contributions are accepted.
    - Defines requirements for opening Issues, submitting PRs, running tests, code conventions, branch strategies, etc.

2. Reducing communication cost
    - Contributors no longer need to repeatedly ask questions like:
        - "How do I submit a PR?"
        - "Which tests should I run?"

    - Maintainers no longer need to repeatedly explain processes.

3. Maintaining project quality

    - By clarifying code style, testing standards, commit format, etc., contributions become consistent.

4. Shaping project culture

    - Communicates what contributors and behaviors are welcomed, fostering a healthy community environment.

## BlockEmulator Directory Conventions

Go projects generally follow these conventions:

1. File and directory names are lowercase, no uppercase. For example, the PBFT folder should be named `pbft`.

2. Except for `_test.go` files, filenames generally do not use dashes (-).
   For example, “node configuration” should be named `nodeconfig` or `node_config` (the former is recommended).

3. The module field in `go.mod` should use a GitHub-style path that others can directly import.

### BlockEmulator Directory Structure

(Generated using `tree -Ld 2`)

```
├── cmd
│   ├── consensusnode
│   ├── loadnetwork
│   └── supervisor
├── config
├── consensus
│   └── pbft
├── pkg
│   ├── broker
│   ├── chain
│   ├── core
│   ├── csvwrite
│   ├── logger
│   ├── message
│   ├── network
│   ├── nodetopo
│   ├── partition
│   ├── storage
│   └── utils
└── supervisor
    ├── committee
    ├── measure
    └── txsource
```

> **If updates occur in the future, regenerate the structure using: `tree -Ld 2`.**

## Go Logging

1. Go’s official logging package is log/slog, which includes 4 levels from low to high:
   **Debug**, **Info**, **Warn**, **Error**.

2. When the log level is set high, lower-level logs are filtered.
   For example, if Level = Info, then Debug logs will not be output, but Info, Warn, and Error logs will.

3. Logger initialization is located in `pkg/logger/logger.go`.
   You can adjust where logs are written, such as console or file.

Example:

```go
func main() {
    slog.Debug("", "", xxx)   // Usually only used during development for debugging
    slog.Info("", "", xxx)    // Basic informational logs
    slog.Warn("", "", xxx)    // Benign anomalies
    slog.Error("", "", xxx)   // Serious errors
}
```

## Go Error Handling

> Based on "100 Go Mistakes and How to Avoid Them" — recommended reading if time permits.

### General Rule

**In Go, you must either handle errors or return them upward; never ignore errors.**

### Returning Errors Upward

Returning upward means passing an error back to the caller.
In this case, the function should return an `error` in its signature.

Use `fmt.Errorf` with `%w` to wrap errors.
Wrapping preserves the original error type so `errors.Is` can detect it.

Example:

```go
var testErr = errors.New("this is test error")

func TestErrorIs() {
    fmt.Printf("%t\n", errors.Is(testErr, ReportWrapError())) // true
    fmt.Printf("%t\n", errors.Is(testErr, ReportNewError())) // false
}
func ReportWrapError() error {
    return fmt.Errorf("report error: %w", testErr)
}
func ReportNewError() error {
    return errors.New("report error: " + testErr.Error())
}

```

### Handling Errors

Handling errors typically involves logging or returning user-friendly messages.

When errors are wrapped layer by layer, the final top-level error contains the entire call chain.

Example:

```go
func main() {
    err := a()
    slog.Info("test err", "err", err)
    // Output: function a failed, err=function b failed, err=c error
    // This records the call chain a -> b -> c
}
func a() error {
    return fmt.Errorf("function a failed, err=%w", b())
}
func b() error {
    return fmt.Errorf("function b failed, err=%w", c())
}
func c() error {
    return fmt.Errorf("c error")
}
```

## Integrating `golangci-lint`

`golangci-lint` checks Go code to prevent non-standard code from being committed to GitHub.

If you are under time pressure (e.g., writing a paper), you may temporarily skip strict linting,
but for open-source release, linting is strongly recommended.

### Installation

See official docs:
https://golangci-lint.run/docs/welcome/install/

### Configuration

The configuration file `.golangci.yaml` is already prepared and usually does not require modification.

Official docs for configuration:
https://golangci-lint.run/docs/configuration/

### Usage

```bash
golangci-lint run ./...            # Check all Go files
golangci-lint run ./... --fix      # Auto-fix
```

### When is it used?

1. CI Process

   Every Pull Request is checked automatically via GitHub Actions. PRs failing lint checks cannot be merged.

2. Daily Development

   Developers should run lint locally before submitting PRs.

## Unit Testing

Unit tests (`go test`) detect errors early in development. Writing Go tests is strongly recommended.

### Installation

`go test` comes with Go by default.

### Writing `xxx_test.go`

Notes:

1. `xxx` is usually the package name, but flexible.

2. Use `t.Run` to improve readability and pinpoint failures.

3. Use `github.com/stretchr/testify` → `assert`, `require`

4. Do **not** let tests depend on implementation details; otherwise tests become useless.

### Running Tests

```bash
go test                                  # Test current directory
go test -gcflags=all='-N -l'             # Disable inlining & optimizations
go test ./...                            # Test recursively
go test ./... -cover                     # Coverage
go test ./... -coverprofile=coverage.out # Output coverage file
go tool cover -html=coverage.out         # View in browser
```

## Git Commit Convention

The project uses `Commitlint`: https://commitlint.js.org/

### Commit message format:

```
type(scope?): subject
body?
footer?
```

Example:

```
feat(storage): add bloom filter for tx lookup

This improves transaction lookup performance.

BREAKING CHANGE: storage format updated.
```

### Explanation of each part

#### type (required)

| type     | Meaning                                  | Typical Use          |
|----------|------------------------------------------|----------------------|
| feat     | New feature                              | Adding functionality |
| fix      | Bug fix                                  | Fixing issues        |
| docs     | Documentation change                     | README, comments     |
| style    | No logic change, formatting only         | Lint fixes           |
| perf     | Performance improvements                 | Speed / memory       |
| refactor | Code refactor, not fixing or adding feat | Structural changes   |
| ci       | CI configuration changes                 | GitHub Actions       |
| test     | Test-related changes                     | Unit tests           |
| chore    | Misc tasks                               | Scripts, deps        |

#### scope (optional)

Usually a module, directory, or component.

#### subject (required)

- Imperative mood (“add”, “update”, “fix”)
- First letter lowercase
- No period
- Under 50 characters

#### body (optional)

Longer description.

#### footer (optional)

Used for:

1. BREAKING CHANGE
2. Issue links

## GitHub Actions

GitHub Actions provides CI/CD services for automated tasks such as building, testing, deployment, checking commit
messages, code style, etc.

Workflow files are stored in: `.github/workflows/`

Docs: https://docs.github.com/en/actions/how-tos/write-workflows

Currently, GitHub Actions checks:

- Go lint (`golangci-lint`)
- Commit message lint (`commitlint`)

And runs on push and pull request events targeting the main branch.

## Development Steps

How developers should correctly submit changes to the repository:

> 你怎么能直接 commit 到我的 main 分支啊？！
> GitHub 上不是这样！你应该先 fork 我的仓库，然后从 develop 分支 checkout 一个新的 feature 分支，
> 比如叫 feature/confession。然后你把你的心意写成代码，并为它写好单元测试和集成测试，
> 确保代码覆盖率达到95%以上。接着你要跑一下 Linter，通过所有的代码风格检查。
> 然后你再 commit，commit message 要遵循 Conventional Commits 规范。
> 之后你把这个分支 push 到你自己的远程仓库，然后给我提一个 Pull Request。
> 在 PR 描述里，你要详细说明你的功能改动和实现思路，并且 @ 我和至少两个其他的评审。
> 我们会 review 你的代码，可能会留下一些评论，你需要解决所有的 thread。
> 等 CI/CD 流水线全部通过，并且拿到至少两个 LGTM 之后，我才会考虑把你的分支 squash and merge 到 develop 里，等待下一个版本发布。
> 你怎么直接上来就想 force push 到 main？！ GitHub 上根本不是这样！我拒绝合并！

> You can’t just commit directly to my master branch! That’s not how GitHub works!
> You should fork my repo, then checkout a new feature branch from develop, such as feature/confession.
> Write your code, add unit and integration tests, ensure coverage is above 95%, run the linter, fix everything,
> follow Conventional Commits, push to your fork, then open a PR.
> In the PR description, explain your changes and implementation details, @ me and at least two reviewers.
> Fix all review comments, wait for CI to pass, get at least two LGTMs,
> then I might squash-merge your branch into develop.
> How dare you force-push to main?! I will not merge it!

### Actual Required Steps

1. Read all development conventions, including Go practices and BlockEmulator directory rules.

2. Fork the repo, develop in the appropriate package, and ensure correctness via unit tests.

3. Run multi-node BlockEmulator to test functionality:
    - At least 4 × 4 nodes (4 shards × 4 nodes per shard)
    - Ensure changes are valid
    - All transactions must be committed
    - No Error/Warn in logs
    - Supervisor statistics match expectations

4. Run lint and fix formatting: `golangci-lint run ./... --fix`

5. Write a Conventional Commit–style message and submit a Pull Request.

6. GitHub Actions runs CI checks, and the repo owner decides whether to accept the PR.

## Thank You

Thank you for taking the time to contribute to BlockEmulator.
Every issue, pull request, line of code, and idea helps improve the project and move it forward.
We deeply appreciate your effort, your attention to quality, and your willingness to collaborate.
Together, we can continue building a more robust, efficient, and innovative BlockEmulator.