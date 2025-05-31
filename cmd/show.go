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
		resizedImg := resize.Resize(maxWidth, 0, img, resize.Lanczos3) // 0 for height means to maintain aspect ratio

		fmt.Printf("➡️  Resized image size: %dx%d\n", resizedImg.Bounds().Dx(), resizedImg.Bounds().Dy())

	},
}

func init() {
	rootCmd.AddCommand(showCmd)
}
