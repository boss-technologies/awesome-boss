package fields

import (
	"crypto/rand"
	"database/sql/driver"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ImageField представляет загруженное изображение и реализует pgx-совместимый интерфейс.
type ImageField struct {
	Path   string // Путь к файлу относительно uploads-директории.
	Width  int    // Ширина изображения.
	Height int    // Высота изображения.
	Size   int64  // Размер файла в байтах.
	URL    string `json:"url"` // Полный URL для доступа к файлу.
}

// Scan реализует интерфейс pgx.Scanner для чтения из базы данных.
func (img *ImageField) Scan(src any) error {
	if src == nil {
		return nil
	}
	var path string
	switch v := src.(type) {
	case string:
		path = v
	case []byte:
		path = string(v)
	default:
		return fmt.Errorf("cannot scan %T into ImageField", src)
	}
	img.Path = path
	// В реальном коде здесь можно прочитать размеры и размер файла.
	return nil
}

// Value реализует интерфейс driver.Valuer для записи в базу данных.
func (img ImageField) Value() (driver.Value, error) {
	return img.Path, nil
}

// ParseUpload обрабатывает загруженный файл из multipart-формы.
func ParseUpload(fileHeader *multipart.FileHeader, uploadDir string) (*ImageField, error) {
	// Базовая валидация типа.
	contentType := fileHeader.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		return nil, fmt.Errorf("файл не является изображением: %s", contentType)
	}

	// Генерация уникального имени файла.
	ext := filepath.Ext(fileHeader.Filename)
	filename := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), randomString(8), ext)
	savePath := filepath.Join(uploadDir, filename)

	// Сохранение файла.
	src, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	dst, err := os.Create(savePath)
	if err != nil {
		return nil, err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return nil, err
	}

	return &ImageField{
		Path: filename,
		Size: fileHeader.Size,
		URL:  "/media/" + filename,
	}, nil
}

// randomString генерирует криптостойкую случайную строку из hex-символов.
func randomString(n int) string {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		// В случае ошибки возвращаем fallback (но вероятность крайне мала)
		panic("не удалось сгенерировать случайную строку: " + err.Error())
	}
	return hex.EncodeToString(b)[:n]
}
