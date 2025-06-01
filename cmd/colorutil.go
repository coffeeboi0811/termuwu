package cmd

import (
	"fmt"
	"image"
	"os"
	"strings"

	"golang.org/x/term"
)

// RGBToANSI256 figures out the best ANSI 256 color for a given RGB value.
func RGBToANSI256(r, g, b uint32) int {
	r8 := clamp8(r >> 8)
	g8 := clamp8(g >> 8)
	b8 := clamp8(b >> 8)

	if r8 == 0 && g8 == 0 && b8 == 0 {
		return 16 // Standard ANSI Black.
	}

	// Give very dark colors a little nudge so they don't just become black.
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

	// If it's almost gray, the grayscale ramp might actually look better.
	if isNearGrayscale(r8, g8, b8) {
		grayColor := mapToGrayscale(r8, g8, b8)
		if colorDistance(r8, g8, b8, grayColor) < colorDistance(r8, g8, b8, cubeColor) {
			return grayColor
		}
	}

	return cubeColor
}

// clamp8 makes sure a value is within the 0-255 range.
func clamp8(val uint32) uint8 {
	if val > 255 {
		return 255
	}
	return uint8(val)
}

// quantizeToSix converts an 8-bit color value to one of the 6 levels in the ANSI color cube.
func quantizeToSix(val uint8) int {
	if val < 48 { // Thresholds for 6 levels (0..5)
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

// isGrayscale checks if a color is basically gray.
func isGrayscale(r, g, b uint8) bool {
	max := maxUint8(r, g, b)
	min := minUint8(r, g, b)
	return max-min <= 10 // How close R, G, and B need to be to count as gray.
}

// isNearGrayscale is for colors that aren't strictly gray but might be better represented by the grayscale ramp.
func isNearGrayscale(r, g, b uint8) bool {
	max := maxUint8(r, g, b)
	min := minUint8(r, g, b)
	return max-min <= 30 // A bit more lenient than isGrayscale.
}

// mapToGrayscale finds the best match in ANSI's 24-step grayscale ramp.
func mapToGrayscale(r, g, b uint8) int {
	gray := uint8(0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) // Standard luminance calculation.

	if gray < 10 {
		return 232 // Use the darkest available gray from the ramp, not pure black.
	}
	if gray > 245 {
		return 255 // Lightest gray.
	}
	// Spread the rest across the grayscale ramp.
	return 232 + int(float64(gray-8)*(23.0/240.0))
}

// colorDistance tries to measure how different two colors look to a human.
func colorDistance(r1, g1, b1 uint8, colorIndex int) float64 {
	r2, g2, b2 := ansiToRGB(colorIndex)

	dr := float64(r1) - float64(r2)
	dg := float64(g1) - float64(g2)
	db := float64(b1) - float64(b2)

	// Using weighted Euclidean distance because greens look brighter, blues darker.
	return 2*dr*dr + 4*dg*dg + 3*db*db
}

// ansiToRGB gives an approximate RGB value for an ANSI 256 color index.
// Needed for colorDistance.
func ansiToRGB(index int) (uint8, uint8, uint8) {
	if index >= 232 && index <= 255 { // Grayscale colors.
		grayVal := float64(index-232)*(240.0/23.0) + 8.0
		if grayVal < 0 {
			grayVal = 0
		}
		if grayVal > 255 {
			grayVal = 255
		}
		gray := uint8(grayVal)
		return gray, gray, gray
	}

	if index >= 16 && index <= 231 { // The 6x6x6 color cube.
		index -= 16
		r := (index / 36) % 6
		g := (index / 6) % 6
		b := index % 6
		// These are the typical RGB values for each level in the color cube.
		valMap := []uint8{0, 47, 95, 142, 189, 236}
		return valMap[r], valMap[g], valMap[b]
	}

	// Standard 0-15 colors. These can vary a lot between terminals.
	if index == 0 || index == 16 {
		return 0, 0, 0
	} // Black
	if index == 7 || index == 231 {
		return 255, 255, 255
	} // White
	// For other standard colors, mid-gray is a rough guess for distance checks.
	return 128, 128, 128
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

// RenderMode tells us how to draw the image pixels.
type RenderMode int

const (
	BlockMode     RenderMode = iota // Uses full character blocks (a space with a background color).
	HalfBlockMode                   // Uses Unicode half blocks '▀' to get more vertical detail.
	BrailleMode                     // Uses Unicode Braille characters for a higher-res monochrome look.
)

// ImageRenderer is responsible for drawing the image in the terminal.
type ImageRenderer struct {
	Mode        RenderMode
	MaxWidth    int     // How many characters wide the image can be.
	MaxHeight   int     // How many lines tall the image can be.
	UseDither   bool    // Whether to use dithering to smooth out colors.
	AspectRatio float64 // The height/width ratio of a character in the terminal.
}

// NewImageRenderer sets up an ImageRenderer, trying to get the current terminal size.
func NewImageRenderer(mode RenderMode) *ImageRenderer {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		width, height = 100, 28 // A sensible default if we can't get the terminal size.
	}

	return &ImageRenderer{
		Mode:        mode,
		MaxWidth:    width - 2,  // Leave a small margin.
		MaxHeight:   height - 3, // Leave a bit more margin at the bottom.
		UseDither:   true,
		AspectRatio: 0.5, // Most terminal fonts are about twice as tall as they are wide.
	}
}

// RenderImage takes an image and turns it into a string of ANSI escape codes for terminal display.
func (r *ImageRenderer) RenderImage(img image.Image) string {
	bounds := img.Bounds()
	imgWidth := bounds.Dx()
	imgHeight := bounds.Dy()

	var outputWidth, outputHeight int

	if r.Mode == HalfBlockMode {
		// Half blocks give us two "pixels" vertically per character row.
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
			outputHeight-- // Needs to be even for half blocks.
		}
		if outputHeight <= 0 {
			outputHeight = 2
		}

	} else { // BlockMode or BrailleMode.
		scaleX := float64(r.MaxWidth) / float64(imgWidth)
		// Character aspect ratio is important for BlockMode's perceived image shape.
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

// renderFullBlocksImproved draws the image using a space character with a colored background for each pixel.
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

// renderHalfBlocksImproved uses Unicode '▀' to draw two "pixels" (top and bottom half) per character cell.
func (r *ImageRenderer) renderHalfBlocksImproved(img image.Image, width, height int) string {
	bounds := img.Bounds()
	var result strings.Builder

	for y := 0; y < height; y += 2 { // Step by 2 because each loop handles two image rows.
		for x := 0; x < width; x++ {
			topColor := r.sampleArea(img, bounds, x, y, width, height)
			var bottomColor Color
			if y+1 < height {
				bottomColor = r.sampleArea(img, bounds, x, y+1, width, height)
			} else {
				// If there's no bottom row (odd image height), just use the top color.
				bottomColor = topColor
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
				// If both halves are the same, just draw a full block.
				result.WriteString(fmt.Sprintf("\033[48;5;%dm \033[0m", topANSI))
			} else {
				// Otherwise, use the half block char with different fg/bg colors.
				result.WriteString(fmt.Sprintf("\033[38;5;%dm\033[48;5;%dm▀\033[0m", topANSI, bottomANSI))
			}
		}
		result.WriteString("\n")
	}
	return result.String()
}

// renderBraille uses Unicode Braille characters. Each char is a 2x4 grid of dots.
// This gives a higher spatial resolution but is monochrome for each dot.
func (r *ImageRenderer) renderBraille(img image.Image, width, height int) string {
	bounds := img.Bounds()
	var result strings.Builder

	brailleWidth := (width + 1) / 2   // Braille chars are 2 output pixels wide.
	brailleHeight := (height + 3) / 4 // And 4 output pixels tall.
	if brailleWidth <= 0 {
		brailleWidth = 1
	}
	if brailleHeight <= 0 {
		brailleHeight = 1
	}

	for by := 0; by < brailleHeight; by++ {
		for bx := 0; bx < brailleWidth; bx++ {
			var pattern uint8                  // This will hold the bitmask for the Braille character.
			var avgR, avgG, avgB, count uint32 // For the average color of the 2x4 region.

			// Go through the 2x4 dots for this Braille character.
			for py := 0; py < 4; py++ { // Dot row in Braille char (0-3)
				for px := 0; px < 2; px++ { // Dot column (0-1)
					imgSampleX := bx*2 + px
					imgSampleY := by*4 + py

					if imgSampleX < width && imgSampleY < height {
						color := r.sampleArea(img, bounds, imgSampleX, imgSampleY, width, height)

						// Turn dot on/off based on luminance.
						lum := 0.299*float64(color.R) + 0.587*float64(color.G) + 0.114*float64(color.B)
						if lum > 128 { // Arbitrary threshold for "on".
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
			if count > 0 { // Calculate the average color for the Braille char's foreground.
				finalR8 = uint8(avgR / count)
				finalG8 = uint8(avgG / count)
				finalB8 = uint8(avgB / count)
			}

			ansiColor := RGBToANSI256(uint32(finalR8)<<8, uint32(finalG8)<<8, uint32(finalB8)<<8)
			brailleChar := 0x2800 + rune(pattern) // Braille chars start at U+2800.

			result.WriteString(fmt.Sprintf("\033[38;5;%dm%c\033[0m", ansiColor, brailleChar))
		}
		result.WriteString("\n")
	}
	return result.String()
}

// brailleDotMask gives the bit for a dot at (x,y) in a Braille char.
// The dot numbering is a bit specific for Braille.
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

// applyDither is an older Bayer matrix dithering method. Kept for reference.
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

// Color is just a simple struct for 8-bit RGB.
type Color struct {
	R, G, B uint8
}

// sampleArea gets a color from the original image for a given "pixel" in our scaled output.
// This version just picks the color from the center of the corresponding area (nearest neighbor).
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

// enhanceContrast can be used to tweak the image contrast.
// It's a no-op by default right now.
func (r *ImageRenderer) enhanceContrast(val uint8) uint8 {
	return val // No contrast enhancement applied by default.
}

// applySubtleDither adds a tiny bit of noise to colors to help with banding.
func (r *ImageRenderer) applySubtleDither(r8, g8, b8 uint8, x, y int) (uint8, uint8, uint8) {
	if !r.UseDither {
		return r8, g8, b8
	}
	// A very small dither matrix.
	matrix := [2][2]int8{
		{-2, 0},
		{1, -1},
	}
	threshold := matrix[y%2][x%2] * 2 // Tiny adjustment.

	return clampAddSigned(r8, threshold), clampAddSigned(g8, threshold), clampAddSigned(b8, threshold)
}

// clampAddSigned adds a (potentially negative) int8 to a uint8, keeping it in the 0-255 range.
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

// RenderImageToTerminal is a quick way to just print an image to the console.
func RenderImageToTerminal(img image.Image, mode RenderMode) {
	renderer := NewImageRenderer(mode)
	output := renderer.RenderImage(img)
	fmt.Print(output)
}

// RenderImagePixelPerfect is a good starting point for rendering an image with decent quality.
func RenderImagePixelPerfect(img image.Image, useHalfBlocks bool) string {
	mode := BlockMode
	if useHalfBlocks {
		mode = HalfBlockMode
	}
	renderer := NewImageRenderer(mode)
	// renderer.UseDither = true // Dithering is on by default in NewImageRenderer
	return renderer.RenderImage(img)
}

// RenderImageAdvanced gives you all the knobs to turn for rendering.
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
