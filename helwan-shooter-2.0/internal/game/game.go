package game

import (
	"encoding/json"
	"fmt"
	"image/color"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	screenW        = 1000
	screenH        = 650
	worldCount     = 5
	levelsPerWorld = 5
	totalLevels    = worldCount * levelsPerWorld
)

type gameScreen int

const (
	menuScreen gameScreen = iota
	worldScreen
	levelScreen
	playScreen
	pauseScreen
	settingsScreen
	resultScreen
	gameOverScreen
)

type targetKind int

const (
	normalTarget targetKind = iota
	bonusTarget
	hazardTarget
	bossTarget
	armoredTarget
	fastTarget
	ammoTarget
)

type target struct {
	x, y      float64
	vx, vy    float64
	radius    float32
	kind      targetKind
	hp, maxHP int
	visible   bool
	phase     float64
}

type saveData struct {
	UnlockedLevel int         `json:"unlocked_level"`
	Stars         map[int]int `json:"stars"`
	BestScores    map[int]int `json:"best_scores"`
	TotalShots    int         `json:"total_shots"`
	TotalHits     int         `json:"total_hits"`
	Sound         bool        `json:"sound"`
}

type Game struct {
	screen gameScreen
	save   saveData

	level        int
	world        int
	wave         int
	wavesTotal   int
	mouseWasDown bool
	clickPressed bool
	clickX       int
	clickY       int

	score int
	shots int
	hits  int
	ammo  int

	timer    float64
	lastTick time.Time

	targets []target

	muted       bool
	cursorX     int
	cursorY     int
	message     string
	messageTill time.Time
}

var worldNames = []string{
	"Training Grounds",
	"Abandoned Factory",
	"Neon City",
	"Desert Outpost",
	"Final Frontier",
}

var worldDescriptions = []string{
	"Learn to aim.",
	"Faster targets and dangerous objects.",
	"Targets fade in and out.",
	"Harder shots and limited ammunition.",
	"The ultimate shooting challenge.",
}

func New() *Game {
	g := &Game{
		screen: menuScreen,
		save: saveData{
			UnlockedLevel: 1,
			Stars:         make(map[int]int),
			BestScores:    make(map[int]int),
			Sound:         true,
		},
	}

	g.loadProgress()
	return g
}

func (g *Game) savePath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(configDir, "HelwanShooter", "progress.json")
}

