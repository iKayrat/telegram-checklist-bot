package pdf

import _ "embed"

// DejaVu Sans is embedded because maroto's built-in PDF fonts (Helvetica/
// Arial and friends) only cover Latin-1 — Cyrillic text renders as dots
// without a font that actually has those glyphs. Shared between the report
// table (via maroto's WithCustomFonts) and the stamp (via gg.SetFontFace on
// the same bytes, parsed through golang.org/x/image/font + freetype).
//
//go:embed fonts/DejaVuSans.ttf
var fontRegular []byte

//go:embed fonts/DejaVuSans-Bold.ttf
var fontBold []byte

const reportFontFamily = "DejaVuSans"
