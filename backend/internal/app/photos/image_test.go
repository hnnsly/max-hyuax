package photos

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func testJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// Красная полоса сверху (20 строк): тонкую линию размоет сжатие цвета в JPEG.
	for y := range min(20, h) {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// withOrientation вставляет после SOI сегмент APP1 с EXIF, где задана ориентация и «геометка»-маркер.
func withOrientation(jpg []byte, orientation uint16) []byte {
	var tiff bytes.Buffer
	tiff.WriteString("MM\x00\x2a")                        // big-endian TIFF
	binary.Write(&tiff, binary.BigEndian, uint32(8))      // смещение IFD0
	binary.Write(&tiff, binary.BigEndian, uint16(1))      // одна запись
	binary.Write(&tiff, binary.BigEndian, uint16(0x0112)) // Orientation
	binary.Write(&tiff, binary.BigEndian, uint16(3))      // SHORT
	binary.Write(&tiff, binary.BigEndian, uint32(1))      // count
	binary.Write(&tiff, binary.BigEndian, orientation)    // значение
	binary.Write(&tiff, binary.BigEndian, uint16(0))      // выравнивание
	binary.Write(&tiff, binary.BigEndian, uint32(0))      // следующего IFD нет
	tiff.WriteString("GPS-SECRET-55.61,37.74")            // эти байты не должны попасть в результат
	payload := append([]byte("Exif\x00\x00"), tiff.Bytes()...)
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(payload)+2))
	out := append([]byte{}, jpg[:2]...)
	out = append(out, seg...)
	out = append(out, payload...)
	return append(out, jpg[2:]...)
}

func TestNormalizeKeepsSmallJPEG(t *testing.T) {
	out, w, h, err := normalize(testJPEG(t, 400, 300))
	if err != nil || w != 400 || h != 300 || !bytes.HasPrefix(out, []byte{0xFF, 0xD8}) {
		t.Fatalf("w=%d h=%d err=%v", w, h, err)
	}
}

func TestNormalizeDownscalesLargeImage(t *testing.T) {
	_, w, h, err := normalize(testJPEG(t, 3200, 2400))
	if err != nil || w != MaxSide || h != 1200 {
		t.Fatalf("w=%d h=%d err=%v, want %dx1200", w, h, err, MaxSide)
	}
}

func TestNormalizeAppliesOrientationAndStripsEXIF(t *testing.T) {
	src := withOrientation(testJPEG(t, 400, 300), 6) // 6: повернуть на 90° по часовой
	if o := exifOrientation(src); o != 6 {
		t.Fatalf("orientation = %d, want 6", o)
	}
	out, w, h, err := normalize(src)
	if err != nil || w != 300 || h != 400 {
		t.Fatalf("w=%d h=%d err=%v, want 300x400", w, h, err)
	}
	if bytes.Contains(out, []byte("Exif")) || bytes.Contains(out, []byte("GPS-SECRET")) {
		t.Fatal("EXIF must be stripped")
	}
	// Красная верхняя строка исходника после поворота по часовой — правый столбец.
	dec, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	red := func(x, y int) bool { r, g, _, _ := dec.At(x, y).RGBA(); return r > 0x8000 && g < 0x4000 }
	if !red(290, 200) || red(10, 200) {
		t.Fatal("image is rotated the wrong way")
	}
}

func TestNormalizePNGWithTransparency(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 50, 40)) // полностью прозрачный
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	out, w, h, err := normalize(buf.Bytes())
	if err != nil || w != 50 || h != 40 {
		t.Fatalf("w=%d h=%d err=%v", w, h, err)
	}
	dec, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	// Прозрачность заливается белым, а не чёрным.
	if r, g, b, _ := dec.At(10, 10).RGBA(); r < 0xf000 || g < 0xf000 || b < 0xf000 {
		t.Fatalf("pixel = %d %d %d, want white", r, g, b)
	}
}

func TestNormalizeRejects(t *testing.T) {
	gif := []byte("GIF89a\x01\x00\x01\x00")
	huge := append(testJPEG(t, 10, 10), make([]byte, MaxBytes)...)
	for name, tc := range map[string]struct {
		raw  []byte
		want error
	}{
		"не картинка":      {[]byte("%PDF-1.7 not an image"), ErrNotImage},
		"GIF":              {gif, ErrNotImage},
		"пусто":            {nil, ErrNotImage},
		"битый JPEG":       {[]byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0}, ErrNotImage},
		"больше 5 МБ":      {huge, ErrTooLarge},
		"огромные размеры": {bombHeader(), ErrTooLarge},
	} {
		if _, _, _, err := normalize(tc.raw); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
}

// 16-битный PNG: 40 Мп проходят предел по пикселям, но при декодировании займут 320 МБ.
func TestNormalizeRejectsDeepColorBomb(t *testing.T) {
	bomb := pngHeader(image.NewRGBA64(image.Rect(0, 0, 1, 1)), 6400, 6250)
	if _, _, _, err := normalize(bomb); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
}

// Поворот делается после уменьшения, а размеры в ответе — уже повёрнутые.
func TestNormalizeRotatesLargeImageAfterDownscale(t *testing.T) {
	_, w, h, err := normalize(withOrientation(testJPEG(t, 3200, 2400), 6))
	if err != nil || w != 1200 || h != MaxSide {
		t.Fatalf("w=%d h=%d err=%v, want 1200x%d", w, h, err, MaxSide)
	}
}

// Очень узкая картинка не должна превратиться в JPEG нулевой высоты.
func TestNormalizeKeepsThinImageVisible(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, 8000, 1))); err != nil {
		t.Fatal(err)
	}
	_, w, h, err := normalize(buf.Bytes())
	if err != nil || w != MaxSide || h != 1 {
		t.Fatalf("w=%d h=%d err=%v, want %dx1", w, h, err, MaxSide)
	}
}

// bombHeader — PNG с заголовком 20000×20000: декодировать такое нельзя, память кончится.
func bombHeader() []byte {
	return pngHeader(image.NewGray(image.Rect(0, 0, 1, 1)), 20000, 20000)
}

// pngHeader кодирует картинку 1×1 и подменяет размеры в заголовке: данных нет, есть только обещание.
func pngHeader(img image.Image, w, h uint32) []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	b := buf.Bytes()
	// IHDR начинается с 16-го байта: ширина и высота по 4 байта.
	binary.BigEndian.PutUint32(b[16:], w)
	binary.BigEndian.PutUint32(b[20:], h)
	// CRC чанка считается по типу и данным (байты 12..28), иначе файл отвергнут как битый.
	binary.BigEndian.PutUint32(b[29:], crc32.ChecksumIEEE(b[12:29]))
	return b
}
