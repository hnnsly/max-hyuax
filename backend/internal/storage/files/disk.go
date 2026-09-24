// Пакет files — хранилище файлов на диске (том в Docker). Для MVP вместо MinIO (ADR-015).
package files

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"

	"dommax/internal/app"
)

// Ключ — непрозрачный id (uuid): без точек и разделителей, выйти за каталог нельзя.
var keyRe = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)

// Disk хранит файлы в одном каталоге; os.Root дополнительно не даёт выйти за его пределы.
type Disk struct{ root *os.Root }

func NewDisk(dir string) (*Disk, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	return &Disk{root: root}, nil
}

func (d *Disk) Close() error { return d.root.Close() }

// Put записывает файл через временное имя и переименование: читатель не увидит половину файла.
func (d *Disk) Put(_ context.Context, key string, data []byte) error {
	if !keyRe.MatchString(key) {
		return fmt.Errorf("%w: bad file key", app.ErrInvalidInput)
	}
	tmp := key + ".tmp-" + rand.Text()
	if err := d.root.WriteFile(tmp, data, 0o640); err != nil {
		return err
	}
	if err := d.root.Rename(tmp, key); err != nil {
		_ = d.root.Remove(tmp)
		return err
	}
	return nil
}

func (d *Disk) Get(_ context.Context, key string) ([]byte, error) {
	if !keyRe.MatchString(key) {
		return nil, fmt.Errorf("%w: bad file key", app.ErrInvalidInput)
	}
	data, err := d.root.ReadFile(key)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, app.ErrNotFound
	}
	return data, err
}
