package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

type Connection struct {
	ID       string
	Conn     *websocket.Conn
	PlayerID string
	LobbyID  string
	Send     chan []byte
	Hub      *Hub
	mu       sync.Mutex
}

type Hub struct {
	connections map[string]*Connection
	register    chan *Connection
	unregister  chan *Connection
	broadcast   chan []byte
	mu          sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		connections: make(map[string]*Connection),
		register:    make(chan *Connection),
		unregister:  make(chan *Connection),
		broadcast:   make(chan []byte),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case conn := <-h.register:
			h.mu.Lock()
			h.connections[conn.ID] = conn
			h.mu.Unlock()
			logInfo("WebSocket connection registered",
				"connectionID", conn.ID,
				"playerID", conn.PlayerID,
				"lobbyID", conn.LobbyID,
			)

		case conn := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.connections[conn.ID]; ok {
				delete(h.connections, conn.ID)
				close(conn.Send)
			}
			h.mu.Unlock()
			logInfo("WebSocket connection unregistered",
				"connectionID", conn.ID,
				"playerID", conn.PlayerID,
				"lobbyID", conn.LobbyID,
			)

		case message := <-h.broadcast:
			h.mu.RLock()
			for _, conn := range h.connections {
				select {
				case conn.Send <- message:
				default:
					close(conn.Send)
					delete(h.connections, conn.ID)
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (c *Connection) readPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
		if c.PlayerID != "" {
			playerTracker.UnregisterPlayer(c.PlayerID, c.ID)
			if c.LobbyID != "" {
				leaveLobby(c.LobbyID, c.PlayerID)
			}
		}
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		if c.PlayerID != "" {
			playerTracker.UpdateHeartbeat(c.PlayerID)
		}
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logError("WebSocket read error", err,
					"connectionID", c.ID,
					"playerID", c.PlayerID,
				)
			}
			break
		}
		if err := c.handleMessage(message); err != nil {
			logError("Failed to handle message", err,
				"connectionID", c.ID,
				"playerID", c.PlayerID,
				"message", string(message),
			)
		}
	}
}

func (c *Connection) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			logInfo("Writing message", "data", string(message))
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Connection) handleMessage(message []byte) error {
	var msg Message
	if err := json.Unmarshal(message, &msg); err != nil {
		return err
	}

	logWebSocketEvent(msg.Type, c.PlayerID, msg.Payload)

	switch msg.Type {
	case "joinLobby":
		return c.handleJoinLobby(msg.Payload)
	case "joinGame":
		return c.handleJoinGame(msg.Payload)
	case "startGame":
		return c.handleStartGame(msg.Payload)
	case "startSinglePlayer":
		return c.handleStartSinglePlayer(msg.Payload)
	case "leaveLobby":
		return c.handleLeaveLobby(msg.Payload)
	case "move":
		return c.handleMove(msg.Payload)
	case "placeBomb":
		return c.handlePlaceBomb(msg.Payload)
	case "remoteDetonate":
		return c.handleRemoteDetonate(msg.Payload)
	case "dash":
		return c.handleDash(msg.Payload)
	case "restartGame":
		return c.handleRestartGame(msg.Payload)
	case "updatePlayerName":
		return c.handleUpdatePlayerName(msg.Payload)
	case "requestLobbyUpdate":
		return c.handleRequestLobbyUpdate(msg.Payload)
	case "requestPlayerInfo":
		return c.handleRequestPlayerInfo(msg.Payload)
	case "ping":
		return c.handlePing()
	default:
		return c.sendError("Unknown message type: " + msg.Type)
	}
}

func (c *Connection) handleJoinLobby(payload interface{}) error {
	data, ok := payload.(map[string]interface{})
	if !ok {
		return c.sendError("Invalid payload format")
	}

	lobbyID, ok := data["lobbyId"].(string)
	if !ok {
		return c.sendError("Missing lobbyId")
	}

	playerID, ok := data["playerId"].(string)
	if !ok {
		return c.sendError("Missing playerId")
	}

	playerName, _ := data["playerName"].(string)
	if playerName == "" {
		playerName = "Player"
	}
	if err := ValidateUUID(lobbyID); err != nil {
		return c.sendError("Invalid lobby ID")
	}

	c.LobbyID = lobbyID
	c.PlayerID = playerID

	playerTracker.RegisterPlayer(playerID, lobbyID, playerName, c.ID, false)

	err := joinLobby(lobbyID, playerID)
	if err != nil {
		logError("Failed to join lobby", err, "lobbyID", lobbyID, "playerID", playerID)
		if strings.Contains(err.Error(), "lobby not found") || strings.Contains(err.Error(), "lobby is full") {
			playerTracker.UnregisterPlayer(playerID, c.ID)
		}
		return c.sendError(err.Error())
	}

	broadcastLobbyUpdate(lobbyID)
	return nil
}

