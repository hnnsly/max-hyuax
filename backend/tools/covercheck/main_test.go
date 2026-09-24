package main

import (
	"strings"
	"testing"
)

const profile = `mode: set
dommax/internal/app/auth/service.go:10.1,12.2 2 1
dommax/internal/app/auth/service.go:14.1,16.2 2 0
dommax/internal/app/auth/service.go:14.1,16.2 2 1
dommax/internal/app/auth/service.go:18.1,20.2 1 0
dommax/internal/transport/httpapi/server.go:5.1,6.2 4 1
dommax/internal/storage/postgres/sqlcdb/db.go:1.1,2.2 10 0
`

func TestParseMergesRepeatedBlocks(t *testing.T) {
	stats, err := parse(strings.NewReader(profile))
	if err != nil {
		t.Fatal(err)
	}
	a := stats["dommax/internal/app/auth"]
	// Один и тот же блок из разных тестовых бинарников считается один раз и покрыт, если покрыт хоть где-то.
	if a == nil || a.total != 5 || a.covered != 4 {
		t.Fatalf("auth = %+v", a)
	}
}

func TestCheckAppliesThresholdsAndExclusions(t *testing.T) {
	stats, _ := parse(strings.NewReader(profile))
	opts := options{minTotal: 80, minCore: 80, core: []string{"dommax/internal/app/"}, exclude: []string{"/sqlcdb"}}
	total, failures := check(stats, opts)
	// auth 4/5 + httpapi 4/4 = 8/9; сгенерированный sqlcdb не учитывается.
	if total < 88.8 || total > 88.9 {
		t.Fatalf("total = %.2f", total)
	}
	if len(failures) != 0 {
		t.Fatalf("failures = %v", failures)
	}
	opts.minCore = 90
	if _, failures := check(stats, opts); len(failures) != 1 || !strings.Contains(failures[0], "app/auth") {
		t.Fatalf("failures = %v, want auth below core threshold", failures)
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	if _, err := parse(strings.NewReader("mode: set\nnot a profile line\n")); err == nil {
		t.Fatal("broken profile must be an error")
	}
}
