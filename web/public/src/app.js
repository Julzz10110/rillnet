// RillNet web application (API + WebRTC + signaling)
// Canonical entry: app.js (copy of this file during build/sync)
class RillNetApp {
    constructor() {
        this.apiClient = new APIClient('');
        this.signalClient = new SignalClient();
        this.streamManager = null;
        this.logger = new Logger('RillNet');
        this.isAuthenticated = false;
        this.currentUser = null;
        this.currentPeerID = null;
        this.currentStreamId = null;
        this.isPublisher = false;
        this.isSubscriber = false;
        this.initializeApp();
    }

    async initializeApp() {
        try {
            this.logger.info('Initializing RillNet application...');
            const savedToken = localStorage.getItem('rillnet_access_token');
            const savedRefreshToken = localStorage.getItem('rillnet_refresh_token');
            const savedPeerID = localStorage.getItem('rillnet_peer_id');
            const savedUsername = localStorage.getItem('rillnet_username');
            if (savedToken && savedPeerID) {
                this.apiClient.setTokens(savedToken, savedRefreshToken);
                this.currentPeerID = savedPeerID;
                this.signalClient.setPeerID(savedPeerID);
                this.signalClient.setAccessToken(savedToken);
                this.isAuthenticated = true;
                if (savedUsername) this.currentUser = { username: savedUsername };
                try {
                    await this.initializeAfterAuth();
                } catch (error) {
                    if (error.message && error.message.includes('token expired')) {
                        localStorage.removeItem('rillnet_access_token');
                        localStorage.removeItem('rillnet_refresh_token');
                        this.isAuthenticated = false;
                        this.showLoginForm();
                    } else {
                        throw error;
                    }
                }
            } else {
                this.showLoginForm();
            }
            this.initializeEventListeners();
            this.startHealthChecks();
        } catch (error) {
            this.logger.error('Failed to initialize application: ' + error.message);
        }
    }

    showLoginForm() {
        if (this.isAuthenticated) return;
        this.removeLoginForm();
        if (document.getElementById('loginForm')) return;
        document.body.insertAdjacentHTML('beforeend', `
            <div id="loginForm" style="position:fixed;top:50%;left:50%;transform:translate(-50%,-50%);
                background:white;padding:2rem;border-radius:8px;box-shadow:0 4px 6px rgba(0,0,0,0.1);z-index:1000;min-width:300px;">
                <h2 style="margin-top:0;">RillNet Login</h2>
                <input type="text" id="loginUsername" placeholder="Username" style="width:100%;padding:0.5rem;margin-bottom:0.5rem;box-sizing:border-box;">
                <input type="password" id="loginPassword" placeholder="Password" style="width:100%;padding:0.5rem;box-sizing:border-box;">
                <button id="loginBtn" style="width:100%;padding:0.5rem;margin-top:0.5rem;background:#007bff;color:white;border:none;border-radius:4px;cursor:pointer;">Login</button>
                <div style="margin-top:1rem;text-align:center;"><small>Or <a href="#" id="registerLink">register</a></small></div>
                <div id="loginError" style="color:red;margin-top:0.5rem;display:none;"></div>
            </div>`);
        document.getElementById('loginBtn')?.addEventListener('click', () => this.handleLogin());
        document.getElementById('registerLink')?.addEventListener('click', (e) => { e.preventDefault(); this.showRegisterForm(); });
    }

    showRegisterForm() {
        document.getElementById('loginForm')?.remove();
        document.body.insertAdjacentHTML('beforeend', `
            <div id="registerForm" style="position:fixed;top:50%;left:50%;transform:translate(-50%,-50%);
                background:white;padding:2rem;border-radius:8px;z-index:1000;">
                <h2>RillNet Register</h2>
                <input type="text" id="regUsername" placeholder="Username" style="width:100%;padding:0.5rem;margin-bottom:0.5rem;">
                <input type="email" id="regEmail" placeholder="Email" style="width:100%;padding:0.5rem;margin-bottom:0.5rem;">
                <input type="password" id="regPassword" placeholder="Password" style="width:100%;padding:0.5rem;">
                <button id="registerBtn" style="width:100%;padding:0.5rem;margin-top:0.5rem;background:#28a745;color:white;border:none;border-radius:4px;">Register</button>
                <div style="margin-top:1rem;text-align:center;"><small><a href="#" id="loginLink">login</a></small></div>
            </div>`);
        document.getElementById('registerBtn')?.addEventListener('click', () => this.handleRegister());
        document.getElementById('loginLink')?.addEventListener('click', (e) => { e.preventDefault(); document.getElementById('registerForm')?.remove(); this.showLoginForm(); });
    }

