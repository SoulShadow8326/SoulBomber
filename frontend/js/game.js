let playerId = null;
let playerName = null;
let currentLobbyId = null;
let websocket = null;
let gameState = null;
let countdownActive = false;
let countdownValue = 3;
let countdownScale = 1;
let countdownRotation = 0;
let gameOverButtonAdded = false;
let deathModalShown = false;
let wasAlive = true;
let autoRestartCalled = false;
let audioContext = null;
let explosionSound = null;
let backgroundMusic = null;
let backgroundMusicSource = null;
let masterGainNode = null;
let explosionGainNode = null;
let playerDirection = 'down';
let playerSprites = {
    back: null,
    front: null,
    left: null,
    right: null
};
let powerupSprites = {
    range1: null,
    range2: null
};
let bombSprite = null;
let canvas = null;
let ctx = null;
let backgroundImageData = null;
let pixelWorkerManager = null;
let explosionEffects = new Map();
let particleSystems = new Map();
let effectFrameCount = 0;
let performanceMetrics = {
    workerOperations: 0,
    mainThreadOperations: 0,
    lastReset: Date.now()
};

function updatePerformanceMetrics() {
    const currentTime = Date.now();
    if (currentTime - performanceMetrics.lastReset >= 10000) {
        if (performanceMetrics.workerOperations > 0) {
            console.log(`Performance: ${performanceMetrics.workerOperations} worker operations, ${performanceMetrics.mainThreadOperations} main thread operations in last 10s`);
        }
        performanceMetrics.workerOperations = 0;
        performanceMetrics.mainThreadOperations = 0;
        performanceMetrics.lastReset = currentTime;
    }
}

document.addEventListener('DOMContentLoaded', function() {
    console.log('DOM loaded');
    console.log('Initializing app');
    initializeApp();
});



function initializeApp() {
    if (!localStorage.getItem('playerId')) {
        localStorage.setItem('playerId', generatePlayerId());
    }
    playerId = localStorage.getItem('playerId');
    playerName = localStorage.getItem('playerName');
    console.log('playerName from localStorage:', playerName);
    
    const pathParts = window.location.pathname.split('/');
    console.log('Path parts:', pathParts);
    if (pathParts.length >= 3 && pathParts[1] === 'game') {
        currentLobbyId = pathParts[2];
        localStorage.setItem('currentLobbyId', currentLobbyId);
        console.log('Set currentLobbyId from URL:', currentLobbyId);
    } else {
        currentLobbyId = localStorage.getItem('currentLobbyId');
        console.log('Got currentLobbyId from localStorage:', currentLobbyId);
    }
    
    if (!currentLobbyId) {
        console.log('No currentLobbyId, redirecting to menu');
        window.location.href = '/menu';
        return;
    }
    
    initializeWebWorker();
    loadPlayerSprites();
    initializeCanvas();
    setupEventListeners();
    initializeAudio();
    connectWebSocket();
    startGameLoop();
}

function generatePlayerId() {
    return 'player_' + Math.random().toString(36).substr(2, 9);
}

function initializeWebWorker() {
    if (window.pixelWorkerManager) {
        pixelWorkerManager = window.pixelWorkerManager;
        console.log('Web worker manager initialized');
    } else {
        console.warn('Web worker manager not available, falling back to main thread rendering');
        pixelWorkerManager = null;
    }
}

function loadPlayerSprites() {
    const spriteNames = ['back', 'front', 'left', 'right'];
    let loadedCount = 0;
    
    spriteNames.forEach(name => {
        const img = new Image();
        img.onload = function() {
            loadedCount++;
            if (loadedCount === spriteNames.length) {
                console.log('All player sprites loaded');
            }
        };
        img.onerror = function() {
            console.warn('Failed to load player sprite:', name);
            playerSprites[name] = null;
        };
        img.src = `/player/${name}.png`;
        playerSprites[name] = img;
    });
    
    const range1Img = new Image();
    range1Img.onload = function() {
        console.log('Range1 powerup sprite loaded');
    };
    range1Img.onerror = function() { powerupSprites.range1 = null; };
    range1Img.src = '/powerups/bomb_range_1.png';
    powerupSprites.range1 = range1Img;
    
    const range2Img = new Image();
    range2Img.onload = function() {
        console.log('Range2 powerup sprite loaded');
    };
    range2Img.onerror = function() { powerupSprites.range2 = null; };
    range2Img.src = '/powerups/bomb_range_2.png';
    powerupSprites.range2 = range2Img;
    
    const sImg = new Image();
    sImg.onload = function() {
        console.log('Shield sprite loaded');
        powerupSprites.shield = sImg;
    };
    sImg.onerror = function() { powerupSprites.shield = null; };
    sImg.src = '/powerups/shield.png';
    
    const bombImg = new Image();
    bombImg.onload = function() {
        console.log('Bomb sprite loaded');
    };
    bombImg.onerror = function() { bombSprite = null; };
    bombImg.src = '/bombs/bomb.png';
    bombSprite = bombImg;

    
}

function isImageLoaded(img) {
    return img && img.complete && img.naturalWidth > 0;
}

function initializeCanvas() {
    canvas = document.getElementById('game-canvas');
    if (!canvas) {
        console.error('Canvas element not found');
        return;
    }
    ctx = canvas.getContext('2d');
    
    canvas.width = numCols * grid;
    canvas.height = numRows * grid;
    
    ctx.imageSmoothingEnabled = false;
    
    renderBasicBackground();
    backgroundImageData = ctx.getImageData(0, 0, canvas.width, canvas.height);
    
    renderGame();
}

