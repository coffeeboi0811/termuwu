package cmd

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png" // importing them blank to register the image formats
	"os"

	"github.com/nfnt/resize"
	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Render an image in the terminal",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			fmt.Println("❌ Please provide an image file path.")
			return
		}
		imagePath := args[0]
		fmt.Printf("📸 Loading image: %s\n", imagePath)

		file, err := os.Open(imagePath)
		if err != nil {
			fmt.Printf("❌ Failed to open image: %v\n", err)
			return
		}
		defer file.Close()

		// decoding the image
		img, format, err := image.Decode(file)
		if err != nil {
			fmt.Printf("❌ Failed to decode image: %v\n", err)
			return
		}

		fmt.Printf("✅ Image loaded successfully! Format: %s, Size: %dx%d\n",
			format, img.Bounds().Dx(), img.Bounds().Dy())

		// resizing the image to fit the terminal
		maxWidth := uint(80)
		resizedImg := resize.Resize(maxWidth, 0, img, resize.Lanczos3) // 0 for height means to maintain aspect ratio. Lanczos3 gives the best quality, especially when scaling down for terminal display.

		for y := 0; y < resizedImg.Bounds().Dy(); y++ {
			for x := 0; x < resizedImg.Bounds().Dx(); x++ {
				r, g, b, _ := resizedImg.At(x, y).RGBA()
				ansi := RGBToANSI256(r, g, b)

				// Print block with background color
				fmt.Printf("\x1b[48;5;%dm ", ansi)
			}
			fmt.Print("\x1b[0m\n") // Reset color, then newline
		}

	},
}

func init() {
	rootCmd.AddCommand(showCmd)
}
