package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/helwan-linux/helwan-shooter/internal/game"
)

func main() {
	ebiten.SetWindowTitle("Helwan Shooter")
	ebiten.SetWindowSize(1000, 650)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	if err := ebiten.RunGame(game.New()); err != nil {
		log.Fatal(err)
	}
}
