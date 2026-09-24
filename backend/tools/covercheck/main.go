// Команда covercheck проверяет покрытие по профилю go test -coverprofile: общий порог
// и порог для каждого пакета ядра (domain, app). Нужна в CI, чтобы покрытие не падало.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
)

type stat struct{ total, covered int }

func (s stat) percent() float64 {
	if s.total == 0 {
		return 100
	}
	return 100 * float64(s.covered) / float64(s.total)
}

type options struct {
	minTotal, minCore float64
	core, exclude     []string
}

// parse читает профиль и считает операторы по пакетам. С -coverpkg один блок встречается
// в профиле несколько раз (по разу на тестовый бинарник): учитываем его один раз.
func parse(r io.Reader) (map[string]*stat, error) {
	type block struct {
		stmts   int
		covered bool
	}
	blocks := map[string]*block{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		pos, rest, ok := strings.Cut(line, " ")
		stmtsStr, countStr, ok2 := strings.Cut(rest, " ")
		if !ok || !ok2 {
			return nil, fmt.Errorf("bad profile line %q", line)
		}
		stmts, err1 := strconv.Atoi(stmtsStr)
		count, err2 := strconv.Atoi(countStr)
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("bad profile line %q", line)
		}
		b := blocks[pos]
		if b == nil {
			b = &block{stmts: stmts}
			blocks[pos] = b
		}
		b.covered = b.covered || count > 0
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	stats := map[string]*stat{}
	for pos, b := range blocks {
		file, _, _ := strings.Cut(pos, ":")
		pkg := path.Dir(file)
		s := stats[pkg]
		if s == nil {
			s = &stat{}
			stats[pkg] = s
		}
		s.total += b.stmts
		if b.covered {
			s.covered += b.stmts
		}
	}
	return stats, nil
}

// check возвращает общее покрытие и список нарушенных порогов.
func check(stats map[string]*stat, o options) (float64, []string) {
	var sum stat
	var failures []string
	for _, pkg := range slices.Sorted(maps.Keys(stats)) {
		if slices.ContainsFunc(o.exclude, func(e string) bool { return strings.Contains(pkg, e) }) {
			continue
		}
		s := stats[pkg]
		sum.total += s.total
		sum.covered += s.covered
		isCore := slices.ContainsFunc(o.core, func(p string) bool { return strings.HasPrefix(pkg, p) })
		if isCore && s.percent() < o.minCore {
			failures = append(failures, fmt.Sprintf("%s: %.1f%% < %.0f%%", pkg, s.percent(), o.minCore))
		}
	}
	if sum.percent() < o.minTotal {
		failures = append(failures, fmt.Sprintf("total: %.1f%% < %.0f%%", sum.percent(), o.minTotal))
	}
	return sum.percent(), failures
}

func main() {
	profile := flag.String("profile", "cover.out", "coverage profile from go test -coverprofile")
	minTotal := flag.Float64("min-total", 80, "minimum total coverage, percent")
	minCore := flag.Float64("min-core", 80, "minimum coverage of each core package, percent")
	core := flag.String("core", "dommax/internal/domain/,dommax/internal/app/", "comma-separated core package prefixes")
	exclude := flag.String("exclude", "/sqlcdb,/apptest,/pgtest", "comma-separated package substrings to skip")
	flag.Parse()

	f, err := os.Open(*profile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer f.Close()
	stats, err := parse(f)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	opts := options{minTotal: *minTotal, minCore: *minCore, core: strings.Split(*core, ","), exclude: strings.Split(*exclude, ",")}
	for _, pkg := range slices.Sorted(maps.Keys(stats)) {
		fmt.Printf("%6.1f%%  %s\n", stats[pkg].percent(), pkg)
	}
	total, failures := check(stats, opts)
	fmt.Printf("%6.1f%%  total (without %s)\n", total, *exclude)
	if len(failures) > 0 {
		fmt.Fprintln(os.Stderr, "coverage below threshold:\n  "+strings.Join(failures, "\n  "))
		os.Exit(1)
	}
}