func (g *Game) loadProgress() {
	path := g.savePath()
	if path == "" {
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var saved saveData
	if json.Unmarshal(data, &saved) != nil {
		return
	}

	if saved.UnlockedLevel < 1 {
		saved.UnlockedLevel = 1
	}
	if saved.UnlockedLevel > totalLevels {
		saved.UnlockedLevel = totalLevels
	}
	if saved.Stars == nil {
		saved.Stars = make(map[int]int)
	}
	if saved.BestScores == nil {
		saved.BestScores = make(map[int]int)
	}

	g.save = saved
	g.muted = !saved.Sound
}

func (g *Game) saveProgress() {
	path := g.savePath()
	if path == "" {
		return
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return
	}

	data, err := json.MarshalIndent(g.save, "", "  ")
	if err != nil {
		return
	}

	_ = os.WriteFile(path, data, 0644)
}

func (g *Game) startLevel(level int) {
	if level < 1 || level > totalLevels {
		return
	}

	g.level = level
	g.world = (level-1)/levelsPerWorld + 1

	g.score = 0
	g.shots = 0
	g.hits = 0

	g.ammo = 24 - g.world - (level-1)%levelsPerWorld
	if g.ammo < 8 {
		g.ammo = 8
	}

	g.timer = 65 - float64(g.world-1)*3
	if g.timer < 30 {
		g.timer = 30
	}

	g.lastTick = time.Now()
	g.message = fmt.Sprintf(
		"WORLD %d - STAGE %d",
		g.world,
		(level-1)%levelsPerWorld+1,
	)
	g.messageTill = time.Now().Add(2 * time.Second)

	g.createTargets()
	g.screen = playScreen
}

func (g *Game) createTargets() {
	g.targets = nil
	g.wave = 1

	// Every fifth stage ends in a boss fight. Other stages contain multiple waves.
	if g.level%levelsPerWorld == 0 {
		g.wavesTotal = 1
		hp := 5 + g.world*2
		g.targets = append(g.targets, target{
			x:       screenW / 2,
			y:       330,
			vx:      2.0 + float64(g.world)*0.65,
			vy:      1.3 + float64(g.world)*0.25,
			radius:  70,
			kind:    bossTarget,
			hp:      hp,
			maxHP:   hp,
			visible: true,
		})
		return
	}

	g.wavesTotal = 2 + ((g.level-1)%3)/2
	g.spawnTargetWave()
}

func (g *Game) spawnTargetWave() {
	g.targets = nil
	count := 4 + (g.world+1)/2 + g.wave
	if g.level >= 16 {
		count++
	}

	stageInWorld := (g.level-1)%levelsPerWorld + 1
	speed := 0.9 + float64(g.world)*0.28 + float64(stageInWorld-1)*0.16 + float64(g.wave-1)*0.15
	for i := 0; i < count; i++ {
		radius := float32(30 - g.world)
		if radius < 13 {
			radius = 13
		}
		t := target{
			x:       float64(100 + rand.Intn(screenW-200)),
			y:       float64(150 + rand.Intn(screenH-235)),
			vx:      (rand.Float64()*2 - 1) * speed,
			vy:      (rand.Float64()*2 - 1) * speed * 0.8,
			radius:  radius,
			kind:    normalTarget,
			hp:      1,
			maxHP:   1,
			visible: true,
			phase:   rand.Float64() * math.Pi * 2,
		}

		// Each world adds a different target challenge.
		if i == count-1 && (g.level+g.wave)%3 == 0 {
			t.kind = bonusTarget
			t.radius = 17
		} else if i == count-2 && (g.level+g.wave)%2 == 0 {
			t.kind = hazardTarget
			t.radius = 21
		} else if i == count-3 && g.world >= 2 {
			t.kind = armoredTarget
			t.radius = 23
			t.hp = 2
			t.maxHP = 2
		} else if i == count-4 && g.world >= 3 {
			t.kind = fastTarget
			t.radius = 13
			t.vx *= 1.65
			t.vy *= 1.65
		} else if i == count-5 && g.world >= 4 {
			t.kind = ammoTarget
			t.radius = 17
		}
		g.targets = append(g.targets, t)
	}
}

func (g *Game) Update() error {
	mx, my := ebiten.CursorPosition()
	g.cursorX, g.cursorY = mx, my
	down := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	g.clickPressed = down && !g.mouseWasDown
	g.clickX, g.clickY = mx, my
	g.mouseWasDown = down

	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		switch g.screen {
		case playScreen:
			g.screen = pauseScreen
			return nil
		case pauseScreen:
			g.screen = playScreen
			g.lastTick = time.Now()
			return nil
		case menuScreen:
			return nil
		default:
			g.screen = menuScreen
			return nil
		}
	}

	switch g.screen {
	case menuScreen:
		g.updateMenu()
	case worldScreen:
		g.updateWorlds()
	case levelScreen:
		g.updateLevels()
	case playScreen:
		g.updatePlaying()
	case pauseScreen:
		g.updatePause()
	case settingsScreen:
		g.updateSettings()
	case resultScreen:
		g.updateResults()
	case gameOverScreen:
		g.updateGameOver()
	}

	return nil
}

func (g *Game) buttonClicked(x, y, w, h int) bool {
	return g.clickPressed &&
		g.clickX >= x && g.clickX <= x+w &&
		g.clickY >= y && g.clickY <= y+h
}

func (g *Game) updateMenu() {
	if g.buttonClicked(400, 220, 200, 48) {
		g.startLevel(g.save.UnlockedLevel)
	} else if g.buttonClicked(400, 280, 200, 48) {
		g.screen = worldScreen
	} else if g.buttonClicked(400, 340, 200, 48) {
		g.screen = settingsScreen
	}
}

