# macsanity

Find what's eating your Mac's disk — fast, zero dependencies.

```
  SIZE      BAR                     PATH
  ────────────────────────────────────────────────────────────────────────────────
  53.2G     [██████████████████]    /Users/you/.cache
  18.4G     [██████░░░░░░░░░░░░]    /Users/you/projects
  10.1G     [███░░░░░░░░░░░░░░░]    /Users/you/Library/Application Support/JetBrains
  8.3G      [██░░░░░░░░░░░░░░░░]    /Users/you/Library/Application Support/Claude
  8.6G      [██░░░░░░░░░░░░░░░░]    /Users/you/.cache/uv
  5.3G      [█░░░░░░░░░░░░░░░░░]    /Users/you/Library/pnpm
```

## Install

```bash
go install github.com/focks/macsanity@latest
```

Or build from source:

```bash
git clone https://github.com/focks/macsanity
cd macsanity
go build -o macsanity .
sudo mv macsanity /usr/local/bin/
```

## Usage

```bash
macsanity                # show everything >= 1 GB
macsanity --min 5        # only show >= 5 GB
macsanity --min 0.5      # show >= 500 MB
macsanity --version
```

## How it works

Scans your home directory and key macOS locations (`Library`, `.cache`,
`Library/Application Support`, `Library/Containers`) at depth 1 each,
deduplicates, filters by the minimum threshold, and prints sorted by size.

Uses the system `du` command — no filesystem walking in userspace, no
dependencies beyond the Go stdlib.

## License

MIT