function generateEnhancedBackground() {
    if (!pixelWorkerManager) {
        console.warn('Pixel worker manager not available, using basic background');
        return;
    }
    
    pixelWorkerManager.generateBackground(canvas.width, canvas.height, 'noise', [
        {r: 100, g: 100, b: 100},
        {r: 80, g: 80, b: 80},
        {r: 60, g: 60, b: 60}
    ])
    .then(result => {
        if (result && result.imageData) {
            backgroundImageData = result.imageData;
            console.log('Enhanced background generated successfully');
        } else {
            console.warn('Failed to generate enhanced background, keeping basic background');
        }
    })
    .catch(error => {
        console.warn('Failed to generate enhanced background, using basic background:', error);
    });
}

function renderBasicBackground() {
    ctx.fillStyle = '#4a4a4a';
    ctx.fillRect(0, 0, canvas.width, canvas.height);
}

function setupEventListeners() {
    document.addEventListener('keydown', handleKeyDown);
    
    const modal = document.getElementById('error-modal');
    modal.addEventListener('click', function(e) {
        if (e.target === modal) {
            closeErrorModal();
        }
    });
}

function initializeAudio() {
    try {
        audioContext = new (window.AudioContext || window.webkitAudioContext)();
        
        masterGainNode = audioContext.createGain();
        masterGainNode.connect(audioContext.destination);
        masterGainNode.gain.value = 0.35;
        
        explosionGainNode = audioContext.createGain();
        explosionGainNode.connect(audioContext.destination);
        explosionGainNode.gain.value = 1.0;
        
        document.addEventListener('click', function() {
            if (audioContext && audioContext.state === 'suspended') {
                audioContext.resume();
            }
        }, { once: true });
        
        fetch('/audio/explode.wav')
            .then(response => response.arrayBuffer())
            .then(arrayBuffer => audioContext.decodeAudioData(arrayBuffer))
            .then(audioBuffer => {
                explosionSound = audioBuffer;
            })
            .catch(error => {
                console.log('Failed to load explosion sound:', error);
            });
            
        fetch('/audio/Background.ogg')
            .then(response => response.arrayBuffer())
            .then(arrayBuffer => audioContext.decodeAudioData(arrayBuffer))
            .then(audioBuffer => {
                backgroundMusic = audioBuffer;
                playBackgroundMusic();
            })
            .catch(error => {
                console.log('Failed to load background music:', error);
            });
    } catch (error) {
        console.log('Audio context not supported:', error);
    }
}

function playBackgroundMusic() {
    if (audioContext && backgroundMusic) {
        backgroundMusicSource = audioContext.createBufferSource();
        backgroundMusicSource.buffer = backgroundMusic;
        backgroundMusicSource.connect(masterGainNode);
        backgroundMusicSource.loop = true;
        backgroundMusicSource.start(0);
    }
}

function playExplosionSound() {
    if (audioContext && explosionSound) {
        const source = audioContext.createBufferSource();
        source.buffer = explosionSound;
        source.connect(explosionGainNode);
        source.start(0);
        
        masterGainNode.gain.setValueAtTime(0.7, audioContext.currentTime);
        masterGainNode.gain.linearRampToValueAtTime(0.2, audioContext.currentTime + 0.1);
        masterGainNode.gain.linearRampToValueAtTime(0.7, audioContext.currentTime + 0.5);
    }
}

function cleanupExplosionEffects() {
    const currentTime = Date.now();
    const effectsToRemove = [];
    
    explosionEffects.forEach((effect, effectId) => {
        if (currentTime - effect.startTime > effect.duration) {
            effectsToRemove.push(effectId);
        }
    });
    
    effectsToRemove.forEach(id => explosionEffects.delete(id));
}

function cleanupParticleSystems() {
    const currentTime = Date.now();
    const systemsToRemove = [];
    
    particleSystems.forEach((system, systemId) => {
        if (currentTime - system.startTime > system.duration) {
            systemsToRemove.push(systemId);
        }
    });
    
    systemsToRemove.forEach(id => particleSystems.delete(id));
}

function cleanupOldEffects() {
    const currentTime = Date.now();
    
    for (const [effectId, effect] of explosionEffects.entries()) {
        if (currentTime - effect.startTime > effect.duration) {
            explosionEffects.delete(effectId);
        }
    }

    for (const [systemId, system] of particleSystems.entries()) {
        if (currentTime - system.startTime > system.duration || system.particles.length === 0) {
            particleSystems.delete(systemId);
        }
    }
    
    for (const [systemId, system] of particleSystems.entries()) {
        if (currentTime - system.startTime > 5000) { 
            particleSystems.delete(systemId);
        }
    }
}

function connectWebSocket() {
    if (websocket && websocket.readyState === WebSocket.OPEN) {
        return; 
    }
    
    if (websocket) {
        websocket.close();
    }
    
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = protocol + '//' + window.location.host + '/ws';
    console.log('Connecting to WebSocket:', wsUrl);
    websocket = new WebSocket(wsUrl);
    
    websocket.onopen = function() {
        console.log('Game WebSocket connected');
        updateConnectionStatus('Connected');
        
        if (window.reconnectTimer) {
            clearTimeout(window.reconnectTimer);
            window.reconnectTimer = null;
        }
        
        setTimeout(() => {
            console.log('Sending joinLobby');
            websocket.send(JSON.stringify({
                type: 'joinLobby',
                payload: {
                    lobbyId: currentLobbyId,
                    playerId: playerId,
                    playerName: playerName
                }
            }));

            if (!autoRestartCalled) {
                autoRestartCalled = true;
                setTimeout(() => {
                    restartGame();
                }, 300);
            }
        }, 500);
    };
    
    websocket.onmessage = function(event) {
        const message = JSON.parse(event.data);
        console.log('Game WebSocket received:', message);
        handleWebSocketMessage(message);
    };
    
    websocket.onclose = function(event) {
        console.log('Game WebSocket closed:', event.code, event.reason);
        updateConnectionStatus('Disconnected');
        
        if (event.code !== 1000 && !window.reconnectTimer) {
            console.log('Scheduling reconnection in 2 seconds...');
            window.reconnectTimer = setTimeout(() => {
                console.log('Attempting to reconnect...');
                connectWebSocket();
            }, 2000);
        }
    };
    
    websocket.onerror = function(error) {
        console.log('Game WebSocket error:', error);
        updateConnectionStatus('Error');
    };
}

