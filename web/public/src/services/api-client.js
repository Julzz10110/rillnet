// API Client for RillNet HTTP API
class APIClient {
    constructor(baseURL = '') {
        // Use relative URL to go through nginx proxy, or absolute URL for direct access
        this.baseURL = baseURL || '';
        this.accessToken = null;
        this.refreshToken = null;
    }

    setTokens(accessToken, refreshToken) {
        this.accessToken = accessToken;
        this.refreshToken = refreshToken;
    }

    persistTokens() {
        if (this.accessToken) {
            localStorage.setItem('rillnet_access_token', this.accessToken);
        }
        if (this.refreshToken) {
            localStorage.setItem('rillnet_refresh_token', this.refreshToken);
        }
    }

    isAccessTokenExpired(skewSeconds = 30) {
        if (!this.accessToken) {
            return true;
        }
        try {
            const payload = this.accessToken.split('.')[1];
            const json = atob(payload.replace(/-/g, '+').replace(/_/g, '/'));
            const claims = JSON.parse(json);
            if (!claims.exp) {
                return true;
            }
            return claims.exp * 1000 <= Date.now() + skewSeconds * 1000;
        } catch {
            return true;
        }
    }

    async ensureAccessToken() {
        if (!this.isAccessTokenExpired()) {
            return this.accessToken;
        }
        await this.refreshAccessToken();
        this.persistTokens();
        return this.accessToken;
    }

    async request(method, endpoint, data = null) {
        const url = `${this.baseURL}${endpoint}`;
        const options = {
            method,
            headers: {
                'Content-Type': 'application/json',
            },
        };

        if (this.accessToken) {
            options.headers['Authorization'] = `Bearer ${this.accessToken}`;
        }

        if (data) {
            options.body = JSON.stringify(data);
        }

        try {
            const response = await fetch(url, options);
            const contentType = response.headers.get('content-type');
            
            if (!response.ok) {
                const errorData = contentType?.includes('application/json')
                    ? await response.json()
                    : { message: await response.text() };
                const message = errorData.message || errorData.error || `HTTP ${response.status}: ${response.statusText}`;
                throw new Error(typeof message === 'string' ? message : JSON.stringify(errorData));
            }

            if (contentType?.includes('application/json')) {
                return await response.json();
            }
            return await response.text();
        } catch (error) {
            console.error(`API ${method} ${endpoint} failed:`, error);
            throw error;
        }
    }

    // Auth endpoints
    async register(username, email, password) {
        return this.request('POST', '/api/v1/auth/register', {
            username,
            email,
            password,
        });
    }

    async login(username, password) {
        const response = await this.request('POST', '/api/v1/auth/login', {
            username,
            password,
        });
        if (response.access_token) {
            this.setTokens(response.access_token, response.refresh_token);
        }
        return response;
    }

    async refreshAccessToken() {
        if (!this.refreshToken) {
            throw new Error('No refresh token available');
        }
        const response = await this.request('POST', '/api/v1/auth/refresh', {
            refresh_token: this.refreshToken,
        });
        if (response.access_token) {
            this.setTokens(response.access_token, response.refresh_token || this.refreshToken);
            this.persistTokens();
        }
        return response;
    }

    // Stream endpoints
    async createStream(name, owner, maxPeers = 100) {
        return this.request('POST', '/api/v1/streams', {
            name,
            owner,
            max_peers: maxPeers,
        });
    }

    async getStream(streamId) {
        return this.request('GET', `/api/v1/streams/${streamId}`);
    }

    async listStreams() {
        return this.request('GET', '/api/v1/streams');
    }

    async joinStream(streamId, peerId, isPublisher = false, capabilities = {}) {
        return this.request('POST', `/api/v1/streams/${streamId}/join`, {
            peer_id: peerId,
            is_publisher: isPublisher,
            capabilities: {
                max_bitrate: capabilities.maxBitrate || 2000,
                codecs: capabilities.codecs || ['VP8', 'Opus'],
            },
        });
    }

    async leaveStream(streamId, peerId) {
        return this.request('POST', `/api/v1/streams/${streamId}/leave`, {
            peer_id: peerId,
        });
    }

    async getStreamStats(streamId) {
        return this.request('GET', `/api/v1/streams/${streamId}/stats`);
    }

    async getWebRTCReadiness(streamId) {
        return this.request('GET', `/api/v1/streams/${streamId}/webrtc/ready`);
    }

    // WebRTC endpoints (SFU: server creates offer, client returns answer)
    async createPublisherOffer(streamId, peerId) {
        return this.request('POST', `/api/v1/streams/${streamId}/publisher/offer`, {
            peer_id: peerId,
        });
    }

    async handlePublisherAnswer(streamId, peerId, answer) {
        return this.request('POST', `/api/v1/streams/${streamId}/publisher/answer`, {
            peer_id: peerId,
            answer: {
                type: answer.type,
                sdp: answer.sdp,
            },
        });
    }

    async createSubscriberOffer(streamId, peerId, sourcePeers = []) {
        return this.request('POST', `/api/v1/streams/${streamId}/subscriber/offer`, {
            peer_id: peerId,
            source_peers: sourcePeers,
        });
    }

    async handleSubscriberAnswer(streamId, peerId, answer) {
        return this.request('POST', `/api/v1/streams/${streamId}/subscriber/answer`, {
            peer_id: peerId,
            answer: {
                type: answer.type,
                sdp: answer.sdp,
            },
        });
    }

    // Health check
    async checkHealth() {
        try {
            // Use relative URL to go through nginx proxy
            const healthURL = this.baseURL ? `${this.baseURL}/health` : '/health';
            const response = await fetch(healthURL);
            if (!response.ok) {
                throw new Error(`Health check failed: ${response.status}`);
            }
            return await response.json();
        } catch (error) {
            return { status: 'unhealthy', error: error.message };
        }
    }
}

