let playerId = null;
let playerName = null;

document.addEventListener('DOMContentLoaded', function() {
    initializeApp();
    setupEventListeners();
});

window.addEventListener('beforeunload', function() {});

function initializeApp() {
    if (!localStorage.getItem('playerId')) {
        localStorage.setItem('playerId', generatePlayerId());
    }
    playerId = localStorage.getItem('playerId');
    playerName = localStorage.getItem('playerName');
}

function generatePlayerId() {
    return 'player_' + Math.random().toString(36).substr(2, 9);
}

function setupEventListeners() {
    document.getElementById('create-lobby-form').addEventListener('submit', handleCreateLobby);
    document.getElementById('player-name-form').addEventListener('submit', handlePlayerNameSubmit);
}

async function handleCreateLobby(e) {
    e.preventDefault();
    const lobbyName = document.getElementById('lobby-name').value.trim();
    if (!lobbyName) {
        showError('Please enter a lobby name');
        return;
    }
    try {
        const response = await fetch('/api/lobbies', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                name: lobbyName,
                isSinglePlayer: false,
                aiPlayers: []
            }),
        });
        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(errorText || 'Server error');
        }
        const lobby = await response.json();
        localStorage.setItem('currentLobbyId', lobby.id);
        if (!playerName || playerName === 'Player' || playerName.trim() === '') {
            showPlayerNameModal(lobby);
        } else {
            window.location.href = '/game-lobby';
        }
    } catch (error) {
        showError('Failed to create lobby: ' + error.message);
    }
}

function showNamePromptModal() {
    const modal = document.getElementById('player-name-modal');
    modal.style.display = 'block';
    modal.dataset.lobbyId = '';
}

function showPlayerNameModal(lobby) {
    const modal = document.getElementById('player-name-modal');
    modal.style.display = 'block';
    modal.dataset.lobbyId = lobby.id;
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
        const lobbyId = document.getElementById('player-name-modal').dataset.lobbyId;
        if (lobbyId) {
            window.location.href = '/game-lobby';
        }
    }
}

function showError(message) {
    document.getElementById('error-message').textContent = message;
    document.getElementById('error-modal').style.display = 'block';
}

function closeErrorModal() {
    document.getElementById('error-modal').style.display = 'none';
} 