
package main

import (
	"bytes"
	_ "embed"
	"image"
	"image/png"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/helwan-linux/helwan-shooter/internal/game"
)

//go:embed helwan-shooter.png
var iconPNG []byte

func main() {
	icon, err := png.Decode(bytes.NewReader(iconPNG))
	if err != nil {
		log.Fatal(err)
	}

	ebiten.SetWindowIcon([]image.Image{icon})
	ebiten.SetWindowTitle("Helwan Shooter")
	ebiten.SetWindowSize(1000, 650)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	if err := ebiten.RunGame(game.New()); err != nil {
		log.Fatal(err)
	}
}
