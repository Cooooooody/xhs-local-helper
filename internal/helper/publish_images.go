package helper

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	xdraw "golang.org/x/image/draw"
)

const (
	maxPublishImageDimension = 2048
	maxPublishImageBytes     = 2 * 1024 * 1024
)

var publishJPEGQualities = []int{88, 80, 72, 64, 56}

func (s *Service) preparePublishImages(inputs []string) (string, []string, error) {
	dir, err := os.MkdirTemp(s.cfg.TmpDir, "publish-")
	if err != nil {
		return "", nil, fmt.Errorf("create publish temp dir: %w", err)
	}

	prepared := make([]string, 0, len(inputs))
	for i, input := range inputs {
		path, err := s.prepareSinglePublishImage(dir, i+1, input)
		if err != nil {
			s.writeHelperLog("publish image preparation failed index=%d source=%q error=%v", i+1, truncateForLog(input, 300), err)
			return dir, nil, err
		}
		prepared = append(prepared, path)
	}

	s.writeHelperLog("publish prepared images count=%d dir=%q", len(prepared), dir)
	return dir, prepared, nil
}

func (s *Service) prepareSinglePublishImage(dir string, index int, input string) (string, error) {
	sourcePath, err := s.materializePublishImageSource(dir, index, input)
	if err != nil {
		return "", err
	}
	preparedPath, err := normalizeImageFile(sourcePath, filepath.Join(dir, fmt.Sprintf("prepared-%02d", index)))
	if err != nil {
		return "", fmt.Errorf("prepare image %d normalize from %q: %w", index, input, err)
	}
	return preparedPath, nil
}

func (s *Service) materializePublishImageSource(dir string, index int, input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", fmt.Errorf("prepare image %d invalid source %q", index, input)
	}
	if isRemoteImageURL(trimmed) {
		target := filepath.Join(dir, fmt.Sprintf("source-%02d%s", index, extFromURL(trimmed)))
		if err := s.downloadPublishImage(trimmed, target); err != nil {
			return "", fmt.Errorf("prepare image %d download from %q: %w", index, trimmed, err)
		}
		return target, nil
	}
	if !filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("prepare image %d invalid source %q", index, input)
	}
	if !fileExists(trimmed) {
		return "", fmt.Errorf("prepare image %d local file not found %q", index, trimmed)
	}
	return trimmed, nil
}

func (s *Service) downloadPublishImage(sourceURL, target string) error {
	resp, err := s.httpClient.Get(sourceURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http status %d", resp.StatusCode)
	}

	file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}

func normalizeImageFile(sourcePath, targetPrefix string) (string, error) {
	file, err := os.Open(sourcePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	src, _, err := image.Decode(file)
	if err != nil {
		return "", err
	}
	normalized := resizeForPublish(src)

	hasAlpha := imageHasAlpha(normalized)
	if hasAlpha {
		return encodePNGWithinLimit(normalized, targetPrefix)
	}
	return encodeJPEGWithinLimit(normalized, targetPrefix)
}

func resizeForPublish(src image.Image) image.Image {
	return resizeToMaxDimension(src, maxPublishImageDimension)
}

func scaledDimensions(width, height, maxDimension int) (int, int) {
	if width >= height {
		return maxDimension, max(1, height*maxDimension/width)
	}
	return max(1, width*maxDimension/height), maxDimension
}

func imageHasAlpha(src image.Image) bool {
	switch img := src.(type) {
	case *image.NRGBA:
		return hasAlphaNRGBA(img)
	case *image.NRGBA64:
		return hasAlphaBounds(img, func(x, y int) uint32 { _, _, _, a := img.At(x, y).RGBA(); return a })
	case *image.RGBA:
		return hasAlphaBounds(img, func(x, y int) uint32 { _, _, _, a := img.At(x, y).RGBA(); return a })
	case *image.RGBA64:
		return hasAlphaBounds(img, func(x, y int) uint32 { _, _, _, a := img.At(x, y).RGBA(); return a })
	case *image.Alpha:
		return true
	case *image.Alpha16:
		return true
	case *image.Paletted:
		for _, c := range img.Palette {
			if _, _, _, a := c.RGBA(); a != 0xffff {
				return true
			}
		}
		return false
	default:
		return hasAlphaBounds(src, func(x, y int) uint32 {
			_, _, _, a := src.At(x, y).RGBA()
			return a
		})
	}
}

func hasAlphaNRGBA(img *image.NRGBA) bool {
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] != 0xff {
			return true
		}
	}
	return false
}

