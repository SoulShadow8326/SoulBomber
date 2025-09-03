package main

import (
	"sync"
	"time"
)

const (
	AI_EASY       = "easy"
	AI_MEDIUM     = "medium"
	AI_HARD       = "hard"
	AI_CHOSEN_ONE = "chosen_one"
)

type Position struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

type Player struct {
	ID            string                    `json:"id"`
	Name          string                    `json:"name"`
	Position      Position                  `json:"position"`
	SpawnPosition Position                  `json:"spawnPosition"`
	Alive         bool                      `json:"alive"`
	BombCount     int                       `json:"bombCount"`
	MaxBombs      int                       `json:"maxBombs"`
	BombRange     int                       `json:"bombRange"`
	IsAI          bool                      `json:"isAI"`
	AIDifficulty  string                    `json:"aiDifficulty"`
	Slot          int                       `json:"slot"`
	Score         int                       `json:"score"`
	Powerups      map[string]*PlayerPowerup `json:"powerups"`
	Shield        bool                      `json:"shield"`
	LastDash      time.Time                 `json:"lastDash,omitempty"`
}

type AIPlayer struct {
	Difficulty string `json:"difficulty"`
	ID         string `json:"id"`
}

type Lobby struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	PlayerCount    int        `json:"playerCount"`
	MaxPlayers     int        `json:"maxPlayers"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"createdAt"`
	IsSinglePlayer bool       `json:"isSinglePlayer"`
	AIPlayers      []AIPlayer `json:"aiPlayers"`
}

type Message struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type Game struct {
	ID           string                  `json:"id"`
	LobbyID      string                  `json:"lobbyId"`
	Board        [][]int                 `json:"board"`
	Players      map[string]*Player      `json:"players"`
	Bombs        map[string]*Bomb        `json:"bombs"`
	Explosions   map[string]*Explosion   `json:"explosions"`
	Powerups     map[string]*Powerup     `json:"powerups"`
	Status       string                  `json:"status"`
	StartTime    time.Time               `json:"startTime"`
	EndTime      time.Time               `json:"endTime"`
	Winner       string                  `json:"winner"`
	GameTimer    *time.Timer             `json:"-"`
	PowerupTimer *time.Timer             `json:"-"`
	mu           sync.RWMutex            `json:"-"`
	aiTickers    map[string]*time.Ticker `json:"-"`
}

type Bomb struct {
	ID       string    `json:"id"`
	PlayerID string    `json:"playerId"`
	Position Position  `json:"position"`
	Range    int       `json:"range"`
	PlacedAt time.Time `json:"placedAt"`
}

type Explosion struct {
	ID       string    `json:"id"`
	Position Position  `json:"position"`
	EndTime  time.Time `json:"endTime"`
}

type ChainExplosion struct {
	OriginalBombID string
	PlayerID       string
	TilesDestroyed int
	PlayersKilled  int
}

const (
	POWERUP_BOMB_RANGE = "bomb_range"
	POWERUP_SHIELD     = "shield"
)

type Powerup struct {
	ID       string    `json:"id"`
	Type     string    `json:"type"`
	Level    int       `json:"level"`
	Position Position  `json:"position"`
	EndTime  time.Time `json:"endTime"`
}

type PlayerPowerup struct {
	Type    string    `json:"type"`
	Level   int       `json:"level"`
	EndTime time.Time `json:"endTime"`
}
