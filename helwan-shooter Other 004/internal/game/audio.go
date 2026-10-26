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

const musicNoteDuration = 0.25

// initAudio initializes the audio system and generates background music.
func (g *Game) initAudio() {
	if g.audioContext == nil {
		g.audioContext = audio.NewContext(sampleRate)
	}

	if g.musicPlayer != nil {
		return
	}

	musicData := generateBackgroundMusic()
	loop := audio.NewInfiniteLoop(
		bytes.NewReader(musicData),
		int64(len(musicData)),
	)

	player, err := audio.NewPlayer(g.audioContext, loop)
	if err != nil {
		return
	}

	g.musicPlayer = player
	g.musicPlayer.SetVolume(0.45)
}

// generateBackgroundMusic creates an original 8-bit style melody.
// The generated music is eight seconds long and loops continuously.
func generateBackgroundMusic() []byte {
	melody := []float64{
		523.25, 659.25, 783.99, 659.25,
		587.33, 698.46, 880.00, 698.46,
		523.25, 659.25, 783.99, 1046.50,
		783.99, 659.25, 587.33, 523.25,
		392.00, 523.25, 659.25, 523.25,
		440.00, 587.33, 698.46, 587.33,
		392.00, 523.25, 659.25, 783.99,
		659.25, 587.33, 523.25, 392.00,
	}

	bass := []float64{
		130.81, 130.81, 164.81, 164.81,
		174.61, 174.61, 146.83, 146.83,
	}

	samplesPerNote := int(float64(sampleRate) * musicNoteDuration)
	totalSamples := samplesPerNote * len(melody)
	pcm := make([]byte, totalSamples*4)

	for i := 0; i < totalSamples; i++ {
		noteIndex := i / samplesPerNote
		position := i % samplesPerNote
		t := float64(i) / float64(sampleRate)

		notePosition := float64(position) / float64(samplesPerNote)

		envelope := 1.0
		fade := 0.025

		if notePosition < fade {
			envelope = notePosition / fade
		} else if notePosition > 1.0-fade {
			envelope = (1.0 - notePosition) / fade
		}

		frequency := melody[noteIndex]
		phase := math.Mod(t*frequency, 1.0)

		melodySample := -0.0
		if phase < 0.5 {
			melodySample = 0.20
		} else {
			melodySample = -0.20
		}

		bassFrequency := bass[(noteIndex/4)%len(bass)]
		bassSample := math.Sin(2*math.Pi*bassFrequency*t) * 0.10

		pulse := 0.88 + 0.12*math.Sin(2*math.Pi*2*t)
		sample := (melodySample + bassSample) * envelope * pulse

		value := int16(sample * 10000)

		offset := i * 4
		binary.LittleEndian.PutUint16(pcm[offset:], uint16(value))
		binary.LittleEndian.PutUint16(pcm[offset+2:], uint16(value))
	}

	return pcm
}

// syncMusic starts or pauses background music according to game settings.
func (g *Game) syncMusic() {
	if g.musicPlayer == nil {
		return
	}

	shouldPlay := !g.muted && g.screen != pauseScreen

	if shouldPlay {
		if !g.musicPlayer.IsPlaying() {
			g.musicPlayer.Play()
		}
	} else if g.musicPlayer.IsPlaying() {
		g.musicPlayer.Pause()
	}
}

// playSynthSound generates and plays a short sound effect.
func (g *Game) playSynthSound(freq float64, durationSec float64, waveType string) {
	if g.muted {
		return
	}

	if g.audioContext == nil {
		g.audioContext = audio.NewContext(sampleRate)
	}

	numSamples := int(float64(sampleRate) * durationSec)
	buf := new(bytes.Buffer)

	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(sampleRate)
		var sampleVal float64

		envelope := 1.0 - float64(i)/float64(numSamples)

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

		val := int16(sampleVal * envelope * 10000)
		_ = binary.Write(buf, binary.LittleEndian, val)
		_ = binary.Write(buf, binary.LittleEndian, val)
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