func (c *Connection) handleJoinGame(payload interface{}) error {
	data, ok := payload.(map[string]interface{})
	if !ok {
		return c.sendError("Invalid payload format")
	}

	lobbyID, ok := data["lobbyId"].(string)
	if !ok {
		return c.sendError("Missing lobbyId")
	}

	game := getGameByLobbyID(lobbyID)
	if game != nil {
		return c.sendMessage("gameState", game)
	} else {
		return c.sendError("Game not found")
	}
}

func (c *Connection) handleStartGame(payload interface{}) error {
	data, ok := payload.(map[string]interface{})
	if !ok {
		return c.sendError("Invalid payload format")
	}

	lobbyID, ok := data["lobbyId"].(string)
	if !ok {
		return c.sendError("Missing lobbyId")
	}

	playerID, ok := data["playerId"].(string)
	if !ok {
		return c.sendError("Missing playerId")
	}

	gamesMu.RLock()
	var existingGame *Game
	for _, game := range games {
		if game.LobbyID == lobbyID {
			existingGame = game
			break
		}
	}
	gamesMu.RUnlock()

	if existingGame != nil {
		broadcastToLobby(lobbyID, "gameState", existingGame)
		return nil
	}

	game, err := startGame(lobbyID, playerID)
	if err != nil {
		return c.sendError(err.Error())
	}
	broadcastToLobby(lobbyID, "gameState", game)
	return nil
}

func (c *Connection) handleStartSinglePlayer(payload interface{}) error {
	data, ok := payload.(map[string]interface{})
	if !ok {
		return c.sendError("Invalid payload format")
	}

	lobbyID, ok := data["lobbyId"].(string)
	if !ok {
		return c.sendError("Missing lobbyId")
	}

	playerID, ok := data["playerId"].(string)
	if !ok {
		return c.sendError("Missing playerId")
	}
	gamesMu.RLock()
	var existingGame *Game
	for _, game := range games {
		if game.LobbyID == lobbyID {
			existingGame = game
			break
		}
	}
	gamesMu.RUnlock()

	if existingGame != nil {
		c.sendMessage("gameState", existingGame)
		return nil
	}
	game, err := startSinglePlayerGame(lobbyID, playerID)
	if err != nil {
		return c.sendError(err.Error())
	}
	broadcastToLobby(lobbyID, "gameState", game)
	return nil
}

func (c *Connection) handleLeaveLobby(payload interface{}) error {
	data, ok := payload.(map[string]interface{})
	if !ok {
		return c.sendError("Invalid payload format")
	}

	lobbyID, ok := data["lobbyId"].(string)
	if !ok {
		return c.sendError("Missing lobbyId")
	}

	playerID, ok := data["playerId"].(string)
	if !ok {
		return c.sendError("Missing playerId")
	}

	leaveLobby(lobbyID, playerID)
	playerTracker.UnregisterPlayer(playerID, c.ID)

	broadcastLobbyUpdate(lobbyID)
	return c.sendMessage("left", "Successfully left lobby")
}

func (c *Connection) handleMove(payload interface{}) error {
	data, ok := payload.(map[string]interface{})
	if !ok {
		return c.sendError("Invalid payload format")
	}

	direction, ok := data["direction"].(string)
	if !ok {
		return c.sendError("Missing direction")
	}
	if err := ValidateDirection(direction); err != nil {
		return c.sendError(err.Error())
	}

	game := getGameByPlayerID(c.PlayerID)
	if game != nil {
		if err := game.movePlayer(c.PlayerID, direction); err != nil {
			return c.sendError(err.Error())
		}
		broadcastToLobby(game.LobbyID, "gameState", game)
		return nil
	}

	return c.sendError("Game not found")
}

func (c *Connection) handlePlaceBomb(_ interface{}) error {
	game := getGameByPlayerID(c.PlayerID)
	if game != nil {
		if err := game.placeBomb(c.PlayerID); err != nil {
			return c.sendError(err.Error())
		}
		broadcastToLobby(game.LobbyID, "gameState", game)
		return nil
	}

	return c.sendError("Game not found")
}

func (c *Connection) handleRemoteDetonate(payload interface{}) error {
	game := getGameByPlayerID(c.PlayerID)
	if game == nil {
		return c.sendError("Game not found")
	}

	if err := game.remoteDetonate(c.PlayerID); err != nil {
		return c.sendError(err.Error())
	}

	broadcastToLobby(game.LobbyID, "gameState", game)
	return nil
}

func (c *Connection) handleDash(payload interface{}) error {
	data, ok := payload.(map[string]interface{})
	if !ok {
		return c.sendError("Invalid payload format")
	}
	direction, ok := data["direction"].(string)
	if !ok {
		return c.sendError("Missing direction")
	}

	game := getGameByPlayerID(c.PlayerID)
	if game == nil {
		return c.sendError("Game not found")
	}

	if err := game.dash(c.PlayerID, direction); err != nil {
		return c.sendError(err.Error())
	}

	broadcastToLobby(game.LobbyID, "gameState", game)
	return nil
}

