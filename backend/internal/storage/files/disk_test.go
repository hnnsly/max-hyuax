package files_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"dommax/internal/app"
	"dommax/internal/storage/files"
)

func TestDiskPutGet(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "photos") // каталога ещё нет: хранилище создаёт его само
	d, err := files.NewDisk(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	data := []byte("\xff\xd8\xff photo bytes")
	if err := d.Put(t.Context(), "0190a000-0000-7000-8000-000000000001", data); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := d.Get(t.Context(), "0190a000-0000-7000-8000-000000000001")
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("Get = %q, err = %v", got, err)
	}
	if _, err := d.Get(t.Context(), "0190a000-0000-7000-8000-000000000002"); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("missing file err = %v, want ErrNotFound", err)
	}
	// Временные файлы после записи не остаются.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("dir has %d entries, want 1", len(entries))
	}
}
