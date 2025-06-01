package cmd

import (
	"fmt"
	"image"
	"os"
	"strings"

	"golang.org/x/term"
)

// RGBToANSI256 tries to find the best ANSI 256 color for a given RGB.
func RGBToANSI256(r, g, b uint32) int {
	r8 := clamp8(r >> 8)
	g8 := clamp8(g >> 8)
	b8 := clamp8(b >> 8)

	if r8 == 0 && g8 == 0 && b8 == 0 {
		return 16 // ANSI Black.
	}

	// Nudge very dark colors up a bit so they don't just become black.
	if r8 < 15 && g8 < 15 && b8 < 15 {
		r8 = clamp8(uint32(r8) + 10)
		g8 = clamp8(uint32(g8) + 10)
		b8 = clamp8(uint32(b8) + 10)
	}

	if isGrayscale(r8, g8, b8) {
		return mapToGrayscale(r8, g8, b8)
	}

	rLevel := quantizeToSix(r8)
	gLevel := quantizeToSix(g8)
	bLevel := quantizeToSix(b8)

	cubeColor := 16 + (36 * rLevel) + (6 * gLevel) + bLevel

	// For near-grays, the dedicated grayscale ramp can sometimes be a better fit.
	if isNearGrayscale(r8, g8, b8) {
		grayColor := mapToGrayscale(r8, g8, b8)
		if colorDistance(r8, g8, b8, grayColor) < colorDistance(r8, g8, b8, cubeColor) {
			return grayColor
		}
	}

	return cubeColor
}

func clamp8(val uint32) uint8 {
	if val > 255 {
		return 255
	}
	return uint8(val)
}

// quantizeToSix maps an 8-bit color to one of 6 ANSI cube levels.
func quantizeToSix(val uint8) int {
	if val < 48 { // Thresholds for 6 levels (0..5).
		return 0
	}
	if val < 95 {
		return 1
	}
	if val < 142 {
		return 2
	}
	if val < 189 {
		return 3
	}
	if val < 236 {
		return 4
	}
	return 5
}

func isGrayscale(r, g, b uint8) bool {
	max := maxUint8(r, g, b)
	min := minUint8(r, g, b)
	return max-min <= 10 // Tolerance for what's "gray enough".
}

func isNearGrayscale(r, g, b uint8) bool {
	max := maxUint8(r, g, b)
	min := minUint8(r, g, b)
	return max-min <= 30 // A bit more forgiving for "almost gray".
}

// mapToGrayscale finds the closest match in ANSI's 24-step grayscale ramp.
func mapToGrayscale(r, g, b uint8) int {
	gray := uint8(0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) // Standard luminance.

	if gray < 10 {
		return 232 // Darkest non-black gray in the ramp.
	}
	if gray > 245 {
		return 255 // Lightest gray.
	}
	return 232 + int(float64(gray-8)*(23.0/240.0))
}

// colorDistance estimates perceptual difference between colors.
func colorDistance(r1, g1, b1 uint8, colorIndex int) float64 {
	r2, g2, b2 := ansiToRGB(colorIndex)

	dr := float64(r1) - float64(r2)
	dg := float64(g1) - float64(g2)
	db := float64(b1) - float64(b2)

	// Weighted to roughly match human perception (greens pop more, etc.).
	return 2*dr*dr + 4*dg*dg + 3*db*db
}

// ansiToRGB converts an ANSI index back to an approximate RGB. Useful for colorDistance.
func ansiToRGB(index int) (uint8, uint8, uint8) {
	if index >= 232 && index <= 255 { // Grays.
		grayVal := float64(index-232)*(240.0/23.0) + 8.0
		if grayVal < 0 {
			grayVal = 0
		}
		if grayVal > 255 {
			grayVal = 255
		}
		return uint8(grayVal), uint8(grayVal), uint8(grayVal)
	}

	if index >= 16 && index <= 231 { // 6x6x6 color cube.
		index -= 16
		r := (index / 36) % 6
		g := (index / 6) % 6
		b := index % 6
		valMap := []uint8{0, 47, 95, 142, 189, 236} // RGB values for cube levels.
		return valMap[r], valMap[g], valMap[b]
	}

	// Standard 0-15 colors are tricky as they vary by terminal.
	if index == 0 || index == 16 {
		return 0, 0, 0
	} // Black
	if index == 7 || index == 231 {
		return 255, 255, 255
	} // White
	return 128, 128, 128 // Default to mid-gray for others.
}

