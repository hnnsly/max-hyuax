//go:build integration

package postgres_test

import (
	"errors"
	"testing"
	"time"
	"uuid"

	"dommax/internal/app"
)

func TestPhotoMetadataRoundTrip(t *testing.T) {
	anna := demoUser(t, "resident_demo_1")
	is := newIssue(t, anna.ID)
	if err := store.Issues().Create(t.Context(), is); err != nil {
		t.Fatal(err)
	}
	at := time.Now().Truncate(time.Microsecond)
	first := app.Photo{ID: uuid.NewV7().String(), IssueID: is.ID(), UploadedBy: anna.ID, Width: 1600, Height: 1200, SizeBytes: 250_000, CreatedAt: at}
	second := first
	second.ID, second.CreatedAt = uuid.NewV7().String(), at.Add(time.Second)
	for _, p := range []app.Photo{second, first} {
		if err := store.Photos().Add(t.Context(), p); err != nil {
			t.Fatal(err)
		}
	}

	list, err := store.Photos().ListByIssue(t.Context(), is.ID())
	if err != nil || len(list) != 2 || list[0].ID != first.ID || list[1].ID != second.ID {
		t.Fatalf("list = %+v, err = %v (want upload order)", list, err)
	}
	got, err := store.Photos().Get(t.Context(), first.ID)
	if err != nil || got.IssueID != is.ID() || got.Width != 1600 || got.SizeBytes != 250_000 || !got.CreatedAt.Equal(at) {
		t.Fatalf("photo = %+v, err = %v", got, err)
	}
	if _, err := store.Photos().Get(t.Context(), uuid.NewV7().String()); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("unknown photo err = %v", err)
	}
	if empty, err := store.Photos().ListByIssue(t.Context(), uuid.NewV7().String()); err != nil || len(empty) != 0 {
		t.Fatalf("empty = %+v, err = %v", empty, err)
	}
}
