//go:build live

// Проверки на живом Bot API: go test -tags live ./internal/storage/maxapi
// Нужны MAX_BOT_TOKEN и либо MAX_API_CA_FILE, либо MAX_API_INSECURE_TLS=true.
package maxapi_test

import (
	"os"
	"testing"

	"dommax/internal/storage/maxapi"
)

func liveClient(t *testing.T) *maxapi.Client {
	t.Helper()
	token := os.Getenv("MAX_BOT_TOKEN")
	if token == "" {
		t.Skip("MAX_BOT_TOKEN is not set")
	}
	hc, err := maxapi.HTTPClient(os.Getenv("MAX_API_CA_FILE"), os.Getenv("MAX_API_INSECURE_TLS") == "true")
	if err != nil {
		t.Fatal(err)
	}
	return maxapi.New(maxapi.DefaultBaseURL, token, hc)
}

func TestLiveMe(t *testing.T) {
	me, err := liveClient(t).Me(t.Context())
	if err != nil {
		t.Fatalf("Me: %v", err)
	}
	if !me.IsBot || me.Username == "" {
		t.Fatalf("me = %+v", me)
	}
	t.Logf("bot %s (%d)", me.Username, me.UserID)
}

func TestLiveUpdatesDecode(t *testing.T) {
	page, err := liveClient(t).Updates(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("Updates: %v", err)
	}
	for _, u := range page.Updates {
		t.Logf("update %s chat=%d user=%d", u.Type, u.ChatID, u.User.UserID)
	}
}
