//go:build integration

package files_test

import (
	"cmp"
	"crypto/rand"
	"os"
	"strings"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"dommax/internal/storage/files"
)

// Настоящий MinIO поднимает task s3. Без TEST_S3_ENDPOINT тест пропускается.
func testS3Config(t *testing.T) files.S3Config {
	t.Helper()
	endpoint := os.Getenv("TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("TEST_S3_ENDPOINT не задан: MinIO не поднят (task s3)")
	}
	cfg := files.S3Config{
		Endpoint:  endpoint,
		AccessKey: cmp.Or(os.Getenv("TEST_S3_ACCESS_KEY"), "minioadmin"),
		SecretKey: cmp.Or(os.Getenv("TEST_S3_SECRET_KEY"), "minioadmin"),
		// Свой бакет на каждый прогон: тесты не трогают фото разработчика и друг друга.
		Bucket: "test-" + strings.ToLower(rand.Text()[:16]),
	}
	t.Cleanup(func() {
		c, err := minio.New(cfg.Endpoint, &minio.Options{Creds: credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, "")})
		if err == nil {
			_ = c.RemoveBucket(t.Context(), cfg.Bucket)
		}
	})
	return cfg
}

func TestS3Contract(t *testing.T) {
	cfg := testS3Config(t)
	s, err := files.NewS3(t.Context(), cfg)
	if err != nil {
		t.Fatalf("NewS3: %v", err)
	}
	defer s.Close()
	storeContract(t, s)

	// Бакет уже есть: второй экземпляр (перезапуск api) стартует без ошибки.
	again, err := files.NewS3(t.Context(), cfg)
	if err != nil {
		t.Fatalf("NewS3 on existing bucket: %v", err)
	}
	_ = again.Close()
}

func TestS3WrongCredentials(t *testing.T) {
	cfg := testS3Config(t)
	cfg.SecretKey = "wrong-secret-key"
	if _, err := files.NewS3(t.Context(), cfg); err == nil {
		t.Fatal("NewS3 with wrong secret succeeded")
	}
}