func maxUint8(a, b, c uint8) uint8 {
	if a >= b && a >= c {
		return a
	}
	if b >= c {
		return b
	}
	return c
}

func minUint8(a, b, c uint8) uint8 {
	if a <= b && a <= c {
		return a
	}
	if b <= c {
		return b
	}
	return c
}

// RenderMode defines how to draw pixels.
type RenderMode int

const (
	BlockMode     RenderMode = iota // Full character blocks.
	HalfBlockMode                   // Unicode half blocks for better vertical detail.
	BrailleMode                     // Unicode Braille for hi-res mono.
)

// ImageRenderer draws images in the terminal.
type ImageRenderer struct {
	Mode        RenderMode
	MaxWidth    int     // Terminal width in characters.
	MaxHeight   int     // Terminal height in lines.
	UseDither   bool    // Apply dithering?
	AspectRatio float64 // Character height/width (usually around 0.5).
}

// NewImageRenderer prepares an ImageRenderer, checking terminal size.
func NewImageRenderer(mode RenderMode) *ImageRenderer {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		width, height = 100, 28 // Fallback if terminal size detection fails.
	}

	return &ImageRenderer{
		Mode:        mode,
		MaxWidth:    width - 2,  // Small margin.
		MaxHeight:   height - 3, // A bit more margin at the bottom for the prompt.
		UseDither:   true,
		AspectRatio: 0.5, // Common for terminal fonts.
	}
}

// RenderImage converts an image to an ANSI string.
func (r *ImageRenderer) RenderImage(img image.Image) string {
	bounds := img.Bounds()
	imgWidth := bounds.Dx()
	imgHeight := bounds.Dy()

	var outputWidth, outputHeight int

	if r.Mode == HalfBlockMode {
		// Half blocks double our effective vertical resolution.
		maxEffectiveHeight := r.MaxHeight * 2

		scaleX := float64(r.MaxWidth) / float64(imgWidth)
		scaleY := float64(maxEffectiveHeight) / float64(imgHeight)
		scale := scaleX
		if scaleY < scaleX {
			scale = scaleY
		}

		outputWidth = int(float64(imgWidth) * scale)
		outputHeight = int(float64(imgHeight) * scale)
		if outputHeight%2 != 0 {
			outputHeight--
		} // Must be even for half block pairs.
		if outputHeight <= 0 {
			outputHeight = 2
		}

	} else { // BlockMode or BrailleMode.
		scaleX := float64(r.MaxWidth) / float64(imgWidth)
		// AspectRatio is key for BlockMode to not look squished/stretched.
		scaleY := (float64(r.MaxHeight) * r.AspectRatio) / float64(imgHeight)
		scale := scaleX
		if scaleY < scaleX {
			scale = scaleY
		}

		outputWidth = int(float64(imgWidth) * scale)
		outputHeight = int(float64(imgHeight) * scale / r.AspectRatio)
		if outputHeight <= 0 {
			outputHeight = 1
		}
	}
	if outputWidth <= 0 {
		outputWidth = 1
	}

	switch r.Mode {
	case HalfBlockMode:
		return r.renderHalfBlocksImproved(img, outputWidth, outputHeight)
	case BrailleMode:
		return r.renderBraille(img, outputWidth, outputHeight)
	default: // BlockMode
		return r.renderFullBlocksImproved(img, outputWidth, outputHeight)
	}
}

// renderFullBlocksImproved uses a space with background color for each "pixel".
func (r *ImageRenderer) renderFullBlocksImproved(img image.Image, width, height int) string {
	bounds := img.Bounds()
	var result strings.Builder

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			color := r.sampleArea(img, bounds, x, y, width, height)
			r8, g8, b8 := color.R, color.G, color.B

			if r.UseDither {
				r8, g8, b8 = r.applySubtleDither(r8, g8, b8, x, y)
			}
			ansiColor := RGBToANSI256(uint32(r8)<<8, uint32(g8)<<8, uint32(b8)<<8)
			result.WriteString(fmt.Sprintf("\033[48;5;%dm \033[0m", ansiColor)) // Space char with background color.
		}
		result.WriteString("\n")
	}
	return result.String()
}

