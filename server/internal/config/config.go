package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Server ServerConfig
	World  WorldConfig
	Player PlayerConfig
	NPC    NPCConfig
	AOI    AOIConfig
	Sync   SyncConfig
}

type ServerConfig struct {
	ListenAddr string
	TickRate   int
}

type WorldConfig struct {
	Width    float64
	Height   float64
	NPCCount int
}

type PlayerConfig struct {
	Speed     float64
	AOIRadius float64
}

type NPCConfig struct {
	Speed      float64
	RandomWalk bool
}

type AOIConfig struct {
	Type     string
	GridSize float64
}

type SyncConfig struct {
	Type         string
	SnapshotRate int
}

func DefaultConfig() Config {
	return Config{
		Server: ServerConfig{ListenAddr: "0.0.0.0:8100", TickRate: 20},
		World:  WorldConfig{Width: 2000, Height: 2000, NPCCount: 100},
		Player: PlayerConfig{Speed: 180, AOIRadius: 200},
		NPC:    NPCConfig{Speed: 80, RandomWalk: true},
		AOI:    AOIConfig{Type: "grid", GridSize: 200},
		Sync:   SyncConfig{Type: "snapshot", SnapshotRate: 10},
	}
}

func Load(path string) (Config, error) {
	cfg := DefaultConfig()
	if path == "" {
		return cfg, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return cfg, err
	}
	defer file.Close()

	section := ""
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		raw := stripComment(scanner.Text())
		if strings.TrimSpace(raw) == "" {
			continue
		}

		trimmed := strings.TrimSpace(raw)
		if !strings.HasPrefix(raw, " ") && strings.HasSuffix(trimmed, ":") {
			section = strings.TrimSuffix(trimmed, ":")
			continue
		}

		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			return cfg, fmt.Errorf("invalid config line %d: %q", lineNumber, scanner.Text())
		}

		key := strings.TrimSpace(parts[0])
		value := cleanValue(parts[1])
		if err := setValue(&cfg, section, key, value); err != nil {
			return cfg, fmt.Errorf("line %d: %w", lineNumber, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return cfg, err
	}

	normalize(&cfg)
	return cfg, nil
}

func stripComment(line string) string {
	inQuote := rune(0)
	for i, r := range line {
		switch r {
		case '\'', '"':
			if inQuote == 0 {
				inQuote = r
			} else if inQuote == r {
				inQuote = 0
			}
		case '#':
			if inQuote == 0 {
				return line[:i]
			}
		}
	}
	return line
}

func cleanValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "\"'")
	return value
}

func normalize(cfg *Config) {
	if cfg.Server.ListenAddr == "" {
		cfg.Server.ListenAddr = "0.0.0.0:8100"
	}
	if cfg.Server.TickRate <= 0 {
		cfg.Server.TickRate = 20
	}
	if cfg.World.Width <= 0 {
		cfg.World.Width = 2000
	}
	if cfg.World.Height <= 0 {
		cfg.World.Height = 2000
	}
	if cfg.Player.Speed <= 0 {
		cfg.Player.Speed = 180
	}
	if cfg.Player.AOIRadius <= 0 {
		cfg.Player.AOIRadius = 200
	}
	if cfg.NPC.Speed <= 0 {
		cfg.NPC.Speed = 80
	}
	if cfg.AOI.Type == "" {
		cfg.AOI.Type = "grid"
	}
	cfg.AOI.Type = strings.ToLower(cfg.AOI.Type)
	if cfg.AOI.GridSize <= 0 {
		cfg.AOI.GridSize = 200
	}
	if cfg.Sync.Type == "" {
		cfg.Sync.Type = "snapshot"
	}
	if cfg.Sync.SnapshotRate <= 0 {
		cfg.Sync.SnapshotRate = 10
	}
}

func setValue(cfg *Config, section, key, value string) error {
	switch section {
	case "server":
		switch key {
		case "listen_addr":
			cfg.Server.ListenAddr = value
		case "tick_rate":
			v, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			cfg.Server.TickRate = v
		}
	case "world":
		switch key {
		case "width":
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return err
			}
			cfg.World.Width = v
		case "height":
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return err
			}
			cfg.World.Height = v
		case "npc_count":
			v, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			cfg.World.NPCCount = v
		}
	case "player":
		switch key {
		case "speed":
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return err
			}
			cfg.Player.Speed = v
		case "aoi_radius":
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return err
			}
			cfg.Player.AOIRadius = v
		}
	case "npc":
		switch key {
		case "speed":
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return err
			}
			cfg.NPC.Speed = v
		case "random_walk":
			v, err := strconv.ParseBool(value)
			if err != nil {
				return err
			}
			cfg.NPC.RandomWalk = v
		}
	case "aoi":
		switch key {
		case "type":
			cfg.AOI.Type = value
		case "grid_size":
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return err
			}
			cfg.AOI.GridSize = v
		}
	case "sync":
		switch key {
		case "type":
			cfg.Sync.Type = value
		case "snapshot_rate":
			v, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			cfg.Sync.SnapshotRate = v
		}
	default:
		return fmt.Errorf("unknown section %q", section)
	}
	return nil
}
