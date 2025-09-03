let playerId = null;
let playerName = null;
let lobbies = [];

document.addEventListener('DOMContentLoaded', function() {
    initializeApp();
    fetchLobbies();
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

async function fetchLobbies() {
    const loadingElement = document.getElementById('loading-lobbies');
    const lobbiesContainer = document.getElementById('lobbies-container');
    loadingElement.style.display = 'flex';
    lobbiesContainer.innerHTML = '';
    try {
        const response = await fetch('/api/lobbies');
        if (response.ok) {
            const data = await response.json();
            if (data && Array.isArray(data)) {
                lobbies = data;
                displayLobbies();
            } else {
                lobbies = [];
                displayLobbies();
            }
        } else {
            throw new Error('Failed to fetch lobbies');
        }
    } catch (error) {
        lobbies = [];
        displayLobbies();
        showError('Failed to load lobbies. Please check if the server is running.');
    } finally {
        loadingElement.style.display = 'none';
    }
}

function displayLobbies() {
    const container = document.getElementById('lobbies-container');
    container.innerHTML = '';
    if (!lobbies || lobbies.length === 0) {
        container.innerHTML = '<p style="text-align: center; color: #fff; font-size: 1.2rem;">No lobbies available</p>';
        return;
    }
    lobbies.forEach(lobby => {
        const lobbyCard = document.createElement('div');
        lobbyCard.className = 'lobby-card';
        lobbyCard.onclick = () => joinLobby(lobby);
        lobbyCard.innerHTML = `
            <h3>${lobby.name}</h3>
            <div class="lobby-info">
                <p>Players: ${lobby.playerCount}/${lobby.maxPlayers}</p>
                <p>Status: ${lobby.status}</p>
                ${lobby.isSinglePlayer ? '<p>Single Player</p>' : ''}
            </div>
        `;
        container.appendChild(lobbyCard);
    });
}

function joinLobby(lobby) {
    if (!playerName || playerName === 'Player' || playerName.trim() === '') {
        showPlayerNameModal(lobby);
    } else {
        performJoinLobby(lobby);
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
        document.getElementById('player-name-input').value = '';
        closePlayerNameModal();
        const lobbyId = document.getElementById('player-name-modal').dataset.lobbyId;
        if (lobbyId) {
            const lobby = lobbies.find(l => l.id === lobbyId);
            if (lobby) {
                performJoinLobby(lobby);
            }
        }
    }
}

async function performJoinLobby(lobby) {
    try {
        const response = await fetch(`/api/lobby/${lobby.id}/join`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                playerId: playerId,
                playerName: playerName
            }),
        });
        if (!response.ok) {
            const errorText = await response.text();
            if (errorText && errorText.toLowerCase().includes('already in lobby')) {
                showError('You are already in this lobby!');
                return;
            }
            throw new Error(errorText || 'Server error');
        }
        localStorage.setItem('currentLobbyId', lobby.id);
        window.location.href = '/game-lobby';
    } catch (error) {
        showError('Failed to join lobby: ' + error.message);
    }
}

function showError(message) {
    document.getElementById('error-message').textContent = message;
    document.getElementById('error-modal').style.display = 'block';
}

function closeErrorModal() {
    document.getElementById('error-modal').style.display = 'none';
}

document.getElementById('player-name-form').addEventListener('submit', handlePlayerNameSubmit); 