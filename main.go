package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const version = "0.1.0"

// drillRoots: show immediate children of these dirs.
var drillRoots = []string{
	"Library/Application Support",
	"Library/Caches",
	".cache",
}

// singleRoots: show total size of each.
var singleRoots = []string{
	"Library/Containers",
	"Library/Group Containers",
	"Library/pnpm",
	"Library/Android",
	"Library/Developer",
	"Library/Unity",
	".npm",
	".nvm",
	".bun",
	"ws",
	"ai",
	"devi",
	"github.com",
	"web3",
}

// cleanupSearchRoots: where to find node_modules/venvs (skip system dirs).
var cleanupSearchRoots = []string{
	"projects",
	"ws",
	"ai",
	"devi",
	"github.com",
	"web3",
	"play",
}

type entry struct {
	path  string
	bytes int64
}

// dirSize walks a directory tree in parallel using a worker pool.
// Much faster than spawning du on cold macOS filesystem.
func dirSize(ctx context.Context, root string) int64 {
	type workItem struct{ path string }

	work := make(chan workItem, 512)
	var total atomic.Int64
	var wg sync.WaitGroup

	workers := runtime.NumCPU() * 4
	for range workers {
		go func() {
			for item := range work {
				select {
				case <-ctx.Done():
					wg.Done()
					continue
				default:
				}
				entries, err := os.ReadDir(item.path)
				if err != nil {
					wg.Done()
					continue
				}
				for _, e := range entries {
					info, err := e.Info()
					if err != nil {
						continue
					}
					total.Add(info.Size())
					if e.IsDir() && !isSymlink(info) {
						wg.Add(1)
						select {
						case work <- workItem{filepath.Join(item.path, e.Name())}:
						default:
							// channel full: process inline to avoid deadlock
							wg.Add(-1)
							inline(ctx, filepath.Join(item.path, e.Name()), &total)
						}
					}
				}
				wg.Done()
			}
		}()
	}

	wg.Add(1)
	work <- workItem{root}
	wg.Wait()
	close(work)
	return total.Load()
}

func isSymlink(info fs.FileInfo) bool {
	return info.Mode()&fs.ModeSymlink != 0
}

// inline walks a subtree synchronously (fallback when work channel is full).
func inline(ctx context.Context, root string, total *atomic.Int64) {
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || ctx.Err() != nil {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err == nil {
				total.Add(info.Size())
			}
		}
		return nil
	})
}

// scanDir returns immediate subdirs of root with their sizes.
func scanDir(ctx context.Context, root string, spinLabel chan<- string) []entry {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	type result struct {
		path  string
		bytes int64
	}
	ch := make(chan result, len(entries))
	var wg sync.WaitGroup
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(root, e.Name())
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			select {
			case spinLabel <- "scanning " + shortenPath(path):
			default:
			}
			ch <- result{path, dirSize(ctx, path)}
		}(p)
	}
	wg.Wait()
	close(ch)

	var out []entry
	for r := range ch {
		if r.bytes > 0 {
			out = append(out, entry{r.path, r.bytes})
		}
	}
	return out
}

func shortenPath(p string) string {
	home, _ := os.UserHomeDir()
	rel := strings.TrimPrefix(p, home)
	// only show top 3 path segments so spinner isn't flooded by deep subdirs
	parts := strings.SplitN(strings.TrimPrefix(rel, "/"), "/", 4)
	if len(parts) > 3 {
		parts = parts[:3]
	}
	return "~/" + strings.Join(parts, "/")
}

// findCleanup locates node_modules/.venv/venv in project dirs and measures them.
func findCleanup(ctx context.Context, home string) []entry {
	var searchPaths []string
	for _, rel := range cleanupSearchRoots {
		p := filepath.Join(home, rel)
		if _, err := os.Stat(p); err == nil {
			searchPaths = append(searchPaths, p)
		}
	}
	if len(searchPaths) == 0 {
		return nil
	}

	args := append([]string{"-maxdepth", "7", "-type", "d",
		"(", "-name", "node_modules", "-o", "-name", ".venv", "-o", "-name", "venv", ")",
		"-prune"}, searchPaths...)
	out, err := exec.CommandContext(ctx, "find", args...).Output()
	if err != nil {
		return nil
	}
	var paths []string
	for _, p := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if p != "" {
			paths = append(paths, p)
		}
	}

	ch := make(chan entry, len(paths))
	var wg sync.WaitGroup
	for _, p := range paths {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			ch <- entry{path, dirSize(ctx, path)}
		}(p)
	}
	wg.Wait()
	close(ch)

	var result []entry
	for e := range ch {
		result = append(result, e)
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
	i, current := 0, "scanning..."
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
	timeoutSec := flag.Int("timeout", 60, "scan timeout in seconds")
	showVer := flag.Bool("version", false, "print version")
	flag.Parse()

	if *showVer {
		fmt.Println("macsanity", version)
		return
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	spinLabel := make(chan string, 16)
	spinDone := make(chan struct{})
	go spinner(spinLabel, spinDone)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutSec)*time.Second)
	defer cancel()

	var mu sync.Mutex
	seen := map[string]bool{}
	var all []entry
	var cleanup []entry
	var wg sync.WaitGroup

	addAll := func(entries []entry) {
		mu.Lock()
		for _, e := range entries {
			if !seen[e.path] {
				seen[e.path] = true
				all = append(all, e)
			}
		}
		mu.Unlock()
	}

	// drill into known hotspot dirs
	for _, rel := range drillRoots {
		rel := rel
		wg.Add(1)
		go func() {
			defer wg.Done()
			addAll(scanDir(ctx, filepath.Join(home, rel), spinLabel))
		}()
	}

	// measure single dirs
	for _, rel := range singleRoots {
		rel := rel
		p := filepath.Join(home, rel)
		if _, err := os.Stat(p); err != nil {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case spinLabel <- "scanning " + shortenPath(p):
			default:
			}
			size := dirSize(ctx, p)
			if size > 0 {
				mu.Lock()
				if !seen[p] {
					seen[p] = true
					all = append(all, entry{p, size})
				}
				mu.Unlock()
			}
		}()
	}

	// find node_modules / venvs
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case spinLabel <- "finding node_modules · .venv · venv...":
		default:
		}
		cleanup = findCleanup(ctx, home)
	}()

	wg.Wait()
	close(spinDone)
	time.Sleep(90 * time.Millisecond)

	min := int64(*minGB * float64(1<<30))
	fmt.Println()
	printSection("DISK HOGS", all, min)
	printSection("CLEANUP CANDIDATES  (node_modules · .venv · venv)", cleanup, 10<<20)

}
