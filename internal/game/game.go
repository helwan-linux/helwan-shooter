// game.go
package game

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image/color"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	screenW        = 1000
	screenH        = 650
	worldCount     = 10
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
	survivalScreen
	achievementsScreen
	leaderboardScreen
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
	fadingTarget
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

type impactEffect struct {
	x, y     float32
	age      float64
	duration float64
	radius   float32
	hit      bool
}

type particle struct {
	x, y    float32
	vx, vy  float32
	col     color.RGBA
	life    float64
	maxLife float64
	size    float32
}

type saveData struct {
	UnlockedLevel int             `json:"unlocked_level"`
	Stars         map[int]int     `json:"stars"`
	BestScores    map[int]int     `json:"best_scores"`
	TotalShots    int             `json:"total_shots"`
	TotalHits     int             `json:"total_hits"`
	Sound         bool            `json:"sound"`
	Achievements  map[string]bool `json:"achievements"`
}

type LevelConfigJSON struct {
	Level       int      `json:"level"`
	Speed       float64  `json:"speed"`
	Waves       int      `json:"waves"`
	TargetTypes []string `json:"target_types"`
	IsBoss      bool     `json:"is_boss"`
	BossHP      int      `json:"boss_hp"`
}

type WorldConfigJSON struct {
	WorldID        int               `json:"world_id"`
	WorldName      string            `json:"world_name"`
	Description    string            `json:"description"`
	Ammo           int               `json:"ammo"`
	Timer          float64           `json:"timer"`
	BackgroundType string            `json:"background_type"`
	Levels         []LevelConfigJSON `json:"levels"`
}

type LeaderboardEntry struct {
	Name  string `json:"name"`
	Score int    `json:"score"`
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

	score         int
	shots         int
	hits          int
	ammo          int
	combo         int
	maxCombo      int
	survivalMode  bool
	survivalLives int
	dailySeed     int64

	timer           float64
	lastTick        time.Time
	currentWorldCfg WorldConfigJSON
	currentLevelCfg LevelConfigJSON

	targets   []target
	effects   []impactEffect
	particles []particle

	shakeDuration  float64
	shakeIntensity float32

	muted            bool
	cursorX          int
	cursorY          int
	message          string
	messageTill      time.Time
	audioContext     *audio.Context
	musicPlayer      *audio.Player
	localLeaderboard []LeaderboardEntry
}

var worldNames = []string{
	"Desert Training",
	"Neon City",
	"Alien Jungle",
	"Frozen Base",
	"Volcano Fortress",
	"Quantum Lab",
	"Cyber Factory",
	"Deep Space",
	"Quantum Storm",
	"The Matrix",
}

var worldDescriptions = []string{
	"Warm-up range with infinite ammunition.",
	"Neon skyline and faster targets.",
	"Alien atmosphere and fading targets.",
	"Frozen battlefield with limited ammunition.",
	"Volcanic arena and the final challenge.",
	"Subatomic instability and fast anomalies.",
	"Heavy industrial automated machinery.",
	"Zero-g orbital station and drifting hazards.",
	"Electrical discharge and rapid energy shifts.",
	"The ultimate transcendent core challenge.",
}

func New() *Game {
	ebiten.SetTPS(60)
	ebiten.SetVsyncEnabled(true)

	g := &Game{
		screen: menuScreen,
		save: saveData{
			UnlockedLevel: 1,
			Stars:         make(map[int]int),
			BestScores:    make(map[int]int),
			Sound:         true,
			Achievements:  make(map[string]bool),
		},
		localLeaderboard: []LeaderboardEntry{
			{"Saeed (Helwan)", 12500},
			{"LinuxDev", 9800},
			{"ArchSniper", 8400},
			{"CyberZero", 6100},
			{"NoviceGun", 3200},
		},
	}

	g.initAudio()
	g.loadProgress()
	g.initDailySeed()
	return g
}

func (g *Game) initDailySeed() {
	y, m, d := time.Now().Date()
	g.dailySeed = int64(y)*10000 + int64(m)*100 + int64(d)
}