// renderHalfBlocksImproved uses Unicode '▀' for two "pixels" (top/bottom) per char cell.
func (r *ImageRenderer) renderHalfBlocksImproved(img image.Image, width, height int) string {
	bounds := img.Bounds()
	var result strings.Builder

	for y := 0; y < height; y += 2 { // Two image rows per terminal line.
		for x := 0; x < width; x++ {
			topColor := r.sampleArea(img, bounds, x, y, width, height)
			var bottomColor Color
			if y+1 < height {
				bottomColor = r.sampleArea(img, bounds, x, y+1, width, height)
			} else {
				bottomColor = topColor // Odd height, duplicate last row's pixel.
			}

			topR, topG, topB := topColor.R, topColor.G, topColor.B
			bottomR, bottomG, bottomB := bottomColor.R, bottomColor.G, bottomColor.B

			if r.UseDither {
				topR, topG, topB = r.applySubtleDither(topR, topG, topB, x, y)
				bottomR, bottomG, bottomB = r.applySubtleDither(bottomR, bottomG, bottomB, x, y+1)
			}

			topANSI := RGBToANSI256(uint32(topR)<<8, uint32(topG)<<8, uint32(topB)<<8)
			bottomANSI := RGBToANSI256(uint32(bottomR)<<8, uint32(bottomG)<<8, uint32(bottomB)<<8)

			if topANSI == bottomANSI {
				result.WriteString(fmt.Sprintf("\033[48;5;%dm \033[0m", topANSI))
			} else {
				// '▀' (Upper Half Block) with fg for top, bg for bottom.
				result.WriteString(fmt.Sprintf("\033[38;5;%dm\033[48;5;%dm▀\033[0m", topANSI, bottomANSI))
			}
		}
		result.WriteString("\n")
	}
	return result.String()
}

// renderBraille uses Unicode Braille (2x4 dot grid per char) for hi-res monochrome.
func (r *ImageRenderer) renderBraille(img image.Image, width, height int) string {
	bounds := img.Bounds()
	var result strings.Builder

	brailleWidth := (width + 1) / 2
	brailleHeight := (height + 3) / 4
	if brailleWidth <= 0 {
		brailleWidth = 1
	}
	if brailleHeight <= 0 {
		brailleHeight = 1
	}

	for by := 0; by < brailleHeight; by++ {
		for bx := 0; bx < brailleWidth; bx++ {
			var pattern uint8                  // Bitmask for the Braille dots.
			var avgR, avgG, avgB, count uint32 // For average color of the 2x4 region.

			// Sample the 2x4 dot region.
			for py := 0; py < 4; py++ { // Dot row in char.
				for px := 0; px < 2; px++ { // Dot col in char.
					imgSampleX := bx*2 + px
					imgSampleY := by*4 + py

					if imgSampleX < width && imgSampleY < height {
						color := r.sampleArea(img, bounds, imgSampleX, imgSampleY, width, height)

						// Dot 'on' if luminance is over a threshold.
						lum := 0.299*float64(color.R) + 0.587*float64(color.G) + 0.114*float64(color.B)
						if lum > 128 { // 128 is a common mid-point threshold.
							pattern |= brailleDotMask(px, py)
						}

						avgR += uint32(color.R)
						avgG += uint32(color.G)
						avgB += uint32(color.B)
						count++
					}
				}
			}

			var finalR8, finalG8, finalB8 uint8
			if count > 0 { // Average color for the Braille char's foreground.
				finalR8 = uint8(avgR / count)
				finalG8 = uint8(avgG / count)
				finalB8 = uint8(avgB / count)
			}

			ansiColor := RGBToANSI256(uint32(finalR8)<<8, uint32(finalG8)<<8, uint32(finalB8)<<8)
			brailleChar := 0x2800 + rune(pattern) // Braille Unicode block starts at U+2800.

			result.WriteString(fmt.Sprintf("\033[38;5;%dm%c\033[0m", ansiColor, brailleChar))
		}
		result.WriteString("\n")
	}
	return result.String()
}

// brailleDotMask provides the bit for a dot at (x,y) in a Braille char.
// The dot numbering is specific to how Braille patterns are encoded.
func brailleDotMask(x, y int) uint8 {
	// Braille dot pattern:
	// 1 (0x01) 4 (0x08)
	// 2 (0x02) 5 (0x10)
	// 3 (0x04) 6 (0x20)
	// 7 (0x40) 8 (0x80) (dot 7 is for 4-row patterns)
	dotMap := [2][4]uint8{
		{0x01, 0x02, 0x04, 0x40}, // Column 0 (dots 1,2,3,7)
		{0x08, 0x10, 0x20, 0x80}, // Column 1 (dots 4,5,6,8)
	}
	if x >= 0 && x < 2 && y >= 0 && y < 4 {
		return dotMap[x][y]
	}
	return 0
}

