package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type PlayerSession struct {
	PlayerID     string          `json:"playerId"`
	LobbyID      string          `json:"lobbyId"`
	PlayerName   string          `json:"playerName"`
	IsAI         bool            `json:"isAI"`
	ConnectedAt  time.Time       `json:"connectedAt"`
	LastSeen     time.Time       `json:"lastSeen"`
	Heartbeat    time.Time       `json:"heartbeat"`
	Status       string          `json:"status"`
	WebSocketIDs map[string]bool `json:"websocketIds"`
}

type PlayerTracker struct {
	sessions     map[string]*PlayerSession
	sessionsMu   sync.RWMutex
	heartbeats   map[string]time.Time
	heartbeatsMu sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
}

func NewPlayerTracker() *PlayerTracker {
	ctx, cancel := context.WithCancel(context.Background())

	tracker := &PlayerTracker{
		sessions:   make(map[string]*PlayerSession),
		heartbeats: make(map[string]time.Time),
		ctx:        ctx,
		cancel:     cancel,
	}

	go tracker.startMonitoring()

	return tracker
}

func (pt *PlayerTracker) RegisterPlayer(playerID, lobbyID, playerName, websocketID string, isAI bool) {
	pt.sessionsMu.Lock()
	defer pt.sessionsMu.Unlock()

	now := time.Now()

	if existingSession, exists := pt.sessions[playerID]; exists {
		existingSession.WebSocketIDs[websocketID] = true
		existingSession.LastSeen = now
		existingSession.Heartbeat = now
		existingSession.Status = "active"

		pt.heartbeatsMu.Lock()
		pt.heartbeats[playerID] = now
		pt.heartbeatsMu.Unlock()

		logInfo("Player reconnected",
			"playerID", playerID,
			"lobbyID", lobbyID,
			"playerName", playerName,
			"websocketID", websocketID,
			"totalConnections", fmt.Sprintf("%d", len(existingSession.WebSocketIDs)),
		)
		return
	}

	session := &PlayerSession{
		PlayerID:     playerID,
		LobbyID:      lobbyID,
		PlayerName:   playerName,
		IsAI:         isAI,
		ConnectedAt:  now,
		LastSeen:     now,
		Heartbeat:    now,
		Status:       "active",
		WebSocketIDs: make(map[string]bool),
	}
	session.WebSocketIDs[websocketID] = true

	pt.sessions[playerID] = session

	pt.heartbeatsMu.Lock()
	pt.heartbeats[playerID] = now
	pt.heartbeatsMu.Unlock()

	logInfo("Player registered",
		"playerID", playerID,
		"lobbyID", lobbyID,
		"playerName", playerName,
		"isAI", fmt.Sprintf("%t", isAI),
		"websocketID", websocketID,
	)
}

func (pt *PlayerTracker) UpdateHeartbeat(playerID string) {
	pt.heartbeatsMu.Lock()
	pt.heartbeats[playerID] = time.Now()
	pt.heartbeatsMu.Unlock()

	pt.sessionsMu.Lock()
	if session, exists := pt.sessions[playerID]; exists {
		session.Heartbeat = time.Now()
		session.LastSeen = time.Now()
		session.Status = "active"
	}
	pt.sessionsMu.Unlock()
}

func (pt *PlayerTracker) UpdatePlayerStatus(playerID, status string) {
	pt.sessionsMu.Lock()
	defer pt.sessionsMu.Unlock()

	if session, exists := pt.sessions[playerID]; exists {
		session.Status = status
		session.LastSeen = time.Now()

		logInfo("Player status updated",
			"playerID", playerID,
			"status", status,
			"lobbyID", session.LobbyID,
		)
	}
}

func (pt *PlayerTracker) UpdatePlayerName(playerID, playerName string) {
	pt.sessionsMu.Lock()
	defer pt.sessionsMu.Unlock()

	if session, exists := pt.sessions[playerID]; exists {
		session.PlayerName = playerName
		session.LastSeen = time.Now()

		logInfo("Player name updated",
			"playerID", playerID,
			"playerName", playerName,
		)
	}
}

func (pt *PlayerTracker) UnregisterPlayer(playerID, websocketID string) {
	pt.sessionsMu.Lock()
	session, exists := pt.sessions[playerID]
	if exists {
		delete(session.WebSocketIDs, websocketID)

		if len(session.WebSocketIDs) == 0 {
			delete(pt.sessions, playerID)

			pt.heartbeatsMu.Lock()
			delete(pt.heartbeats, playerID)
			pt.heartbeatsMu.Unlock()

			logInfo("Player unregistered",
				"playerID", playerID,
				"lobbyID", session.LobbyID,
				"playerName", session.PlayerName,
				"sessionDuration", time.Since(session.ConnectedAt).String(),
			)
		} else {
			logInfo("Player connection closed",
				"playerID", playerID,
				"lobbyID", session.LobbyID,
				"playerName", session.PlayerName,
				"remainingConnections", fmt.Sprintf("%d", len(session.WebSocketIDs)),
			)
		}
	}
	pt.sessionsMu.Unlock()
}

func (pt *PlayerTracker) GetPlayerSession(playerID string) *PlayerSession {
	pt.sessionsMu.RLock()
	defer pt.sessionsMu.RUnlock()

	return pt.sessions[playerID]
}

