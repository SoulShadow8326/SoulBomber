package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/google/uuid"
)

var (
	lobbyCleanupRunning bool
)

func startLobbyCleanup() {
	if lobbyCleanupRunning {
		return
	}
	lobbyCleanupRunning = true

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		logInfo("Lobby cleanup system started")

		for range ticker.C {
			cleanupEmptyLobbies()
		}
	}()
}

func cleanupEmptyLobbies() {
	rows, err := db.Query(`
		SELECT id, name, player_count, created_at
		FROM lobbies
		WHERE status = 'waiting'
	`)
	if err != nil {
		logError("Error querying lobbies for cleanup", err)
		return
	}
	defer rows.Close()

	var lobbiesToCheck []struct {
		ID          string
		Name        string
		PlayerCount int
		CreatedAt   time.Time
	}

	for rows.Next() {
		var lobby struct {
			ID          string
			Name        string
			PlayerCount int
			CreatedAt   time.Time
		}
		err := rows.Scan(&lobby.ID, &lobby.Name, &lobby.PlayerCount, &lobby.CreatedAt)
		if err != nil {
			logError("Error scanning lobby for cleanup", err)
			continue
		}
		lobbiesToCheck = append(lobbiesToCheck, lobby)
	}

	for _, lobby := range lobbiesToCheck {
		if time.Since(lobby.CreatedAt) < 300*time.Second {
			continue
		}

		activePlayers := getActivePlayersInLobby(lobby.ID)

		if len(activePlayers) == 0 {
			err := deleteLobby(lobby.ID)
			if err != nil {
				logError("Error deleting empty lobby", err, "lobbyID", lobby.ID, "lobbyName", lobby.Name)
			} else {
				logInfo("Deleted empty lobby", "lobbyID", lobby.ID, "lobbyName", lobby.Name)
			}
		} else {
			if len(activePlayers) != lobby.PlayerCount {
				err := updateLobbyPlayerCount(lobby.ID, len(activePlayers))
				if err != nil {
					logError("Error updating lobby player count", err, "lobbyID", lobby.ID)
				}
			}
		}
	}
}

func getActivePlayersInLobby(lobbyID string) []Player {
	if playerTracker == nil {
		return []Player{}
	}

	sessions := playerTracker.GetLobbyPlayers(lobbyID)
	var activePlayers []Player

	for _, session := range sessions {
		if session.Status == "active" {
			player := Player{
				ID:   session.PlayerID,
				Name: session.PlayerName,
				IsAI: session.IsAI,
			}
			activePlayers = append(activePlayers, player)
		}
	}

	return activePlayers
}

func deleteLobby(lobbyID string) error {
	_, err := db.Exec("DELETE FROM lobbies WHERE id = ?", lobbyID)
	return err
}

func updateLobbyPlayerCount(lobbyID string, playerCount int) error {
	_, err := db.Exec("UPDATE lobbies SET player_count = ? WHERE id = ?", playerCount, lobbyID)
	return err
}

func createLobby(name string, isSinglePlayer bool, aiPlayers []AIPlayer) (*Lobby, error) {
	id := uuid.New().String()
	now := time.Now()

	aiPlayersJSON, err := json.Marshal(aiPlayers)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
		INSERT INTO lobbies (id, name, player_count, max_players, status, created_at, is_single_player, ai_players)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, id, name, 0, 4, "waiting", now, isSinglePlayer, aiPlayersJSON)

	if err != nil {
		return nil, err
	}

	lobby := &Lobby{
		ID:             id,
		Name:           name,
		PlayerCount:    0,
		MaxPlayers:     4,
		Status:         "waiting",
		CreatedAt:      now,
		IsSinglePlayer: isSinglePlayer,
		AIPlayers:      aiPlayers,
	}

	return lobby, nil
}

func getLobbies() []Lobby {
	rows, err := db.Query(`
		SELECT id, name, player_count, max_players, status, created_at, is_single_player, ai_players
		FROM lobbies
		WHERE status = 'waiting'
		ORDER BY created_at DESC
	`)
	if err != nil {
		log.Printf("Error querying lobbies: %v", err)
		return []Lobby{}
	}
	defer rows.Close()

	var lobbies []Lobby
	for rows.Next() {
		var lobby Lobby
		var aiPlayersJSON string
		err := rows.Scan(
			&lobby.ID,
			&lobby.Name,
			&lobby.PlayerCount,
			&lobby.MaxPlayers,
			&lobby.Status,
			&lobby.CreatedAt,
			&lobby.IsSinglePlayer,
			&aiPlayersJSON,
		)
		if err != nil {
			log.Printf("Error scanning lobby: %v", err)
			continue
		}

		if aiPlayersJSON != "" {
			json.Unmarshal([]byte(aiPlayersJSON), &lobby.AIPlayers)
		}

		lobbies = append(lobbies, lobby)
	}

	return lobbies
}

