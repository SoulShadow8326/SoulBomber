


class PixelWorkerManager {
    constructor() {
        this.worker = null;
        this.callbacks = new Map();
        this.requestId = 0;
        this.isSupported = typeof Worker !== 'undefined';
        
        if (this.isSupported) {
            this.initializeWorker();
        }
    }
    
    initializeWorker() {
        try {
            this.worker = new Worker('/js/pixel-worker.js');
            this.worker.onmessage = this.handleWorkerMessage.bind(this);
            this.worker.onerror = this.handleWorkerError.bind(this);
        } catch (error) {
            console.warn('Web Worker not supported, falling back to main thread:', error);
            this.isSupported = false;
        }
    }
    
    handleWorkerMessage(event) {
        const { type, imageData, id, error, frame } = event.data;
        
        if (error) {
            console.error('Worker error:', error);
            return;
        }
        
        const callback = this.callbacks.get(id);
        if (callback) {
            callback({ type, imageData, frame });
            this.callbacks.delete(id);
        }
    }
    
    handleWorkerError(error) {
        console.error('Worker error:', error);
        this.isSupported = false;
    }
    
    generateRequestId() {
        return `req_${++this.requestId}_${Date.now()}`;
    }
    sendMessage(type, data) {
        return new Promise((resolve, reject) => {
            if (!this.isSupported || !this.worker) {
                this.processInMainThread(type, data).then(resolve).catch(reject);
                return;
            }
            
            const id = this.generateRequestId();
            this.callbacks.set(id, resolve);
            
            this.worker.postMessage({
                type: type,
                data: { ...data, id }
            });
            setTimeout(() => {
                if (this.callbacks.has(id)) {
                    this.callbacks.delete(id);
                    reject(new Error('Worker timeout'));
                }
            }, 1000);
        });
    }
    async processInMainThread(type, data) {
        const workerFunctions = await this.getWorkerFunctions();
        
        switch (type) {
            case 'explosion_effect':
                return workerFunctions.handleExplosionEffect(data);
            case 'particle_system':
                return workerFunctions.handleParticleSystem(data);
            case 'image_processing':
                return workerFunctions.handleImageProcessing(data);
            case 'background_generation':
                return workerFunctions.handleBackgroundGeneration(data);
            case 'texture_manipulation':
                return workerFunctions.handleTextureManipulation(data);
            default:
                throw new Error('Unknown operation type');
        }
    }
    async getWorkerFunctions() {
        return {
            handleExplosionEffect: (data) => {
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
                
                return { type: 'explosion_effect', imageData: imageData };
            },
            
            handleParticleSystem: (data) => {
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
                
                return { type: 'particle_system', imageData: imageData, frame: frame };
            },
            
            handleImageProcessing: (data) => {
                return { type: 'image_processing', imageData: data.imageData };
            },
            
            handleBackgroundGeneration: (data) => {
                return { type: 'background_generation', imageData: data.imageData };
            },
            
            handleTextureManipulation: (data) => {
                return { type: 'texture_manipulation', imageData: data.imageData };
            }
        };
    }
    async createExplosionEffect(width, height, centerX, centerY, radius, intensity = 1.0) {
        return this.sendMessage('explosion_effect', {
            width,
            height,
            centerX,
            centerY,
            radius,
            intensity
        });
    }
    async createParticleSystem(width, height, particles, frame = 0) {
        return this.sendMessage('particle_system', {
            width,
            height,
            particles,
            frame
        });
    }
    async processImage(imageData, operation, params = {}) {
        return this.sendMessage('image_processing', {
            imageData,
            operation,
            params
        });
    }
    async generateBackground(width, height, pattern = 'noise', colors = [{r: 144, g: 238, b: 144}]) {
        return this.sendMessage('background_generation', {
            width,
            height,
            pattern,
            colors
        });
    }
    async manipulateTexture(imageData, operation, params = {}) {
        return this.sendMessage('texture_manipulation', {
            imageData,
            operation,
            params
        });
    }
    async blurImage(imageData, radius = 1) {
        return this.processImage(imageData, 'blur', { radius });
    }
    async adjustBrightness(imageData, factor = 1.0) {
        return this.processImage(imageData, 'brightness', { factor });
    }
    
    async adjustContrast(imageData, factor = 1.0) {
        return this.processImage(imageData, 'contrast', { factor });
    }
    
    async adjustSaturation(imageData, factor = 1.0) {
        return this.processImage(imageData, 'saturation', { factor });
    }
    
    async applyCrackEffect(imageData, lines = 3, width = 2) {
        return this.manipulateTexture(imageData, 'crack', { lines, width });
    }
    
    async applyBurnEffect(imageData, intensity = 0.5) {
        return this.manipulateTexture(imageData, 'burn', { intensity });
    }
    
    async applyFadeEffect(imageData, alpha = 0.5) {
        return this.manipulateTexture(imageData, 'fade', { alpha });
    }
    
    async createParticleExplosion(width, height, centerX, centerY, particleCount = 50) {
        const particles = [];
        
        for (let i = 0; i < particleCount; i++) {
            const angle = (Math.PI * 2 * i) / particleCount;
            const speed = 2 + Math.random() * 3;
            const life = 0.5 + Math.random() * 0.5;
            
            particles.push({
                x: centerX,
                y: centerY,
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
        
        return this.createParticleSystem(width, height, particles);
    }
    
    async updateParticleSystem(width, height, particles, deltaTime = 0.016) {
        particles.forEach(particle => {
            particle.x += particle.vx;
            particle.y += particle.vy;
            particle.life -= deltaTime;
            particle.alpha = particle.life / particle.maxLife;
            particle.vy += 0.1;
        });
        const aliveParticles = particles.filter(p => p.life > 0);
        
        return this.createParticleSystem(width, height, aliveParticles);
    }
    
    destroy() {
        if (this.worker) {
            this.worker.terminate();
            this.worker = null;
        }
        this.callbacks.clear();
    }
}


window.pixelWorkerManager = new PixelWorkerManager(); 