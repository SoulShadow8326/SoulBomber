package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"runtime"
	"time"
)

type HealthStatus struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Uptime    time.Duration     `json:"uptime"`
	Version   string            `json:"version"`
	Services  map[string]string `json:"services"`
	Stats     map[string]int    `json:"stats"`
}

var (
	startTime = time.Now()
	version   = "1.0.0"
)

func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	status := HealthStatus{
		Status:    "healthy",
		Timestamp: time.Now(),
		Uptime:    time.Since(startTime),
		Version:   version,
		Services:  make(map[string]string),
		Stats:     make(map[string]int),
	}
	if err := checkDatabaseHealth(); err != nil {
		status.Status = "unhealthy"
		status.Services["database"] = "error: " + err.Error()
	} else {
		status.Services["database"] = "healthy"
	}
	activeConnCount := 0
	if hub != nil {
		hub.mu.RLock()
		activeConnCount = len(hub.connections)
		hub.mu.RUnlock()
	}
	status.Services["websocket"] = "healthy"
	status.Stats["activeConnections"] = activeConnCount

	if playerTracker != nil {
		playerStats := playerTracker.GetPlayerStats()
		status.Services["playerTracker"] = "healthy"
		status.Stats["totalSessions"] = playerStats["totalSessions"].(int)
		status.Stats["activePlayers"] = playerStats["activePlayers"].(int)
		status.Stats["idlePlayers"] = playerStats["idlePlayers"].(int)
		status.Stats["disconnectedPlayers"] = playerStats["disconnectedPlayers"].(int)
	} else {
		status.Services["playerTracker"] = "error: not initialized"
	}

	gamesMu.RLock()
	activeGameCount := len(games)
	gamesMu.RUnlock()
	status.Stats["activeGames"] = activeGameCount

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	status.Stats["goroutines"] = runtime.NumGoroutine()
	status.Stats["memoryMB"] = int(m.Alloc / 1024 / 1024)
	if status.Status == "healthy" {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func checkDatabaseHealth() error {
	var result int
	err := db.QueryRow("SELECT 1").Scan(&result)
	if err != nil {
		return err
	}

	if result != 1 {
		return sql.ErrNoRows
	}

	return nil
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := map[string]interface{}{
		"uptime":            time.Since(startTime).Seconds(),
		"goroutines":        runtime.NumGoroutine(),
		"activeConnections": 0,
		"activeGames":       0,
		"memory": map[string]int64{
			"alloc":      0,
			"totalAlloc": 0,
			"sys":        0,
		},
	}
	if hub != nil {
		hub.mu.RLock()
		metrics["activeConnections"] = len(hub.connections)
		hub.mu.RUnlock()
	} else {
		metrics["activeConnections"] = 0
	}
	gamesMu.RLock()
	metrics["activeGames"] = len(games)
	gamesMu.RUnlock()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	metrics["memory"] = map[string]int64{
		"alloc":      int64(m.Alloc),
		"totalAlloc": int64(m.TotalAlloc),
		"sys":        int64(m.Sys),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}