func (g *Game) loadWorldConfig(worldNum int) {
	if worldNum < 1 || worldNum > worldCount {
		worldNum = 1
	}

	filename := fmt.Sprintf("world_%d.json", worldNum)
	candidates := []string{
		filepath.Join("worlds", filename),
		filepath.Join("..", "..", "worlds", filename),
	}

	// Also search relative to the executable so launching from a desktop
	// shortcut does not depend on the current working directory.
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "worlds", filename),
			filepath.Join(exeDir, "..", "share", "helwan-shooter", "worlds", filename),
			filepath.Join(exeDir, "..", "..", "share", "helwan-shooter", "worlds", filename),
		)
	}

	loaded := false
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var cfg WorldConfigJSON
		if err := json.Unmarshal(data, &cfg); err != nil {
			continue
		}
		if cfg.WorldID != 0 && cfg.WorldID != worldNum {
			continue
		}
		if cfg.Timer <= 0 || len(cfg.Levels) == 0 {
			continue
		}

		g.currentWorldCfg = cfg
		loaded = true
		break
	}

	if loaded {
		return
	}

	// Safe fallback: the game remains playable even when world JSON files
	// are missing or cannot be read from the installation directory.
	g.currentWorldCfg = WorldConfigJSON{
		WorldID:        worldNum,
		WorldName:      worldNames[worldNum-1],
		Description:    worldDescriptions[worldNum-1],
		Ammo:           30,
		Timer:          45.0,
		BackgroundType: "default",
		Levels: []LevelConfigJSON{
			{Level: (worldNum-1)*levelsPerWorld + 1, Speed: 2.5, Waves: 3, TargetTypes: []string{"normal"}},
			{Level: (worldNum-1)*levelsPerWorld + 2, Speed: 3.0, Waves: 3, TargetTypes: []string{"normal", "fast"}},
			{Level: (worldNum-1)*levelsPerWorld + 3, Speed: 3.5, Waves: 4, TargetTypes: []string{"normal", "armored"}},
			{Level: (worldNum-1)*levelsPerWorld + 4, Speed: 4.0, Waves: 4, TargetTypes: []string{"bonus", "fast"}},
			{Level: (worldNum-1)*levelsPerWorld + 5, Speed: 4.5, Waves: 1, TargetTypes: []string{"boss"}, IsBoss: true, BossHP: 10},
		},
	}
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
	if saved.Achievements == nil {
		saved.Achievements = make(map[string]bool)
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

func (g *Game) triggerScreenShake(duration float64, intensity float32) {
	g.shakeDuration = duration
	g.shakeIntensity = intensity
}

func (g *Game) spawnParticles(x, y float32, col color.RGBA, count int) {
	for i := 0; i < count; i++ {
		angle := rand.Float64() * math.Pi * 2
		speed := 1.0 + rand.Float64()*4.0
		life := 0.2 + rand.Float64()*0.3
		g.particles = append(g.particles, particle{
			x:       x,
			y:       y,
			vx:      float32(math.Cos(angle) * speed),
			vy:      float32(math.Sin(angle) * speed),
			col:     col,
			life:    life,
			maxLife: life,
			size:    2.0 + rand.Float32()*3.0,
		})
	}
}

func (g *Game) checkAchievements() {
	if g.score >= 1000 && !g.save.Achievements["score_1k"] {
		g.save.Achievements["score_1k"] = true
		g.triggerMessage("ACHIEVEMENT: HIGH ROLLER (1000+ PTS)")
	}
	if g.maxCombo >= 5 && !g.save.Achievements["combo_5"] {
		g.save.Achievements["combo_5"] = true
		g.triggerMessage("ACHIEVEMENT: COMBO MASTER (5x COMBO)")
	}
	if g.level == 50 && !g.save.Achievements["matrix_beat"] {
		g.save.Achievements["matrix_beat"] = true
		g.triggerMessage("ACHIEVEMENT: TRANSCENDENT (BEAT MATRIX)")
	}
	g.saveProgress()
}

func (g *Game) triggerMessage(text string) {
	g.message = text
	g.messageTill = time.Now().Add(1800 * time.Millisecond)
}

func (g *Game) startLevel(level int) {
	if level < 1 || level > totalLevels {
		return
	}

	g.survivalMode = false
	g.level = level
	g.world = (level-1)/levelsPerWorld + 1
	g.loadWorldConfig(g.world)

	idx := (level - 1) % levelsPerWorld
	if idx < len(g.currentWorldCfg.Levels) {
		g.currentLevelCfg = g.currentWorldCfg.Levels[idx]
	} else {
		g.currentLevelCfg = LevelConfigJSON{
			Level:       level,
			Speed:       3.0,
			Waves:       3,
			TargetTypes: []string{"normal"},
		}
	}

	g.resetMatchState()
	g.message = fmt.Sprintf("WORLD %d - STAGE %d", g.world, (level-1)%levelsPerWorld+1)
	g.messageTill = time.Now().Add(2 * time.Second)

	g.createTargets()
	g.screen = playScreen
}

func (g *Game) startSurvivalMode() {
	g.survivalMode = true
	g.survivalLives = 3
	g.level = 1
	g.world = 1
	g.loadWorldConfig(1)
	g.currentLevelCfg = LevelConfigJSON{
		Level:       99,
		Speed:       4.5,
		Waves:       999,
		TargetTypes: []string{"normal", "fast", "fading", "armored", "hazard"},
		IsBoss:      false,
	}

	g.resetMatchState()
	g.timer = 60.0
	g.ammo = 999
	g.triggerMessage("SURVIVAL MODE: 3 LIVES - SURVIVE!")
	g.createTargets()
	g.screen = playScreen
}

func (g *Game) resetMatchState() {
	g.score = 0
	g.shots = 0
	g.hits = 0
	g.combo = 0
	g.maxCombo = 0
	g.ammo = g.currentWorldCfg.Ammo
	if g.ammo <= 0 {
		g.ammo = 30
	}

	g.timer = g.currentWorldCfg.Timer
	if g.timer <= 0 {
		g.timer = 45.0
	}

	if g.survivalMode {
		g.ammo = 999
		g.timer = 60.0
	}

	g.lastTick = time.Now()
	g.targets = nil
	g.effects = nil
	g.particles = nil
}

func (g *Game) createTargets() {
	g.targets = nil
	g.effects = nil
	g.wave = 1
	g.wavesTotal = g.currentLevelCfg.Waves

	if g.currentLevelCfg.IsBoss {
		g.wavesTotal = 1
		hp := g.currentLevelCfg.BossHP
		if hp <= 0 {
			hp = 5
		}
		g.targets = append(g.targets, target{
			x:       screenW / 2,
			y:       330,
			vx:      1.2 + float64(g.world)*0.25,
			vy:      0.8 + float64(g.world)*0.15,
			radius:  80,
			kind:    bossTarget,
			hp:      hp,
			maxHP:   hp,
			visible: true,
		})
		return
	}

	g.spawnTargetWave()
}

func (g *Game) spawnTargetWave() {
	g.targets = nil
	count := 3 + g.world/2 + g.wave - 1
	speed := g.currentLevelCfg.Speed

	types := g.currentLevelCfg.TargetTypes
	if len(types) == 0 {
		types = []string{"normal"}
	}

	for i := 0; i < count; i++ {
		radius := float32(34 - g.world)
		if radius < 22 {
			radius = 22
		}

		tKind := normalTarget
		chosenTypeStr := types[i%len(types)]

		switch chosenTypeStr {
		case "bonus":
			tKind = bonusTarget
			radius = 23
		case "hazard":
			tKind = hazardTarget
			radius = 25
		case "armored":
			tKind = armoredTarget
			radius = 27
		case "fast":
			tKind = fastTarget
			radius = 20
		case "ammo":
			tKind = ammoTarget
			radius = 22
		case "fading":
			tKind = fadingTarget
			radius = 24
		}

		vx := (rand.Float64()*2 - 1) * speed
		vy := (rand.Float64()*2 - 1) * speed * 0.8
		hp := 1
		if tKind == armoredTarget {
			hp = 2
		}
		if tKind == fastTarget {
			vx *= 1.35
			vy *= 1.35
		}

		t := target{
			x:       float64(100 + rand.Intn(screenW-200)),
			y:       float64(150 + rand.Intn(screenH-235)),
			vx:      vx,
			vy:      vy,
			radius:  radius,
			kind:    tKind,
			hp:      hp,
			maxHP:   hp,
			visible: true,
			phase:   rand.Float64() * math.Pi * 2,
		}

		g.targets = append(g.targets, t)
	}
}

func (g *Game) Update() error {
	g.syncMusic()

	if g.shakeDuration > 0 {
		g.shakeDuration -= 1.0 / 60.0
		if g.shakeDuration < 0 {
			g.shakeDuration = 0
		}
	}

	for i := 0; i < len(g.particles); {
		p := &g.particles[i]
		p.x += p.vx
		p.y += p.vy
		p.life -= 1.0 / 60.0
		if p.life <= 0 {
			g.particles = append(g.particles[:i], g.particles[i+1:]...)
			continue
		}
		i++
	}

	for i := 0; i < len(g.effects); {
		g.effects[i].age += 1.0 / 60.0
		if g.effects[i].age >= g.effects[i].duration {
			g.effects = append(g.effects[:i], g.effects[i+1:]...)
			continue
		}
		i++
	}

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
	case achievementsScreen:
		g.updateAchievements()
	case leaderboardScreen:
		g.updateLeaderboard()
	}

	return nil
}