function handleWebSocketMessage(message) {
    switch (message.type) {
        case 'joined':
            setTimeout(() => {
                websocket.send(JSON.stringify({
                    type: 'startGame',
                    payload: {
                        lobbyId: currentLobbyId,
                        playerId: playerId
                    }
                }));
            }, 100);
            break;
        case 'gameState':
            const wasWaiting = !gameState || gameState.status !== 'playing';
            const oldStartTime = gameState ? gameState.startTime : null;
            const oldExplosions = gameState ? Object.keys(gameState.explosions || {}).length : 0;

            const prevPlayers = (gameState && gameState.players) ? JSON.parse(JSON.stringify(gameState.players)) : null;
            gameState = message.payload;

            if (gameState.players && prevPlayers) {
                Object.keys(gameState.players).forEach(pid => {
                    const p = gameState.players[pid];
                    const prev = prevPlayers[pid];
                    if (p && p.position && prev && prev.position) {
                        const dRow = p.position.row - prev.position.row;
                        const dCol = p.position.col - prev.position.col;
                        if (dRow < 0) p.direction = 'up';
                        else if (dRow > 0) p.direction = 'down';
                        else if (dCol < 0) p.direction = 'left';
                        else if (dCol > 0) p.direction = 'right';
                        else if (!p.direction) p.direction = 'down';
                    } else if (!p.direction) {
                        p.direction = 'down';
                    }
                });
            } else if (gameState.players) {
                Object.values(gameState.players).forEach(p => { if (!p.direction) p.direction = 'down'; });
            }
            
            if (gameState.players && gameState.players[playerId]) {
                const player = gameState.players[playerId];
                if (player.alive) {
                    const controlButtons = document.querySelectorAll('.control-buttons button');
                    controlButtons.forEach(button => {
                        button.disabled = false;
                        button.style.opacity = '1';
                    });
                }
            }
            
            const newExplosions = Object.keys(gameState.explosions || {}).length;
            if (newExplosions > oldExplosions) {
                playExplosionSound();
            }
            
            updateGamePlayersList();
            renderGame();
            
            if ((wasWaiting && gameState.status === 'playing') || 
                (gameState.status === 'playing' && gameState.startTime !== oldStartTime)) {
                startCountdown();
            }
            break;
        case 'gameStarted':
            break;
        case 'powerupSpawn':
            if (gameState) {
                gameState.powerups = message.payload;
            }
            break;
        case 'left':
            break;
        case 'pong':
            break;
        case 'playerInfo':
            if (message.payload && message.payload.playerName) {
                playerName = message.payload.playerName;
                console.log('Got player name from server:', playerName);
                websocket.send(JSON.stringify({
                    type: 'joinGame',
                    payload: {
                        lobbyId: currentLobbyId
                    }
                }));
            }
            break;
        case 'error':
            console.log('Game WebSocket error received:', message.payload);
            if (message.payload.includes('lobby not found')) {
                localStorage.removeItem('currentLobbyId');
                window.location.href = '/menu';
            } else if (typeof message.payload === 'string' && message.payload.toLowerCase().includes('dash')) {
                showDashToast(message.payload);
            } else if (!message.payload.includes('invalid move')) {
                showError(message.payload);
            }
            break;
        default:
            break;
    }
}

function updateGamePlayersList() {
    const playersList = document.getElementById('game-players-list');
    if (!playersList || !gameState || !gameState.players) return;
    
    playersList.innerHTML = '';
    
    const players = Object.values(gameState.players);
    players.sort((a, b) => b.score - a.score);
    
    if (gameState.startTime) {
        const gameTimeElapsed = Math.floor((new Date() - new Date(gameState.startTime)) / 1000);
        const gameTimeRemaining = Math.max(0, 120 - gameTimeElapsed);
        const minutes = Math.floor(gameTimeRemaining / 60);
        const seconds = gameTimeRemaining % 60;
        
        const timerElement = document.createElement('div');
        timerElement.className = 'game-timer';
        timerElement.innerHTML = `
            <span class="timer-label">Time:</span>
            <span class="timer-value">${minutes}:${seconds.toString().padStart(2, '0')}</span>
        `;
        playersList.appendChild(timerElement);
    }
    
    players.forEach(player => {
        const playerElement = document.createElement('div');
        playerElement.className = 'player-item';
        
        let powerupInfo = '';
        if (player.powerups && Object.keys(player.powerups).length > 0) {
            const powerupType = Object.keys(player.powerups)[0];
            const powerup = player.powerups[powerupType];
            const timeLeft = Math.max(0, Math.ceil((new Date(powerup.endTime) - new Date()) / 1000));
            
            let powerupName = '';
            if (powerupType === 'bomb_range') {
                powerupName = `Range Lv${powerup.level}`;
            }
            
            powerupInfo = `
                <div class="player-powerup">
                    <span class="powerup-name">${powerupName}</span>
                    <span class="powerup-timer">${timeLeft}s</span>
                </div>
            `;
        }
        
        playerElement.innerHTML = `
            <span class="player-name">${player.name || 'Unknown'}</span>
            <span class="player-score">${player.score}</span>
            ${powerupInfo}
        `;
        playersList.appendChild(playerElement);
    });
}



