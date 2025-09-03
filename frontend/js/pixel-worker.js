const WORKER_TYPES = {
    EXPLOSION_EFFECT: 'explosion_effect',
    PARTICLE_SYSTEM: 'particle_system',
    IMAGE_PROCESSING: 'image_processing',
    BACKGROUND_GENERATION: 'background_generation',
    TEXTURE_MANIPULATION: 'texture_manipulation'
};

self.addEventListener('message', function(e) {
    const { type, data } = e.data;
    
    switch (type) {
        case WORKER_TYPES.EXPLOSION_EFFECT:
            handleExplosionEffect(data);
            break;
        case WORKER_TYPES.PARTICLE_SYSTEM:
            handleParticleSystem(data);
            break;
        case WORKER_TYPES.IMAGE_PROCESSING:
            handleImageProcessing(data);
            break;
        case WORKER_TYPES.BACKGROUND_GENERATION:
            handleBackgroundGeneration(data);
            break;
        case WORKER_TYPES.TEXTURE_MANIPULATION:
            handleTextureManipulation(data);
            break;
        default:
            self.postMessage({ error: 'Unknown worker type' });
    }
});


function handleExplosionEffect(data) {
    const { width, height, centerX, centerY, radius, intensity } = data;
    const imageData = new ImageData(width, height);
    const pixels = imageData.data;
    for (let y = 0; y < height; y++) {
        for (let x = 0; x < width; x++) {
            const distance = Math.sqrt((x - centerX) ** 2 + (y - centerY) ** 2);
            const index = (y * width + x) * 4;
            
            if (distance <= radius) {
                const intensityFactor = 1 - (distance / radius);
                const alpha = Math.floor(intensity * intensityFactor * 255);
                
                pixels[index] = 215;     
                pixels[index + 1] = 43;  
                pixels[index + 2] = 22;  
                pixels[index + 3] = alpha; 
            } else {
                pixels[index + 3] = 0;
            }
        }
    }
    
    self.postMessage({
        type: WORKER_TYPES.EXPLOSION_EFFECT,
        imageData: imageData,
        id: data.id
    });
}


function handleParticleSystem(data) {
    const { width, height, particles, frame } = data;
    
    const imageData = new ImageData(width, height);
    const pixels = imageData.data;
    
    for (let i = 0; i < pixels.length; i += 4) {
        pixels[i + 3] = 0; 
    }
    
    particles.forEach(particle => {
        const x = Math.floor(particle.x);
        const y = Math.floor(particle.y);
        
        if (x >= 0 && x < width && y >= 0 && y < height) {
            const index = (y * width + x) * 4;
            const alpha = Math.floor(particle.alpha * 255);
            
            pixels[index] = particle.r;
            pixels[index + 1] = particle.g;
            pixels[index + 2] = particle.b;
            pixels[index + 3] = alpha;
        }
    });
    
    self.postMessage({
        type: WORKER_TYPES.PARTICLE_SYSTEM,
        imageData: imageData,
        frame: frame,
        id: data.id
    });
}


function handleImageProcessing(data) {
    const { imageData, operation, params } = data;
    const pixels = imageData.data;
    const newImageData = new ImageData(imageData.width, imageData.height);
    const newPixels = newImageData.data;
    
    switch (operation) {
        case 'blur':
            applyBlur(pixels, newPixels, imageData.width, imageData.height, params.radius || 1);
            break;
        case 'brightness':
            applyBrightness(pixels, newPixels, params.factor || 1.0);
            break;
        case 'contrast':
            applyContrast(pixels, newPixels, params.factor || 1.0);
            break;
        case 'saturation':
            applySaturation(pixels, newPixels, params.factor || 1.0);
            break;
        default:
            newPixels.set(pixels);
    }
    
    self.postMessage({
        type: WORKER_TYPES.IMAGE_PROCESSING,
        imageData: newImageData,
        operation: operation,
        id: data.id
    });
}


