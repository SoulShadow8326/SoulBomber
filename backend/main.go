package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	_ "github.com/mattn/go-sqlite3"
)

var (
	db       *sql.DB
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		EnableCompression: true,
	}
)

func main() {
	initLogger()
	logInfo("Starting SoulBomber server")

	var err error
	db, err = sql.Open("sqlite3", "./soulbomber.db")
	if err != nil {
		logError("Failed to open database", err)
		log.Fatal(err)
	}
	defer db.Close()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	createTables()

	hub = NewHub()
	go hub.Run()

	InitializePlayerTracker()

	startLobbyCleanup()

	setupRoutes()

	logInfo("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func createTables() {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS lobbies (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			player_count INTEGER DEFAULT 0,
			max_players INTEGER DEFAULT 4,
			status TEXT DEFAULT 'waiting',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			is_single_player BOOLEAN DEFAULT FALSE
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS players (
			id TEXT PRIMARY KEY,
			lobby_id TEXT,
			name TEXT NOT NULL,
			position_row INTEGER DEFAULT 1,
			position_col INTEGER DEFAULT 1,
			alive BOOLEAN DEFAULT TRUE,
			bomb_count INTEGER DEFAULT 0,
			max_bombs INTEGER DEFAULT 1,
			bomb_range INTEGER DEFAULT 1,
			is_ai BOOLEAN DEFAULT FALSE,
			ai_difficulty TEXT DEFAULT '',
			score INTEGER DEFAULT 0,
			FOREIGN KEY (lobby_id) REFERENCES lobbies(id)
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS games (
			id TEXT PRIMARY KEY,
			lobby_id TEXT,
			status TEXT DEFAULT 'waiting',
			start_time DATETIME DEFAULT CURRENT_TIMESTAMP,
			end_time DATETIME,
			winner TEXT,
			board TEXT DEFAULT '[]',
			FOREIGN KEY (lobby_id) REFERENCES lobbies(id)
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`ALTER TABLE players ADD COLUMN score INTEGER DEFAULT 0`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		log.Printf("Migration warning: %v", err)
	}
}

func handleLobbies(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	switch r.Method {
	case "GET":
		lobbies := getLobbies()
		json.NewEncoder(w).Encode(lobbies)

	case "POST":
		var request struct {
			Name           string     `json:"name"`
			IsSinglePlayer bool       `json:"isSinglePlayer"`
			AIPlayers      []AIPlayer `json:"aiPlayers"`
		}

		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			logError("Error decoding request", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if err := ValidateLobbyName(request.Name); err != nil {
			logError("Invalid lobby name", err, "name", request.Name)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		request.Name = SanitizeString(request.Name)

		lobby, err := createLobby(request.Name, request.IsSinglePlayer, request.AIPlayers)
		if err != nil {
			logError("Error creating lobby", err, "name", request.Name)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(lobby)
	}
}

func handleLobby(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	lobbyID := r.URL.Path[len("/api/lobby/"):]
	if lobbyID == "" {
		http.Error(w, "Lobby ID required", http.StatusBadRequest)
		return
	}

	lobby, players, exists := getLobbyWithPlayers(lobbyID)
	if !exists {
		http.Error(w, "Lobby not found", http.StatusNotFound)
		return
	}

	response := map[string]interface{}{
		"lobby":   lobby,
		"players": players,
	}

	json.NewEncoder(w).Encode(response)
}

func handleLobbyJoin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Path[len("/api/lobby/"):]
	pathParts := strings.Split(path, "/")

	if len(pathParts) < 2 || pathParts[1] != "join" {
		http.Error(w, "Invalid endpoint", http.StatusBadRequest)
		return
	}

	lobbyID := pathParts[0]
	if lobbyID == "" {
		http.Error(w, "Lobby ID required", http.StatusBadRequest)
		return
	}

	var request struct {
		PlayerID   string `json:"playerId"`
		PlayerName string `json:"playerName"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		logError("Error decoding join request", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := ValidateUUID(lobbyID); err != nil {
		logError("Invalid lobby ID", err, "lobbyID", lobbyID)
		http.Error(w, "Invalid lobby ID", http.StatusBadRequest)
		return
	}

	if err := ValidatePlayerName(request.PlayerName); err != nil {
		logError("Invalid player name", err, "playerName", request.PlayerName)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	request.PlayerName = SanitizeString(request.PlayerName)

	err := joinLobbyWithName(lobbyID, request.PlayerID, request.PlayerName)
	if err != nil {
		logError("Failed to join lobby", err,
			"lobbyID", lobbyID,
			"playerName", request.PlayerName,
			"playerID", request.PlayerID,
		)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "joined"})
}

func handleLobbyRoutes(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[len("/api/lobby/"):]
	pathParts := strings.Split(path, "/")

	if len(pathParts) > 1 {
		switch pathParts[1] {
		case "join":
			handleLobbyJoin(w, r)
		case "add-ai":
			handleAddAI(w, r)
		case "remove-ai":
			handleRemoveAI(w, r)
		default:
			handleLobby(w, r)
		}
	} else {
		handleLobby(w, r)
	}
}

func handleStartSinglePlayer(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	var request struct {
		LobbyID  string `json:"lobbyId"`
		PlayerID string `json:"playerId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	game, err := startSinglePlayerGame(request.LobbyID, request.PlayerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(game)
}

func handleAddAI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := r.URL.Path[len("/api/lobby/"):]
	pathParts := strings.Split(path, "/")
	if len(pathParts) < 2 || pathParts[1] != "add-ai" {
		http.Error(w, "Invalid endpoint", http.StatusBadRequest)
		return
	}
	lobbyID := pathParts[0]
	if lobbyID == "" {
		http.Error(w, "Lobby ID required", http.StatusBadRequest)
		return
	}
	var req struct {
		Difficulty string `json:"difficulty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logError("Error decoding add AI request", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := ValidateDifficulty(req.Difficulty); err != nil {
		logError("Invalid AI difficulty", err, "difficulty", req.Difficulty)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := addAIToLobby(lobbyID, req.Difficulty)
	if err != nil {
		logError("Failed to add AI to lobby", err,
			"lobbyID", lobbyID,
			"difficulty", req.Difficulty,
		)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ai added"})
}

func handleRemoveAI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := r.URL.Path[len("/api/lobby/"):]
	pathParts := strings.Split(path, "/")
	if len(pathParts) < 2 || pathParts[1] != "remove-ai" {
		http.Error(w, "Invalid endpoint", http.StatusBadRequest)
		return
	}
	lobbyID := pathParts[0]
	if lobbyID == "" {
		http.Error(w, "Lobby ID required", http.StatusBadRequest)
		return
	}
	err := removeAIFromLobby(lobbyID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ai removed"})
}

func handlePlayerStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := playerTracker.GetPlayerStats()

	stats["timestamp"] = time.Now()
	stats["uptime"] = time.Since(startTime).Seconds()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