func (g *Game) updateWorlds() {
	for i := 0; i < worldCount; i++ {
		y := 135 + i*88
		if g.buttonClicked(745, y-17, 120, 42) {
			firstLevel := i*levelsPerWorld + 1
			if g.save.UnlockedLevel >= firstLevel {
				g.world = i + 1
				g.screen = levelScreen
			}
			return
		}
	}

	if g.buttonClicked(40, 575, 150, 45) {
		g.screen = menuScreen
	}
}

func (g *Game) updateLevels() {
	for i := 0; i < levelsPerWorld; i++ {
		level := (g.world-1)*levelsPerWorld + i + 1
		x := 155 + (i%3)*235
		y := 170 + (i/3)*135

		if g.buttonClicked(x, y, 190, 62) {
			if level <= g.save.UnlockedLevel {
				g.startLevel(level)
			}
			return
		}
	}

	if g.buttonClicked(40, 575, 150, 45) {
		g.screen = worldScreen
	}
}

func (g *Game) updatePlaying() {
	now := time.Now()
	dt := now.Sub(g.lastTick).Seconds()
	g.lastTick = now
	if dt > 0.1 {
		dt = 0.1
	}

	g.timer -= dt
	if g.timer <= 0 || (g.ammo <= 0 && len(g.targets) > 0) {
		g.screen = gameOverScreen
		return
	}

	if g.clickPressed {
		g.shoot(float64(g.clickX), float64(g.clickY))
	}

	for i := range g.targets {
		t := &g.targets[i]
		t.x += t.vx
		t.y += t.vy
		t.phase += dt * (2.8 + float64(g.world)*0.25)
		if t.x < 90 || t.x > screenW-90 {
			t.vx = -t.vx
		}
		if t.y < 140 || t.y > screenH-75 {
			t.vy = -t.vy
		}
		if g.world == 3 && t.kind != bossTarget {
			t.visible = math.Sin(t.phase*1.7) > -0.25
		} else if g.world == 5 && t.kind != bossTarget {
			t.visible = math.Sin(t.phase*2.3) > -0.7
		} else {
			t.visible = true
		}
	}

	if len(g.targets) == 0 {
		if g.wave < g.wavesTotal {
			g.wave++
			g.message = fmt.Sprintf("WAVE %d / %d", g.wave, g.wavesTotal)
			g.messageTill = time.Now().Add(1300 * time.Millisecond)
			g.spawnTargetWave()
		} else {
			g.finishLevel()
		}
	}
}

func (g *Game) updatePause() {
	if g.buttonClicked(400, 355, 200, 52) {
		g.screen = playScreen
		g.lastTick = time.Now()
	} else if g.buttonClicked(400, 420, 200, 52) {
		g.screen = menuScreen
	}
}

func (g *Game) updateSettings() {
	if g.buttonClicked(400, 200, 200, 52) {
		g.muted = !g.muted
		g.save.Sound = !g.muted
		g.saveProgress()
	} else if g.buttonClicked(400, 460, 200, 52) {
		g.screen = menuScreen
	}
}

func (g *Game) updateResults() {
	if g.level < totalLevels && g.buttonClicked(400, 360, 200, 52) {
		g.startLevel(g.level + 1)
	} else if g.buttonClicked(400, 425, 200, 52) {
		g.world = (g.level-1)/levelsPerWorld + 1
		g.screen = levelScreen
	} else if g.buttonClicked(400, 490, 200, 52) {
		g.screen = menuScreen
	}
}

func (g *Game) updateGameOver() {
	if g.buttonClicked(400, 360, 200, 52) {
		g.startLevel(g.level)
	} else if g.buttonClicked(400, 425, 200, 52) {
		g.world = (g.level-1)/levelsPerWorld + 1
		g.screen = levelScreen
	} else if g.buttonClicked(400, 490, 200, 52) {
		g.screen = menuScreen
	}
}

