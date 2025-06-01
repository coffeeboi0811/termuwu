package cmd

// converting an rgb color to a closest 256-color terminal code
func RGBToANSI256(r, g, b uint32) int {
	// Convert 16-bit color values to 8-bit (0–255)
	r8 := uint8(r >> 8)
	g8 := uint8(g >> 8)
	b8 := uint8(b >> 8)

	// Map 0–255 to 0–5
	rIndex := int(r8) * 6 / 256
	gIndex := int(g8) * 6 / 256
	bIndex := int(b8) * 6 / 256

	// 16 is the start index of the 6x6x6 color cube
	return 16 + (36 * rIndex) + (6 * gIndex) + bIndex
}
