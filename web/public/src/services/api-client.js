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
                    : { error: await response.text() };
                throw new Error(errorData.error || `HTTP ${response.status}: ${response.statusText}`);
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
            this.accessToken = response.access_token;
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

    // WebRTC endpoints
    async createPublisherOffer(streamId, sdp) {
        return this.request('POST', `/api/v1/streams/${streamId}/publisher/offer`, {
            sdp,
        });
    }

    async handlePublisherAnswer(streamId, sdp) {
        return this.request('POST', `/api/v1/streams/${streamId}/publisher/answer`, {
            sdp,
        });
    }

    async createSubscriberOffer(streamId, sdp) {
        return this.request('POST', `/api/v1/streams/${streamId}/subscriber/offer`, {
            sdp,
        });
    }

    async handleSubscriberAnswer(streamId, sdp) {
        return this.request('POST', `/api/v1/streams/${streamId}/subscriber/answer`, {
            sdp,
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

