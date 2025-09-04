package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"
)

var (
	games   = make(map[string]*Game)
	gamesMu sync.RWMutex
)

func getGameByLobbyID(lobbyID string) *Game {
	gamesMu.RLock()
	defer gamesMu.RUnlock()
	for _, game := range games {
		if game.LobbyID == lobbyID {
			return game
		}
	}
	return nil
}

func getGameByPlayerID(playerID string) *Game {
	gamesMu.RLock()
	defer gamesMu.RUnlock()
	for _, game := range games {
		if _, exists := game.Players[playerID]; exists {
			return game
		}
	}
	return nil
}

func (g *Game) snapshot() *Game {
	g.mu.RLock()
	defer g.mu.RUnlock()

	boardCopy := make([][]int, len(g.Board))
	for i := range g.Board {
		boardCopy[i] = make([]int, len(g.Board[i]))
		copy(boardCopy[i], g.Board[i])
	}

	playersCopy := make(map[string]*Player)
	for id, player := range g.Players {
		pc := *player
		playersCopy[id] = &pc
	}

	bombsCopy := make(map[string]*Bomb)
	for id, bomb := range g.Bombs {
		bc := *bomb
		bombsCopy[id] = &bc
	}

	explosionsCopy := make(map[string]*Explosion)
	for id, ex := range g.Explosions {
		ec := *ex
		explosionsCopy[id] = &ec
	}

	powerupsCopy := make(map[string]*Powerup)
	for id, pu := range g.Powerups {
		puc := *pu
		powerupsCopy[id] = &puc
	}

	gameCopy := &Game{
		ID:         g.ID,
		LobbyID:    g.LobbyID,
		Board:      boardCopy,
		Players:    playersCopy,
		Bombs:      bombsCopy,
		Explosions: explosionsCopy,
		Powerups:   powerupsCopy,
		Status:     g.Status,
		StartTime:  g.StartTime,
		EndTime:    g.EndTime,
		Winner:     g.Winner,
	}

	return gameCopy
}

func (g *Game) isActive() bool {
	gamesMu.RLock()
	defer gamesMu.RUnlock()
	existing, ok := games[g.ID]
	return ok && existing == g
}

func (g *Game) startAITicker(playerID string) {
	player, exists := g.Players[playerID]
	if !exists || !player.IsAI {
		return
	}

	interval := time.Second
	switch player.AIDifficulty {
	case AI_EASY:
		interval = 1500 * time.Millisecond
	case AI_MEDIUM:
		interval = 1000 * time.Millisecond
	case AI_HARD:
		interval = 600 * time.Millisecond
	case AI_CHOSEN_ONE:
		interval = 300 * time.Millisecond
	}

	ticker := time.NewTicker(interval)
	g.aiTickers[playerID] = ticker

	go func() {
		for range ticker.C {
			g.makeAIMove(playerID)
		}
	}()
}

