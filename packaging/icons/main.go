// Command icons draws kvit-coder's icon, kvit-coder.svg, at the sizes Windows
// asks for and writes them as kvit-coder.ico. The installer shows that file
// as its own icon (SetupIconFile in scripts/windows-installer.iss), and
// make-syso.sh puts it in the Windows resources of both programs.
//
// The .ico and the resources are committed, so no build needs this program;
// rerun both after editing the SVG:
//
//	go -C packaging/icons run .
//	packaging/icons/make-syso.sh
//
// The SVG is drawn by unison's raster canvas, the one Kvit Works draws it with
// on the project's tile in its rail, so the .exe shows the same picture as the
// rail. unison needs cgo, which is why this is a module of its own: it stays
// out of kvit-coder's go.mod and out of its builds, which are CGO_ENABLED=0.
package main

import (
	"bytes"
	"fmt"
	"image"
	"os"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/tc-hib/winres"
)

const (
	source = "kvit-coder.svg"
	ico    = "kvit-coder.ico"
	// ppi draws one pixel of the picture to one point of the drawing.
	ppi = 72
)

// sizes are the sizes the .ico holds: what Explorer, the taskbar and the
// title bar ask for at 100% to 200% scale, each drawn from the SVG rather
// than scaled down from a larger picture, so the small ones stay sharp.
var sizes = []int{16, 24, 32, 48, 64, 128, 256}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "icons:", err)
		os.Exit(1)
	}
}

func run() error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	svg, err := unison.NewSVGFromContentString(string(data))
	if err != nil {
		return fmt.Errorf("%s: %w", source, err)
	}
	var images []image.Image
	for _, size := range sizes {
		img, err := draw(svg, size)
		if err != nil {
			return err
		}
		images = append(images, img)
	}
	icon, err := winres.NewIconFromImages(images)
	if err != nil {
		return err
	}
	var out bytes.Buffer
	if err := icon.SaveICO(&out); err != nil {
		return err
	}
	if err := os.WriteFile(ico, out.Bytes(), 0o644); err != nil {
		return err
	}
	fmt.Println("wrote packaging/icons/" + ico)
	return nil
}

// draw is the SVG drawn size pixels square.
func draw(svg *unison.SVG, size int) (image.Image, error) {
	img, err := unison.NewImageFromDrawing(size, size, ppi, func(c *unison.Canvas) {
		svg.DrawInRect(c, geom.NewRect(0, 0, float32(size), float32(size)), nil, nil)
	})
	if err != nil {
		return nil, err
	}
	return img.ToNRGBA()
}