// applyDither: Old Bayer matrix dithering. Kept for reference.
func (r *ImageRenderer) applyDither(r32, g32, b32 uint32, x, y int) (uint32, uint32, uint32) {
	bayerMatrix := [4][4]float64{
		{0.0625, 0.5625, 0.1875, 0.6875},
		{0.8125, 0.3125, 0.9375, 0.4375},
		{0.2500, 0.7500, 0.1250, 0.6250},
		{1.0000, 0.5000, 0.8750, 0.3750},
	}
	threshold := bayerMatrix[y%4][x%4] * 4096
	r32 = uint32(float64(r32) + threshold)
	g32 = uint32(float64(g32) + threshold)
	b32 = uint32(float64(b32) + threshold)
	if r32 > 65535 {
		r32 = 65535
	}
	if g32 > 65535 {
		g32 = 65535
	}
	if b32 > 65535 {
		b32 = 65535
	}
	return r32, g32, b32
}

type Color struct {
	R, G, B uint8
}

// sampleArea grabs a color from the source image for a target output "pixel".
// Uses nearest-neighbor sampling (center of the target area).
func (r *ImageRenderer) sampleArea(img image.Image, bounds image.Rectangle, x, y, outWidth, outHeight int) Color {
	srcX := float64(x) * float64(bounds.Dx()) / float64(outWidth)
	srcY := float64(y) * float64(bounds.Dy()) / float64(outHeight)

	imgPixelX := bounds.Min.X + int(srcX)
	imgPixelY := bounds.Min.Y + int(srcY)

	if imgPixelX >= bounds.Max.X {
		imgPixelX = bounds.Max.X - 1
	}
	if imgPixelY >= bounds.Max.Y {
		imgPixelY = bounds.Max.Y - 1
	}
	if imgPixelX < bounds.Min.X {
		imgPixelX = bounds.Min.X
	}
	if imgPixelY < bounds.Min.Y {
		imgPixelY = bounds.Min.Y
	}

	pixel := img.At(imgPixelX, imgPixelY)
	r32, g32, b32, _ := pixel.RGBA()

	return Color{R: uint8(r32 >> 8), G: uint8(g32 >> 8), B: uint8(b32 >> 8)}
}

// enhanceContrast: Can be used for contrast tweaks. Currently a no-op.
func (r *ImageRenderer) enhanceContrast(val uint8) uint8 {
	return val // No contrast enhancement applied by default.
}

// applySubtleDither adds a tiny bit of noise to reduce color banding.
func (r *ImageRenderer) applySubtleDither(r8, g8, b8 uint8, x, y int) (uint8, uint8, uint8) {
	if !r.UseDither {
		return r8, g8, b8
	}
	// Small, simple dither matrix.
	matrix := [2][2]int8{
		{-2, 0},
		{1, -1},
	}
	threshold := matrix[y%2][x%2] * 2 // Max adjustment of +/- 4.

	return clampAddSigned(r8, threshold), clampAddSigned(g8, threshold), clampAddSigned(b8, threshold)
}

// clampAddSigned adds a signed value to a uint8, clamping to 0-255.
// Uses int16 internally to avoid overflow/underflow during the add.
func clampAddSigned(base uint8, add int8) uint8 {
	result := int16(base) + int16(add)
	if result > 255 {
		return 255
	}
	if result < 0 {
		return 0
	}
	return uint8(result)
}

// RenderImageToTerminal is a helper to quickly print an image.
func RenderImageToTerminal(img image.Image, mode RenderMode) {
	renderer := NewImageRenderer(mode)
	output := renderer.RenderImage(img)
	fmt.Print(output)
}

// RenderImagePixelPerfect aims for good quality with sensible defaults.
func RenderImagePixelPerfect(img image.Image, useHalfBlocks bool) string {
	mode := BlockMode
	if useHalfBlocks {
		mode = HalfBlockMode
	}
	renderer := NewImageRenderer(mode)
	// renderer.UseDither = true // Dithering is on by default in NewImageRenderer
	return renderer.RenderImage(img)
}

// RenderImageAdvanced offers more control over rendering.
func RenderImageAdvanced(img image.Image, mode RenderMode, maxWidth, maxHeight int, useDither bool) string {
	renderer := &ImageRenderer{
		Mode:        mode,
		MaxWidth:    maxWidth,
		MaxHeight:   maxHeight,
		UseDither:   useDither,
		AspectRatio: 0.5, // Default, can be customized if ImageRenderer is exposed more
	}
	return renderer.RenderImage(img)
}
