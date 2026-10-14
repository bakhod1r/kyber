package domain

import (
	"bytes"
	"errors"
)

// KindImage shows an uploaded picture (S96); the element's Text is the image's id on the board.
const KindImage ElementKind = "image"

// MaxImage bounds an uploaded image.
const MaxImage = 10 << 20

var (
	ErrNotImage     = errors.New("only PNG, JPEG, GIF or WebP images up to 10 MB")
	ErrImageMissing = errors.New("image not found")
)

// ValidImageID: image ids are generated ids, never paths.
func ValidImageID(id string) bool { return elementID.MatchString(id) }

// ImageKey is where a board's image is stored (blob keys are ids, never paths).
func ImageKey(board WhiteboardID, image string) string { return "wb_" + string(board) + "_" + image }

// ImageType tells a picture's media type by its first bytes. SVG is refused: it can carry scripts.
func ImageType(head []byte) (string, error) {
	switch {
	case bytes.HasPrefix(head, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png", nil
	case bytes.HasPrefix(head, []byte("\xff\xd8\xff")):
		return "image/jpeg", nil
	case bytes.HasPrefix(head, []byte("GIF87a")), bytes.HasPrefix(head, []byte("GIF89a")):
		return "image/gif", nil
	case len(head) >= 12 && bytes.Equal(head[:4], []byte("RIFF")) && bytes.Equal(head[8:12], []byte("WEBP")):
		return "image/webp", nil
	}
	return "", ErrNotImage
}