func (c *Connection) handleRestartGame(payload interface{}) error {
	data, ok := payload.(map[string]interface{})
	if !ok {
		return c.sendError("Invalid payload format")
	}

	lobbyID, ok := data["lobbyId"].(string)
	if !ok {
		return c.sendError("Missing lobbyId")
	}

	playerID, ok := data["playerId"].(string)
	if !ok {
		return c.sendError("Missing playerId")
	}
	gamesMu.Lock()
	for gameID, game := range games {
		if game.LobbyID == lobbyID {
			delete(games, gameID)
			break
		}
	}
	gamesMu.Unlock()
	game, err := startGame(lobbyID, playerID)
	if err != nil {
		return c.sendError(err.Error())
	}
	broadcastToLobby(lobbyID, "gameState", game)
	return nil
}

func (c *Connection) handleUpdatePlayerName(payload interface{}) error {
	data, ok := payload.(map[string]interface{})
	if !ok {
		return c.sendError("Invalid payload format")
	}

	playerID, ok := data["playerId"].(string)
	if !ok {
		return c.sendError("Missing playerId")
	}

	playerName, ok := data["playerName"].(string)
	if !ok || playerName == "" {
		return c.sendError("Missing or invalid playerName")
	}

	if playerTracker != nil {
		playerTracker.UpdatePlayerName(playerID, playerName)
	}

	if c.LobbyID != "" {
		broadcastLobbyUpdate(c.LobbyID)
	}

	return c.sendMessage("playerNameUpdated", "Player name updated successfully")
}

func (c *Connection) handleRequestLobbyUpdate(payload interface{}) error {
	data, ok := payload.(map[string]interface{})
	if !ok {
		return c.sendError("Invalid payload format")
	}

	lobbyID, ok := data["lobbyId"].(string)
	if !ok {
		return c.sendError("Missing lobbyId")
	}

	broadcastLobbyUpdate(lobbyID)
	return nil
}

func (c *Connection) handleRequestPlayerInfo(payload interface{}) error {
	data, ok := payload.(map[string]interface{})
	if !ok {
		return c.sendError("Invalid payload format")
	}

	playerID, ok := data["playerId"].(string)
	if !ok {
		return c.sendError("Missing playerId")
	}

	if playerTracker == nil {
		return c.sendError("Player tracker not available")
	}

	session := playerTracker.GetPlayerSession(playerID)
	if session == nil {
		return c.sendError("Player not found")
	}

	playerInfo := map[string]interface{}{
		"playerName": session.PlayerName,
	}

	return c.sendMessage("playerInfo", playerInfo)
}

func (c *Connection) handlePing() error {
	if c.PlayerID != "" {
		playerTracker.UpdateHeartbeat(c.PlayerID)
	}
	return c.sendMessage("pong", "pong")
}

func (c *Connection) sendMessage(msgType string, payload interface{}) error {
	message := Message{
		Type:    msgType,
		Payload: payload,
	}

	data, err := json.Marshal(message)
	if err != nil {
		return err
	}

	logInfo("Sending message", "type", msgType, "data", string(data))

	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case c.Send <- data:
		return nil
	default:
		return c.sendError("Connection buffer full")
	}
}

func (c *Connection) sendError(message string) error {
	return c.sendMessage("error", message)
}

func broadcastToLobby(lobbyID string, messageType string, payload interface{}) {
	message := Message{
		Type:    messageType,
		Payload: payload,
	}

	data, err := json.Marshal(message)
	if err != nil {
		logError("Failed to marshal broadcast message", err)
		return
	}

	hub.mu.RLock()
	defer hub.mu.RUnlock()

	count := 0
	for _, conn := range hub.connections {
		if conn.LobbyID == lobbyID {
			select {
			case conn.Send <- data:
				count++
				logInfo("Sent to connection", "connectionID", conn.ID, "playerID", conn.PlayerID)
			default:
				logInfo("Connection buffer full", "connectionID", conn.ID)
			}
		}
	}
	logInfo("Broadcast complete", "lobbyID", lobbyID, "sentTo", fmt.Sprintf("%d", count))
}

func broadcastLobbyUpdate(lobbyID string) {
	lobby, players, exists := getLobbyWithPlayers(lobbyID)
	if !exists {
		logInfo("Lobby not found for broadcast", "lobbyID", lobbyID)
		return
	}

	playersMap := make(map[string]*Player)
	for i, player := range players {
		playerCopy := player
		playerCopy.Slot = i
		playersMap[player.ID] = &playerCopy
	}

	lobbyState := map[string]interface{}{
		"lobby":   lobby,
		"players": playersMap,
	}

	logInfo("Broadcasting lobby update", "lobbyID", lobbyID, "playerCount", fmt.Sprintf("%d", len(players)))
	broadcastToLobby(lobbyID, "lobbyUpdate", lobbyState)
}

var hub *Hub

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logError("WebSocket upgrade failed", err)
		return
	}

	connection := &Connection{
		ID:   conn.RemoteAddr().String(),
		Conn: conn,
		Hub:  hub,
		Send: make(chan []byte, 256),
	}

	connection.Hub.register <- connection

	go connection.writePump()
	go connection.readPump()
}
