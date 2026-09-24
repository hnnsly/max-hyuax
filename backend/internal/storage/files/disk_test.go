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

func TestDiskDelete(t *testing.T) {
	d, err := files.NewDisk(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	const key = "0190a000-0000-7000-8000-000000000001"
	if err := d.Put(t.Context(), key, []byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := d.Delete(t.Context(), key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := d.Get(t.Context(), key); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("after delete err = %v, want ErrNotFound", err)
	}
	// Повторное удаление — не ошибка: файл мог убрать прошлый неудачный запрос.
	if err := d.Delete(t.Context(), key); err != nil {
		t.Fatalf("second Delete: %v", err)
	}
	if err := d.Delete(t.Context(), "../escape"); err == nil {
		t.Fatal("Delete with unsafe key succeeded")
	}
}

// Ключ — только непрозрачный id: пути, точки и разделители отклоняются.
func TestDiskRejectsUnsafeKeys(t *testing.T) {
	d, err := files.NewDisk(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	for _, key := range []string{"", "../escape", "a/b", `a\b`, "..", "x.jpg", string(make([]byte, 100))} {
		if err := d.Put(t.Context(), key, []byte("x")); err == nil {
			t.Errorf("Put(%q) succeeded", key)
		}
		if _, err := d.Get(t.Context(), key); err == nil {
			t.Errorf("Get(%q) succeeded", key)
		}
	}
}
