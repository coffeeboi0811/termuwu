package cmd

import (
	"fmt"
	"image"
	"os"
	"strings"

	"golang.org/x/term"
)

type RenderMode int

const (
	BlockMode RenderMode = iota
	HalfBlockMode
	BrailleMode
)

type ImageRenderer struct {
	Mode        RenderMode
	MaxWidth    int
	MaxHeight   int
	UseDither   bool
	AspectRatio float64
}

func NewImageRenderer(mode RenderMode) *ImageRenderer {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		width, height = 100, 28 // fallback if terminal size detection fails
	}

	return &ImageRenderer{
		Mode:        mode,
		MaxWidth:    width - 2,
		MaxHeight:   height - 3,
		UseDither:   true,
		AspectRatio: 0.5, // common for terminal fonts
	}
}

func (r *ImageRenderer) RenderImage(img image.Image) string {
	bounds := img.Bounds()
	imgWidth := bounds.Dx()
	imgHeight := bounds.Dy()

	var outputWidth, outputHeight int

	if r.Mode == HalfBlockMode {
		// half blocks double our effective vertical resolution
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
			outputHeight-- // must be even for half block pairs
		}
		if outputHeight <= 0 {
			outputHeight = 2
		}

	} else { // BlockMode or BrailleMode
		scaleX := float64(r.MaxWidth) / float64(imgWidth)
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

func (r *ImageRenderer) renderFullBlocksImproved(img image.Image, width, height int) string {
	bounds := img.Bounds()
	var result strings.Builder

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			sampledColor := r.sampleArea(img, bounds, x, y, width, height)
			r8, g8, b8 := sampledColor.R, sampledColor.G, sampledColor.B

			if r.UseDither {
				r8, g8, b8 = r.applySubtleDither(r8, g8, b8, x, y)
			}
			ansiColor := RGBToANSI256(uint32(r8)<<8, uint32(g8)<<8, uint32(b8)<<8)
			result.WriteString(fmt.Sprintf("\033[48;5;%dm \033[0m", ansiColor))
		}
		result.WriteString("\n")
	}
	return result.String()
}

func (r *ImageRenderer) renderHalfBlocksImproved(img image.Image, width, height int) string {
	bounds := img.Bounds()
	var result strings.Builder

	for y := 0; y < height; y += 2 { // two image rows per terminal line
		for x := 0; x < width; x++ {
			topColorStruct := r.sampleArea(img, bounds, x, y, width, height)
			var bottomColorStruct Color
			if y+1 < height {
				bottomColorStruct = r.sampleArea(img, bounds, x, y+1, width, height)
			} else {
				bottomColorStruct = topColorStruct
			}

			topR, topG, topB := topColorStruct.R, topColorStruct.G, topColorStruct.B
			bottomR, bottomG, bottomB := bottomColorStruct.R, bottomColorStruct.G, bottomColorStruct.B

			if r.UseDither {
				topR, topG, topB = r.applySubtleDither(topR, topG, topB, x, y)
				bottomR, bottomG, bottomB = r.applySubtleDither(bottomR, bottomG, bottomB, x, y+1)
			}

			topANSI := RGBToANSI256(uint32(topR)<<8, uint32(topG)<<8, uint32(topB)<<8)
			bottomANSI := RGBToANSI256(uint32(bottomR)<<8, uint32(bottomG)<<8, uint32(bottomB)<<8)

			if topANSI == bottomANSI {
				result.WriteString(fmt.Sprintf("\033[48;5;%dm \033[0m", topANSI))
			} else {
				// '▀' (Upper Half Block) with fg for top, bg for bottom
				result.WriteString(fmt.Sprintf("\033[38;5;%dm\033[48;5;%dm▀\033[0m", topANSI, bottomANSI))
			}
		}
		result.WriteString("\n")
	}
	return result.String()
}

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
			var pattern uint8
			var avgR, avgG, avgB, count uint32

			for py := 0; py < 4; py++ {
				for px := 0; px < 2; px++ {
					imgSampleX := bx*2 + px
					imgSampleY := by*4 + py

					if imgSampleX < width && imgSampleY < height {
						sampledColor := r.sampleArea(img, bounds, imgSampleX, imgSampleY, width, height)

						lum := 0.299*float64(sampledColor.R) + 0.587*float64(sampledColor.G) + 0.114*float64(sampledColor.B)
						if lum > 128 { // 128 is a common mid-point threshold
							pattern |= brailleDotMask(px, py)
						}

						avgR += uint32(sampledColor.R)
						avgG += uint32(sampledColor.G)
						avgB += uint32(sampledColor.B)
						count++
					}
				}
			}

			var finalR8, finalG8, finalB8 uint8
			if count > 0 {
				finalR8 = uint8(avgR / count)
				finalG8 = uint8(avgG / count)
				finalB8 = uint8(avgB / count)
			}

			ansiColor := RGBToANSI256(uint32(finalR8)<<8, uint32(finalG8)<<8, uint32(finalB8)<<8)
			brailleChar := 0x2800 + rune(pattern) // braille unicode block starts at U+2800

			result.WriteString(fmt.Sprintf("\033[38;5;%dm%c\033[0m", ansiColor, brailleChar))
		}
		result.WriteString("\n")
	}
	return result.String()
}

func brailleDotMask(x, y int) uint8 {
	// braille dot pattern:
	// 1 (0x01) 4 (0x08)
	// 2 (0x02) 5 (0x10)
	// 3 (0x04) 6 (0x20)
	// 7 (0x40) 8 (0x80)
	dotMap := [2][4]uint8{
		{0x01, 0x02, 0x04, 0x40}, // column 0
		{0x08, 0x10, 0x20, 0x80}, // column 1
	}
	if x >= 0 && x < 2 && y >= 0 && y < 4 {
		return dotMap[x][y]
	}
	return 0
}

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

func (r *ImageRenderer) applySubtleDither(r8, g8, b8 uint8, x, y int) (uint8, uint8, uint8) {
	if !r.UseDither {
		return r8, g8, b8
	}
	matrix := [2][2]int8{
		{-2, 0},
		{1, -1},
	}
	threshold := matrix[y%2][x%2] * 2 // small, simple dither matrix

	return clampAddSigned(r8, threshold), clampAddSigned(g8, threshold), clampAddSigned(b8, threshold)
}

func clampAddSigned(base uint8, add int8) uint8 {
	result := int16(base) + int16(add) // uses int16 to avoid overflow/underflow
	if result > 255 {
		return 255
	}
	if result < 0 {
		return 0
	}
	return uint8(result)
}