func (g *Game) shoot(x, y float64) {
	if g.ammo <= 0 {
		return
	}
	g.ammo--
	g.shots++
	g.save.TotalShots++

	for i := len(g.targets) - 1; i >= 0; i-- {
		t := &g.targets[i]
		if !t.visible {
			continue
		}
		dx, dy := x-t.x, y-t.y
		distance := math.Hypot(dx, dy)
		if distance > float64(t.radius) {
			continue
		}

		if t.kind == hazardTarget {
			// A hazard is destroyed by one shot, but the player loses points.
			g.score -= 30
			if g.score < 0 {
				g.score = 0
			}
			g.targets = append(g.targets[:i], g.targets[i+1:]...)
			g.message = "HAZARD DESTROYED! -30 POINTS"
			g.messageTill = time.Now().Add(time.Second)
			return
		}

		if t.kind == bossTarget {
			// The boss only takes damage when its glowing core is hit.
			if distance > float64(t.radius)*0.32 {
				g.message = "BOSS ARMOR! AIM FOR THE CORE"
				g.messageTill = time.Now().Add(1100 * time.Millisecond)
				return
			}
			g.hits++
			g.save.TotalHits++
			t.hp--
			g.score += 75
			g.message = fmt.Sprintf("CORE HIT! BOSS HP: %d", t.hp)
			g.messageTill = time.Now().Add(time.Second)
			if t.hp <= 0 {
				g.targets = append(g.targets[:i], g.targets[i+1:]...)
				g.score += 750
			}
			return
		}

		g.hits++
		g.save.TotalHits++
		switch t.kind {
		case bonusTarget:
			g.score += 150
		case ammoTarget:
			g.score += 40
			g.ammo += 4
			g.message = "AMMO BONUS! +4 SHOTS"
			g.messageTill = time.Now().Add(time.Second)
		case armoredTarget:
			t.hp--
			if t.hp > 0 {
				g.score += 15
				g.message = "ARMORED TARGET: ONE MORE HIT"
				g.messageTill = time.Now().Add(time.Second)
				return
			}
			g.score += 60
		case fastTarget:
			g.score += 75
		default:
			g.score += 25
		}
		g.targets = append(g.targets[:i], g.targets[i+1:]...)
		return
	}
}

func (g *Game) finishLevel() {
	accuracy := 0
	if g.shots > 0 {
		accuracy = g.hits * 100 / g.shots
	}

	stars := 1
	if accuracy >= 55 && g.timer > 5 {
		stars = 2
	}
	if accuracy >= 80 && g.timer > 10 {
		stars = 3
	}

	if stars > g.save.Stars[g.level] {
		g.save.Stars[g.level] = stars
	}
	if g.score > g.save.BestScores[g.level] {
		g.save.BestScores[g.level] = g.score
	}

	if g.level < totalLevels && g.save.UnlockedLevel < g.level+1 {
		g.save.UnlockedLevel = g.level + 1
	}

	g.saveProgress()
	g.screen = resultScreen
}

func (g *Game) Draw(dst *ebiten.Image) {
	dst.Fill(color.RGBA{13, 18, 31, 255})

	switch g.screen {
	case menuScreen:
		g.drawMenu(dst)
	case worldScreen:
		g.drawWorlds(dst)
	case levelScreen:
		g.drawLevels(dst)
	case playScreen:
		g.drawPlaying(dst)
	case pauseScreen:
		g.drawPlaying(dst)
		g.drawPanel(dst, "PAUSED", "Click RESUME to continue")
		g.drawButton(dst, 400, 355, 200, 52, "RESUME")
		g.drawButton(dst, 400, 420, 200, 52, "MAIN MENU")
	case settingsScreen:
		g.drawSettings(dst)
	case resultScreen:
		g.drawResults(dst)
	case gameOverScreen:
		g.drawPanel(dst, "TIME OR AMMO EXHAUSTED", "Try again and improve your accuracy.")
		g.drawButton(dst, 400, 360, 200, 52, "RETRY")
		g.drawButton(dst, 400, 425, 200, 52, "LEVEL SELECT")
		g.drawButton(dst, 400, 490, 200, 52, "MAIN MENU")
	}

	g.drawCrosshair(dst)
}