function updateConnectionStatus(status) {
    const statusElements = document.querySelectorAll('.connection-status');
    statusElements.forEach(element => {
        element.textContent = `Status: ${status}`;
    });
}

function renderGame() {
    if (!gameState || !gameState.board) {
        if (!backgroundImageData) {
            renderBasicBackground();
        } else {
            ctx.putImageData(backgroundImageData, 0, 0);
        }
        
        ctx.fillStyle = 'black';
        ctx.font = '20px Arial';
        ctx.textAlign = 'center';
        ctx.fillText('Waiting for game state...', canvas.width / 2, canvas.height / 2);
        return;
    }
    
    if (!backgroundImageData) {
        renderBasicBackground();
    } else {
        ctx.putImageData(backgroundImageData, 0, 0);
    }
    
    if (gameState.powerups) {
        Object.values(gameState.powerups).forEach(powerup => {
            renderPowerup(powerup);
        });
    }
    
    gameState.board.forEach((row, rowIndex) => {
        row.forEach((cell, colIndex) => {
            const x = colIndex * grid;
            const y = rowIndex * grid;
            
            if (cell === types.wall) {
                ctx.drawImage(indestructibleWallCanvas, x, y);
            } else if (cell === types.softWall) {
                ctx.drawImage(destructibleWallCanvas, x, y);
            }
        });
    });
    
    if (gameState.players) {
        Object.values(gameState.players).forEach(player => {
            if (player.alive) {
                renderPlayer(player);
            }
        });
    }
    
    if (gameState.bombs) {
        Object.values(gameState.bombs).forEach(bomb => {
            renderBomb(bomb);
        });
    }
    
    if (gameState.explosions) {
        Object.values(gameState.explosions).forEach(explosion => {
            renderExplosion(explosion);
            createExplosionEffect(explosion, explosion.position.col * grid, explosion.position.row * grid);
        });
    }
    
    updateParticleSystems();
    renderParticleSystems();
    
    if (countdownActive) {
        renderCountdown();
    }
    
    if (gameState.players && gameState.players[playerId]) {
        const player = gameState.players[playerId];
        wasAlive = player.alive;
    }
    
    if (gameState.status === 'finished') {
        renderGameOverScreen();
    }
    
    effectFrameCount++;
    
    cleanupOldEffects();
}

function renderPlayer(player) {
    const x = (player.position.col + 0.5) * grid;
    const y = (player.position.row + 0.5) * grid;
    let spriteDir = player.direction || (player.id === playerId ? playerDirection : 'down');
    let spriteName = 'front';
    if (spriteDir === 'up') spriteName = 'back';
    if (spriteDir === 'down') spriteName = 'front';
    if (spriteDir === 'left') spriteName = 'left';
    if (spriteDir === 'right') spriteName = 'right';

    if (player.alive && isImageLoaded(playerSprites[spriteName])) {
        const sprite = playerSprites[spriteName];
        const spriteSize = grid * 0.8;
        const offsetX = x - spriteSize / 2;
        const offsetY = y - spriteSize / 2;

        ctx.save();
        ctx.drawImage(sprite, offsetX, offsetY, spriteSize, spriteSize);
        ctx.restore();
    } else {
        const radius = grid * 0.35;
        ctx.save();
        ctx.fillStyle = '#4d4fd6';
        ctx.beginPath();
        ctx.arc(x, y, radius, 0, 2 * Math.PI);
        ctx.fill();
        ctx.restore();
    }
    if (player.Shield) {
        ctx.save();
        ctx.strokeStyle = '#00ffff';
        ctx.lineWidth = 3;
        ctx.beginPath();
        ctx.arc(x, y, grid * 0.42, 0, Math.PI * 2);
        ctx.stroke();
        ctx.restore();
    }
    
    renderPlayerName(player, x, y);
}

function renderPlayerName(player, x, y) {
    const name = player.name || 'Player';
    const nameY = y - grid * 0.6;
    const nameWidth = name.length * 8;
    const padding = 8;
    const totalWidth = nameWidth + padding * 2;
    
    ctx.save();
    
    ctx.fillStyle = 'rgba(0, 0, 0, 0.9)';
    ctx.fillRect(x - totalWidth/2 - 2, nameY - 12, totalWidth + 4, 24);
    
    ctx.fillStyle = '#ff4444';
    ctx.fillRect(x - totalWidth/2, nameY - 10, totalWidth, 20);
    
    ctx.fillStyle = '#000000';
    ctx.fillRect(x - totalWidth/2 + 2, nameY - 8, totalWidth - 4, 16);
    
    ctx.fillStyle = '#ffff00';
    ctx.font = 'bold 10px "Press Start 2P", monospace';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(name, x, nameY);
    
    ctx.restore();
}

function renderBomb(bomb) {
    const x = (bomb.position.col + 0.5) * grid;
    const y = (bomb.position.row + 0.5) * grid;
    
    const size = grid * 0.8;
    ctx.save();
    ctx.imageSmoothingEnabled = false;
    
    if (isImageLoaded(bombSprite)) {
        ctx.drawImage(bombSprite, x - size/2, y - size/2, size, size);
    }
    
    ctx.restore();
}

