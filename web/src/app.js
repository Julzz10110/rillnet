class RillNetApp {
    constructor() {
        this.signalClient = new SignalClient();
        this.streamManager = new StreamManager(this.signalClient);
        this.metricsCollector = new MetricsCollector();
        this.logger = new Logger('RillNet');
        
        this.isPublisher = false;
        this.isSubscriber = false;
        this.currentStreamId = null;
        
        this.initializeApp();
    }

    async initializeApp() {
        try {
            this.logger.info('Initializing RillNet application...');
            
            // Initialize components
            await this.initializeSignalConnection();
            this.initializeEventListeners();
            this.initializeUI();
            this.startMetricsCollection();
            
            this.logger.success('Application initialized successfully');
            
        } catch (error) {
            this.logger.error('Failed to initialize application: ' + error.message);
        }
    }

    async initializeSignalConnection() {
        try {
            await this.signalClient.connect();
            this.setConnectionStatus('connected');
            
            // Listen for incoming streams
            this.signalClient.on('streamAvailable', (streamInfo) => {
                this.onStreamAvailable(streamInfo);
            });
            
            this.signalClient.on('streamEnded', (streamId) => {
                this.onStreamEnded(streamId);
            });
            
        } catch (error) {
            this.logger.error('Signal connection failed: ' + error.message);
            this.setConnectionStatus('error');
        }
    }

    initializeEventListeners() {
        // Publisher controls
        document.getElementById('startPublisher').addEventListener('click', () => this.startPublisher());
        document.getElementById('stopPublisher').addEventListener('click', () => this.stopPublisher());
        document.getElementById('switchCamera').addEventListener('click', () => this.switchCamera());

        // Subscriber controls
        document.getElementById('joinStream').addEventListener('click', () => this.joinStream());
        document.getElementById('leaveStream').addEventListener('click', () => this.leaveStream());
        document.getElementById('refreshStream').addEventListener('click', () => this.refreshStreams());

        // Quality selection
        document.querySelectorAll('.quality-btn').forEach(btn => {
            btn.addEventListener('click', (e) => this.changeQuality(e.target.dataset.quality));
        });

        // Stream selection
        document.getElementById('streamList').addEventListener('change', (e) => {
            this.selectStream(e.target.value);
        });

        // Log controls
        document.getElementById('clearLog').addEventListener('click', () => this.logger.clear());
        document.getElementById('exportLog').addEventListener('click', () => this.exportLog());
        document.getElementById('autoScroll').addEventListener('click', (e) => {
            this.logger.setAutoScroll(e.target.classList.contains('active'));
        });

        // Log filters
        document.querySelectorAll('.log-filters input').forEach(checkbox => {
            checkbox.addEventListener('change', () => this.updateLogFilters());
        });
    }

    initializeUI() {
        this.updatePublisherUI(false);
        this.updateSubscriberUI(false);
        this.updateMetrics({
            peers: 0,
            streams: 0,
            bandwidth: 0,
            latency: 0,
            packetLoss: '0%',
            connections: 0
        });
    }

    // Publisher methods
    async startPublisher() {
        try {
            this.logger.info('Starting publisher...');
            
            const stream = await this.streamManager.startPublisher();
            this.currentStreamId = await this.signalClient.publishStream(stream);
            
            this.isPublisher = true;
            this.updatePublisherUI(true);
            this.updateStreamList();
            
            this.logger.success(`Publisher started with stream ID: ${this.currentStreamId}`);
            
        } catch (error) {
            this.logger.error('Failed to start publisher: ' + error.message);
        }
    }

    async stopPublisher() {
        try {
            this.logger.info('Stopping publisher...');
            
            await this.streamManager.stopPublisher();
            if (this.currentStreamId) {
                await this.signalClient.unpublishStream(this.currentStreamId);
            }
            
            this.isPublisher = false;
            this.currentStreamId = null;
            this.updatePublisherUI(false);
            this.updateStreamList();
            
            this.logger.success('Publisher stopped');
            
        } catch (error) {
            this.logger.error('Error stopping publisher: ' + error.message);
        }
    }

    async switchCamera() {
        try {
            this.logger.info('Switching camera...');
            await this.streamManager.switchCamera();
            this.logger.success('Camera switched successfully');
        } catch (error) {
            this.logger.error('Error switching camera: ' + error.message);
        }
    }

    // Subscriber methods
    async joinStream() {
        try {
            const selectedStream = document.getElementById('streamList').value;
            if (!selectedStream) {
                this.logger.warning('Please select a stream first');
                return;
            }

            this.logger.info(`Joining stream: ${selectedStream}`);
            
            await this.streamManager.joinStream(selectedStream);
            this.isSubscriber = true;
            this.updateSubscriberUI(true);
            
            this.logger.success(`Successfully joined stream: ${selectedStream}`);
            
        } catch (error) {
            this.logger.error('Failed to join stream: ' + error.message);
        }
    }

    async leaveStream() {
        try {
            this.logger.info('Leaving stream...');
            
            await this.streamManager.leaveStream();
            this.isSubscriber = false;
            this.updateSubscriberUI(false);
            
            this.logger.success('Left stream successfully');
            
        } catch (error) {
            this.logger.error('Error leaving stream: ' + error.message);
        }
    }

    async refreshStreams() {
        try {
            this.logger.info('Refreshing available streams...');
            const streams = await this.signalClient.getAvailableStreams();
            this.updateStreamList(streams);
            this.logger.success(`Found ${streams.length} available streams`);
        } catch (error) {
            this.logger.error('Error refreshing streams: ' + error.message);
        }
    }

    // UI update methods
    updatePublisherUI(isActive) {
        const startBtn = document.getElementById('startPublisher');
        const stopBtn = document.getElementById('stopPublisher');
        const infoEl = document.getElementById('publisherInfo');
        const overlay = document.getElementById('localOverlay');

        startBtn.disabled = isActive;
        stopBtn.disabled = !isActive;

        if (isActive) {
            infoEl.textContent = 'LIVE';
            infoEl.className = 'stream-info live';
            overlay.classList.remove('active');
        } else {
            infoEl.textContent = 'OFFLINE';
            infoEl.className = 'stream-info';
            overlay.classList.add('active');
        }
    }

    updateSubscriberUI(isActive) {
        const joinBtn = document.getElementById('joinStream');
        const leaveBtn = document.getElementById('leaveStream');
        const infoEl = document.getElementById('subscriberInfo');
        const overlay = document.getElementById('remoteOverlay');

        joinBtn.disabled = isActive;
        leaveBtn.disabled = !isActive;

        if (isActive) {
            infoEl.textContent = 'CONNECTED';
            infoEl.className = 'stream-info live';
            overlay.classList.remove('active');
        } else {
            infoEl.textContent = 'DISCONNECTED';
            infoEl.className = 'stream-info';
            overlay.classList.add('active');
        }
    }

    updateStreamList(streams = []) {
        const streamList = document.getElementById('streamList');
        const currentValue = streamList.value;
        
        streamList.innerHTML = '<option value="">Select a stream...</option>';
        
        streams.forEach(stream => {
            const option = document.createElement('option');
            option.value = stream.id;
            option.textContent = `${stream.name} (${stream.viewers} viewers)`;
            streamList.appendChild(option);
        });
        
        // Restore selection if still available
        if (streams.find(s => s.id === currentValue)) {
            streamList.value = currentValue;
        }
    }

    updateMetrics(metrics) {
        document.getElementById('peerCount').textContent = metrics.peers;
        document.getElementById('streamCount').textContent = metrics.streams;
        document.getElementById('bandwidth').textContent = metrics.bandwidth;
        document.getElementById('latency').textContent = metrics.latency;
        document.getElementById('packetLoss').textContent = metrics.packetLoss;
        document.getElementById('connections').textContent = metrics.connections;
    }

    // Event handlers
    onStreamAvailable(streamInfo) {
        this.logger.info(`New stream available: ${streamInfo.name}`);
        this.updateStreamList([...this.getCurrentStreams(), streamInfo]);
    }

    onStreamEnded(streamId) {
        this.logger.info(`Stream ended: ${streamId}`);
        const currentStreams = this.getCurrentStreams().filter(s => s.id !== streamId);
        this.updateStreamList(currentStreams);
        
        if (this.isSubscriber && this.streamManager.currentStreamId === streamId) {
            this.leaveStream();
        }
    }

    getCurrentStreams() {
        const streamList = document.getElementById('streamList');
        return Array.from(streamList.options)
            .slice(1)
            .map(option => ({
                id: option.value,
                name: option.textContent.split(' (')[0],
                viewers: parseInt(option.textContent.match(/\((\d+) viewers\)/)?.[1] || 0)
            }));
    }

    changeQuality(quality) {
        this.logger.info(`Changing video quality to: ${quality}`);
        
        // Update UI
        document.querySelectorAll('.quality-btn').forEach(btn => {
            btn.classList.toggle('active', btn.dataset.quality === quality);
        });
        
        // Apply quality settings
        this.streamManager.setVideoQuality(quality);
    }

    selectStream(streamId) {
        if (streamId && this.isSubscriber) {
            this.leaveStream().then(() => {
                setTimeout(() => this.joinStream(), 500);
            });
        }
    }

    setConnectionStatus(status) {
        const indicator = document.getElementById('connectionStatus');
        const serverInfo = document.getElementById('serverInfo');
        
        indicator.className = 'status-indicator';
        indicator.textContent = status.toUpperCase();
        
        switch (status) {
            case 'connected':
                indicator.classList.add('connected');
                serverInfo.textContent = 'Connected to RillNet network';
                break;
            case 'connecting':
                indicator.classList.add('connecting');
                serverInfo.textContent = 'Connecting to servers...';
                break;
            case 'error':
                indicator.classList.add('error');
                serverInfo.textContent = 'Connection failed - retrying...';
                break;
            default:
                serverInfo.textContent = 'Disconnected from network';
        }
    }

    startMetricsCollection() {
        // Simulate metrics updates
        setInterval(() => {
            const metrics = this.metricsCollector.collect();
            this.updateMetrics(metrics);
            
            // Update video stats
            this.updateVideoStats();
            
        }, 2000);
    }

    updateVideoStats() {
        const localStats = this.streamManager.getLocalStats();
        const remoteStats = this.streamManager.getRemoteStats();
        
        if (localStats) {
            document.getElementById('localResolution').textContent = localStats.resolution;
            document.getElementById('localBitrate').textContent = localStats.bitrate;
            document.getElementById('localFPS').textContent = localStats.fps;
        }
        
        if (remoteStats) {
            document.getElementById('remoteResolution').textContent = remoteStats.resolution;
            document.getElementById('remoteBitrate').textContent = remoteStats.bitrate;
            document.getElementById('remoteLatency').textContent = remoteStats.latency;
        }
    }

    updateLogFilters() {
        const filters = {
            info: document.getElementById('filterInfo').checked,
            success: document.getElementById('filterSuccess').checked,
            warning: document.getElementById('filterWarning').checked,
            error: document.getElementById('filterError').checked
        };
        this.logger.setFilters(filters);
    }

    exportLog() {
        this.logger.export();
    }

    // Utility methods
    async checkServices() {
        try {
            const services = await this.signalClient.getServiceStatus();
            this.logger.info('Service status: ' + JSON.stringify(services));
            return services;
        } catch (error) {
            this.logger.error('Service check failed: ' + error.message);
            return null;
        }
    }
}

// Initialize when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    window.rillNetApp = new RillNetApp();
});