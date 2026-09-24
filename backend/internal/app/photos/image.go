package photos

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	stddraw "image/draw"
	"image/jpeg"
	_ "image/png" // регистрирует декодер PNG для image.Decode

	"golang.org/x/image/draw"
)

const (
	// MaxBytes — предел размера одного загружаемого файла.
	MaxBytes = 5 << 20
	// MaxSide — длинная сторона после уменьшения: для заявки деталей хватает.
	MaxSide = 1600
	// maxPixels и maxDecodedBytes — пределы до декодирования: защита от «бомб» с огромными размерами.
	// Байты считаются с глубиной цвета: 16-битный PNG на 40 Мп занял бы 320 МБ.
	maxPixels       = 40_000_000
	maxDecodedBytes = 128 << 20
	quality         = 82
)

var (
	ErrNotImage = errors.New("photos: file is not a JPEG or PNG image")
	ErrTooLarge = errors.New("photos: image is too large")
)

// normalize проверяет файл и перекодирует его в JPEG: поворот по EXIF применяется,
// сам EXIF (с геометкой и данными камеры) в результат не попадает.
func normalize(raw []byte) (out []byte, width, height int, err error) {
	if len(raw) > MaxBytes {
		return nil, 0, 0, ErrTooLarge
	}
	isJPEG := bytes.HasPrefix(raw, []byte{0xFF, 0xD8, 0xFF})
	isPNG := bytes.HasPrefix(raw, []byte("\x89PNG\r\n\x1a\n"))
	if !isJPEG && !isPNG {
		return nil, 0, 0, ErrNotImage
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, 0, 0, ErrNotImage
	}
	pixels := cfg.Width * cfg.Height
	if cfg.Width <= 0 || cfg.Height <= 0 || pixels > maxPixels || pixels*bytesPerPixel(cfg.ColorModel) > maxDecodedBytes {
		return nil, 0, 0, ErrTooLarge
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, 0, 0, ErrNotImage
	}

	// Холст с белым фоном: прозрачные места PNG не станут чёрными.
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if long := max(w, h); long > MaxSide {
		// Узкая полоска не должна стать картинкой нулевой высоты.
		w, h = max(1, w*MaxSide/long), max(1, h*MaxSide/long)
	}
	var dst image.Image = scaled(src, w, h)
	// Поворот по EXIF — после уменьшения: крутить 12 Мп попиксельно долго и дорого по памяти.
	if isJPEG {
		dst = orient(dst, exifOrientation(raw))
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: quality}); err != nil {
		return nil, 0, 0, err
	}
	size := dst.Bounds()
	return buf.Bytes(), size.Dx(), size.Dy(), nil
}

// scaled рисует src в холст w×h на белом фоне.
func scaled(src image.Image, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	stddraw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, stddraw.Src)
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}

// bytesPerPixel — сколько памяти займёт пиксель после декодирования (с запасом для JPEG 4:4:4).
func bytesPerPixel(m color.Model) int {
	switch m {
	case color.RGBA64Model, color.NRGBA64Model:
		return 8
	case color.Gray16Model:
		return 2
	case color.GrayModel, color.AlphaModel:
		return 1
	case color.YCbCrModel:
		return 3
	}
	if _, ok := m.(color.Palette); ok {
		return 1
	}
	return 4
}

// exifOrientation читает тег Orientation (0x0112) из APP1 JPEG; 1 — если его нет или он битый.
func exifOrientation(jpg []byte) int {
	for i := 2; i+4 <= len(jpg); {
		if jpg[i] != 0xFF {
			return 1
		}
		marker := jpg[i+1]
		if marker == 0xDA || marker == 0xD9 { // дальше данные изображения: EXIF не встретился
			return 1
		}
		size := int(binary.BigEndian.Uint16(jpg[i+2:]))
		end := i + 2 + size
		if size < 2 || end > len(jpg) {
			return 1
		}
		if seg := jpg[i+4 : end]; marker == 0xE1 && bytes.HasPrefix(seg, []byte("Exif\x00\x00")) {
			return tiffOrientation(seg[6:])
		}
		i = end
	}
	return 1
}

func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	ifd := int(order.Uint32(t[4:]))
	if ifd < 8 || ifd+2 > len(t) {
		return 1
	}
	n := int(order.Uint16(t[ifd:]))
	for k := range n {
		e := ifd + 2 + k*12
		if e+12 > len(t) {
			return 1
		}
		if order.Uint16(t[e:]) == 0x0112 {
			if v := int(order.Uint16(t[e+8:])); v >= 1 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}

// orient приводит изображение к нормальному положению по значению EXIF Orientation (1–8).
func orient(src image.Image, o int) image.Image {
	if o <= 1 || o > 8 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if o >= 5 { // 5–8: стороны меняются местами
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := range h {
		for x := range w {
			var dx, dy int
			switch o {
			case 2:
				dx, dy = w-1-x, y
			case 3:
				dx, dy = w-1-x, h-1-y
			case 4:
				dx, dy = x, h-1-y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = h-1-y, x
			case 7:
				dx, dy = h-1-y, w-1-x
			case 8:
				dx, dy = y, w-1-x
			}
			dst.Set(dx, dy, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}