function renderExplosion(explosion) {
    const x = explosion.position.col * grid;
    const y = explosion.position.row * grid;
    
    ctx.save();
    ctx.imageSmoothingEnabled = false;
    
    ctx.fillStyle = '#FFB347'; 
    ctx.fillRect(x, y, grid, grid);

    ctx.fillStyle = '#ff6200ff';
    ctx.fillRect(x, y + 6, grid, grid - 12);
    ctx.fillRect(x + 6, y, grid - 12, grid);

    ctx.fillStyle = '#F4E4BC'; 
    ctx.fillRect(x, y + 12, grid, grid - 24);
    ctx.fillRect(x + 12, y, grid - 24, grid);
    ctx.restore();
}

function createExplosionEffect(explosion, x, y) {
    const effectId = explosion.id;
    const radius = grid;
    const centerX = radius / 2;
    const centerY = radius / 2;
    
    if (pixelWorkerManager) {
        performanceMetrics.workerOperations++;
        pixelWorkerManager.createExplosionEffect(radius, radius, centerX, centerY, radius, 1.0)
            .then(result => {
                if (result && result.imageData) {
                    explosionEffects.set(effectId, {
                        imageData: result.imageData,
                        startTime: Date.now(),
                        duration: 1000
                    });
                }
            })
            .catch(error => {
                console.warn('Failed to create explosion effect with worker, falling back to basic rendering:', error);
                explosionEffects.set(effectId, {
                    imageData: null,
                    startTime: Date.now(),
                    duration: 1000,
                    useBasicRendering: true
                });
            });
    } else {
        performanceMetrics.mainThreadOperations++;
        explosionEffects.set(effectId, {
            imageData: null,
            startTime: Date.now(),
            duration: 1000,
            useBasicRendering: true
        });
    }
}

function renderBasicExplosion(x, y) {
    ctx.save();
    ctx.fillStyle = '#D72B16';
    ctx.fillRect(x, y, grid, grid);

    ctx.fillStyle = '#F39642';
    ctx.fillRect(x, y + 6, grid, grid - 12);
    ctx.fillRect(x + 6, y, grid - 12, grid);

    ctx.fillStyle = '#FFE5A8';
    ctx.fillRect(x, y + 12, grid, grid - 24);
    ctx.fillRect(x + 12, y, grid - 24, grid);
    ctx.restore();
}

function createParticleExplosion(x, y) {
    if (!pixelWorkerManager) return;
    
    const particleCount = 15; 
    const particles = [];
    
    for (let i = 0; i < particleCount; i++) {
        const angle = (Math.PI * 2 * i) / particleCount;
        const speed = 1 + Math.random() * 2;
        const life = 0.3 + Math.random() * 0.7;
        
        particles.push({
            x: x + grid / 2,
            y: y + grid / 2,
            vx: Math.cos(angle) * speed,
            vy: Math.sin(angle) * speed,
            life: life,
            maxLife: life,
            r: 255,
            g: 100 + Math.random() * 155,
            b: 0,
            alpha: 1.0
        });
    }
    
    const systemId = `particle_${Date.now()}_${Math.random()}`;
    particleSystems.set(systemId, {
        particles: particles,
        startTime: Date.now(),
        duration: 1500,
        currentImageData: null
    });
}

function updateParticleSystems() {
    if (!pixelWorkerManager) return;
    
    const currentTime = Date.now();
    const systemsToRemove = [];
    
    particleSystems.forEach((system, systemId) => {
        const elapsed = currentTime - system.startTime;
        if (elapsed > system.duration) {
            systemsToRemove.push(systemId);
            return;
        }
        
        const deltaTime = 0.016;
        let particlesChanged = false;
        
        system.particles.forEach(particle => {
            const oldX = particle.x;
            const oldY = particle.y;
            const oldAlpha = particle.alpha;
            
            particle.x += particle.vx;
            particle.y += particle.vy;
            particle.life -= deltaTime;
            particle.alpha = Math.max(0, particle.life / particle.maxLife);
            particle.vy += 0.05;
            
            if (Math.abs(particle.x - oldX) > 0.1 || Math.abs(particle.y - oldY) > 0.1 || Math.abs(particle.alpha - oldAlpha) > 0.05) {
                particlesChanged = true;
            }
        });
        
        const aliveParticles = system.particles.filter(p => p.life > 0);
        if (aliveParticles.length > 0) {
            system.particles = aliveParticles;
            
            if (particlesChanged || !system.currentImageData) {
                renderParticleSystem(system, systemId);
            }
        } else {
            systemsToRemove.push(systemId);
        }
    });
    
    systemsToRemove.forEach(id => particleSystems.delete(id));
}

function renderParticleSystem(system, systemId) {
    if (!pixelWorkerManager) {
        performanceMetrics.mainThreadOperations++;
        return;
    }
    
    if (system.currentImageData === null && system.particles.length > 0) {
        performanceMetrics.workerOperations++;
        pixelWorkerManager.createParticleSystem(canvas.width, canvas.height, system.particles, effectFrameCount)
            .then(result => {
                if (result && result.imageData) {
                    system.currentImageData = result.imageData;
                }
            })
            .catch(error => {
                console.warn('Failed to update particle system with worker, using fallback:', error);
                const imageData = new ImageData(canvas.width, canvas.height);
                const pixels = imageData.data;
                
                for (let i = 0; i < pixels.length; i += 4) {
                    pixels[i + 3] = 0;
                }
                system.particles.forEach(particle => {
                    const x = Math.floor(particle.x);
                    const y = Math.floor(particle.y);
                    
                    if (x >= 0 && x < canvas.width && y >= 0 && y < canvas.height) {
                        const index = (y * canvas.width + x) * 4;
                        const alpha = Math.floor(particle.alpha * 255);
                        
                        pixels[index] = particle.r;
                        pixels[index + 1] = particle.g;
                        pixels[index + 2] = particle.b;
                        pixels[index + 3] = alpha;
                    }
                });
                
                system.currentImageData = imageData;
            });
    }
}