func getLobbyWithPlayers(lobbyID string) (*Lobby, []Player, bool) {
	var lobby Lobby
	var aiPlayersJSON string

	err := db.QueryRow(`
		SELECT id, name, player_count, max_players, status, created_at, is_single_player, ai_players
		FROM lobbies
		WHERE id = ?
	`, lobbyID).Scan(
		&lobby.ID,
		&lobby.Name,
		&lobby.PlayerCount,
		&lobby.MaxPlayers,
		&lobby.Status,
		&lobby.CreatedAt,
		&lobby.IsSinglePlayer,
		&aiPlayersJSON,
	)

	if err != nil {
		return nil, nil, false
	}

	if aiPlayersJSON != "" {
		json.Unmarshal([]byte(aiPlayersJSON), &lobby.AIPlayers)
	}

	players := getPlayersForLobby(lobbyID)

	return &lobby, players, true
}

func getPlayersForLobby(lobbyID string) []Player {
	if playerTracker == nil {
		return []Player{}
	}

	sessions := playerTracker.GetLobbyPlayers(lobbyID)
	var players []Player

	for _, session := range sessions {
		player := Player{
			ID:   session.PlayerID,
			Name: session.PlayerName,
			IsAI: session.IsAI,
		}
		players = append(players, player)
	}

	return players
}

func joinLobby(lobbyID, playerID string) error {
	var lobby Lobby
	err := db.QueryRow(`
		SELECT player_count, max_players
		FROM lobbies
		WHERE id = ?
	`, lobbyID).Scan(&lobby.PlayerCount, &lobby.MaxPlayers)

	if err != nil {
		logError("Lobby not found in joinLobby", err, "lobbyID", lobbyID)
		return fmt.Errorf("lobby not found")
	}

	if playerTracker != nil {
		sessions := playerTracker.GetLobbyPlayers(lobbyID)
		playerCount := len(sessions)

		if playerCount >= lobby.MaxPlayers {
			return fmt.Errorf("lobby is full")
		}

		_, err = db.Exec(`
			UPDATE lobbies
			SET player_count = ?
			WHERE id = ?
		`, playerCount+1, lobbyID)

		if err != nil {
			logError("Failed to update lobby player count", err, "lobbyID", lobbyID)
		}
	}

	return nil
}

func leaveLobby(lobbyID, playerID string) error {
	if playerTracker != nil {
		if session := playerTracker.GetPlayerSession(playerID); session != nil {
			if len(session.WebSocketIDs) > 0 {
				return nil
			}
		}

		sessions := playerTracker.GetLobbyPlayers(lobbyID)
		playerCount := len(sessions)

		_, err := db.Exec(`
			UPDATE lobbies
			SET player_count = ?
			WHERE id = ?
		`, playerCount, lobbyID)

		return err
	}

	return nil
}

func joinLobbyWithName(lobbyID, playerID, playerName string) error {
	var lobby Lobby
	err := db.QueryRow(`
		SELECT player_count, max_players
		FROM lobbies
		WHERE id = ?
	`, lobbyID).Scan(&lobby.PlayerCount, &lobby.MaxPlayers)

	if err != nil {
		return fmt.Errorf("lobby not found")
	}

	if playerTracker != nil {
		sessions := playerTracker.GetLobbyPlayers(lobbyID)
		playerCount := len(sessions)

		if playerCount >= lobby.MaxPlayers {
			return fmt.Errorf("lobby is full")
		}

		_, err = db.Exec(`
			UPDATE lobbies
			SET player_count = ?
			WHERE id = ?
		`, playerCount+1, lobbyID)

		if err != nil {
			logError("Failed to update lobby player count", err, "lobbyID", lobbyID)
		}

		playerTracker.UpdatePlayerStatus(playerID, "active")
	}

	return nil
}

