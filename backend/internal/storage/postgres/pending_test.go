//go:build integration

package postgres_test

import (
	"testing"
	"time"

	"dommax/internal/app"
)

// Ожидающее действие бота: одно на пользователя, забирается один раз, истёкшее не отдаётся.
func TestBotPending(t *testing.T) {
	ctx := t.Context()
	anna := demoUser(t, "resident_demo_1")
	now := time.Now().Truncate(time.Microsecond)
	repo := store.Pending()

	if err := repo.Set(ctx, anna.ID, app.BotPending{Action: "reopen", Ref: "x", ExpiresAt: now.Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	// Новое действие заменяет старое.
	if err := repo.Set(ctx, anna.ID, app.BotPending{Action: "propose", ExpiresAt: now.Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	p, ok, err := repo.Take(ctx, anna.ID, now)
	if err != nil || !ok || p.Action != "propose" {
		t.Fatalf("Take = %+v %v %v", p, ok, err)
	}
	if _, ok, _ := repo.Take(ctx, anna.ID, now); ok {
		t.Fatal("pending action taken twice")
	}
	_ = repo.Set(ctx, anna.ID, app.BotPending{Action: "reopen", ExpiresAt: now.Add(-time.Second)})
	if _, ok, _ := repo.Take(ctx, anna.ID, now); ok {
		t.Fatal("expired pending action returned")
	}
}