func (g *Game) buttonClicked(x, y, w, h int) bool {
	return g.clickPressed &&
		g.clickX >= x && g.clickX <= x+w &&
		g.clickY >= y && g.clickY <= y+h
}

func (g *Game) updateMenu() {
	if g.buttonClicked(400, 190, 200, 45) {
		g.startLevel(g.save.UnlockedLevel)
	} else if g.buttonClicked(400, 245, 200, 45) {
		g.screen = worldScreen
	} else if g.buttonClicked(400, 300, 200, 45) {
		g.startSurvivalMode()
	} else if g.buttonClicked(400, 355, 200, 45) {
		g.screen = leaderboardScreen
	} else if g.buttonClicked(400, 410, 200, 45) {
		g.screen = achievementsScreen
	} else if g.buttonClicked(400, 465, 200, 45) {
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

	if g.timer <= 0 || (!g.survivalMode && g.world > 1 && g.ammo <= 0 && len(g.targets) > 0) || (g.survivalMode && g.survivalLives <= 0) {
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

		if t.kind == fadingTarget || (g.world == 3 && t.kind != bossTarget) {
			t.visible = math.Sin(t.phase*1.7) > -0.25
		} else if g.world == 5 && t.kind != bossTarget {
			t.visible = math.Sin(t.phase*2.3) > -0.7
		} else {
			t.visible = true
		}
	}

	if len(g.targets) == 0 {
		if g.survivalMode {
			g.score += 500
			g.spawnTargetWave()
			g.triggerMessage("WAVE CLEARED! +500 PTS")
		} else if g.wave < g.wavesTotal {
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
		if g.survivalMode {
			g.startSurvivalMode()
		} else {
			g.startLevel(g.level)
		}
	} else if g.buttonClicked(400, 425, 200, 52) {
		g.screen = menuScreen
	}
}

func (g *Game) updateAchievements() {
	if g.buttonClicked(400, 510, 200, 45) {
		g.screen = menuScreen
	}
}

func (g *Game) updateLeaderboard() {
	if g.buttonClicked(400, 510, 200, 45) {
		g.screen = menuScreen
	}
}

func (g *Game) shoot(x, y float64) {
	if !g.survivalMode && g.world > 1 && g.ammo <= 0 {
		return
	}

	if !g.survivalMode && g.world > 1 {
		g.ammo--
	}

	g.shots++
	g.save.TotalShots++
	g.soundShoot()

	g.effects = append(g.effects, impactEffect{
		x: float32(x), y: float32(y), duration: 0.12,
		radius: 12, hit: false,
	})

	hitSomething := false

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

		hitSomething = true
		g.triggerScreenShake(0.18, 4.0)

		g.effects = append(g.effects, impactEffect{
			x: float32(t.x), y: float32(t.y), duration: 0.28,
			radius: t.radius + 8, hit: true,
		})

		g.spawnParticles(float32(t.x), float32(t.y), color.RGBA{255, 200, 100, 255}, 12)

		if t.kind == hazardTarget {
			g.combo = 0
			g.score -= 30
			if g.score < 0 {
				g.score = 0
			}
			if g.survivalMode {
				g.survivalLives--
			}

			g.soundHazard()
			g.targets = append(g.targets[:i], g.targets[i+1:]...)
			g.triggerMessage("HAZARD! COMBO LOST (-30)")
			return
		}

		if t.kind == bossTarget {
			if distance > float64(t.radius)*0.32 {
				g.triggerMessage("BOSS ARMOR! AIM FOR CORE")
				return
			}

			g.combo++
			if g.combo > g.maxCombo {
				g.maxCombo = g.combo
			}

			g.hits++
			g.save.TotalHits++
			t.hp--
			pts := 75 * (1 + g.combo/5)
			g.score += pts
			g.soundHit()
			g.triggerMessage(fmt.Sprintf("CORE HIT! +%d (COMBO %dx)", pts, g.combo))

			if t.hp <= 0 {
				g.targets = append(g.targets[:i], g.targets[i+1:]...)
				g.score += 750
				g.soundWin()
				g.triggerMessage("BOSS DEFEATED! +750 PTS")
			}
			g.checkAchievements()
			return
		}

		g.combo++
		if g.combo > g.maxCombo {
			g.maxCombo = g.combo
		}

		g.hits++
		g.save.TotalHits++
		g.soundHit()

		basePts := 25
		switch t.kind {
		case bonusTarget:
			basePts = 150
		case ammoTarget:
			basePts = 40
			g.ammo += 4
			g.triggerMessage("AMMO BONUS! +4 SHOTS")
		case armoredTarget:
			t.hp--
			if t.hp > 0 {
				g.score += 15 * (1 + g.combo/5)
				g.triggerMessage("ARMOR DAMAGED!")
				return
			}
			basePts = 60
		case fastTarget:
			basePts = 75
		case fadingTarget:
			basePts = 50
		}

		earned := basePts * (1 + g.combo/4)
		g.score += earned
		g.targets = append(g.targets[:i], g.targets[i+1:]...)
		g.checkAchievements()
		return
	}

	if !hitSomething {
		g.combo = 0
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

	g.soundWin()
	g.checkAchievements()
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
		g.drawPanel(dst, "GAME OVER", "Try again and improve your accuracy.")
		g.drawButton(dst, 400, 360, 200, 52, "RETRY")
		g.drawButton(dst, 400, 425, 200, 52, "MAIN MENU")
	case achievementsScreen:
		g.drawAchievements(dst)
	case leaderboardScreen:
		g.drawLeaderboard(dst)
	}

	g.drawCrosshair(dst)
}

func (g *Game) drawMenu(dst *ebiten.Image) {
	g.drawCentered(dst, "HELWAN SHOOTER", 95, color.RGBA{90, 220, 255, 255})
	g.drawCentered(dst, "COMPETITIVE EDITION", 135, color.White)

	g.drawButton(dst, 400, 190, 200, 45, "PLAY CAMPAIGN")
	g.drawButton(dst, 400, 245, 200, 45, "LEVEL SELECT")
	g.drawButton(dst, 400, 300, 200, 45, "SURVIVAL MODE")
	g.drawButton(dst, 400, 355, 200, 45, "LEADERBOARD")
	g.drawButton(dst, 400, 410, 200, 45, "ACHIEVEMENTS")
	g.drawButton(dst, 400, 465, 200, 45, "SETTINGS")

	g.drawCentered(dst, fmt.Sprintf("DAILY SEED HASH: %s", g.getDailyHash()), 545, color.RGBA{140, 200, 160, 255})
	g.drawCentered(dst, "ESC: PAUSE / BACK     LEFT CLICK: SHOOT", 590, color.RGBA{140, 155, 175, 255})
}

func (g *Game) getDailyHash() string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%d", g.dailySeed)))
	return hex.EncodeToString(h[:4])
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
		g.drawTextAt(dst, worldDescriptions[i], 194, y+29, color.RGBA{155, 170, 190, 255})
		g.drawButton(dst, 745, y-17, 120, 42, "OPEN")
	}

	g.drawButton(dst, 40, 575, 150, 45, "BACK")
}