func (g *Game) drawMenu(dst *ebiten.Image) {
	g.drawCentered(dst, "HELWAN SHOOTER", 105, color.RGBA{90, 220, 255, 255})
	g.drawCentered(dst, "AIM. REACT. MASTER.", 150, color.White)

	g.drawButton(dst, 400, 220, 200, 48, "PLAY")
	g.drawButton(dst, 400, 280, 200, 48, "LEVEL SELECT")
	g.drawButton(dst, 400, 340, 200, 48, "SETTINGS")

	g.drawCentered(dst,
		fmt.Sprintf("UNLOCKED: %d / %d", g.save.UnlockedLevel, totalLevels),
		530, color.RGBA{190, 205, 220, 255})
	g.drawCentered(dst, "ESC: PAUSE / BACK     LEFT CLICK: SHOOT",
		590, color.RGBA{140, 155, 175, 255})
}

func (g *Game) drawWorlds(dst *ebiten.Image) {
	g.drawCentered(dst, "CHOOSE YOUR WORLD", 70, color.White)

	for i := 0; i < worldCount; i++ {
		y := 135 + i*88
		unlocked := g.save.UnlockedLevel >= i*levelsPerWorld+1

		label := fmt.Sprintf("%d. %s", i+1, worldNames[i])
		col := color.RGBA{85, 220, 255, 255}
		if !unlocked {
			label += " [LOCKED]"
			col = color.RGBA{100, 110, 125, 255}
		}

		g.drawTextAt(dst, label, 170, y, col)
		g.drawTextAt(dst, worldDescriptions[i], 194, y+29,
			color.RGBA{155, 170, 190, 255})
		g.drawButton(dst, 745, y-17, 120, 42, "OPEN")
	}

	g.drawButton(dst, 40, 575, 150, 45, "BACK")
}

func (g *Game) drawLevels(dst *ebiten.Image) {
	g.drawCentered(dst,
		fmt.Sprintf("WORLD %d: %s", g.world, worldNames[g.world-1]),
		75, color.White)

	for i := 0; i < levelsPerWorld; i++ {
		level := (g.world-1)*levelsPerWorld + i + 1
		x := 155 + (i%3)*235
		y := 170 + (i/3)*135

		label := fmt.Sprintf("STAGE %02d", level)
		if level > g.save.UnlockedLevel {
			label += " LOCKED"
		}

		g.drawButton(dst, x, y, 190, 62, label)
		g.drawTextAt(dst,
			fmt.Sprintf("BEST %d   STARS %s",
				g.save.BestScores[level], starString(g.save.Stars[level])),
			x+5, y+78, color.RGBA{235, 205, 90, 255})
	}

	g.drawButton(dst, 40, 575, 150, 45, "WORLDS")
}

