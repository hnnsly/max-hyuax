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
// Перед переименованием данные сбрасываются на диск, иначе после сбоя питания файл может оказаться пустым.
func (d *Disk) Put(_ context.Context, key string, data []byte) error {
	if !keyRe.MatchString(key) {
		return fmt.Errorf("%w: bad file key", app.ErrInvalidInput)
	}
	tmp := key + ".tmp-" + rand.Text()
	if err := d.writeSynced(tmp, data); err != nil {
		_ = d.root.Remove(tmp)
		return err
	}
	if err := d.root.Rename(tmp, key); err != nil {
		_ = d.root.Remove(tmp)
		return err
	}
	return nil
}

func (d *Disk) writeSynced(name string, data []byte) error {
	f, err := d.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	return errors.Join(err, f.Close())
}

// Delete удаляет файл; если его уже нет, это не ошибка.
func (d *Disk) Delete(_ context.Context, key string) error {
	if !keyRe.MatchString(key) {
		return fmt.Errorf("%w: bad file key", app.ErrInvalidInput)
	}
	if err := d.root.Remove(key); err != nil && !errors.Is(err, fs.ErrNotExist) {
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
