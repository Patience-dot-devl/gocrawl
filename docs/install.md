# Installing & running gocrawl

How to install, build, and run `gocrawl` on **Windows**, **macOS**, and **Linux**.

`gocrawl` is a single self-contained binary with no runtime dependencies — except for
[headless rendering](#headless-rendering-optional), which needs a Chromium-class browser.

## Prerequisites

| What | When you need it | Notes |
| --- | --- | --- |
| [**Go 1.26+**](https://go.dev/dl/) | To `go install` or build from source | Not needed if you download a prebuilt binary. Check with `go version`. |
| **Git** | Only to build from source | [git-scm.com/downloads](https://git-scm.com/downloads) |
| **Chrome / Chromium / Edge** | Only for `--render headless` | Must be on `PATH`. See [Headless rendering](#headless-rendering-optional). |

## Install with `go install` (all platforms)

The simplest route if you have Go. This compiles and drops the binary in your Go bin
directory:

```sh
go install github.com/Patience-dot-devl/gocrawl/cmd/gocrawl@latest
```

The binary lands in `$(go env GOBIN)`, or `$(go env GOPATH)/bin` if `GOBIN` is unset.
For the `gocrawl` command to work from anywhere, that directory must be on your `PATH`.

### Putting the Go bin directory on your PATH

The quickest way is to let `gocrawl` do it for you. Run it once by its full path — it
detects where it lives and, if that directory isn't on your `PATH`, offers to add it:

```sh
# macOS / Linux (adjust if GOBIN is set)
"$(go env GOPATH)/bin/gocrawl" path
```

```powershell
# Windows
& "$(go env GOPATH)\bin\gocrawl.exe" path
```

On macOS/Linux it appends a line to your shell profile (`~/.zshrc`, `~/.bashrc`,
`~/.bash_profile`, or fish's `config.fish`, picked from your `$SHELL`). On Windows it updates
your user `PATH` in the registry. It's safe to re-run — it does nothing if you're already set
up — and pass `--yes` to skip the prompt. Then restart your terminal.

Prefer to do it by hand? The manual steps per platform:

<details>
<summary><b>macOS / Linux</b> (bash or zsh)</summary>

```sh
# See where Go installs binaries
go env GOPATH        # e.g. /Users/you/go  →  binaries in /Users/you/go/bin

# Add it to your shell profile (~/.zshrc on macOS, ~/.bashrc on most Linux)
echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.zshrc
source ~/.zshrc
```
</details>

<details>
<summary><b>Windows</b> (PowerShell)</summary>

The installed binary is `gocrawl.exe`. Find and add the Go bin dir to your user `PATH`:

```powershell
# See where Go installs binaries
go env GOPATH        # e.g. C:\Users\you\go  →  binaries in C:\Users\you\go\bin

# Add it to your user PATH permanently
$gobin = "$(go env GOPATH)\bin"
[Environment]::SetEnvironmentVariable(
  "Path", "$([Environment]::GetEnvironmentVariable('Path','User'));$gobin", "User")
```

Close and reopen your terminal afterwards so the new `PATH` takes effect.
</details>

## Build from source

```sh
git clone https://github.com/Patience-dot-devl/gocrawl
cd gocrawl
```

**macOS / Linux** (uses the [`Makefile`](../Makefile)):

```sh
make build          # produces ./gocrawl
make install        # or install into your Go bin dir (go install ./cmd/gocrawl)
```

**Windows** (PowerShell — `make` is usually not present, so call `go` directly):

```powershell
go build -o gocrawl.exe ./cmd/gocrawl     # produces .\gocrawl.exe
# or install into your Go bin dir:
go install ./cmd/gocrawl
```

> The plain `go build` / `go install` commands work identically on every platform if you
> prefer not to use `make`.

## Verify the install

```sh
gocrawl --help
gocrawl analyzers list
```

On Windows, if `gocrawl` isn't found but you built locally, run it as `.\gocrawl.exe` from
the build directory, or finish the [PATH setup](#putting-the-go-bin-directory-on-your-path).

## Running gocrawl

The command is identical across platforms — on Windows the binary is `gocrawl.exe`, but you
can still type `gocrawl` once it's on your `PATH`.

```sh
# Crawl one level deep and write a JSON report
gocrawl crawl https://example.com --depth 1 --out report.json

# CSV with a page cap and higher concurrency
gocrawl crawl https://example.com --max-pages 200 --concurrency 8 --format csv --out report.csv

# Self-contained HTML report to open in a browser
gocrawl crawl https://example.com --format html --out report.html

# Run gocrawl with no arguments for an interactive menu
gocrawl
```

See the [Quick start](../README.md#quick-start) and [Configuration guide](configuration.md)
for every flag and option.

### A note on file paths

`--out` accepts platform-native paths:

- **macOS / Linux:** `--out reports/site.json`
- **Windows (PowerShell / cmd):** `--out reports\site.json` (or forward slashes — both work)

## Headless rendering (optional)

`--render headless` renders pages in a real browser tab (running JavaScript) and captures
lab-mode Core Web Vitals (LCP, FCP, CLS, TBT, TTFB) for the `perf` analyzer. This requires a
**Chromium-class browser — Chrome, Chromium, or Microsoft Edge — discoverable on `PATH`**.
`gocrawl` launches it; it does not download one for you. If none is found, the crawl fails
fast with a launch error (raw crawling still works without a browser).

```sh
gocrawl crawl https://example.com --render headless --format html --out report.html
```

| OS | Getting a compatible browser |
| --- | --- |
| **macOS** | Install [Google Chrome](https://www.google.com/chrome/) (or `brew install --cask chromium` / `google-chrome`). chromedp finds Chrome in `/Applications` automatically. |
| **Linux** | Install Chrome or Chromium via your package manager, e.g. `sudo apt install chromium` (Debian/Ubuntu) or `sudo dnf install chromium` (Fedora). On headless servers it runs without a display. |
| **Windows** | Google Chrome or Microsoft Edge (preinstalled on Windows 10/11) both work; chromedp locates them automatically. |

## Use as an MCP server

`gocrawl mcp` runs it as a Model Context Protocol server over stdio so agent tools can drive
crawls. The `gocrawl` command must be on the client's `PATH` (or referenced by absolute path
— `C:\Users\you\go\bin\gocrawl.exe` on Windows). See the [MCP server guide](mcp.md).

## Troubleshooting

| Symptom | Fix |
| --- | --- |
| `command not found: gocrawl` / `'gocrawl' is not recognized` | The Go bin dir isn't on your `PATH`. Run `gocrawl path` (by full path — see [PATH setup](#putting-the-go-bin-directory-on-your-path)) to add it, then restart your terminal. |
| `go: command not found` | Install [Go 1.26+](https://go.dev/dl/) and reopen your terminal. |
| `launching headless browser: ...` | No Chrome/Chromium/Edge on `PATH` — install one (see [Headless rendering](#headless-rendering-optional)) or drop `--render headless`. |
| `make: command not found` (Windows) | Use the `go build` / `go install` commands directly — see [Build from source](#build-from-source). |
