package cmd

import (
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var asciiArt = `
████████╗███████╗██████╗ ███╗   ███╗██╗   ██╗██╗    ██╗██╗   ██╗
╚══██╔══╝██╔════╝██╔══██╗████╗ ████║██║   ██║██║    ██║██║   ██║
   ██║   █████╗  ██████╔╝██╔████╔██║██║   ██║██║ █╗ ██║██║   ██║
   ██║   ██╔══╝  ██╔══██╗██║╚██╔╝██║██║   ██║██║███╗██║██║   ██║
   ██║   ███████╗██║  ██║██║ ╚═╝ ██║╚██████╔╝╚███╔███╔╝╚██████╔╝
   ╚═╝   ╚══════╝╚═╝  ╚═╝╚═╝     ╚═╝ ╚═════╝  ╚══╝╚══╝  ╚═════╝
`

var rootCmd = &cobra.Command{
	Use:     "termuwu",
	Short:   "🎨 Render images beautifully in your terminal",
	Version: "0.69.420",
	Long: color.New(color.FgMagenta).Sprint(asciiArt) + `

🖼️  TermUwU is a simple and flexible CLI tool for rendering images directly in your terminal.
It supports both local files and URLs, and displays them using detailed, colorful ANSI output — no GUI required.

✨ Features:
	• 📁 Local image files (PNG, JPEG, GIF, WebP)
	• 🌐 Direct URL downloads with progress bars
	• 🧱 Multiple rendering modes (blocks, half-blocks, braille)
	• ✏️ Adjustable width, height, and optional dithering
	• 🎨 Beautiful colored output

💡 Examples:
	# Render a local image
	termuwu show /path/to/your/image.jpg

	# Download and render from URL (wrap in quotes)
	termuwu show "https://example.com/image.jpg"
	
	# Custom dimensions with full blocks
	termuwu show image.png --width 80 --height 40 --full
	
	# High-detail rendering with braille patterns
	termuwu show image.jpg --braille --no-dither

🚀 Get started by running: termuwu show --help`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
