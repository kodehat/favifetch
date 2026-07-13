package favifetch

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"

	// Import image formats for decoding
	_ "image/gif"

	"github.com/chai2010/webp"
	"golang.org/x/image/draw"
)

// processImage resizes and/or converts the format of an image buffer.
// Returns the processed data, format, width, height.
func processImage(data []byte, opts *Options) ([]byte, string, int, int, error) {
	format := detectFormat(data, "", "")
	size := opts.Size
	targetFormat := opts.Format

	// SVG pass-through unless raster conversion is explicitly requested
	if format == "svg" {
		if targetFormat == "" || targetFormat == "svg" {
			w, h := extractSVGDimensions(data)
			if size > 0 {
				w, h = size, size
			}
			return data, "svg", w, h, nil
		}
		// SVG to raster conversion requested — not supported without external renderer
		// Return original SVG
		w, h := extractSVGDimensions(data)
		return data, "svg", w, h, nil
	}

	// Decode the image
	img, _, err := decodeImage(data)
	if err != nil {
		// Can't decode, return original
		w, h := decodeDimensions(data, format)
		return data, format, w, h, nil
	}

	srcBounds := img.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	// If no processing needed, return as-is
	if size == 0 && (targetFormat == "" || targetFormat == format) {
		return data, format, srcW, srcH, nil
	}

	// Resize if needed
	var processed image.Image = img
	if size > 0 {
		processed = resizeImage(img, size)
	}

	// Encode to target format
	if targetFormat == "" {
		targetFormat = format
	}

	bounds := processed.Bounds()
	outW := bounds.Dx()
	outH := bounds.Dy()

	encoded, err := encodeImage(processed, targetFormat)
	if err != nil {
		return data, format, srcW, srcH, nil
	}

	return encoded, targetFormat, outW, outH, nil
}

// decodeImage decodes image data into an image.Image. Supports PNG, JPEG, GIF, WebP.
func decodeImage(data []byte) (image.Image, string, error) {
	// Try standard library decoders first
	img, format, err := image.Decode(bytes.NewReader(data))
	if err == nil {
		return img, format, nil
	}

	// Try WebP decoder
	if img, err := webp.Decode(bytes.NewReader(data)); err == nil {
		return img, "webp", nil
	}

	return nil, "", err
}

// decodeDimensions returns the dimensions of an image without fully decoding the pixel data.
func decodeDimensions(data []byte, format string) (int, int) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err == nil {
		return cfg.Width, cfg.Height
	}

	// Try WebP decode config
	img, err := webp.Decode(bytes.NewReader(data))
	if err == nil {
		b := img.Bounds()
		return b.Dx(), b.Dy()
	}

	return 0, 0
}

// resizeImage resizes an image to size×size, maintaining aspect ratio with transparent padding.
func resizeImage(img image.Image, size int) image.Image {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	if srcW == size && srcH == size {
		return img
	}

	// Calculate new dimensions maintaining aspect ratio
	var newW, newH int
	if srcW > srcH {
		newW = size
		newH = int(float64(srcH) * float64(size) / float64(srcW))
	} else {
		newH = size
		newW = int(float64(srcW) * float64(size) / float64(srcH))
	}
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}

	// Create output image with transparent background
	dst := image.NewRGBA(image.Rect(0, 0, size, size))

	// Scale the image
	scaled := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), img, img.Bounds(), draw.Over, nil)

	// Center in the output image
	offsetX := (size - newW) / 2
	offsetY := (size - newH) / 2
	draw.Draw(dst, image.Rect(offsetX, offsetY, offsetX+newW, offsetY+newH), scaled, image.Point{}, draw.Over)

	return dst
}

// encodeImage encodes an image.Image to the specified format.
func encodeImage(img image.Image, format string) ([]byte, error) {
	var buf bytes.Buffer
	switch normalizeFormat(format) {
	case "png":
		err := png.Encode(&buf, img)
		return buf.Bytes(), err
	case "jpg", "jpeg":
		err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85})
		return buf.Bytes(), err
	case "webp":
		err := webp.Encode(&buf, img, &webp.Options{Quality: 90})
		return buf.Bytes(), err
	default:
		// Default to PNG
		err := png.Encode(&buf, img)
		return buf.Bytes(), err
	}
}

func normalizeFormat(format string) string {
	switch format {
	case "jpeg":
		return "jpg"
	default:
		return format
	}
}