func startSinglePlayerGame(lobbyID, playerID string) (*Game, error) {
	_, players, exists := getLobbyWithPlayers(lobbyID)
	if !exists {
		return nil, fmt.Errorf("lobby not found")
	}

	gamesMu.Lock()
	for gameID, game := range games {
		if game.LobbyID == lobbyID {
			delete(games, gameID)
		}
	}
	gamesMu.Unlock()

	gameID := uuid.New().String()
	game := &Game{
		ID:         gameID,
		LobbyID:    lobbyID,
		Board:      generateBoard(),
		Players:    make(map[string]*Player),
		Bombs:      make(map[string]*Bomb),
		Explosions: make(map[string]*Explosion),
		Powerups:   generatePowerups(1),
		Status:     "playing",
		StartTime:  time.Now(),
		aiTickers:  make(map[string]*time.Ticker),
	}

	_, err := db.Exec(`
		INSERT INTO games (id, lobby_id, status, start_time, board)
		VALUES (?, ?, ?, ?, ?)
	`, gameID, lobbyID, "playing", game.StartTime, "[]")

	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
		UPDATE lobbies
		SET status = 'playing'
		WHERE id = ?
	`, lobbyID)

	if err != nil {
		return nil, err
	}
	spawnPositions := []Position{
		{Row: 2, Col: 2},
		{Row: 2, Col: 12},
		{Row: 12, Col: 2},
		{Row: 12, Col: 12},
	}

	playerIndex := 0
	for _, player := range players {
		playerCopy := player
		playerCopy.Alive = true
		playerCopy.BombCount = 0
		playerCopy.MaxBombs = 1
		playerCopy.BombRange = 1
		playerCopy.Score = 0
		playerCopy.Powerups = make(map[string]*PlayerPowerup)
		playerCopy.Position = spawnPositions[playerIndex%4]
		playerCopy.SpawnPosition = spawnPositions[playerIndex%4]
		game.Players[playerCopy.ID] = &playerCopy
		if playerCopy.IsAI {
			game.startAITicker(playerCopy.ID)
		}
		playerIndex++
	}

	if _, exists := game.Players[playerID]; !exists {
		playerName := "Player"
		if playerTracker != nil {
			if session := playerTracker.GetPlayerSession(playerID); session != nil {
				playerName = session.PlayerName
			}
		}

		spawnPos := spawnPositions[playerIndex%4]

		player := &Player{
			ID:            playerID,
			Name:          playerName,
			Position:      spawnPos,
			SpawnPosition: spawnPos,
			Alive:         true,
			BombCount:     0,
			MaxBombs:      1,
			BombRange:     1,
			IsAI:          false,
			Score:         0,
			Powerups:      make(map[string]*PlayerPowerup),
		}
		game.Players[playerID] = player
	}

	games[gameID] = game

	game.startGameTimers()

	return game, nil
}

func startGame(lobbyID, playerID string) (*Game, error) {
	players := getPlayersForLobby(lobbyID)
	if len(players) == 0 {
		return nil, fmt.Errorf("no players in lobby")
	}

	gamesMu.Lock()
	for gameID, game := range games {
		if game.LobbyID == lobbyID {
			delete(games, gameID)
		}
	}
	gamesMu.Unlock()

	gameID := uuid.New().String()
	game := &Game{
		ID:         gameID,
		LobbyID:    lobbyID,
		Board:      generateBoard(),
		Players:    make(map[string]*Player),
		Bombs:      make(map[string]*Bomb),
		Explosions: make(map[string]*Explosion),
		Powerups:   generatePowerups(1),
		Status:     "playing",
		StartTime:  time.Now(),
		aiTickers:  make(map[string]*time.Ticker),
	}

	spawnPositions := []Position{
		{Row: 2, Col: 2},
		{Row: 2, Col: 12},
		{Row: 12, Col: 2},
		{Row: 12, Col: 12},
	}

	playerIndex := 0
	for _, player := range players {
		playerCopy := player
		playerCopy.Alive = true
		playerCopy.BombCount = 0
		playerCopy.MaxBombs = 1
		playerCopy.BombRange = 1
		playerCopy.Score = 0
		playerCopy.Powerups = make(map[string]*PlayerPowerup)
		playerCopy.Position = spawnPositions[playerIndex%4]
		playerCopy.SpawnPosition = spawnPositions[playerIndex%4]
		game.Players[playerCopy.ID] = &playerCopy
		if playerCopy.IsAI {
			game.startAITicker(playerCopy.ID)
		}
		playerIndex++
	}

	if _, exists := game.Players[playerID]; !exists {
		playerName := "Player"
		if playerTracker != nil {
			if session := playerTracker.GetPlayerSession(playerID); session != nil {
				playerName = session.PlayerName
			}
		}

		spawnPos := spawnPositions[playerIndex%4]

		player := &Player{
			ID:            playerID,
			Name:          playerName,
			Position:      spawnPos,
			SpawnPosition: spawnPos,
			Alive:         true,
			BombCount:     0,
			MaxBombs:      1,
			BombRange:     1,
			IsAI:          false,
			Score:         0,
			Powerups:      make(map[string]*PlayerPowerup),
		}
		game.Players[playerID] = player
	}

	games[gameID] = game

	game.startGameTimers()

	return game, nil
}

func generateBoard() [][]int {
	board := make([][]int, 15)
	for i := range board {
		board[i] = make([]int, 15)
	}

	for i := 0; i < 15; i++ {
		for j := 0; j < 15; j++ {
			if i == 0 || i == 14 || j == 0 || j == 14 {
				board[i][j] = 1
			} else if i%2 == 0 && j%2 == 0 {
				board[i][j] = 1
			} else if rand.Float64() < 0.6 {
				board[i][j] = 2
			}
		}
	}

	board[1][1] = 0
	board[1][2] = 0
	board[2][1] = 0

	board[1][12] = 0
	board[1][13] = 0
	board[2][13] = 0

	board[12][1] = 0
	board[13][1] = 0
	board[13][2] = 0

	board[12][13] = 0
	board[13][12] = 0
	board[13][13] = 0

	return board
}

func generatePowerups(level int) map[string]*Powerup {
	powerups := make(map[string]*Powerup)

	centerRow := 7
	centerCol := 7

	shieldID := uuid.New().String()
	powerups[shieldID] = &Powerup{
		ID:       shieldID,
		Type:     POWERUP_SHIELD,
		Level:    1,
		Position: Position{Row: centerRow, Col: centerCol},
	}

	var validPositions []Position
	for i := 1; i < 14; i++ {
		for j := 1; j < 14; j++ {
			if i%2 == 0 && j%2 == 0 {
				continue
			}
			if i == centerRow && j == centerCol {
				continue
			}
			if ((i == centerRow-3 || i == centerRow+3) && (j >= centerCol-3 && j <= centerCol+3)) ||
				((j == centerCol-3 || j == centerCol+3) && (i >= centerRow-3 && i <= centerRow+3)) {
				validPositions = append(validPositions, Position{Row: i, Col: j})
			}
		}
	}

	for len(powerups) < 3 && len(validPositions) > 0 {
		idx := rand.Intn(len(validPositions))
		randomPos := validPositions[idx]
		validPositions = append(validPositions[:idx], validPositions[idx+1:]...)
		powerupID := uuid.New().String()
		powerups[powerupID] = &Powerup{
			ID:       powerupID,
			Type:     POWERUP_BOMB_RANGE,
			Level:    level,
			Position: randomPos,
		}
	}

	return powerups
}

func removeAIFromLobby(lobbyID string) error {
	var aiPlayerID string
	err := db.QueryRow(`
		SELECT id
		FROM players
		WHERE lobby_id = ? AND is_ai = true
		ORDER BY id DESC
		LIMIT 1
	`, lobbyID).Scan(&aiPlayerID)

	if err != nil {
		return fmt.Errorf("no AI players found")
	}

	_, err = db.Exec(`
		DELETE FROM players
		WHERE id = ?
	`, aiPlayerID)

	if err != nil {
		return err
	}

	_, err = db.Exec(`
		UPDATE lobbies
		SET player_count = player_count - 1
		WHERE id = ?
	`, lobbyID)

	return err
}

func addAIToLobby(lobbyID, difficulty string) error {
	var lobby Lobby
	err := db.QueryRow(`
		SELECT player_count, max_players
		FROM lobbies
		WHERE id = ?
	`, lobbyID).Scan(&lobby.PlayerCount, &lobby.MaxPlayers)

	if err != nil {
		return fmt.Errorf("lobby not found")
	}

	if lobby.PlayerCount >= lobby.MaxPlayers {
		return fmt.Errorf("lobby is full")
	}

	aiPlayerID := uuid.New().String()
	_, err = db.Exec(`
		INSERT INTO players (id, lobby_id, name, position_row, position_col, alive, bomb_count, max_bombs, bomb_range, is_ai, ai_difficulty, score)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, aiPlayerID, lobbyID, "AI Player", 2, 2, true, 0, 1, 1, true, difficulty, 0)

	if err != nil {
		return err
	}

	_, err = db.Exec(`
		UPDATE lobbies
		SET player_count = player_count + 1
		WHERE id = ?
	`, lobbyID)

	return err
}
