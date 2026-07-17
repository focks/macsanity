# macsanity

Find what's eating your Mac's disk — fast, zero dependencies.

```
  DISK HOGS
  ────────────────────────────────────────────────────────────────────────────────
  63.3G  [██████████████████]  /Users/you/Library/Containers
  11.2G  [███░░░░░░░░░░░░░░░]  /Users/you/.npm
  10.8G  [███░░░░░░░░░░░░░░░]  /Users/you/Library/Application Support/Claude
   7.8G  [██░░░░░░░░░░░░░░░░]  /Users/you/.cache/uv
   6.5G  [█░░░░░░░░░░░░░░░░░]  /Users/you/Library/Android
  ──────────────────────────────  total: 131.4G

  CLEANUP CANDIDATES  (node_modules · .venv · venv)
  ────────────────────────────────────────────────────────────────────────────────
  1.2G   [██████████████████]  /Users/you/projects/aalo/node_modules
  513M   [███████░░░░░░░░░░░]  /Users/you/ws/vbook/node_modules
  ──────────────────────────────  total: 4.6G
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
macsanity                    # scan home hotspots, show >= 3 GB (default)
macsanity .                  # scan current directory
macsanity ~/projects         # scan a specific directory
macsanity --min 5            # only show >= 5 GB
macsanity --min 0.5          # show >= 500 MB
macsanity --timeout 120      # longer timeout for slow disks (default: 60s)
macsanity --version          # print version
macsanity --help             # show all flags
```

## How it works

- **No args**: drills into `~/Library/Application Support`, `~/.cache`, and other known macOS hotspots in parallel, plus measures common project dirs and npm/nvm stores.
- **With a dir arg**: scans that directory's immediate children and finds `node_modules`/`.venv`/`venv` inside it.
- Uses a parallel Go filesystem walker (not `du`) so it saturates NVMe I/O and stays responsive on cold cache.
- Shows a braille spinner with live path updates while scanning.

## License

MIT
