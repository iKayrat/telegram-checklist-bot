package pdf

import (
	"bytes"
	"fmt"
	"math"
	"sync"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
)

// Stamp text — the group's own fund/checklist name, not a real organization.
const (
	stampTopText    = "ЖАН АЗЫК"
	stampBottomText = "★ ЧЕК-ЛИСТ БОТУ ★"
	stampCenterText = "ОТЧЁТ"
)

const (
	stampCanvasSize = 700.0 // extra margin around the circle so the tilt doesn't clip corners
	stampCenter     = stampCanvasSize / 2
	stampOuterR     = 285.0
	stampInnerR     = 220.0
	stampTiltDeg    = -9.0 // slight rotation for a "hand-stamped" look
)

var (
	stampOnce  sync.Once
	stampBytes []byte
	stampErr   error
)

// GenerateStamp renders (once, then caches in memory for the process
// lifetime) a circular ink-stamp PNG with a transparent background, meant to
// be placed at the bottom of a weekly PDF report for an "official" look.
func GenerateStamp() ([]byte, error) {
	stampOnce.Do(func() {
		stampBytes, stampErr = buildStamp()
	})
	return stampBytes, stampErr
}

func buildStamp() ([]byte, error) {
	face, err := stampFontFace(38)
	if err != nil {
		return nil, fmt.Errorf("load stamp font: %w", err)
	}
	// The top word is the group's name — make it the most prominent part of
	// the ring, sized and placed close to the outer edge. The bottom arc has
	// more characters (plus stars either side), so it stays smaller to avoid
	// wrapping past the sides of the circle.
	topArcFace, err := stampFontFace(36)
	if err != nil {
		return nil, fmt.Errorf("load stamp top arc font: %w", err)
	}
	bottomArcFace, err := stampFontFace(24)
	if err != nil {
		return nil, fmt.Errorf("load stamp bottom arc font: %w", err)
	}

	dc := gg.NewContext(stampCanvasSize, stampCanvasSize)
	// Muted maroon with alpha ~200/255 — reads as ink, not a flat fill.
	dc.SetRGBA255(139, 0, 0, 200)

	dc.RotateAbout(stampTiltDeg*math.Pi/180, stampCenter, stampCenter)

	dc.SetLineWidth(4)
	dc.DrawCircle(stampCenter, stampCenter, stampOuterR)
	dc.Stroke()
	dc.SetLineWidth(2)
	dc.DrawCircle(stampCenter, stampCenter, stampInnerR)
	dc.Stroke()

	edgeRadius := stampOuterR - 28 // hug the outer ring rather than sit mid-band, with clearance so glyphs don't cross it

	dc.SetFontFace(topArcFace)
	drawTopArcText(dc, stampTopText, stampCenter, stampCenter, edgeRadius)

	dc.SetFontFace(bottomArcFace)
	drawBottomArcText(dc, stampBottomText, stampCenter, stampCenter, edgeRadius)

	dc.SetFontFace(face)
	dc.DrawStringAnchored(stampCenterText, stampCenter, stampCenter, 0.5, 0.5)

	var buf bytes.Buffer
	if err := dc.EncodePNG(&buf); err != nil {
		return nil, fmt.Errorf("encode stamp png: %w", err)
	}
	return buf.Bytes(), nil
}

// stampFontFace builds a gg font face at the given point size from the
// already-embedded DejaVu Sans Bold bytes (see fonts.go) — reusing the same
// font the report table uses, just loaded via freetype instead of maroto's
// font registry (gg.LoadFontFace only reads from a file path, not bytes).
func stampFontFace(points float64) (font.Face, error) {
	f, err := truetype.Parse(fontBold)
	if err != nil {
		return nil, fmt.Errorf("parse embedded font: %w", err)
	}
	return truetype.NewFace(f, &truetype.Options{Size: points}), nil
}

// drawTopArcText draws s centered on the top of the circle (angle -90°),
// each glyph upright and fanning out symmetrically, reading left to right.
func drawTopArcText(dc *gg.Context, s string, cx, cy, radius float64) {
	drawArcText(dc, s, cx, cy, radius, -math.Pi/2, 1)
}

// drawBottomArcText draws s centered on the bottom of the circle (angle
// +90°), each glyph upright, reading left to right along the underside.
func drawBottomArcText(dc *gg.Context, s string, cx, cy, radius float64) {
	drawArcText(dc, s, cx, cy, radius, math.Pi/2, -1)
}

// drawArcText places each rune of s along the circle of the given radius,
// centered on centerAngle (radians; standard image convention: 0=right,
// +90°=down, -90°=up, clockwise positive). sweep is +1 when reading
// left-to-right means increasing angle (top arcs) or -1 when it means
// decreasing angle (bottom arcs, since the underside of a circle runs the
// other way).
func drawArcText(dc *gg.Context, s string, cx, cy, radius, centerAngle, sweep float64) {
	runes := []rune(s)
	widths := make([]float64, len(runes))
	var totalAngle float64
	for i, r := range runes {
		w, _ := dc.MeasureString(string(r))
		widths[i] = w
		totalAngle += w / radius
	}

	cursor := centerAngle - sweep*totalAngle/2
	for i, r := range runes {
		halfAngle := (widths[i] / radius) / 2
		theta := cursor + sweep*halfAngle

		x := cx + radius*math.Cos(theta)
		y := cy + radius*math.Sin(theta)

		dc.Push()
		dc.Translate(x, y)
		if sweep > 0 {
			dc.Rotate(theta + math.Pi/2)
		} else {
			dc.Rotate(theta - math.Pi/2)
		}
		dc.DrawStringAnchored(string(r), 0, 0, 0.5, 0.5)
		dc.Pop()

		cursor += sweep * (halfAngle * 2)
	}
}