func (g *Game) drawLevels(dst *ebiten.Image) {
	g.drawCentered(dst, fmt.Sprintf("WORLD %d: %s", g.world, worldNames[g.world-1]), 75, color.White)

	for i := 0; i < levelsPerWorld; i++ {
		level := (g.world-1)*levelsPerWorld + i + 1
		x := 155 + (i%3)*235
		y := 170 + (i/3)*135

		label := fmt.Sprintf("STAGE %02d", level)
		if level > g.save.UnlockedLevel {
			label += " LOCKED"
		}

		g.drawButton(dst, x, y, 190, 62, label)
		g.drawTextAt(dst, fmt.Sprintf("BEST %d   STARS %s", g.save.BestScores[level], starString(g.save.Stars[level])), x+5, y+78, color.RGBA{235, 205, 90, 255})
	}

	g.drawButton(dst, 40, 575, 150, 45, "WORLDS")
}

func (g *Game) drawPlaying(dst *ebiten.Image) {
	g.drawWorldBackground(dst)

	ebitenutil.DrawRect(dst, 0, 105, screenW, 3, color.RGBA{50, 70, 95, 255})
	ebitenutil.DrawRect(dst, 0, screenH-35, screenW, 35, color.RGBA{25, 32, 49, 255})

	if g.survivalMode {
		g.drawTextAt(dst, fmt.Sprintf("SURVIVAL MODE | LIVES: %d", g.survivalLives), 25, 25, color.RGBA{255, 100, 100, 255})
	} else {
		g.drawTextAt(dst, fmt.Sprintf("WORLD %d / STAGE %02d", g.world, g.level), 25, 25, color.White)
		g.drawTextAt(dst, fmt.Sprintf("WAVE %d/%d", g.wave, g.wavesTotal), 25, 70, color.White)
	}

	g.drawTextAt(dst, fmt.Sprintf("SCORE %05d", g.score), 290, 25, color.RGBA{255, 220, 95, 255})
	g.drawTextAt(dst, fmt.Sprintf("COMBO %dx", g.combo), 290, 70, color.RGBA{255, 140, 255, 255})

	ammoLabel := fmt.Sprintf("AMMO %02d", g.ammo)
	if g.world == 1 || g.survivalMode {
		ammoLabel = "AMMO INFINITE"
	}

	g.drawTextAt(dst, ammoLabel, 520, 25, color.RGBA{255, 150, 130, 255})
	g.drawTextAt(dst, fmt.Sprintf("TIME %02d", int(math.Ceil(g.timer))), 720, 25, color.RGBA{120, 230, 255, 255})

	for _, t := range g.targets {
		if !t.visible {
			continue
		}

		x, y := float32(t.x), float32(t.y)

		switch t.kind {
		case normalTarget:
			vector.DrawFilledCircle(dst, x, y, t.radius, color.RGBA{210, 220, 230, 255}, true)
			vector.DrawFilledCircle(dst, x, y, t.radius*0.72, color.RGBA{70, 155, 220, 255}, true)
			vector.DrawFilledCircle(dst, x, y, t.radius*0.43, color.RGBA{235, 80, 90, 255}, true)
			vector.DrawFilledCircle(dst, x, y, t.radius*0.16, color.RGBA{255, 235, 150, 255}, true)
		case bonusTarget:
			vector.DrawFilledCircle(dst, x, y, t.radius, color.RGBA{255, 205, 55, 255}, true)
			vector.DrawFilledCircle(dst, x, y, t.radius*0.55, color.RGBA{255, 245, 170, 255}, true)
			g.drawTextAt(dst, "+", int(x)-3, int(y)-5, color.RGBA{120, 75, 0, 255})
		case hazardTarget:
			vector.DrawFilledCircle(dst, x, y, t.radius, color.RGBA{220, 55, 75, 255}, true)
			vector.DrawFilledCircle(dst, x, y, t.radius*0.52, color.RGBA{80, 20, 35, 255}, true)
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
		case fadingTarget:
			vector.DrawFilledCircle(dst, x, y, t.radius, color.RGBA{150, 80, 220, 255}, true)
			vector.DrawFilledCircle(dst, x, y, t.radius*0.5, color.RGBA{210, 170, 255, 255}, true)
			g.drawTextAt(dst, "F", int(x)-4, int(y)-5, color.White)
		case bossTarget:
			vector.DrawFilledCircle(dst, x, y, t.radius, color.RGBA{125, 65, 190, 255}, true)
			vector.DrawFilledCircle(dst, x, y, t.radius*0.78, color.RGBA{55, 35, 95, 255}, true)
			vector.DrawFilledCircle(dst, x, y, t.radius*0.52, color.RGBA{235, 75, 100, 255}, true)
			vector.DrawFilledCircle(dst, x, y, t.radius*0.32, color.RGBA{255, 225, 120, 255}, true)
			vector.DrawFilledCircle(dst, x, y, t.radius*0.13, color.RGBA{255, 255, 235, 255}, true)
			g.drawTextAt(dst, fmt.Sprintf("BOSS HP %d/%d", t.hp, t.maxHP), int(x)-45, int(y)+int(t.radius)+20, color.White)
		}
	}

	for _, p := range g.particles {
		vector.DrawFilledCircle(dst, p.x, p.y, p.size, p.col, true)
	}

	for _, e := range g.effects {
		progress := float32(e.age / e.duration)
		radius := e.radius * progress
		effectColor := color.RGBA{255, 235, 150, 255}
		if e.hit {
			effectColor = color.RGBA{255, 105, 75, 255}
		}
		vector.StrokeCircle(dst, e.x, e.y, radius, 2, effectColor, false)
	}

	if time.Now().Before(g.messageTill) {
		g.drawCentered(dst, g.message, 130, color.RGBA{255, 235, 150, 255})
	}
}

