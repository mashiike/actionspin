# actionspin

Bulk replace GitHub Actions references from version tags to commit hashes for locked, reproducible workflows.

## Overview

`actionspin` is a tool that replaces version tags with commit hashes in GitHub Actions workflow files. This ensures reproducible workflows.

## Installation

You can install it using the following command:

```sh
$go install github.com/mashiike/actionspin/cmd/actionspin@latest 
```

Alternatively, you can download the binary from [GitHub Releases](https://github.com/mashiike/actionspin/releases).

or, Homebrew:

```sh
$ brew install mashiike/tap/actionspin
```

## Usage

Use `actionspin` to process GitHub Actions workflow files in the specified directory.

```sh
Usage: actionspin --target=".github" [flags]

Bulk replace GitHub Actions references from version tags to commit hashes for locked, reproducible workflows.

Flags:
  -h, --help                   Show context-sensitive help.
      --log-format="json"      Log format ($LOG_FORMAT)
      --[no-]color             Enable color output
      --log-level="info"       Log level ($LOG_LEVEL)
      --version                Show version and exit
      --target=".github"       Replace Target dir or file
      --output=""              Output dir
      --github-token=STRING    GitHub token ($GITHUB_TOKEN)
      --ghe-host=STRING        GitHub Enterprise Server host ($GHE_HOST)
      --ghe-token=STRING       GitHub Enterprise Server token ($GHE_TOKEN)
```

For example, consider the following Actions workflow:

```yaml
name: Test
on:
  push:
    branches:
      - master
      - main
  pull_request:
    types:
      - opened
      - synchronize
      - reopened

jobs:
  test:
    strategy:
      matrix:
        go:
          - "1.24"
    name: Build
    runs-on: ubuntu-latest
    steps:
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ matrix.go }}
        id: go

      - name: Check out code into the Go module directory
        uses: actions/checkout@v4

      - name: Build & Test
        run: |
          go test -race ./... -timeout 30s
```

Run the following command in the root directory of the repository, and `actionspin` will replace `actions/setup-go@v5` and `actions/checkout@v4` with their respective commit hashes.

```sh
$ actionspin
{"time":"2025-03-18T14:42:20.911018+09:00","level":"INFO","msg":"replace uses","path":"workflows/test.yaml","owner":"actions","repo":"setup-go","ref":"v5","commitHash":"f111f3307d8850f501ac008e886eec1fd1932a34"}
{"time":"2025-03-18T14:42:21.415795+09:00","level":"INFO","msg":"replace uses","path":"workflows/test.yaml","owner":"actions","repo":"checkout","ref":"v4","commitHash":"11bd71901bbe5b1630ceea73d27597364c9af683"}
Replaced uses:
  - actions/setup-go@v5 -> f111f3307d8850f501ac008e886eec1fd1932a34
  - actions/checkout@v4 -> 11bd71901bbe5b1630ceea73d27597364c9af683

Replaced files:
  - .github/workflows/test.yaml
```

The result will be as follows:

```yaml
name: Test
on:
  push:
    branches:
      - master
      - main
  pull_request:
    types:
      - opened
      - synchronize
      - reopened

jobs:
  test:
    strategy:
      matrix:
        go:
          - "1.24"
    name: Build
    runs-on: ubuntu-latest
    steps:
      - name: Set up Go
        uses: actions/setup-go@f111f3307d8850f501ac008e886eec1fd1932a34 # v5
        with:
          go-version: ${{ matrix.go }}
        id: go

      - name: Check out code into the Go module directory
        uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4

      - name: Build & Test
        run: |
          go test -race ./... -timeout 30s
```

### GitHub Enterprise Server Support

`actionspin` now supports GitHub Enterprise Server (GHES). When working with both GitHub.com and GHES repositories, you can configure the tool to resolve action references from your enterprise instance:

```sh
# Using environment variables
export GHE_HOST=github.example.com
export GHE_TOKEN=ghp_enterprise_token
actionspin

# Or using command line flags
actionspin --ghe-host=github.example.com --ghe-token=ghp_enterprise_token
```

When GHES configuration is provided, `actionspin` will:
1. First attempt to resolve action references from your GitHub Enterprise Server instance
2. Fall back to GitHub.com if the action is not found on GHES
3. This allows seamless operation with mixed environments where some actions are hosted on GHES and others on GitHub.com

## Contributing

Please use GitHub's issue tracker for bug reports and feature requests. Pull requests are also welcome.

1. Fork the repository.
2. Create a feature branch.
3. Commit your changes.
4. Create a pull request.

## License

This project is licensed under the MIT License. See the [LICENSE](./LICENSE) file for details.
