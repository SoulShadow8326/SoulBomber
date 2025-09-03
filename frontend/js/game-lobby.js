let playerId = null;
let playerName = null;
let currentLobbyId = null;
let websocket = null;
let gameState = null;

document.addEventListener('DOMContentLoaded', function() {
    const ready = initializeApp();
    setupEventListeners();
    if (ready) {
        connectWebSocket();
    }
});

window.addEventListener('beforeunload', function() {});

function initializeApp() {
    if (!localStorage.getItem('playerId')) {
        localStorage.setItem('playerId', generatePlayerId());
    }
    playerId = localStorage.getItem('playerId');
    playerName = localStorage.getItem('playerName');
    currentLobbyId = localStorage.getItem('currentLobbyId');
    if (!currentLobbyId) {
        window.location.href = '/menu';
        return false;
    }
    document.getElementById('lobby-id').textContent = `Lobby: ${currentLobbyId}`;
    if (!playerName || playerName === 'Player' || playerName.trim() === '') {
        changePlayerName();
        return false;
    }
    return true;
}

function generatePlayerId() {
    return 'player_' + Math.random().toString(36).substr(2, 9);
}

function connectWebSocket() {
    if (websocket) {
        if (websocket.readyState === WebSocket.OPEN) return;
        try { websocket.close(); } catch (e) {}
    }
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = protocol + '//' + window.location.host + '/ws';
    console.log('Connecting to WebSocket:', wsUrl);
    websocket = new WebSocket(wsUrl);
    websocket.onopen = function() {
        console.log('WebSocket connected');
        updateConnectionStatus('Connected');
        console.log('Sending joinLobby with:', { lobbyId: currentLobbyId, playerId: playerId, playerName: playerName });
        websocket.send(JSON.stringify({
            type: 'joinLobby',
            payload: {
                lobbyId: currentLobbyId,
                playerId: playerId,
                playerName: playerName
            }
        }));
    };
    websocket.onmessage = function(event) {
        try {
            const message = JSON.parse(event.data);
            console.log('Received WebSocket message:', message);
            handleWebSocketMessage(message);
        } catch (error) {
            console.error('Failed to parse WebSocket message:', error);
            console.log('Raw message data:', event.data);
        }
    };
    websocket.onclose = function() {
        console.log('WebSocket disconnected');
        updateConnectionStatus('Disconnected');
    };
    websocket.onerror = function(error) {
        console.error('WebSocket error:', error);
        updateConnectionStatus('Error');
    };
}

function handleWebSocketMessage(message) {
    switch (message.type) {
        case 'joined':
            console.log('Successfully joined lobby:', message.payload);
            if (websocket && websocket.readyState === WebSocket.OPEN) {
                websocket.send(JSON.stringify({
                    type: 'requestLobbyUpdate',
                    payload: { lobbyId: currentLobbyId }
                }));
            }
            break;
        case 'gameState':
            gameState = message.payload;
            window.location.href = `/game/${currentLobbyId}`;
            break;
        case 'lobbyUpdate':
            console.log('Received lobbyUpdate:', message.payload);
            if (message.payload && message.payload.players) {
                gameState = {
                    players: message.payload.players,
                    isSinglePlayer: message.payload.lobby ? message.payload.lobby.isSinglePlayer : false
                };
                console.log('Updated gameState:', gameState);
            }
            updateGameLobby();
            break;
        case 'gameStarted':
            window.location.href = '/game';
            break;
        case 'error':
            showError(message.payload || 'Unknown error');
            break;
    }
}

function updateConnectionStatus(status) {
    const statusElements = document.querySelectorAll('.connection-status');
    statusElements.forEach(element => {
        element.textContent = `Status: ${status}`;
    });
}

function updateGameLobby() {
    if (!gameState) return;
    const startBtn = document.getElementById('start-btn');
    if (startBtn) {
        startBtn.disabled = !(gameState.players && Object.keys(gameState.players).length >= 1);
    }
    updatePlayersGrid();
}

function updatePlayersGrid() {
    const container = document.getElementById('players-grid');
    if (!container) return;
    container.innerHTML = '';
    const maxPlayers = 4;
    const players = gameState.players || {};
    console.log('updatePlayersGrid - players:', players);
    console.log('Player objects:', Object.values(players));
    for (let i = 0; i < maxPlayers; i++) {
        const playerSlot = document.createElement('div');
        playerSlot.className = 'player-slot empty';
        const player = Object.values(players).find(p => p.slot === i);
        console.log(`Slot ${i}:`, player);
        if (player) {
            playerSlot.className = 'player-slot occupied';
            console.log('Rendering player:', player.name, 'in slot', i);
            playerSlot.innerHTML = `
                <div class="player-info">
                    <div class="player-avatar"></div>
                    <div class="player-name">${player.name}</div>
                    <div class="player-status">${player.alive ? 'Alive' : 'Dead'}</div>
                    ${player.isAI ? `<div class="ai-difficulty">${player.aiDifficulty}</div>` : ''}
                </div>
            `;
        } else {
            playerSlot.innerHTML = `
                <div class="player-info">
                    <div class="empty-avatar">
                        <svg viewBox="0 0 24 24" fill="currentColor">
                            <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8z"/>
                        </svg>
                    </div>
                    <div class="empty-slot">Empty Slot</div>
                    <div class="empty-text">Waiting for player...</div>
                </div>
            `;
        }
        container.appendChild(playerSlot);
    }
}

function startGame() {
    if (websocket && websocket.readyState === WebSocket.OPEN) {
        websocket.send(JSON.stringify({
            type: 'startGame',
            payload: {
                lobbyId: currentLobbyId,
                playerId: playerId
            }
        }));
    }
}

function leaveLobby() {
    if (websocket && websocket.readyState === WebSocket.OPEN) {
        websocket.send(JSON.stringify({
            type: 'leaveLobby',
            payload: {
                lobbyId: currentLobbyId,
                playerId: playerId
            }
        }));
    }
    currentLobbyId = null;
    localStorage.removeItem('currentLobbyId');
    if (websocket) try { websocket.close(); } catch (e) {}
    window.location.href = '/menu';
}

function showError(message) {
    document.getElementById('error-message').textContent = message;
    document.getElementById('error-modal').style.display = 'block';
}

function setupEventListeners() {
    document.getElementById('player-name-form').addEventListener('submit', handlePlayerNameSubmit);
}

function changePlayerName() {
    const modal = document.getElementById('player-name-modal');
    const input = document.getElementById('player-name-input');
    input.value = playerName || '';
    modal.style.display = 'block';
}

function closePlayerNameModal() {
    document.getElementById('player-name-modal').style.display = 'none';
}

function handlePlayerNameSubmit(e) {
    e.preventDefault();
    const name = document.getElementById('player-name-input').value.trim();
    if (name) {
        playerName = name;
        localStorage.setItem('playerName', name);
        closePlayerNameModal();
        if (!websocket || websocket.readyState !== WebSocket.OPEN) {
            connectWebSocket();
        } else {
            websocket.send(JSON.stringify({
                type: 'updatePlayerName',
                payload: {
                    playerId: playerId,
                    playerName: playerName
                }
            }));
        }
    }
}

function closeErrorModal() {
    document.getElementById('error-modal').style.display = 'none';
} 