func (g *Game) drawWorldBackground(dst *ebiten.Image) {
	var sky, far, ground, accent color.RGBA

	switch g.world {
	case 1:
		sky = color.RGBA{44, 34, 43, 255}
		far = color.RGBA{112, 75, 65, 255}
		ground = color.RGBA{165, 112, 65, 255}
		accent = color.RGBA{255, 194, 105, 255}
	case 2:
		sky = color.RGBA{12, 18, 48, 255}
		far = color.RGBA{27, 43, 82, 255}
		ground = color.RGBA{17, 24, 46, 255}
		accent = color.RGBA{65, 235, 230, 255}
	case 3:
		sky = color.RGBA{10, 39, 43, 255}
		far = color.RGBA{22, 86, 67, 255}
		ground = color.RGBA{12, 48, 42, 255}
		accent = color.RGBA{115, 255, 125, 255}
	case 4:
		sky = color.RGBA{28, 53, 82, 255}
		far = color.RGBA{80, 132, 163, 255}
		ground = color.RGBA{37, 73, 102, 255}
		accent = color.RGBA{185, 240, 255, 255}
	case 5:
		sky = color.RGBA{47, 15, 25, 255}
		far = color.RGBA{101, 33, 35, 255}
		ground = color.RGBA{45, 22, 30, 255}
		accent = color.RGBA{255, 100, 35, 255}
	case 6:
		sky = color.RGBA{18, 12, 38, 255}
		far = color.RGBA{60, 35, 100, 255}
		ground = color.RGBA{25, 15, 50, 255}
		accent = color.RGBA{180, 100, 255, 255}
	case 7:
		sky = color.RGBA{30, 30, 35, 255}
		far = color.RGBA{70, 75, 85, 255}
		ground = color.RGBA{40, 42, 50, 255}
		accent = color.RGBA{255, 140, 50, 255}
	case 8:
		sky = color.RGBA{5, 8, 20, 255}
		far = color.RGBA{20, 30, 60, 255}
		ground = color.RGBA{10, 15, 30, 255}
		accent = color.RGBA{100, 220, 255, 255}
	case 9:
		sky = color.RGBA{25, 10, 35, 255}
		far = color.RGBA{65, 25, 85, 255}
		ground = color.RGBA{30, 12, 45, 255}
		accent = color.RGBA{255, 230, 80, 255}
	default:
		sky = color.RGBA{5, 20, 10, 255}
		far = color.RGBA{10, 70, 30, 255}
		ground = color.RGBA{8, 35, 15, 255}
		accent = color.RGBA{50, 255, 100, 255}
	}

	dst.Fill(sky)
	vector.DrawFilledCircle(dst, 820, 180, 82, color.RGBA{accent.R, accent.G, accent.B, 55}, true)
	ebitenutil.DrawRect(dst, 0, 440, screenW, 175, ground)

	switch g.world {
	case 1:
		for i := 0; i < 7; i++ {
			x := float64(i*170 - 40)
			vector.DrawFilledCircle(dst, float32(x+55), 435, 95, far, true)
		}
		for i := 0; i < 4; i++ {
			ebitenutil.DrawRect(dst, float64(80+i*250), 385, 35, 75, far)
		}
	case 2:
		for i := 0; i < 12; i++ {
			h := float64(65 + (i*37)%150)
			x := float64(i * 88)
			ebitenutil.DrawRect(dst, x, 440-h, 58, h, far)
			for wy := 455 - h; wy < 430; wy += 22 {
				ebitenutil.DrawRect(dst, x+12, wy, 7, 5, accent)
				ebitenutil.DrawRect(dst, x+34, wy, 7, 5, accent)
			}
		}
	case 3:
		for i := 0; i < 9; i++ {
			x := float32(i*125 + 25)
			vector.DrawFilledCircle(dst, x, 420, 52, far, true)
			ebitenutil.DrawRect(dst, float64(x)-9, 410, 18, 70, far)
			vector.DrawFilledCircle(dst, x+26, 385, 23, accent, true)
		}
	case 4:
		for i := 0; i < 8; i++ {
			x := float64(i*145 - 20)
			vector.DrawFilledCircle(dst, float32(x+65), 438, 85, far, true)
		}
		for i := 0; i < 7; i++ {
			ebitenutil.DrawRect(dst, float64(i*155+15), 455, 95, 4, accent)
		}
	case 5, 9:
		for i := 0; i < 8; i++ {
			x := float64(i*145 - 25)
			vector.DrawFilledCircle(dst, float32(x+70), 440, 88, far, true)
		}
		for i := 0; i < 6; i++ {
			ebitenutil.DrawRect(dst, float64(i*180+20), 472+float64(i%2*10), 95, 5, accent)
		}
	case 6, 8:
		for i := 0; i < 6; i++ {
			x := float64(i*180 + 50)
			ebitenutil.DrawRect(dst, x, 360, 40, 80, far)
		}
	case 7:
		for i := 0; i < 10; i++ {
			x := float64(i * 100)
			ebitenutil.DrawRect(dst, x, 340, 60, 100, far)
			ebitenutil.DrawRect(dst, x+20, 320, 20, 30, accent)
		}
	default:
		for i := 0; i < 15; i++ {
			x := float64(i * 70)
			ebitenutil.DrawRect(dst, x, 350, 30, 90, far)
			ebitenutil.DrawRect(dst, x+10, 370, 10, 10, accent)
		}
	}

	for y := 470; y < screenH-35; y += 34 {
		ebitenutil.DrawRect(dst, 0, float64(y), screenW, 1, color.RGBA{accent.R, accent.G, accent.B, 45})
	}
}