function renderParticleSystems() {
    particleSystems.forEach(system => {
        if (system.currentImageData) {
            ctx.putImageData(system.currentImageData, 0, 0);
        } else if (!pixelWorkerManager || !pixelWorkerManager.isSupported) {
            ctx.save();
            ctx.globalCompositeOperation = 'source-over';
            
            system.particles.forEach(particle => {
                if (particle.life > 0) {
                    ctx.globalAlpha = particle.alpha;
                    ctx.fillStyle = `rgb(${particle.r}, ${particle.g}, ${particle.b})`;
                    ctx.beginPath();
                    ctx.arc(particle.x, particle.y, 2, 0, Math.PI * 2);
                    ctx.fill();
                }
            });
            
            ctx.restore();
        }
    });
}

function renderPowerup(powerup) {
    const x = (powerup.position.col + 0.5) * grid;
    const y = (powerup.position.row + 0.5) * grid;
    
    const size = grid * 0.6;
    ctx.save();
    ctx.imageSmoothingEnabled = false;
    
    if (powerup.type === 'bomb_range') {
        if (powerup.level === 1 && isImageLoaded(powerupSprites.range1)) {
            ctx.drawImage(powerupSprites.range1, x - size/2, y - size/2, size, size);
        } else if (powerup.level === 2 && isImageLoaded(powerupSprites.range2)) {
            ctx.drawImage(powerupSprites.range2, x - size/2, y - size/2, size, size);
        }
    }
    if (powerup.type === 'shield') {
        if (isImageLoaded(powerupSprites.shield)) {
            ctx.drawImage(powerupSprites.shield, x - size/2, y - size/2, size, size);
        } else {
            ctx.save();
            ctx.fillStyle = '#00ffff';
            ctx.fillRect(x - size/2, y - size/2, size, size);
            ctx.restore();
        }
    }
    
    if (pixelWorkerManager && powerupSprites.range1) {
        addPowerupGlowEffect(powerup, x, y, size);
    }
    
    ctx.restore();
}

function addPowerupGlowEffect(powerup, x, y, size) {
    const glowRadius = size * 0.8;
    const glowIntensity = 0.3 + Math.sin(Date.now() * 0.005) * 0.2;
    
    ctx.save();
    ctx.globalCompositeOperation = 'screen';
    ctx.globalAlpha = glowIntensity;
    
    const gradient = ctx.createRadialGradient(x, y, 0, x, y, glowRadius);
    if (powerup.type === 'bomb_range') {
        gradient.addColorStop(0, 'rgba(255, 255, 0, 1)');
        gradient.addColorStop(0.5, 'rgba(255, 200, 0, 0.5)');
        gradient.addColorStop(1, 'rgba(255, 100, 0, 0)');
    }
    
    ctx.fillStyle = gradient;
    ctx.fillRect(x - glowRadius, y - glowRadius, glowRadius * 2, glowRadius * 2);
    ctx.restore();
}

function renderCountdown() {
    ctx.save();
    
    const x = canvas.width / 2;
    const y = canvas.height / 2;
    
    ctx.translate(x, y);
    ctx.scale(countdownScale, countdownScale);
    ctx.rotate(countdownRotation);
    
    ctx.fillStyle = '#ff6b35';
    ctx.font = 'bold 140px "Press Start 2P", "Courier New", monospace';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    
    ctx.shadowColor = '#000';
    ctx.shadowBlur = 10;
    ctx.shadowOffsetX = 4;
    ctx.shadowOffsetY = 4;
    ctx.fillText(countdownValue.toString(), 0, 0);
    
    ctx.shadowColor = '#fff';
    ctx.shadowBlur = 5;
    ctx.shadowOffsetX = -2;
    ctx.shadowOffsetY = -2;
    ctx.fillStyle = '#ffd700';
    ctx.fillText(countdownValue.toString(), 0, 0);
    
    ctx.imageSmoothingEnabled = false;
    
    ctx.restore();
}

function showDeathModal() {
    if (deathModalShown || document.getElementById('death-modal')) {
        return;
    }
    
    deathModalShown = true;
    
    const modal = document.createElement('div');
    modal.id = 'death-modal';
    modal.style.cssText = `
        position: fixed;
        top: 50%;
        left: 50%;
        transform: translate(-50%, -50%);
        background: #1a1a1a;
        border: 4px solid #ff0000;
        border-radius: 0;
        padding: 30px;
        z-index: 1000;
        text-align: center;
        box-shadow: 0 0 0 4px #000, 0 0 0 8px #ff0000;
        image-rendering: pixelated;
        font-family: 'Courier New', monospace;
    `;
    
    const title = document.createElement('h2');
    title.textContent = 'YOU DIED';
    title.style.cssText = `
        color: #ff0000;
        font-family: 'Courier New', monospace;
        font-size: 36px;
        font-weight: bold;
        margin: 0 0 15px 0;
        text-shadow: 2px 2px 0 #000, 4px 4px 0 #000;
        letter-spacing: 2px;
        text-transform: uppercase;
    `;
    
    const message = document.createElement('p');
    message.textContent = 'SPECTATE MODE';
    message.style.cssText = `
        color: #ffff00;
        font-family: 'Courier New', monospace;
        font-size: 16px;
        margin: 0 0 20px 0;
        text-shadow: 1px 1px 0 #000;
        letter-spacing: 1px;
    `;
    
    modal.appendChild(title);
    modal.appendChild(message);
    document.body.appendChild(modal);
    
    setTimeout(() => {
        const modalToRemove = document.getElementById('death-modal');
        if (modalToRemove) {
            modalToRemove.remove();
        }
        deathModalShown = false;
    }, 3000);
}

