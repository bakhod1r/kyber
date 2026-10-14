package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/whiteboard/domain"
)

// S96: an image element shows an uploaded picture; its text is the image's id on the board.
func TestImageElements(t *testing.T) {
	b, _ := domain.New("b-1", "w-1", "c-1", "Arch", "u-a", t0)
	img := domain.Element{ID: "e-img", Kind: domain.KindImage, X: 10, Y: 10, W: 320, H: 200, Text: "0a1b2c3d-0000-4000-8000-000000000001"}
	if r := b.Apply("u-a", []domain.Op{{Kind: domain.OpAdd, Element: img}}, t0); !r[0].Applied {
		t.Fatalf("image = %+v", r)
	}
	for _, bad := range []domain.Element{
		{ID: "e-2", Kind: domain.KindImage, W: 10, H: 10, Text: "../../etc/passwd"},
		{ID: "e-3", Kind: domain.KindImage, W: 10, H: 10},
		{ID: "e-4", Kind: domain.KindImage, Text: "abc"},
	} {
		if r := b.Apply("u-a", []domain.Op{{Kind: domain.OpAdd, Element: bad}}, t0); r[0].Applied || !errors.Is(r[0].Error, domain.ErrInvalidElement) {
			t.Errorf("%+v = %+v", bad, r)
		}
	}
	if key := domain.ImageKey("b-1", "img-1"); key != "wb_b-1_img-1" {
		t.Fatalf("key = %q", key)
	}
}

func TestImageTypes(t *testing.T) {
	png := "\x89PNG\r\n\x1a\n" + strings.Repeat("\x00", 20)
	for head, want := range map[string]string{png: "image/png", "\xff\xd8\xff\xe0" + strings.Repeat("\x00", 20): "image/jpeg", "GIF89a" + strings.Repeat("\x00", 20): "image/gif"} {
		if got, err := domain.ImageType([]byte(head)); err != nil || got != want {
			t.Errorf("%q = %q %v", head[:4], got, err)
		}
	}
	for _, bad := range []string{"<svg xmlns='http://www.w3.org/2000/svg'/>", "%PDF-1.7", "hello"} {
		if _, err := domain.ImageType([]byte(bad)); !errors.Is(err, domain.ErrNotImage) {
			t.Errorf("%q accepted: %v", bad, err)
		}
	}
}