func (g *Game) drawResults(dst *ebiten.Image) {
	accuracy := 0
	if g.shots > 0 {
		accuracy = g.hits * 100 / g.shots
	}

	g.drawPanel(dst, "STAGE CLEARED!", fmt.Sprintf("Score %d   Accuracy %d%%   Stars %s", g.score, accuracy, starString(g.save.Stars[g.level])))

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

	g.drawCentered(dst, fmt.Sprintf("TOTAL SHOTS: %d", g.save.TotalShots), 305, color.RGBA{190, 210, 230, 255})
	g.drawCentered(dst, fmt.Sprintf("TOTAL HITS: %d", g.save.TotalHits), 340, color.RGBA{190, 210, 230, 255})

	accuracy := 0
	if g.save.TotalShots > 0 {
		accuracy = g.save.TotalHits * 100 / g.save.TotalShots
	}

	g.drawCentered(dst, fmt.Sprintf("OVERALL ACCURACY: %d%%", accuracy), 375, color.RGBA{190, 210, 230, 255})
	g.drawButton(dst, 400, 460, 200, 52, "BACK")
}

func (g *Game) drawAchievements(dst *ebiten.Image) {
	g.drawCentered(dst, "ACHIEVEMENTS", 70, color.White)
	g.drawCentered(dst, fmt.Sprintf("High Roller (1000+ pts): %v", g.save.Achievements["score_1k"]), 160, color.RGBA{200, 220, 255, 255})
	g.drawCentered(dst, fmt.Sprintf("Combo Master (5x combo): %v", g.save.Achievements["combo_5"]), 210, color.RGBA{200, 220, 255, 255})
	g.drawCentered(dst, fmt.Sprintf("Transcendent (Beat Matrix): %v", g.save.Achievements["matrix_beat"]), 260, color.RGBA{200, 220, 255, 255})
	g.drawButton(dst, 400, 510, 200, 45, "BACK")
}