func (g *Game) drawPlaying(dst *ebiten.Image) {
	ebitenutil.DrawRect(dst, 0, 105, screenW, 3,
		color.RGBA{50, 70, 95, 255})
	ebitenutil.DrawRect(dst, 0, screenH-35, screenW, 35,
		color.RGBA{25, 32, 49, 255})

	g.drawTextAt(dst,
		fmt.Sprintf("WORLD %d / STAGE %02d", g.world, g.level),
		25, 25, color.White)
	if g.level%levelsPerWorld != 0 {
		g.drawTextAt(dst, fmt.Sprintf("WAVE %d/%d", g.wave, g.wavesTotal), 25, 70, color.White)
	}
	g.drawTextAt(dst, fmt.Sprintf("SCORE %05d", g.score),
		290, 25, color.RGBA{255, 220, 95, 255})
	g.drawTextAt(dst, fmt.Sprintf("AMMO %02d", g.ammo),
		520, 25, color.RGBA{255, 150, 130, 255})
	g.drawTextAt(dst, fmt.Sprintf("TIME %02d", int(math.Ceil(g.timer))),
		720, 25, color.RGBA{120, 230, 255, 255})
	g.drawTextAt(dst, "ESC: PAUSE", 25, 70,
		color.RGBA{150, 170, 190, 255})

	for _, t := range g.targets {
		if !t.visible {
			continue
		}

		x, y := float32(t.x), float32(t.y)

		switch t.kind {
		case normalTarget:
			vector.DrawFilledCircle(dst, x, y, t.radius,
				color.RGBA{210, 220, 230, 255}, true)
			vector.DrawFilledCircle(dst, x, y, t.radius*0.72,
				color.RGBA{70, 155, 220, 255}, true)
			vector.DrawFilledCircle(dst, x, y, t.radius*0.43,
				color.RGBA{235, 80, 90, 255}, true)
			vector.DrawFilledCircle(dst, x, y, t.radius*0.16,
				color.RGBA{255, 235, 150, 255}, true)

		case bonusTarget:
			vector.DrawFilledCircle(dst, x, y, t.radius,
				color.RGBA{255, 205, 55, 255}, true)
			vector.DrawFilledCircle(dst, x, y, t.radius*0.55,
				color.RGBA{255, 245, 170, 255}, true)
			g.drawTextAt(dst, "+", int(x)-3, int(y)-5,
				color.RGBA{120, 75, 0, 255})

		case hazardTarget:
			vector.DrawFilledCircle(dst, x, y, t.radius,
				color.RGBA{220, 55, 75, 255}, true)
			vector.DrawFilledCircle(dst, x, y, t.radius*0.52,
				color.RGBA{80, 20, 35, 255}, true)
			g.drawTextAt(dst, "!", int(x)-3, int(y)-5, color.White)

		case armoredTarget:
			vector.DrawFilledCircle(dst, x, y, t.radius, color.RGBA{220, 135, 55, 255}, true)
			vector.DrawFilledCircle(dst, x, y, t.radius*0.66, color.RGBA{90, 100, 120, 255}, true)
			vector.DrawFilledCircle(dst, x, y, t.radius*0.3, color.RGBA{240, 220, 170, 255}, true)
			g.drawTextAt(dst, fmt.Sprintf("%d", t.hp), int(x)-3, int(y)-5, color.White)

		case fastTarget:
			vector.DrawFilledCircle(dst, x, y, t.radius, color.RGBA{65, 235, 205, 255}, true)
			vector.DrawFilledCircle(dst, x, y, t.radius*0.48, color.RGBA{20, 100, 100, 255}, true)

		case ammoTarget:
			vector.DrawFilledCircle(dst, x, y, t.radius, color.RGBA{80, 175, 255, 255}, true)
			vector.DrawFilledCircle(dst, x, y, t.radius*0.58, color.RGBA{210, 240, 255, 255}, true)
			g.drawTextAt(dst, "A", int(x)-4, int(y)-5, color.RGBA{20, 65, 110, 255})

		case bossTarget:
			vector.DrawFilledCircle(dst, x, y, t.radius,
				color.RGBA{125, 65, 190, 255}, true)
			vector.DrawFilledCircle(dst, x, y, t.radius*0.78,
				color.RGBA{55, 35, 95, 255}, true)
			vector.DrawFilledCircle(dst, x, y, t.radius*0.52,
				color.RGBA{235, 75, 100, 255}, true)
			vector.DrawFilledCircle(dst, x, y, t.radius*0.32,
				color.RGBA{255, 225, 120, 255}, true)
			vector.DrawFilledCircle(dst, x, y, t.radius*0.13,
				color.RGBA{255, 255, 235, 255}, true)
			g.drawTextAt(dst,
				fmt.Sprintf("BOSS HP %d/%d", t.hp, t.maxHP),
				int(x)-45, int(y)+int(t.radius)+20, color.White)
		}
	}

	if time.Now().Before(g.messageTill) {
		g.drawCentered(dst, g.message, 130,
			color.RGBA{255, 235, 150, 255})
	}
}