function renderGameOverScreen() {
    ctx.save();
    
    ctx.fillStyle = 'rgba(0, 0, 0, 0.9)';
    ctx.fillRect(0, 0, canvas.width, canvas.height);
    
    const centerX = canvas.width / 2;
    let currentY = 150;
    
    ctx.fillStyle = '#ff6b35';
    ctx.font = 'bold 60px "Courier New", monospace';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    
    ctx.shadowColor = '#000';
    ctx.shadowBlur = 8;
    ctx.shadowOffsetX = 3;
    ctx.shadowOffsetY = 3;
    ctx.fillText('GAME OVER', centerX, currentY);
    
    currentY += 120;
    
    ctx.fillStyle = '#ffd700';
    ctx.font = 'bold 40px "Courier New", monospace';
    ctx.fillText('FINAL SCORES', centerX, currentY);
    
    currentY += 80;
    
    if (gameState.players) {
        const players = Object.values(gameState.players).sort((a, b) => b.score - a.score);
        
        players.forEach((player, index) => {
            const isWinner = index === 0 && player.score > 0;
            const rank = index + 1;
            
            ctx.fillStyle = isWinner ? '#ffd700' : '#ffffff';
            ctx.font = 'bold 30px "Courier New", monospace';
            
            const rankText = `${rank}.`;
            const nameText = player.name || 'Unknown';
            const scoreText = `${player.score}`;
            
            const rankX = centerX - 200;
            const nameX = centerX - 50;
            const scoreX = centerX + 150;
            
            ctx.fillText(rankText, rankX, currentY);
            ctx.fillText(nameText, nameX, currentY);
            ctx.fillText(scoreText, scoreX, currentY);
            
            if (isWinner) {
                ctx.fillStyle = '#ff6b35';
                ctx.font = 'bold 20px "Courier New", monospace';
                ctx.fillText('WINNER!', centerX + 250, currentY);
            }
            
            currentY += 50;
        });
    }
    
    currentY += 60;
    
    const buttonWidth = 200;
    const buttonHeight = 50;
    const buttonX = centerX - buttonWidth / 2;
    const buttonY = currentY;
    
    ctx.fillStyle = '#2977F5';
    ctx.fillRect(buttonX, buttonY, buttonWidth, buttonHeight);
    
    ctx.fillStyle = '#ffffff';
    ctx.font = 'bold 20px "Courier New", monospace';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText('MAIN MENU', centerX, buttonY + buttonHeight / 2);
    
    ctx.restore();
    
    if (!gameOverButtonAdded) {
        addGameOverButton(buttonX, buttonY, buttonWidth, buttonHeight);
    }
}

function addGameOverButton(buttonX, buttonY, buttonWidth, buttonHeight) {
    const canvasRect = canvas.getBoundingClientRect();
    const scaleX = canvas.width / canvasRect.width;
    const scaleY = canvas.height / canvasRect.height;
    
    const realButtonX = buttonX / scaleX;
    const realButtonY = buttonY / scaleY;
    const realButtonWidth = buttonWidth / scaleX;
    const realButtonHeight = buttonHeight / scaleY;
    
    canvas.addEventListener('click', function gameOverClickHandler(e) {
        const rect = canvas.getBoundingClientRect();
        const x = e.clientX - rect.left;
        const y = e.clientY - rect.top;
        
        if (x >= realButtonX && x <= realButtonX + realButtonWidth &&
            y >= realButtonY && y <= realButtonY + realButtonHeight) {
            goToMainMenu();
        }
    });
    
    gameOverButtonAdded = true;
}

function startCountdown() {
    countdownActive = true;
    countdownValue = 3;
    countdownScale = 1;
    countdownRotation = 0;
    gameOverButtonAdded = false;
    deathModalShown = false;
    wasAlive = true;
    
    const controlButtons = document.querySelectorAll('.control-buttons button');
    controlButtons.forEach(button => {
        button.disabled = true;
        button.style.opacity = '0.5';
    });
    
    const countdownInterval = setInterval(() => {
        countdownValue--;
        countdownScale = 1.5;
        countdownRotation = 0.1;
        
        setTimeout(() => {
            countdownScale = 1;
            countdownRotation = 0;
        }, 200);
        
        if (countdownValue <= 0) {
            countdownActive = false;
            clearInterval(countdownInterval);
            
            const controlButtons = document.querySelectorAll('.control-buttons button');
            controlButtons.forEach(button => {
                button.disabled = false;
                button.style.opacity = '1';
            });
        }
    }, 1000);
}

let lastMoveTime = 0;
const MOVE_COOLDOWN = 150; 

function movePlayer(direction) {
    if (!websocket || websocket.readyState !== WebSocket.OPEN) {
        return;
    }
    
    if (!gameState || !gameState.players || !gameState.players[playerId] || !gameState.players[playerId].alive) {
        return;
    }
    
    const currentTime = Date.now();
    if (currentTime - lastMoveTime < MOVE_COOLDOWN) {
        return;
    }
    
    const player = gameState.players[playerId];
    const currentPos = player.position;
    let newPos = { row: currentPos.row, col: currentPos.col };
    
    switch (direction) {
        case 'up':
            newPos.row--;
            break;
        case 'down':
            newPos.row++;
            break;
        case 'left':
            newPos.col--;
            break;
        case 'right':
            newPos.col++;
            break;
    }
    
    if (newPos.row < 0 || newPos.row >= gameState.board.length || 
        newPos.col < 0 || newPos.col >= gameState.board[0].length) {
        return; 
    }
    
    if (gameState.board[newPos.row][newPos.col] !== 0) {
        return; 
    }
    
    if (gameState.bombs) {
        for (const bombId in gameState.bombs) {
            const bomb = gameState.bombs[bombId];
            if (bomb.position.row === newPos.row && bomb.position.col === newPos.col && bomb.playerId !== playerId) {
                return; 
            }
        }
    }
    
    lastMoveTime = currentTime;
    
    websocket.send(JSON.stringify({
        type: 'move',
        payload: {
            direction: direction,
            playerId: playerId
        }
    }));
}

