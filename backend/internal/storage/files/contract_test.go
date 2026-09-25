package files_test

import (
	"bytes"
	"errors"
	"testing"
	"uuid"

	"dommax/internal/app"
	"dommax/internal/storage/files"
)

// storeContract — общие требования порта app.FileStore: им отвечают и диск, и S3.
func storeContract(t *testing.T, s app.FileStore) {
	t.Helper()
	ctx := t.Context()
	key := uuid.NewV7().String()
	data := []byte("\xff\xd8\xff photo bytes")

	if err := s.Put(ctx, key, data); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := s.Get(ctx, key)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("Get = %q, err = %v", got, err)
	}
	// Повторная запись по тому же ключу заменяет файл целиком.
	if err := s.Put(ctx, key, []byte("x")); err != nil {
		t.Fatalf("second Put: %v", err)
	}
	if got, _ := s.Get(ctx, key); string(got) != "x" {
		t.Fatalf("after overwrite Get = %q, want %q", got, "x")
	}
	if _, err := s.Get(ctx, uuid.NewV7().String()); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("missing file err = %v, want ErrNotFound", err)
	}

	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(ctx, key); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("after delete err = %v, want ErrNotFound", err)
	}
	// Повторное удаление не ошибка: файл мог убрать прошлый неудачный запрос.
	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("second Delete: %v", err)
	}

	// Ключ только непрозрачный id: пути, точки и разделители отклоняются.
	for _, bad := range []string{"", "../escape", "a/b", `a\b`, "..", "x.jpg", string(make([]byte, 100))} {
		if err := s.Put(ctx, bad, data); !errors.Is(err, app.ErrInvalidInput) {
			t.Errorf("Put(%q) err = %v, want ErrInvalidInput", bad, err)
		}
		if _, err := s.Get(ctx, bad); !errors.Is(err, app.ErrInvalidInput) {
			t.Errorf("Get(%q) err = %v, want ErrInvalidInput", bad, err)
		}
		if err := s.Delete(ctx, bad); !errors.Is(err, app.ErrInvalidInput) {
			t.Errorf("Delete(%q) err = %v, want ErrInvalidInput", bad, err)
		}
	}
}

func TestDiskContract(t *testing.T) {
	d, err := files.NewDisk(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	storeContract(t, d)
}