function handleBackgroundGeneration(data) {
    const { width, height, pattern, colors } = data;
    
    const imageData = new ImageData(width, height);
    const pixels = imageData.data;
    for (let y = 0; y < height; y++) {
        for (let x = 0; x < width; x++) {
            const index = (y * width + x) * 4;
            
            const noise = (Math.sin(x * 0.1) + Math.cos(y * 0.1)) * 0.5 + 0.5;
            const colorIndex = Math.floor(noise * colors.length);
            const color = colors[colorIndex] || colors[0];
            
            pixels[index] = color.r;
            pixels[index + 1] = color.g;
            pixels[index + 2] = color.b;
            pixels[index + 3] = 255;
        }
    }
    
    self.postMessage({
        type: WORKER_TYPES.BACKGROUND_GENERATION,
        imageData: imageData,
        id: data.id
    });
}


function handleTextureManipulation(data) {
    const { imageData, operation, params } = data;
    const pixels = imageData.data;
    const newImageData = new ImageData(imageData.width, imageData.height);
    const newPixels = newImageData.data;
    
    switch (operation) {
        case 'crack':
            applyCrackEffect(pixels, newPixels, imageData.width, imageData.height, params);
            break;
        case 'burn':
            applyBurnEffect(pixels, newPixels, imageData.width, imageData.height, params);
            break;
        case 'fade':
            applyFadeEffect(pixels, newPixels, params.alpha || 0.5);
            break;
        default:
            newPixels.set(pixels);
    }
    
    self.postMessage({
        type: WORKER_TYPES.TEXTURE_MANIPULATION,
        imageData: newImageData,
        operation: operation,
        id: data.id
    });
}


function applyBlur(sourcePixels, targetPixels, width, height, radius) {
    const kernel = createGaussianKernel(radius);
    const kernelSize = kernel.length;
    const halfKernel = Math.floor(kernelSize / 2);
    for (let y = 0; y < height; y++) {
        for (let x = 0; x < width; x++) {
            let r = 0, g = 0, b = 0, a = 0, weightSum = 0;
            
            for (let i = 0; i < kernelSize; i++) {
                const sampleX = Math.max(0, Math.min(width - 1, x + i - halfKernel));
                const sourceIndex = (y * width + sampleX) * 4;
                const weight = kernel[i];
                
                r += sourcePixels[sourceIndex] * weight;
                g += sourcePixels[sourceIndex + 1] * weight;
                b += sourcePixels[sourceIndex + 2] * weight;
                a += sourcePixels[sourceIndex + 3] * weight;
                weightSum += weight;
            }
            
            const targetIndex = (y * width + x) * 4;
            targetPixels[targetIndex] = r / weightSum;
            targetPixels[targetIndex + 1] = g / weightSum;
            targetPixels[targetIndex + 2] = b / weightSum;
            targetPixels[targetIndex + 3] = a / weightSum;
        }
    }
}

function createGaussianKernel(radius) {
    const size = radius * 2 + 1;
    const kernel = new Array(size);
    const sigma = radius / 3;
    let sum = 0;
    
    for (let i = 0; i < size; i++) {
        const x = i - radius;
        kernel[i] = Math.exp(-(x * x) / (2 * sigma * sigma));
        sum += kernel[i];
    }
    for (let i = 0; i < size; i++) {
        kernel[i] /= sum;
    }
    
    return kernel;
}

function applyBrightness(sourcePixels, targetPixels, factor) {
    for (let i = 0; i < sourcePixels.length; i += 4) {
        targetPixels[i] = Math.min(255, Math.max(0, sourcePixels[i] * factor));
        targetPixels[i + 1] = Math.min(255, Math.max(0, sourcePixels[i + 1] * factor));
        targetPixels[i + 2] = Math.min(255, Math.max(0, sourcePixels[i + 2] * factor));
        targetPixels[i + 3] = sourcePixels[i + 3];
    }
}

