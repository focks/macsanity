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

func main() {
	minGB := flag.Float64("min", 1.0, "minimum size in GB to show")
	flag.Parse()

	home, _ := os.UserHomeDir()

	roots := []string{
		home,
		filepath.Join(home, "Library"),
		filepath.Join(home, ".cache"),
		filepath.Join(home, "Library", "Application Support"),
		filepath.Join(home, "Library", "Containers"),
		filepath.Join(home, "Library", "Group Containers"),
	}

	seen := map[string]bool{}
	var all []entry
	for _, root := range roots {
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

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].bytes > filtered[j].bytes
	})

	fmt.Printf("\n  %-8s  %s\n", "SIZE", "PATH")
	fmt.Println("  " + strings.Repeat("─", 72))
	for _, e := range filtered {
		fmt.Printf("  %-8s  %s\n", human(e.bytes), e.path)
	}
	fmt.Println()
}
