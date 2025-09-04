package main

import (
	"net/http"
	"strings"
	"time"
)

func setupRoutes() {
	http.HandleFunc("/api/lobbies", handleLobbiesRoute)
	http.HandleFunc("/api/lobby/", handleLobbyRoutesRoute)
	http.HandleFunc("/api/startSinglePlayer", handleStartSinglePlayerRoute)
	http.HandleFunc("/ws", handleWebSocketRoute)
	http.HandleFunc("/health", handleHealthCheckRoute)
	http.HandleFunc("/metrics", handleMetricsRoute)
	http.HandleFunc("/api/players/stats", handlePlayerStatsRoute)

	http.HandleFunc("/css/", handleStaticFiles(http.StripPrefix("/css/", http.FileServer(http.Dir("../frontend/css")))))
	http.HandleFunc("/js/", handleStaticFiles(http.StripPrefix("/js/", http.FileServer(http.Dir("../frontend/js")))))
	http.HandleFunc("/player/", handleStaticFiles(http.StripPrefix("/player/", http.FileServer(http.Dir("../frontend/player")))))
	http.HandleFunc("/powerups/", handleStaticFiles(http.StripPrefix("/powerups/", http.FileServer(http.Dir("../frontend/powerups")))))
	http.HandleFunc("/bombs/", handleStaticFiles(http.StripPrefix("/bombs/", http.FileServer(http.Dir("../frontend/bombs")))))
	http.HandleFunc("/audio/", handleStaticFiles(http.StripPrefix("/audio/", http.FileServer(http.Dir("../frontend/audio")))))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/game/") {
			http.ServeFile(w, r, "../frontend/game.html")
			return
		}

		if r.URL.Path == "/" {
			http.ServeFile(w, r, "../frontend/menu.html")
			return
		}
		switch r.URL.Path {
		case "/menu":
			http.ServeFile(w, r, "../frontend/menu.html")
		case "/lobby-list":
			http.ServeFile(w, r, "../frontend/lobby-list.html")
		case "/create-lobby":
			http.ServeFile(w, r, "../frontend/create-lobby.html")
		case "/game-lobby":
			http.ServeFile(w, r, "../frontend/game-lobby.html")
		case "/game":
			http.Redirect(w, r, "/create-lobby", http.StatusTemporaryRedirect)
		default:
			if strings.HasPrefix(r.URL.Path, "/game/") {
				logInfo("Serving game.html for path", "path", r.URL.Path)
				http.ServeFile(w, r, "../frontend/game.html")
				return
			}
			if r.URL.Path == "/game.html" {
				http.Redirect(w, r, "/game", http.StatusMovedPermanently)
				return
			}
			if r.URL.Path == "/menu.html" {
				http.Redirect(w, r, "/menu", http.StatusMovedPermanently)
				return
			}
			if r.URL.Path == "/lobby-list.html" {
				http.Redirect(w, r, "/lobby-list", http.StatusMovedPermanently)
				return
			}
			if r.URL.Path == "/create-lobby.html" {
				http.Redirect(w, r, "/create-lobby", http.StatusMovedPermanently)
				return
			}
			if r.URL.Path == "/game-lobby.html" {
				http.Redirect(w, r, "/game-lobby", http.StatusMovedPermanently)
				return
			}
			http.NotFound(w, r)
		}
	})
}

func handleLobbiesRoute(w http.ResponseWriter, r *http.Request) {
	RecoveryMiddleware(
		LoggingMiddleware(
			RateLimitMiddleware(100, time.Minute)(
				CORSMiddleware(handleLobbies),
			),
		),
	)(w, r)
}

func handleLobbyRoutesRoute(w http.ResponseWriter, r *http.Request) {
	RecoveryMiddleware(
		LoggingMiddleware(
			RateLimitMiddleware(100, time.Minute)(
				CORSMiddleware(handleLobbyRoutes),
			),
		),
	)(w, r)
}

func handleStartSinglePlayerRoute(w http.ResponseWriter, r *http.Request) {
	RecoveryMiddleware(
		LoggingMiddleware(
			RateLimitMiddleware(50, time.Minute)(
				CORSMiddleware(handleStartSinglePlayer),
			),
		),
	)(w, r)
}

func handleWebSocketRoute(w http.ResponseWriter, r *http.Request) {
	handleWebSocket(w, r)
}

func handleHealthCheckRoute(w http.ResponseWriter, r *http.Request) {
	RecoveryMiddleware(
		LoggingMiddleware(handleHealthCheck),
	)(w, r)
}

func handleMetricsRoute(w http.ResponseWriter, r *http.Request) {
	RecoveryMiddleware(
		LoggingMiddleware(handleMetrics),
	)(w, r)
}

func handlePlayerStatsRoute(w http.ResponseWriter, r *http.Request) {
	RecoveryMiddleware(
		LoggingMiddleware(
			RateLimitMiddleware(10, time.Minute)(
				CORSMiddleware(handlePlayerStats),
			),
		),
	)(w, r)
}

func handleStaticFiles(fs http.Handler) http.HandlerFunc {
	return RecoveryMiddleware(
		LoggingMiddleware(fs.ServeHTTP),
	)
}