function applyContrast(sourcePixels, targetPixels, factor) {
    const midpoint = 128;
    for (let i = 0; i < sourcePixels.length; i += 4) {
        targetPixels[i] = Math.min(255, Math.max(0, (sourcePixels[i] - midpoint) * factor + midpoint));
        targetPixels[i + 1] = Math.min(255, Math.max(0, (sourcePixels[i + 1] - midpoint) * factor + midpoint));
        targetPixels[i + 2] = Math.min(255, Math.max(0, (sourcePixels[i + 2] - midpoint) * factor + midpoint));
        targetPixels[i + 3] = sourcePixels[i + 3];
    }
}

function applySaturation(sourcePixels, targetPixels, factor) {
    for (let i = 0; i < sourcePixels.length; i += 4) {
        const r = sourcePixels[i];
        const g = sourcePixels[i + 1];
        const b = sourcePixels[i + 2];
        
        const gray = 0.299 * r + 0.587 * g + 0.114 * b;
        
        targetPixels[i] = Math.min(255, Math.max(0, gray + (r - gray) * factor));
        targetPixels[i + 1] = Math.min(255, Math.max(0, gray + (g - gray) * factor));
        targetPixels[i + 2] = Math.min(255, Math.max(0, gray + (b - gray) * factor));
        targetPixels[i + 3] = sourcePixels[i + 3];
    }
}


function applyCrackEffect(sourcePixels, targetPixels, width, height, params) {
    const crackLines = params.lines || 3;
    const crackWidth = params.width || 2;
    targetPixels.set(sourcePixels);
    for (let i = 0; i < crackLines; i++) {
        const startX = Math.random() * width;
        const startY = Math.random() * height;
        const endX = Math.random() * width;
        const endY = Math.random() * height;
        
        drawLine(targetPixels, width, height, startX, startY, endX, endY, crackWidth, [0, 0, 0, 255]);
    }
}

function applyBurnEffect(sourcePixels, targetPixels, width, height, params) {
    const burnIntensity = params.intensity || 0.5;
    
    for (let i = 0; i < sourcePixels.length; i += 4) {
        const r = sourcePixels[i];
        const g = sourcePixels[i + 1];
        const b = sourcePixels[i + 2];
        targetPixels[i] = Math.min(255, r + burnIntensity * 50);
        targetPixels[i + 1] = Math.max(0, g - burnIntensity * 30);
        targetPixels[i + 2] = Math.max(0, b - burnIntensity * 50);
        targetPixels[i + 3] = sourcePixels[i + 3];
    }
}

function applyFadeEffect(sourcePixels, targetPixels, alpha) {
    for (let i = 0; i < sourcePixels.length; i += 4) {
        targetPixels[i] = sourcePixels[i];
        targetPixels[i + 1] = sourcePixels[i + 1];
        targetPixels[i + 2] = sourcePixels[i + 2];
        targetPixels[i + 3] = Math.floor(sourcePixels[i + 3] * alpha);
    }
}

function drawLine(pixels, width, height, x1, y1, x2, y2, lineWidth, color) {
    const dx = Math.abs(x2 - x1);
    const dy = Math.abs(y2 - y1);
    const sx = x1 < x2 ? 1 : -1;
    const sy = y1 < y2 ? 1 : -1;
    let err = dx - dy;
    
    let x = x1;
    let y = y1;
    
    while (true) {
        for (let wy = -Math.floor(lineWidth/2); wy <= Math.floor(lineWidth/2); wy++) {
            for (let wx = -Math.floor(lineWidth/2); wx <= Math.floor(lineWidth/2); wx++) {
                const px = Math.floor(x + wx);
                const py = Math.floor(y + wy);
                
                if (px >= 0 && px < width && py >= 0 && py < height) {
                    const index = (py * width + px) * 4;
                    pixels[index] = color[0];
                    pixels[index + 1] = color[1];
                    pixels[index + 2] = color[2];
                    pixels[index + 3] = color[3];
                }
            }
        }
        
        if (x === x2 && y === y2) break;
        
        const e2 = 2 * err;
        if (e2 > -dy) {
            err -= dy;
            x += sx;
        }
        if (e2 < dx) {
            err += dx;
            y += sy;
        }
    }
} 