let lastBombTime = 0;
const BOMB_COOLDOWN = 200;

function placeBomb() {
    if (!websocket || websocket.readyState !== WebSocket.OPEN) {
        return;
    }
    
    if (!gameState || !gameState.players || !gameState.players[playerId] || !gameState.players[playerId].alive) {
        return;
    }
    
    const currentTime = Date.now();
    if (currentTime - lastBombTime < BOMB_COOLDOWN) {
        return;
    }
    
    lastBombTime = currentTime;
    
    websocket.send(JSON.stringify({
        type: 'placeBomb',
        payload: {
            playerId: playerId
        }
    }));
}

function handleKeyDown(e) {
    if (countdownActive) {
        e.preventDefault();
        return;
    }
    
    switch (e.key) {
        case 'ArrowUp':
        case 'w':
        case 'W':
            e.preventDefault();
            playerDirection = 'up';
            movePlayer('up');
            triggerButtonAnimation('up');
            break;
        case 'ArrowDown':
        case 's':
        case 'S':
            e.preventDefault();
            playerDirection = 'down';
            movePlayer('down');
            triggerButtonAnimation('down');
            break;
        case 'ArrowLeft':
        case 'a':
        case 'A':
            e.preventDefault();
            playerDirection = 'left';
            movePlayer('left');
            triggerButtonAnimation('left');
            break;
        case 'ArrowRight':
        case 'd':
        case 'D':
            e.preventDefault();
            playerDirection = 'right';
            movePlayer('right');
            triggerButtonAnimation('right');
            break;
        case ' ':
            e.preventDefault();
            placeBomb();
            triggerButtonAnimation('bomb');
            break;
        case 'r':
        case 'R':
            e.preventDefault();
            sendRemoteDetonate();
            break;
        case 'Shift':
        case 'ShiftLeft':
        case 'ShiftRight':
            e.preventDefault();
            sendDash(playerDirection);
            break;
    }
}

function sendRemoteDetonate() {
    if (!websocket || websocket.readyState !== WebSocket.OPEN) return;
    websocket.send(JSON.stringify({ type: 'remoteDetonate', payload: {} }));
}

function sendDash(direction) {
    if (!websocket || websocket.readyState !== WebSocket.OPEN) return;
    websocket.send(JSON.stringify({ type: 'dash', payload: { direction: direction } }));
}

function showDashToast(text) {
    const toast = document.getElementById('dash-toast');
    const txt = document.getElementById('dash-toast-text');
    if (!toast) return;
    if (txt && text) txt.textContent = text;
    toast.style.display = 'block';
    setTimeout(() => { toast.style.display = 'none'; }, 1400);
}

function triggerButtonAnimation(direction) {
    const buttons = document.querySelectorAll('.control-buttons button');
    let targetButton = null;
    
    switch (direction) {
        case 'up':
            targetButton = buttons[0];
            break;
        case 'down':
            targetButton = buttons[1];
            break;
        case 'left':
            targetButton = buttons[2];
            break;
        case 'right':
            targetButton = buttons[3];
            break;
        case 'bomb':
            targetButton = buttons[4];
            break;
    }
    
    if (targetButton) {
        targetButton.classList.add('active');
        setTimeout(() => {
            targetButton.classList.remove('active');
        }, 150);
    }
}

function restartGame() {
    if (websocket && websocket.readyState === WebSocket.OPEN) {
        websocket.send(JSON.stringify({
            type: 'restartGame',
            payload: {
                lobbyId: currentLobbyId,
                playerId: playerId
            }
        }));
    }
}

function startGameLoop() {
    let lastUpdate = Date.now();
    let lastCleanup = Date.now();
    let lastPerformanceUpdate = Date.now();
    
    function gameLoop() {
        renderGame();
        
        const now = Date.now();
        if (now - lastUpdate >= 1000) {
            updateGamePlayersList();
            lastUpdate = now;
        }
        
        if (now - lastCleanup >= 5000) {
            cleanupExplosionEffects();
            cleanupParticleSystems();
            lastCleanup = now;
        }
        
        if (now - lastPerformanceUpdate >= 10000) {
            updatePerformanceMetrics();
            lastPerformanceUpdate = now;
        }
        
        requestAnimationFrame(gameLoop);
    }
    gameLoop();
}

function showError(message) {
    const modal = document.getElementById('error-modal');
    const errorMessage = document.getElementById('error-message');
    errorMessage.textContent = message;
    modal.style.display = 'block';
}

function closeErrorModal() {
    const modal = document.getElementById('error-modal');
    modal.style.display = 'none';
}

function goToMainMenu() {
    cleanupWebWorkerResources();
    localStorage.removeItem('currentLobbyId');
    window.location.href = '/menu';
}

function cleanupWebWorkerResources() {
    if (pixelWorkerManager) {
        explosionEffects.clear();
        particleSystems.clear();
        effectFrameCount = 0;
    }
}

window.addEventListener('beforeunload', function() {
    cleanupWebWorkerResources();
    if (pixelWorkerManager) {
        pixelWorkerManager.destroy();
    }
});
 