func (pt *PlayerTracker) GetLobbyPlayers(lobbyID string) []*PlayerSession {
	pt.sessionsMu.RLock()
	defer pt.sessionsMu.RUnlock()

	var players []*PlayerSession
	for _, session := range pt.sessions {
		if session.LobbyID == lobbyID {
			players = append(players, session)
		}
	}

	return players
}

func (pt *PlayerTracker) GetActivePlayers() []*PlayerSession {
	pt.sessionsMu.RLock()
	defer pt.sessionsMu.RUnlock()

	var players []*PlayerSession
	for _, session := range pt.sessions {
		if session.Status == "active" {
			players = append(players, session)
		}
	}

	return players
}

func (pt *PlayerTracker) GetPlayerStats() map[string]interface{} {
	pt.sessionsMu.RLock()
	defer pt.sessionsMu.RUnlock()

	stats := map[string]interface{}{
		"totalSessions":       len(pt.sessions),
		"activePlayers":       0,
		"idlePlayers":         0,
		"disconnectedPlayers": 0,
		"aiPlayers":           0,
		"humanPlayers":        0,
		"lobbies":             make(map[string]int),
	}

	for _, session := range pt.sessions {
		switch session.Status {
		case "active":
			stats["activePlayers"] = stats["activePlayers"].(int) + 1
		case "idle":
			stats["idlePlayers"] = stats["idlePlayers"].(int) + 1
		case "disconnected":
			stats["disconnectedPlayers"] = stats["disconnectedPlayers"].(int) + 1
		}

		if session.IsAI {
			stats["aiPlayers"] = stats["aiPlayers"].(int) + 1
		} else {
			stats["humanPlayers"] = stats["humanPlayers"].(int) + 1
		}

		lobbyCount := stats["lobbies"].(map[string]int)
		lobbyCount[session.LobbyID]++
	}

	return stats
}

func (pt *PlayerTracker) startMonitoring() {
	idleTicker := time.NewTicker(30 * time.Second)
	defer idleTicker.Stop()

	disconnectTicker := time.NewTicker(60 * time.Second)
	defer disconnectTicker.Stop()

	cleanupTicker := time.NewTicker(5 * time.Minute)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-pt.ctx.Done():
			return
		case <-idleTicker.C:
			pt.checkIdlePlayers()
		case <-disconnectTicker.C:
			pt.checkDisconnectedPlayers()
		case <-cleanupTicker.C:
			pt.cleanupStaleSessions()
		}
	}
}

func (pt *PlayerTracker) checkIdlePlayers() {
	pt.sessionsMu.Lock()
	defer pt.sessionsMu.Unlock()

	now := time.Now()
	idleThreshold := 2 * time.Minute

	for playerID, session := range pt.sessions {
		if session.Status == "active" && now.Sub(session.LastSeen) > idleThreshold {
			session.Status = "idle"
			logInfo("Player marked as idle",
				"playerID", playerID,
				"lobbyID", session.LobbyID,
				"idleTime", now.Sub(session.LastSeen).String(),
			)
		}
	}
}

func (pt *PlayerTracker) checkDisconnectedPlayers() {
	pt.heartbeatsMu.RLock()
	heartbeats := make(map[string]time.Time)
	for k, v := range pt.heartbeats {
		heartbeats[k] = v
	}
	pt.heartbeatsMu.RUnlock()

	pt.sessionsMu.Lock()
	defer pt.sessionsMu.Unlock()

	now := time.Now()
	disconnectThreshold := 3 * time.Minute

	for playerID, session := range pt.sessions {
		if !session.IsAI {
			lastHeartbeat, exists := heartbeats[playerID]
			if !exists || now.Sub(lastHeartbeat) > disconnectThreshold {
				if session.Status != "disconnected" {
					session.Status = "disconnected"
					session.LastSeen = now

					logInfo("Player marked as disconnected",
						"playerID", playerID,
						"lobbyID", session.LobbyID,
						"timeSinceHeartbeat", now.Sub(lastHeartbeat).String(),
					)

					go func(pid, lid string) {
						leaveLobby(lid, pid)
					}(playerID, session.LobbyID)
				}
			}
		}
	}
}

func (pt *PlayerTracker) cleanupStaleSessions() {
	pt.sessionsMu.Lock()
	defer pt.sessionsMu.Unlock()

	now := time.Now()
	staleThreshold := 10 * time.Minute

	var toRemove []string
	for playerID, session := range pt.sessions {
		if session.Status == "disconnected" && now.Sub(session.LastSeen) > staleThreshold {
			toRemove = append(toRemove, playerID)
		}
	}

	for _, playerID := range toRemove {
		session := pt.sessions[playerID]
		delete(pt.sessions, playerID)

		logInfo("Removed stale player session",
			"playerID", playerID,
			"lobbyID", session.LobbyID,
			"sessionDuration", now.Sub(session.ConnectedAt).String(),
			"timeSinceDisconnect", now.Sub(session.LastSeen).String(),
		)
	}

	pt.heartbeatsMu.Lock()
	for _, playerID := range toRemove {
		delete(pt.heartbeats, playerID)
	}
	pt.heartbeatsMu.Unlock()
}

func (pt *PlayerTracker) Stop() {
	pt.cancel()
}

var playerTracker *PlayerTracker

func InitializePlayerTracker() {
	playerTracker = NewPlayerTracker()
	logInfo("Player tracker initialized")
}