func (g *Game) drawLeaderboard(dst *ebiten.Image) {
	g.drawCentered(dst, "LOCAL LEADERBOARD", 70, color.White)
	for i, entry := range g.localLeaderboard {
		text := fmt.Sprintf("%d. %-15s %d PTS", i+1, entry.Name, entry.Score)
		g.drawCentered(dst, text, 140+i*45, color.RGBA{235, 220, 120, 255})
	}
	g.drawButton(dst, 400, 510, 200, 45, "BACK")
}

func (g *Game) drawPanel(dst *ebiten.Image, title, subtitle string) {
	ebitenutil.DrawRect(dst, 190, 205, 620, 330, color.RGBA{24, 32, 51, 250})
	ebitenutil.DrawRect(dst, 190, 205, 620, 3, color.RGBA{70, 205, 255, 255})
	g.drawCentered(dst, title, 275, color.RGBA{100, 225, 255, 255})
	g.drawCentered(dst, subtitle, 315, color.White)
}

func (g *Game) drawButton(dst *ebiten.Image, x, y, w, h int, label string) {
	hover := g.cursorX >= x && g.cursorX <= x+w && g.cursorY >= y && g.cursorY <= y+h
	fill := color.RGBA{35, 49, 73, 255}
	if hover {
		fill = color.RGBA{38, 105, 135, 255}
	}
	ebitenutil.DrawRect(dst, float64(x), float64(y), float64(w), float64(h), fill)
	ebitenutil.DrawRect(dst, float64(x), float64(y), float64(w), 2, color.RGBA{65, 190, 225, 255})
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
