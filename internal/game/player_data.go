package game

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const maxLeaderboardEntries = 10

// PlayerData stores the selected player, every player's campaign save,
// the selected player's achievements, and the shared local leaderboard.
type PlayerData struct {
	PlayerName   string              `json:"player_name"`
	Achievements map[string]bool     `json:"achievements"`
	Leaderboard  []LeaderboardEntry  `json:"leaderboard"`
	Profiles     map[string]saveData `json:"profiles"`
}

func PlayerDataPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "HelwanShooter", "player_data.json"), nil
}

func defaultPlayerData() PlayerData {
	return PlayerData{
		Achievements: make(map[string]bool),
		Leaderboard:  make([]LeaderboardEntry, 0),
		Profiles:     make(map[string]saveData),
	}
}

func LoadPlayerData() (PlayerData, error) {
	data := defaultPlayerData()
	path, err := PlayerDataPath()
	if err != nil {
		return data, err
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return data, nil
		}
		return data, err
	}
	if err := json.Unmarshal(contents, &data); err != nil {
		return defaultPlayerData(), err
	}
	if data.Achievements == nil {
		data.Achievements = make(map[string]bool)
	}
	if data.Leaderboard == nil {
		data.Leaderboard = make([]LeaderboardEntry, 0)
	}
	if data.Profiles == nil {
		data.Profiles = make(map[string]saveData)
	}
	return data, nil
}

func SavePlayerData(data PlayerData) error {
	path, err := PlayerDataPath()
	if err != nil {
		return err
	}
	if data.Achievements == nil {
		data.Achievements = make(map[string]bool)
	}
	if data.Leaderboard == nil {
		data.Leaderboard = make([]LeaderboardEntry, 0)
	}
	if data.Profiles == nil {
		data.Profiles = make(map[string]saveData)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	contents, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, contents, 0644); err != nil {
		return err
	}
	// Removing the old file first also permits replacing it on Windows.
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(tempPath)
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	return nil
}

func normalizePlayerName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Player"
	}
	if utf8.RuneCountInString(name) > 20 {
		runes := []rune(name)
		name = string(runes[:20])
	}
	return name
}

func playerProfileKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func SetPlayerName(name string) error {
	data, err := LoadPlayerData()
	if err != nil {
		return err
	}
	data.PlayerName = normalizePlayerName(name)
	return SavePlayerData(data)
}

func UnlockAchievement(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	data, err := LoadPlayerData()
	if err != nil {
		return err
	}
	data.Achievements[id] = true
	if data.PlayerName != "" {
		key := playerProfileKey(data.PlayerName)
		profile := data.Profiles[key]
		if profile.Achievements == nil {
			profile.Achievements = make(map[string]bool)
		}
		profile.Achievements[id] = true
		data.Profiles[key] = profile
	}
	return SavePlayerData(data)
}

func IsAchievementUnlocked(id string) (bool, error) {
	data, err := LoadPlayerData()
	if err != nil {
		return false, err
	}
	return data.Achievements[strings.TrimSpace(id)], nil
}

// RecordPlayerScore keeps the best score for each player and sorts the shared
// leaderboard from highest to lowest, retaining at most ten players.
func RecordPlayerScore(name string, score int) error {
	name = normalizePlayerName(name)
	if score < 0 {
		score = 0
	}
	data, err := LoadPlayerData()
	if err != nil {
		return err
	}

	found := false
	for i := range data.Leaderboard {
		if strings.EqualFold(strings.TrimSpace(data.Leaderboard[i].Name), name) {
			found = true
			if score > data.Leaderboard[i].Score {
				data.Leaderboard[i].Score = score
			}
			break
		}
	}
	if !found {
		data.Leaderboard = append(data.Leaderboard, LeaderboardEntry{Name: name, Score: score})
	}
	sort.SliceStable(data.Leaderboard, func(i, j int) bool {
		return data.Leaderboard[i].Score > data.Leaderboard[j].Score
	})
	if len(data.Leaderboard) > maxLeaderboardEntries {
		data.Leaderboard = data.Leaderboard[:maxLeaderboardEntries]
	}
	return SavePlayerData(data)
}