func (g *Game) drawResults(dst *ebiten.Image) {
	accuracy := 0
	if g.shots > 0 {
		accuracy = g.hits * 100 / g.shots
	}

	g.drawPanel(dst, "STAGE CLEARED!",
		fmt.Sprintf("Score %d   Accuracy %d%%   Stars %s",
			g.score, accuracy, starString(g.save.Stars[g.level])))

	if g.level < totalLevels {
		g.drawButton(dst, 400, 360, 200, 52, "NEXT STAGE")
	} else {
		g.drawButton(dst, 400, 360, 200, 52, "CAMPAIGN COMPLETE")
	}
	g.drawButton(dst, 400, 425, 200, 52, "LEVEL SELECT")
	g.drawButton(dst, 400, 490, 200, 52, "MAIN MENU")
}

func (g *Game) drawSettings(dst *ebiten.Image) {
	g.drawCentered(dst, "SETTINGS & STATISTICS", 105, color.White)

	sound := "SOUND: ON"
	if g.muted {
		sound = "SOUND: OFF"
	}
	g.drawButton(dst, 400, 200, 200, 52, sound)

	g.drawCentered(dst,
		fmt.Sprintf("TOTAL SHOTS: %d", g.save.TotalShots),
		305, color.RGBA{190, 210, 230, 255})
	g.drawCentered(dst,
		fmt.Sprintf("TOTAL HITS: %d", g.save.TotalHits),
		340, color.RGBA{190, 210, 230, 255})

	accuracy := 0
	if g.save.TotalShots > 0 {
		accuracy = g.save.TotalHits * 100 / g.save.TotalShots
	}
	g.drawCentered(dst,
		fmt.Sprintf("OVERALL ACCURACY: %d%%", accuracy),
		375, color.RGBA{190, 210, 230, 255})

	g.drawButton(dst, 400, 460, 200, 52, "BACK")
}

func (g *Game) drawPanel(dst *ebiten.Image, title, subtitle string) {
	ebitenutil.DrawRect(dst, 190, 205, 620, 330,
		color.RGBA{24, 32, 51, 250})
	ebitenutil.DrawRect(dst, 190, 205, 620, 3,
		color.RGBA{70, 205, 255, 255})

	g.drawCentered(dst, title, 275,
		color.RGBA{100, 225, 255, 255})
	g.drawCentered(dst, subtitle, 315, color.White)
}

func (g *Game) drawButton(dst *ebiten.Image, x, y, w, h int, label string) {
	hover := g.cursorX >= x && g.cursorX <= x+w &&
		g.cursorY >= y && g.cursorY <= y+h

	fill := color.RGBA{35, 49, 73, 255}
	if hover {
		fill = color.RGBA{38, 105, 135, 255}
	}

	ebitenutil.DrawRect(dst, float64(x), float64(y),
		float64(w), float64(h), fill)
	ebitenutil.DrawRect(dst, float64(x), float64(y),
		float64(w), 2, color.RGBA{65, 190, 225, 255})

	g.drawTextAt(dst, label, x+12, y+h/2-4, color.White)
}

func (g *Game) drawCentered(dst *ebiten.Image, s string, y int, c color.Color) {
	x := (screenW - len(s)*8) / 2
	if x < 5 {
		x = 5
	}
	g.drawTextAt(dst, s, x, y, c)
}

func (g *Game) drawTextAt(dst *ebiten.Image, s string, x, y int, c color.Color) {
	_ = c
	ebitenutil.DebugPrintAt(dst, s, x, y)
}

func (g *Game) drawCrosshair(dst *ebiten.Image) {
	if g.screen != playScreen {
		return
	}

	x, y := float32(g.cursorX), float32(g.cursorY)
	vector.StrokeLine(dst, x-13, y, x-4, y, 2, color.White, false)
	vector.StrokeLine(dst, x+4, y, x+13, y, 2, color.White, false)
	vector.StrokeLine(dst, x, y-13, x, y-4, 2, color.White, false)
	vector.StrokeLine(dst, x, y+4, x, y+13, 2, color.White, false)
	vector.StrokeCircle(dst, x, y, 17, 1, color.White, false)
}

func starString(n int) string {
	switch n {
	case 1:
		return "*--"
	case 2:
		return "**-"
	case 3:
		return "***"
	default:
		return "---"
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenW, screenH
}