func hasAlphaBounds(src image.Image, alphaAt func(x, y int) uint32) bool {
	bounds := src.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if alphaAt(x, y) != 0xffff {
				return true
			}
		}
	}
	return false
}

func isRemoteImageURL(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func extFromURL(value string) string {
	parsed, err := url.Parse(value)
	if err != nil {
		return ".img"
	}
	ext := strings.ToLower(filepath.Ext(parsed.Path))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		return ext
	default:
		return ".img"
	}
}

func encodeJPEGWithinLimit(src image.Image, targetPrefix string) (string, error) {
	current := src
	lastSize := 0
	for round := 0; round < 3; round++ {
		for _, quality := range publishJPEGQualities {
			data, err := encodeJPEGBytes(current, quality)
			if err != nil {
				return "", err
			}
			lastSize = len(data)
			if lastSize <= maxPublishImageBytes {
				return writePreparedBytes(targetPrefix+".jpg", data)
			}
		}

		next := shrinkForSizeLimit(current)
		if next.Bounds().Dx() == current.Bounds().Dx() && next.Bounds().Dy() == current.Bounds().Dy() {
			break
		}
		current = next
	}
	return "", fmt.Errorf("encoded image exceeds %d bytes after compression, got %d bytes", maxPublishImageBytes, lastSize)
}

func encodePNGWithinLimit(src image.Image, targetPrefix string) (string, error) {
	current := src
	lastSize := 0
	for round := 0; round < 3; round++ {
		data, err := encodePNGBytes(current)
		if err != nil {
			return "", err
		}
		lastSize = len(data)
		if lastSize <= maxPublishImageBytes {
			return writePreparedBytes(targetPrefix+".png", data)
		}

		next := shrinkForSizeLimit(current)
		if next.Bounds().Dx() == current.Bounds().Dx() && next.Bounds().Dy() == current.Bounds().Dy() {
			break
		}
		current = next
	}
	return "", fmt.Errorf("encoded image exceeds %d bytes after compression, got %d bytes", maxPublishImageBytes, lastSize)
}

func encodeJPEGBytes(src image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodePNGBytes(src image.Image) ([]byte, error) {
	var buf bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestCompression}
	if err := encoder.Encode(&buf, src); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writePreparedBytes(path string, data []byte) (string, error) {
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func resizeToMaxDimension(src image.Image, maxDimension int) image.Image {
	bounds := src.Bounds()
	if bounds.Dx() <= maxDimension && bounds.Dy() <= maxDimension {
		return src
	}
	newWidth, newHeight := scaledDimensions(bounds.Dx(), bounds.Dy(), maxDimension)
	return resizeToDimensions(src, newWidth, newHeight)
}

func shrinkForSizeLimit(src image.Image) image.Image {
	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	newWidth := max(1, width*85/100)
	newHeight := max(1, height*85/100)
	if newWidth == width && width > 1 {
		newWidth--
	}
	if newHeight == height && height > 1 {
		newHeight--
	}
	return resizeToDimensions(src, newWidth, newHeight)
}

func resizeToDimensions(src image.Image, width, height int) image.Image {
	bounds := src.Bounds()
	if bounds.Dx() == width && bounds.Dy() == height {
		return src
	}
	dst := image.NewNRGBA(image.Rect(0, 0, width, height))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, xdraw.Over, nil)
	return dst
}