    async handleLogin() {
        const username = document.getElementById('loginUsername')?.value;
        const password = document.getElementById('loginPassword')?.value;
        if (!username || !password) return;
        try {
            const response = await this.apiClient.login(username, password);
            const peerID = `peer-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
            localStorage.setItem('rillnet_access_token', response.access_token);
            localStorage.setItem('rillnet_refresh_token', response.refresh_token);
            localStorage.setItem('rillnet_peer_id', peerID);
            localStorage.setItem('rillnet_username', response.username);
            this.currentUser = { username: response.username, user_id: response.user_id };
            this.currentPeerID = peerID;
            this.isAuthenticated = true;
            this.removeLoginForm();
            await this.initializeAfterAuth();
        } catch (error) {
            const errorDiv = document.getElementById('loginError');
            if (errorDiv) { errorDiv.textContent = error.message; errorDiv.style.display = 'block'; }
            this.logger.error('Login failed: ' + error.message);
        }
    }

    removeLoginForm() {
        document.getElementById('loginForm')?.remove();
        document.getElementById('registerForm')?.remove();
    }

    async handleRegister() {
        const username = document.getElementById('regUsername')?.value;
        const email = document.getElementById('regEmail')?.value;
        const password = document.getElementById('regPassword')?.value;
        if (!username || !email || !password) return;
        try {
            await this.apiClient.register(username, email, password);
            this.removeLoginForm();
            this.showLoginForm();
            this.logger.success('Registration successful! Please login.');
        } catch (error) {
            this.logger.error('Registration failed: ' + error.message);
        }
    }

    async initializeAfterAuth() {
        this.streamManager = new StreamManager(this.signalClient, this.apiClient);
        await this.signalClient.connect(this.currentPeerID, this.apiClient.accessToken);
        this.setupSignalHandlers();
        this.initializeUI();
        await this.refreshStreams();
        this.setConnectionStatus('connected');
    }

    setupSignalHandlers() {
        this.signalClient.on('connected', () => this.setConnectionStatus('connected'));
        this.signalClient.on('disconnected', () => this.setConnectionStatus('disconnected'));
        this.signalClient.on('offer', (data) => this.streamManager?.handleOffer(data));
        this.signalClient.on('answer', (data) => this.streamManager?.handleAnswer(data));
        this.signalClient.on('ice_candidate', (data) => this.streamManager?.handleICECandidate(data));
    }

    initializeEventListeners() {
        document.getElementById('startPublisher')?.addEventListener('click', () => this.startPublisher());
        document.getElementById('stopPublisher')?.addEventListener('click', () => this.stopPublisher());
        document.getElementById('switchCamera')?.addEventListener('click', () => this.switchCamera());
        document.getElementById('joinStream')?.addEventListener('click', () => this.joinStream());
        document.getElementById('leaveStream')?.addEventListener('click', () => this.leaveStream());
        document.getElementById('refreshStream')?.addEventListener('click', () => this.refreshStreams());
        document.querySelectorAll('.quality-btn').forEach(btn => {
            btn.addEventListener('click', (e) => this.changeQuality(e.target.dataset.quality));
        });
        document.getElementById('streamList')?.addEventListener('change', (e) => this.selectStream(e.target.value));
        document.getElementById('clearLog')?.addEventListener('click', () => this.logger.clear());
        document.getElementById('exportLog')?.addEventListener('click', () => this.logger.export());
        document.getElementById('autoScroll')?.addEventListener('click', (e) => {
            this.logger.setAutoScroll(!e.target.classList.contains('active'));
        });
        document.querySelectorAll('.log-filters input').forEach(checkbox => {
            checkbox.addEventListener('change', () => this.updateLogFilters());
        });
    }

    initializeUI() {
        this.updatePublisherUI(false);
        this.updateSubscriberUI(false);
        this.updateMetrics({ peers: 0, streams: 0, bandwidth: 0, latency: 0, packetLoss: '0%', connections: 0 });
    }

    async startPublisher() {
        if (!this.isAuthenticated || !this.apiClient.accessToken) {
            this.showLoginForm();
            return;
        }
        if (!this.streamManager) {
            this.streamManager = new StreamManager(this.signalClient, this.apiClient);
            if (!this.signalClient.connected) {
                await this.signalClient.connect(this.currentPeerID, this.apiClient.accessToken);
                this.setupSignalHandlers();
            }
        }
        const streamName = prompt('Enter stream name:', `Stream ${Date.now()}`);
        if (!streamName) return;
        try {
            const streamResponse = await this.apiClient.createStream(streamName, this.currentPeerID, 100);
            const stream = streamResponse.stream || streamResponse;
            this.currentStreamId = stream.ID || stream.id || streamResponse.ID || streamResponse.id;
            if (!this.currentStreamId) throw new Error('Stream ID not found in response');
            await this.apiClient.joinStream(this.currentStreamId, this.currentPeerID, true, { maxBitrate: 2000, codecs: ['VP8', 'Opus'] });
            await this.streamManager.startPublisher(this.currentStreamId);
            this.signalClient.joinStream(this.currentStreamId, true, { maxBitrate: 2000, codecs: ['VP8', 'Opus'] });
            this.isPublisher = true;
            this.updatePublisherUI(true);
            await this.refreshStreams();
        } catch (error) {
            this.logger.error('Failed to start publisher: ' + error.message);
        }
    }

    async stopPublisher() {
        try {
            if (this.streamManager) await this.streamManager.stopPublisher();
            if (this.currentStreamId) await this.apiClient.leaveStream(this.currentStreamId, this.currentPeerID);
            this.isPublisher = false;
            this.currentStreamId = null;
            this.updatePublisherUI(false);
            await this.refreshStreams();
        } catch (error) {
            this.logger.error('Error stopping publisher: ' + error.message);
        }
    }

    async switchCamera() {
        if (this.streamManager) await this.streamManager.switchCamera();
    }

    async joinStream() {
        if (!this.isAuthenticated) { this.showLoginForm(); return; }
        if (!this.streamManager) {
            this.streamManager = new StreamManager(this.signalClient, this.apiClient);
            if (!this.signalClient.connected) {
                await this.signalClient.connect(this.currentPeerID, this.apiClient.accessToken);
                this.setupSignalHandlers();
            }
        }
        const selectedStream = document.getElementById('streamList')?.value;
        if (!selectedStream) return;
        try {
            await this.apiClient.joinStream(selectedStream, this.currentPeerID, false, { maxBitrate: 2000, codecs: ['VP8', 'Opus'] });
            this.signalClient.joinStream(selectedStream, false, { maxBitrate: 2000, codecs: ['VP8', 'Opus'] });
            await this.streamManager.joinStream(selectedStream);
            this.isSubscriber = true;
            this.currentStreamId = selectedStream;
            this.updateSubscriberUI(true);
        } catch (error) {
            this.logger.error('Failed to join stream: ' + error.message);
        }
    }

    async leaveStream() {
        try {
            if (this.streamManager) await this.streamManager.leaveStream();
            if (this.currentStreamId) await this.apiClient.leaveStream(this.currentStreamId, this.currentPeerID);
            this.isSubscriber = false;
            this.currentStreamId = null;
            this.updateSubscriberUI(false);
        } catch (error) {
            this.logger.error('Error leaving stream: ' + error.message);
        }
    }

    async refreshStreams() {
        try {
            const response = await this.apiClient.listStreams();
            this.updateStreamList(response.streams || []);
        } catch (error) {
            this.logger.error('Error refreshing streams: ' + error.message);
        }
    }

    updatePublisherUI(isActive) {
        const startBtn = document.getElementById('startPublisher');
        const stopBtn = document.getElementById('stopPublisher');
        const infoEl = document.getElementById('publisherInfo');
        const overlay = document.getElementById('localOverlay');
        if (startBtn) startBtn.disabled = isActive;
        if (stopBtn) stopBtn.disabled = !isActive;
        if (infoEl) { infoEl.textContent = isActive ? 'LIVE' : 'OFFLINE'; infoEl.className = isActive ? 'stream-info live' : 'stream-info'; }
        if (overlay) overlay.classList.toggle('active', !isActive);
    }

    updateSubscriberUI(isActive) {
        const joinBtn = document.getElementById('joinStream');
        const leaveBtn = document.getElementById('leaveStream');
        const infoEl = document.getElementById('subscriberInfo');
        const overlay = document.getElementById('remoteOverlay');
        if (joinBtn) joinBtn.disabled = isActive;
        if (leaveBtn) leaveBtn.disabled = !isActive;
        if (infoEl) { infoEl.textContent = isActive ? 'CONNECTED' : 'DISCONNECTED'; infoEl.className = isActive ? 'stream-info live' : 'stream-info'; }
        if (overlay) overlay.classList.toggle('active', !isActive);
    }

    updateStreamList(streams = []) {
        const streamList = document.getElementById('streamList');
        if (!streamList) return;
        const currentValue = streamList.value;
        streamList.innerHTML = '<option value="">Select a stream...</option>';
        streams.forEach(stream => {
            const option = document.createElement('option');
            option.value = stream.id || stream.stream_id || stream.ID;
            const name = stream.name || `Stream ${option.value}`;
            option.textContent = `${name} (${stream.peer_count || 0} peers)`;
            streamList.appendChild(option);
        });
        if (streams.find(s => (s.id || s.stream_id || s.ID) === currentValue)) streamList.value = currentValue;
    }

    updateMetrics(metrics) {
        const set = (id, v) => { const el = document.getElementById(id); if (el) el.textContent = v; };
        set('peerCount', metrics.peers || 0);
        set('streamCount', metrics.streams || 0);
        set('bandwidth', metrics.bandwidth || 0);
        set('latency', metrics.latency || 0);
        set('packetLoss', metrics.packetLoss || '0%');
        set('connections', metrics.connections || 0);
    }

    changeQuality(quality) {
        document.querySelectorAll('.quality-btn').forEach(btn => btn.classList.toggle('active', btn.dataset.quality === quality));
        this.streamManager?.setVideoQuality(quality);
    }

    selectStream(streamId) {
        if (streamId && this.isSubscriber) this.leaveStream().then(() => setTimeout(() => this.joinStream(), 500));
    }

    setConnectionStatus(status) {
        const indicator = document.getElementById('connectionStatus');
        const serverInfo = document.getElementById('serverInfo');
        if (indicator) { indicator.className = 'status-indicator ' + status; indicator.textContent = status.toUpperCase(); }
        if (serverInfo) {
            const messages = { connected: 'Connected to RillNet network', connecting: 'Connecting...', error: 'Connection failed', disconnected: 'Disconnected' };
            serverInfo.textContent = messages[status] || 'Unknown';
        }
    }

    startHealthChecks() {
        setInterval(async () => {
            try {
                const health = await this.apiClient.checkHealth();
                if (health.status === 'healthy') this.setConnectionStatus('connected');
            } catch { this.setConnectionStatus('error'); }
        }, 10000);
    }

    updateLogFilters() {
        this.logger.setFilters({
            info: document.getElementById('filterInfo')?.checked ?? true,
            success: document.getElementById('filterSuccess')?.checked ?? true,
            warning: document.getElementById('filterWarning')?.checked ?? true,
            error: document.getElementById('filterError')?.checked ?? true,
        });
    }
}

document.addEventListener('DOMContentLoaded', () => {
    if (typeof APIClient === 'undefined' || typeof SignalClient === 'undefined' || typeof StreamManager === 'undefined') {
        console.error('Required classes not loaded');
        return;
    }
    window.rillNetApp = new RillNetApp();
});
