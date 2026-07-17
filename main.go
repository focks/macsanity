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
	"time"
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

// findCleanup locates node_modules, .venv, and venv dirs and returns their sizes.
func findCleanup(home string) []entry {
	out, err := exec.Command("find", home, "-maxdepth", "7", "-type", "d",
		"(", "-name", "node_modules", "-o", "-name", ".venv", "-o", "-name", "venv", ")",
		"-prune").Output()
	if err != nil {
		return nil
	}
	var paths []string
	for _, p := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	if len(paths) == 0 {
		return nil
	}

	// du -sk on all paths at once
	args := append([]string{"-sk"}, paths...)
	out, err = exec.Command("du", args...).Output()
	if err != nil {
		return nil
	}
	var result []entry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		kb, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
		if err != nil {
			continue
		}
		result = append(result, entry{parts[1], kb * 1024})
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

func printSection(title string, entries []entry, minBytes int64) {
	var filtered []entry
	for _, e := range entries {
		if e.bytes >= minBytes {
			filtered = append(filtered, e)
		}
	}
	if len(filtered) == 0 {
		return
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].bytes > filtered[j].bytes
	})
	maxBytes := filtered[0].bytes
	var total int64
	for _, e := range filtered {
		total += e.bytes
	}

	fmt.Printf("  %s\n", title)
	fmt.Println("  " + strings.Repeat("─", 80))
	for _, e := range filtered {
		fmt.Printf("  %-8s  [%s]  %s\n", human(e.bytes), bar(e.bytes, maxBytes, 18), e.path)
	}
	fmt.Printf("  %s  total: %s\n\n", strings.Repeat("─", 30), human(total))
}

func spinner(label <-chan string, done <-chan struct{}) {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	i := 0
	current := "scanning..."
	for {
		select {
		case <-done:
			fmt.Print("\r\033[K")
			return
		case l := <-label:
			current = l
		default:
			fmt.Printf("\r  %s %s", frames[i%len(frames)], current)
			time.Sleep(80 * time.Millisecond)
			i++
		}
	}
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

	spinLabel := make(chan string, 1)
	spinDone := make(chan struct{})
	go spinner(spinLabel, spinDone)

	// phase 1: top-level dir sizes
	seen := map[string]bool{}
	var all []entry
	for _, rel := range scanRoots {
		root := home
		if rel != "" {
			root = filepath.Join(home, rel)
			select {
			case spinLabel <- "scanning ~/" + rel:
			default:
			}
		}
		for _, e := range scan(root) {
			if !seen[e.path] {
				seen[e.path] = true
				all = append(all, e)
			}
		}
	}

	// phase 2: find cleanup candidates
	select {
	case spinLabel <- "finding node_modules · .venv · venv ...":
	default:
	}
	cleanup := findCleanup(home)

	close(spinDone)
	time.Sleep(90 * time.Millisecond)

	min := int64(*minGB * float64(1<<30))

	fmt.Println()
	printSection("DISK HOGS", all, min)
	printSection("CLEANUP CANDIDATES  (node_modules · .venv · venv)", cleanup, 10<<20) // >= 10 MB
}