func (g *Game) makeAIMove(playerID string) {
	if !g.isActive() {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.isActive() {
		return
	}
	player, exists := g.Players[playerID]
	if !exists || !player.Alive {
		return
	}

	directions := []string{"up", "down", "left", "right"}
	validMoves := []string{}

	for _, dir := range directions {
		if g.isValidMove(playerID, dir) {
			validMoves = append(validMoves, dir)
		}
	}

	if len(validMoves) > 0 {
		bestMove := g.findBestMove(playerID, validMoves)
		g.movePlayer(playerID, bestMove)
	}

	bombChance := g.getBombChance(player.AIDifficulty)
	if rand.Float64() < bombChance && g.shouldPlaceBomb(playerID) {
		if g.isInDanger(player.Position) {

		} else {
			g.placeBomb(playerID)
		}
	}
}

func (g *Game) findBestMove(playerID string, validMoves []string) string {
	player := g.Players[playerID]

	if g.isInDanger(player.Position) {
		safeMoves := g.findSafeMoves(playerID, validMoves)
		if len(safeMoves) > 0 {
			return safeMoves[rand.Intn(len(safeMoves))]
		}
	}

	if g.hasTarget(player.Position) {
		targetMoves := g.findTargetMoves(playerID, validMoves)
		if len(targetMoves) > 0 {
			return targetMoves[rand.Intn(len(targetMoves))]
		}
	}

	return validMoves[rand.Intn(len(validMoves))]
}

func (g *Game) isInDanger(pos Position) bool {
	for _, bomb := range g.Bombs {
		if g.isInBombRange(pos, bomb) {
			return true
		}
	}
	return false
}

func (g *Game) isInBombRange(pos Position, bomb *Bomb) bool {
	dx := abs(pos.Col - bomb.Position.Col)
	dy := abs(pos.Row - bomb.Position.Row)
	return dx <= bomb.Range && dy <= bomb.Range
}

func (g *Game) hasTarget(pos Position) bool {
	for _, otherPlayer := range g.Players {
		if otherPlayer.Alive {
			dx := abs(pos.Col - otherPlayer.Position.Col)
			dy := abs(pos.Row - otherPlayer.Position.Row)
			if dx <= 3 && dy <= 3 {
				return true
			}
		}
	}
	return false
}

func (g *Game) findSafeMoves(playerID string, validMoves []string) []string {
	var safeMoves []string
	player := g.Players[playerID]

	for _, move := range validMoves {
		newPos := g.getNewPosition(player.Position, move)
		if !g.isInDanger(newPos) {
			safeMoves = append(safeMoves, move)
		}
	}

	return safeMoves
}

func (g *Game) findTargetMoves(playerID string, validMoves []string) []string {
	var targetMoves []string
	player := g.Players[playerID]

	for _, move := range validMoves {
		newPos := g.getNewPosition(player.Position, move)
		if g.hasTarget(newPos) {
			targetMoves = append(targetMoves, move)
		}
	}

	return targetMoves
}

func (g *Game) getNewPosition(pos Position, direction string) Position {
	newPos := pos
	switch direction {
	case "up":
		newPos.Row--
	case "down":
		newPos.Row++
	case "left":
		newPos.Col--
	case "right":
		newPos.Col++
	}
	return newPos
}

func (g *Game) getBombChance(difficulty string) float64 {
	switch difficulty {
	case AI_EASY:
		return 0.15
	case AI_MEDIUM:
		return 0.35
	case AI_HARD:
		return 0.55
	case AI_CHOSEN_ONE:
		return 0.75
	default:
		return 0.25
	}
}

func (g *Game) shouldPlaceBomb(playerID string) bool {
	player := g.Players[playerID]

	if g.hasTarget(player.Position) {
		return true
	}

	if g.Board[player.Position.Row][player.Position.Col] == 1 {
		return true
	}

	return false
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func (g *Game) movePlayer(playerID, direction string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	player, exists := g.Players[playerID]
	if !exists {

		return errors.New("player not found")
	}

	if !player.Alive {
		return errors.New("player not alive")
	}

	newPos := player.Position
	switch direction {
	case "up":
		newPos.Row--
	case "down":
		newPos.Row++
	case "left":
		newPos.Col--
	case "right":
		newPos.Col++
	default:
		return errors.New("invalid direction")
	}

	if !g.isValidPosition(newPos) {
		return errors.New("invalid move")
	}

	cellValue := g.Board[newPos.Row][newPos.Col]

	if cellValue == 0 {
		player.Position = newPos

		for powerupID, powerup := range g.Powerups {
			if powerup.Position == newPos {
				g.collectPowerup(playerID, powerupID)
			}
		}

		return nil
	}

	for _, bomb := range g.Bombs {
		if bomb.Position == newPos {
			pushPos := newPos
			switch direction {
			case "up":
				pushPos.Row--
			case "down":
				pushPos.Row++
			case "left":
				pushPos.Col--
			case "right":
				pushPos.Col++
			}
			if !g.isValidPosition(pushPos) {
				return errors.New("invalid move")
			}
			if g.Board[pushPos.Row][pushPos.Col] != 0 {
				return errors.New("invalid move")
			}
			occupied := false
			for _, otherBomb := range g.Bombs {
				if otherBomb.Position == pushPos {
					occupied = true
					break
				}
			}
			if occupied {
				return errors.New("invalid move")
			}

			bomb.Position = pushPos
			player.Position = newPos

			for powerupID, powerup := range g.Powerups {
				if powerup.Position == newPos {
					g.collectPowerup(playerID, powerupID)
				}
			}

			return nil
		}
	}

	return errors.New("invalid move")
}

func (g *Game) remoteDetonate(playerID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	var toExplode []string
	for id, bomb := range g.Bombs {
		if bomb.PlayerID == playerID {
			toExplode = append(toExplode, id)
		}
	}

	for _, id := range toExplode {
		go g.explodeBomb(id)
	}

	return nil
}

func (g *Game) dash(playerID, direction string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	player, exists := g.Players[playerID]
	if !exists || !player.Alive {
		return errors.New("player not found or not alive")
	}

	if time.Since(player.LastDash) < 7*time.Second {
		return errors.New("dash on cooldown")
	}

	cur := player.Position
	var furthest Position
	furthest = cur

	for i := 1; ; i++ {
		next := cur
		switch direction {
		case "up":
			next.Row = cur.Row - i
		case "down":
			next.Row = cur.Row + i
		case "left":
			next.Col = cur.Col - i
		case "right":
			next.Col = cur.Col + i
		default:
			return errors.New("invalid direction")
		}
		if !g.isValidPosition(next) {
			break
		}
		if g.Board[next.Row][next.Col] != 0 {
			break
		}
		blocked := false
		for _, bomb := range g.Bombs {
			if bomb.Position == next {
				blocked = true
				break
			}
		}
		if blocked {
			break
		}
		furthest = next
	}

	if furthest == cur {
		return errors.New("no available dash target")
	}

	player.Position = furthest
	player.LastDash = time.Now()

	for powerupID, powerup := range g.Powerups {
		if powerup.Position == furthest {
			g.collectPowerup(playerID, powerupID)
		}
	}

	return nil
}

func (g *Game) placeBomb(playerID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	player, exists := g.Players[playerID]
	if !exists || !player.Alive {
		return errors.New("player not found or not alive")
	}

	var playerBombs []string
	for bombID, bomb := range g.Bombs {
		if bomb.PlayerID == playerID {
			playerBombs = append(playerBombs, bombID)
		}
	}

	if len(playerBombs) >= player.MaxBombs {
		if len(playerBombs) > 0 {
			latestBombID := playerBombs[len(playerBombs)-1]
			go g.explodeBomb(latestBombID)
		}
		return nil
	}

	bombID := fmt.Sprintf("bomb_%s_%d", playerID, len(playerBombs))
	bomb := &Bomb{
		ID:       bombID,
		PlayerID: playerID,
		Position: player.Position,
		Range:    player.BombRange,
		PlacedAt: time.Now(),
	}

	g.Bombs[bombID] = bomb
	return nil
}

func (g *Game) isValidPosition(pos Position) bool {
	return pos.Row >= 0 && pos.Row < len(g.Board) && pos.Col >= 0 && pos.Col < len(g.Board[0])
}

func (g *Game) isValidMove(playerID, direction string) bool {
	player, exists := g.Players[playerID]
	if !exists {
		return false
	}

	newPos := player.Position
	switch direction {
	case "up":
		newPos.Row--
	case "down":
		newPos.Row++
	case "left":
		newPos.Col--
	case "right":
		newPos.Col++
	default:
		return false
	}

	if !g.isValidPosition(newPos) {
		return false
	}
	cellValue := g.Board[newPos.Row][newPos.Col]
	if cellValue != 0 {
		return false
	}
	for _, bomb := range g.Bombs {
		if bomb.Position == newPos && bomb.PlayerID != playerID {
			return false
		}
	}

	return true
}

func (g *Game) explodeBomb(bombID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.explodeBombInternal(bombID)
}

func (g *Game) explodeBombInternal(bombID string) {
	if !g.isActive() {
		return
	}
	bomb, exists := g.Bombs[bombID]
	if !exists {
		return
	}

	chainExplosion := &ChainExplosion{
		OriginalBombID: bombID,
		PlayerID:       bomb.PlayerID,
		TilesDestroyed: 0,
		PlayersKilled:  0,
	}

	explosionID := newUUID()
	explosion := &Explosion{
		ID:       explosionID,
		Position: bomb.Position,
		EndTime:  time.Now().Add(500 * time.Millisecond),
	}
	g.Explosions[explosionID] = explosion
	explosionIDs := []string{explosionID}

	for _, player := range g.Players {
		if player.Alive && player.Position == bomb.Position {
			if player.Shield {
				player.Shield = false
			} else {
				g.respawnPlayer(player.ID)
				if player.ID != bomb.PlayerID {
					chainExplosion.PlayersKilled++
				}
			}
		}
	}

	for otherBombID, otherBomb := range g.Bombs {
		if otherBomb.Position == bomb.Position && otherBombID != bombID {
			go g.explodeBomb(otherBombID)
		}
	}

	directions := []Position{
		{Row: -1, Col: 0},
		{Row: 1, Col: 0},
		{Row: 0, Col: -1},
		{Row: 0, Col: 1},
	}

	for _, dir := range directions {
		for i := 1; i <= bomb.Range; i++ {
			explosionPos := Position{
				Row: bomb.Position.Row + dir.Row*i,
				Col: bomb.Position.Col + dir.Col*i,
			}
			if !g.isValidPosition(explosionPos) {
				break
			}
			cell := g.Board[explosionPos.Row][explosionPos.Col]
			if cell == 1 {
				break
			}
			dirExplosionID := newUUID()
			dirExplosion := &Explosion{
				ID:       dirExplosionID,
				Position: explosionPos,
				EndTime:  time.Now().Add(500 * time.Millisecond),
			}
			g.Explosions[dirExplosionID] = dirExplosion
			explosionIDs = append(explosionIDs, dirExplosionID)
			if cell == 2 {
				g.Board[explosionPos.Row][explosionPos.Col] = 0
				chainExplosion.TilesDestroyed++

				for powerupID, powerup := range g.Powerups {
					if powerup.Position == explosionPos {
						for _, player := range g.Players {
							if player.Alive && player.Position == explosionPos {
								g.collectPowerup(player.ID, powerupID)
								break
							}
						}
					}
				}
			}
			for _, player := range g.Players {
				if player.Alive && player.Position == explosionPos {
					if player.Shield {
						player.Shield = false
					} else {
						g.respawnPlayer(player.ID)
						if player.ID != bomb.PlayerID {
							chainExplosion.PlayersKilled++
						}
					}
				}
			}
			for otherBombID, otherBomb := range g.Bombs {
				if otherBomb.Position == explosionPos && otherBombID != bombID {
					go g.explodeBomb(otherBombID)
				}
			}
			if cell == 1 {
				break
			}
		}
	}

	g.awardPoints(chainExplosion)
	g.checkWinCondition()

	lobbyID := g.LobbyID

	boardCopy := make([][]int, len(g.Board))
	for i := range g.Board {
		boardCopy[i] = make([]int, len(g.Board[i]))
		copy(boardCopy[i], g.Board[i])
	}

	playersCopy := make(map[string]*Player)
	for id, player := range g.Players {
		playerCopy := *player
		playersCopy[id] = &playerCopy
	}

	bombsCopy := make(map[string]*Bomb)
	for id, bomb := range g.Bombs {
		bombCopy := *bomb
		bombsCopy[id] = &bombCopy
	}

	explosionsCopy := make(map[string]*Explosion)
	for id, explosion := range g.Explosions {
		explosionCopy := *explosion
		explosionsCopy[id] = &explosionCopy
	}

	powerupsCopy := make(map[string]*Powerup)
	for id, powerup := range g.Powerups {
		powerupCopy := *powerup
		powerupsCopy[id] = &powerupCopy
	}

	gameCopy := g.snapshot()
	go func() { broadcastToLobby(lobbyID, "gameState", gameCopy) }()

	time.AfterFunc(500*time.Millisecond, func() {
		if !g.isActive() {
			return
		}
		g.mu.Lock()
		defer g.mu.Unlock()

		for _, explosionID := range explosionIDs {
			delete(g.Explosions, explosionID)
		}
		delete(g.Bombs, bombID)

		if !g.isActive() {
			return
		}

		gameCopy := g.snapshot()
		broadcastToLobby(lobbyID, "gameState", gameCopy)
	})
}

func (g *Game) awardPoints(chain *ChainExplosion) {
	player, exists := g.Players[chain.PlayerID]
	if !exists {
		return
	}

	baseTilePoints := chain.TilesDestroyed * 10
	playerKillPoints := chain.PlayersKilled * 250

	var multiplier float64
	switch chain.TilesDestroyed {
	case 0:
		multiplier = 1.0
	case 1:
		multiplier = 1.0
	case 2:
		multiplier = 1.2
	case 3:
		multiplier = 1.6
	default:
		multiplier = 2.0
	}

	tilePoints := int(float64(baseTilePoints) * multiplier)
	totalPoints := tilePoints + playerKillPoints

	player.Score += totalPoints
}

func (g *Game) checkWinCondition() {
	if g.Status == "finished" {
		return
	}
}

func (g *Game) collectPowerup(playerID string, powerupID string) {
	powerup, exists := g.Powerups[powerupID]
	if !exists {
		return
	}

	player, exists := g.Players[playerID]
	if !exists {
		return
	}

	if len(player.Powerups) >= 1 && powerup.Type != POWERUP_SHIELD {
		return
	}

	level := powerup.Level
	duration := 30 * time.Second
	if level == 2 {
		duration = 10 * time.Second
	}

	playerPowerup := &PlayerPowerup{
		Type:    powerup.Type,
		Level:   level,
		EndTime: time.Now().Add(duration),
	}

	player.Powerups[powerup.Type] = playerPowerup

	switch powerup.Type {
	case POWERUP_BOMB_RANGE:
		if level == 1 {
			player.BombRange = 2
		} else {
			player.BombRange = 3
		}
	case POWERUP_SHIELD:
		player.Shield = true
	}

	delete(g.Powerups, powerupID)

	go func() {
		time.Sleep(duration)
		g.expirePowerup(playerID, powerup.Type)
	}()
}

func (g *Game) expirePowerup(playerID string, powerupType string) {
	if !g.isActive() {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.isActive() {
		return
	}
	player, exists := g.Players[playerID]
	if !exists {
		return
	}

	delete(player.Powerups, powerupType)

	switch powerupType {
	case POWERUP_BOMB_RANGE:
		player.BombRange = 1
	case POWERUP_SHIELD:
		player.Shield = false
	}
}

func (g *Game) respawnPlayer(playerID string) {
	player, exists := g.Players[playerID]
	if !exists {
		return
	}

	player.Score = max(0, player.Score-100)
	player.Position = player.SpawnPosition
	player.Alive = true
	player.BombCount = 0
	player.Shield = false
}

func (g *Game) startGameTimers() {
	g.PowerupTimer = time.AfterFunc(1*time.Minute, func() {
		g.mu.Lock()
		defer g.mu.Unlock()

		g.Powerups = generatePowerups(2)
		broadcastToLobby(g.LobbyID, "powerupSpawn", g.Powerups)
	})

	g.GameTimer = time.AfterFunc(2*time.Minute, func() {
		g.mu.Lock()
		defer g.mu.Unlock()

		g.Status = "finished"
		g.EndTime = time.Now()

		var winner string
		var maxScore int
		for playerID, player := range g.Players {
			if player.Score > maxScore {
				maxScore = player.Score
				winner = playerID
			}
		}
		g.Winner = winner

		for _, ticker := range g.aiTickers {
			ticker.Stop()
		}

		winnerJSON, _ := json.Marshal(g.Winner)
		_, err := db.Exec(`
			UPDATE games SET status = ?, end_time = ?, winner = ?
			WHERE id = ?
		`, g.Status, g.EndTime, winnerJSON, g.ID)

		if err != nil {
			log.Printf("Error updating game in database: %v", err)
		}

		boardCopy := make([][]int, len(g.Board))
		for i := range g.Board {
			boardCopy[i] = make([]int, len(g.Board[i]))
			copy(boardCopy[i], g.Board[i])
		}

		playersCopy := make(map[string]*Player)
		for id, player := range g.Players {
			playerCopy := *player
			playersCopy[id] = &playerCopy
		}

		bombsCopy := make(map[string]*Bomb)
		for id, bomb := range g.Bombs {
			bombCopy := *bomb
			bombsCopy[id] = &bombCopy
		}

		explosionsCopy := make(map[string]*Explosion)
		for id, explosion := range g.Explosions {
			explosionCopy := *explosion
			explosionsCopy[id] = &explosionCopy
		}

		powerupsCopy := make(map[string]*Powerup)
		for id, powerup := range g.Powerups {
			powerupCopy := *powerup
			powerupsCopy[id] = &powerupCopy
		}

		gameCopy := &Game{
			ID:         g.ID,
			LobbyID:    g.LobbyID,
			Board:      boardCopy,
			Players:    playersCopy,
			Bombs:      bombsCopy,
			Explosions: explosionsCopy,
			Powerups:   powerupsCopy,
			Status:     g.Status,
			StartTime:  g.StartTime,
			EndTime:    g.EndTime,
			Winner:     g.Winner,
		}

		broadcastToLobby(g.LobbyID, "gameState", gameCopy)

		time.AfterFunc(5*time.Second, func() {
			g.endGame()
		})
	})
}

func (g *Game) endGame() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.PowerupTimer != nil {
		g.PowerupTimer.Stop()
		g.PowerupTimer = nil
	}
	if g.GameTimer != nil {
		g.GameTimer.Stop()
		g.GameTimer = nil
	}

	for _, t := range g.aiTickers {
		t.Stop()
	}
	g.aiTickers = nil

	g.Players = nil
	g.Bombs = nil
	g.Explosions = nil
	g.Powerups = nil

	cleanupGame(g.ID)
}

func cleanupGame(gameID string) {
	gamesMu.Lock()
	g, ok := games[gameID]
	if ok {
		for _, t := range g.aiTickers {
			t.Stop()
		}
		g.aiTickers = nil
		g.Players = nil
		g.Bombs = nil
		g.Explosions = nil
		g.Powerups = nil
		delete(games, gameID)
	}
	gamesMu.Unlock()
}
