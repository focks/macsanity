package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const version = "0.1.0"

// scanRoots are drilled into at depth 1 to surface nested large dirs.
var scanRoots = []string{
	"",
	"Library",
	".cache",
	"Library/Application Support",
	"Library/Containers",
	"Library/Group Containers",
}

type entry struct {
	path  string
	bytes int64
}

func scan(root string) []entry {
	out, err := exec.Command("du", "-d", "1", "-k", root).Output()
	if err != nil {
		return nil
	}
	var result []entry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		p := parts[1]
		if p == root {
			continue
		}
		kb, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
		if err != nil {
			continue
		}
		result = append(result, entry{p, kb * 1024})
	}
	return result
}

func human(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1fG", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.0fM", float64(b)/float64(1<<20))
	default:
		return fmt.Sprintf("%.0fK", float64(b)/float64(1<<10))
	}
}

func bar(b, max int64, width int) string {
	if max == 0 {
		return strings.Repeat("░", width)
	}
	filled := int(float64(b) / float64(max) * float64(width))
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func main() {
	minGB := flag.Float64("min", 1.0, "minimum size in GB to show")
	showVer := flag.Bool("version", false, "print version")
	flag.Parse()

	if *showVer {
		fmt.Println("macsanity", version)
		return
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: cannot determine home directory:", err)
		os.Exit(1)
	}

	seen := map[string]bool{}
	var all []entry
	for _, rel := range scanRoots {
		root := home
		if rel != "" {
			root = filepath.Join(home, rel)
		}
		for _, e := range scan(root) {
			if !seen[e.path] {
				seen[e.path] = true
				all = append(all, e)
			}
		}
	}

	min := int64(*minGB * float64(1<<30))
	var filtered []entry
	for _, e := range all {
		if e.bytes >= min {
			filtered = append(filtered, e)
		}
	}

	if len(filtered) == 0 {
		fmt.Printf("nothing above %.1f GB\n", *minGB)
		return
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].bytes > filtered[j].bytes
	})

	maxBytes := filtered[0].bytes
	fmt.Printf("\n  %-8s  %-20s  %s\n", "SIZE", "BAR", "PATH")
	fmt.Println("  " + strings.Repeat("─", 80))
	for _, e := range filtered {
		fmt.Printf("  %-8s  [%s]  %s\n", human(e.bytes), bar(e.bytes, maxBytes, 18), e.path)
	}
	fmt.Println()
}
