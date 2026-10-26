package game

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

const sampleRate = 44100

func (g *Game) initAudio() {
	g.audioContext = audio.NewContext(sampleRate)
}

func (g *Game) playSynthSound(freq float64, durationSec float64, waveType string) {
	if g.muted || g.audioContext == nil {
		return
	}

	numSamples := int(float64(sampleRate) * durationSec)
	buf := new(bytes.Buffer)

	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(sampleRate)
		var sampleVal float64

		envelope := 1.0 - (float64(i) / float64(numSamples))

		switch waveType {
		case "square":
			if math.Sin(2*math.Pi*freq*t) > 0 {
				sampleVal = 0.5
			} else {
				sampleVal = -0.5
			}
		case "saw":
			sampleVal = 2.0 * (t*freq - math.Floor(0.5+t*freq))
		case "noise":
			sampleVal = rand.Float64()*2.0 - 1.0
		default:
			sampleVal = math.Sin(2 * math.Pi * freq * t)
		}

		val := int16(sampleVal * envelope * 8000)
		_ = binary.Write(buf, binary.LittleEndian, val)
		_ = binary.Write(buf, binary.LittleEndian, val) // Stereo
	}

	player := audio.NewPlayerFromBytes(g.audioContext, buf.Bytes())
	player.Play()
}

func (g *Game) soundShoot() {
	g.playSynthSound(440, 0.08, "square")
}

func (g *Game) soundHit() {
	g.playSynthSound(880, 0.12, "sine")
}

func (g *Game) soundHazard() {
	g.playSynthSound(150, 0.2, "saw")
}

func (g *Game) soundWin() {
	go func() {
		g.playSynthSound(523.25, 0.1, "sine")
		time.Sleep(100 * time.Millisecond)
		g.playSynthSound(659.25, 0.1, "sine")
		time.Sleep(100 * time.Millisecond)
		g.playSynthSound(783.99, 0.2, "sine")
	}()
}