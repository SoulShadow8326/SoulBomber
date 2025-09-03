let backgroundCanvas = document.getElementById('background-canvas');
let bgCtx = backgroundCanvas.getContext('2d');
let grid = 40;
let numRows = 15;
let numCols = 15;
let types = {
    empty: 0,
    wall: 1,
    softWall: 2,
    bomb: 3,
    explosion: 4
};

let destructibleWallCanvas = null;
let indestructibleWallCanvas = null;

function initializeBackground() {
    backgroundCanvas.width = window.innerWidth;
    backgroundCanvas.height = window.innerHeight;
    
    createWallCanvases();
    renderBackground();
}

function createWallCanvases() {
    destructibleWallCanvas = document.createElement('canvas');
    const destructibleWall = destructibleWallCanvas.getContext('2d');
    destructibleWallCanvas.width = destructibleWallCanvas.height = grid;

    destructibleWall.fillStyle = 'black';
    destructibleWall.fillRect(0, 0, grid, grid);
    destructibleWall.fillStyle = '#ab783a';

    destructibleWall.fillRect(1, 1, grid - 2, 20);
    destructibleWall.fillRect(0, 23, 20, 18);
    destructibleWall.fillRect(22, 23, 42, 18);
    destructibleWall.fillRect(0, 43, 42, 20);
    destructibleWall.fillRect(44, 43, 20, 20);

    indestructibleWallCanvas = document.createElement('canvas');
    const indestructibleWall = indestructibleWallCanvas.getContext('2d');
    indestructibleWallCanvas.width = indestructibleWallCanvas.height = grid;

    indestructibleWall.fillStyle = 'black';
    indestructibleWall.fillRect(0, 0, grid, grid);
    indestructibleWall.fillStyle = 'white';
    indestructibleWall.fillRect(0, 0, grid - 2, grid - 2);
    indestructibleWall.fillStyle = '#ab783a';
    indestructibleWall.fillRect(2, 2, grid - 4, grid - 4);
}

function generateBackgroundLevel() {
    const cells = [];
    const template = [];

    for (let row = 0; row < numRows; row++) {
        cells[row] = [];
        template[row] = [];
        
        for (let col = 0; col < numCols; col++) {
            if (row === 0 || row === numRows - 1 || col === 0 || col === numCols - 1) {
                template[row][col] = types.wall;
            } else if (row % 2 === 0 && col % 2 === 0) {
                template[row][col] = types.wall;
            } else {
                template[row][col] = null;
            }
        }
    }

    for (let row = 0; row < numRows; row++) {
        for (let col = 0; col < numCols; col++) {
            if (!template[row][col] && Math.random() < 0.85) {
                cells[row][col] = types.softWall;
            } else if (template[row][col] === types.wall) {
                cells[row][col] = types.wall;
            }
        }
    }

    return cells;
}

function renderBackground() {
    bgCtx.clearRect(0, 0, backgroundCanvas.width, backgroundCanvas.height);
    
    if (!destructibleWallCanvas || !indestructibleWallCanvas) {
        return;
    }
    
    const cells = generateBackgroundLevel();
    const scale = Math.max(backgroundCanvas.width / (numCols * grid), backgroundCanvas.height / (numRows * grid)) * 1.5;
    
    bgCtx.save();
    bgCtx.translate(backgroundCanvas.width / 2, backgroundCanvas.height / 2);
    bgCtx.scale(scale, scale);
    bgCtx.translate(-(numCols * grid) / 2, -(numRows * grid) / 2);
    
    cells.forEach((row, rowIndex) => {
        row.forEach((cell, colIndex) => {
            const x = colIndex * grid;
            const y = rowIndex * grid;
            
            if (cell === types.wall) {
                bgCtx.drawImage(indestructibleWallCanvas, x, y);
            } else if (cell === types.softWall) {
                bgCtx.drawImage(destructibleWallCanvas, x, y);
            }
        });
    });
    
    bgCtx.restore();
    
    if (window.pixelWorkerManager) {
        createEnhancedBackground();
    }
}
async function createEnhancedBackground() {
    if (!window.pixelWorkerManager) return;
    try {
        const colors = [
            { r: 144, g: 238, b: 144 }, 
            { r: 124, g: 218, b: 124 }, 
            { r: 164, g: 258, b: 164 }  
        ];
        
        const result = await window.pixelWorkerManager.generateBackground(
            backgroundCanvas.width, 
            backgroundCanvas.height, 
            'noise', 
            colors
        );
        bgCtx.globalAlpha = 0.3;
        bgCtx.putImageData(result.imageData, 0, 0);
        bgCtx.globalAlpha = 1.0;
    } catch (error) {
        console.warn('Failed to create enhanced background:', error);
    }
}

window.addEventListener('resize', () => {
    backgroundCanvas.width = window.innerWidth;
    backgroundCanvas.height = window.innerHeight;
    renderBackground();
});

document.addEventListener('DOMContentLoaded', initializeBackground); 