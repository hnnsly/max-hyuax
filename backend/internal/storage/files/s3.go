package files

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"dommax/internal/app"
)

// S3Config — подключение к S3-совместимому хранилищу: MinIO в compose или облачный S3 (ADR-018).
type S3Config struct {
	Endpoint  string // host:port без схемы, например minio:9000
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

// S3 хранит фото объектами в одном бакете; ключ объекта — id фото.
type S3 struct {
	client *minio.Client
	bucket string
}

// NewS3 подключается к хранилищу и создаёт бакет, если его нет.
// Ошибка доступа видна сразу при старте, а не при первой загрузке фото жителем.
func NewS3(ctx context.Context, cfg S3Config) (*S3, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, err
	}
	s := &S3{client: client, bucket: cfg.Bucket}
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("s3 bucket %q: %w", cfg.Bucket, err)
	}
	if exists {
		return s, nil
	}
	err = client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{})
	// Два экземпляра api могли стартовать одновременно: бакет, созданный соседом, тоже подходит.
	if err != nil && minio.ToErrorResponse(err).Code != minio.BucketAlreadyOwnedByYou {
		return nil, fmt.Errorf("s3 make bucket %q: %w", cfg.Bucket, err)
	}
	return s, nil
}

// Close ничего не держит: клиент ходит по HTTP без постоянных соединений, которые нужно закрывать.
func (s *S3) Close() error { return nil }

// Put записывает объект целиком одним запросом: читатель не увидит половину файла.
func (s *S3) Put(ctx context.Context, key string, data []byte) error {
	if !keyRe.MatchString(key) {
		return fmt.Errorf("%w: bad file key", app.ErrInvalidInput)
	}
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)),
		minio.PutObjectOptions{ContentType: "image/jpeg"})
	return err
}

func (s *S3) Get(ctx context.Context, key string) ([]byte, error) {
	if !keyRe.MatchString(key) {
		return nil, fmt.Errorf("%w: bad file key", app.ErrInvalidInput)
	}
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, notFound(err)
	}
	defer obj.Close()
	// GetObject ленивый: ошибка «нет объекта» приходит только при чтении.
	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, notFound(err)
	}
	return data, nil
}

// Delete удаляет объект; S3 не считает ошибкой удаление отсутствующего ключа.
func (s *S3) Delete(ctx context.Context, key string) error {
	if !keyRe.MatchString(key) {
		return fmt.Errorf("%w: bad file key", app.ErrInvalidInput)
	}
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

func notFound(err error) error {
	if minio.ToErrorResponse(err).Code == minio.NoSuchKey {
		return app.ErrNotFound
	}
	return err
}
