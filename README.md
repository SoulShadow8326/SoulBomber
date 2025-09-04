# SoulBomber

Bomberman Clone! A multiplayer bomberman game made for the web built using golang, each player controls one bomberman, you get points by blowing up other players or the breakable tiles, player with most points win. powerups exist, right now only increase range and a shield. The goal is basically to have fun and blow up each other!

## Features

- Multiplayer lobbies: create, list, join, and leave game lobbies.
- Real-time gameplay using WebSockets.
- Destructible and indestructible tiles and chain-reaction explosions.
- Power-ups: bomb range increases and shields.
- Bomb mechanics: timed explosions and score for destroying players and tiles.
- Persistent lobby data stored in SQLite (`backend/soulbomber.db`).
- Static frontend served from the `frontend/` directory (HTML, CSS, JS).
- Server-side logging and health endpoints for monitoring.

