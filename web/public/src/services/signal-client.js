// WebSocket Signal Client for RillNet
class SignalClient {
    constructor(wsURL = null, accessToken = null) {
        // Use relative WebSocket URL through nginx proxy, or absolute URL for direct access
        this.wsURL = wsURL || (window.location.protocol === 'https:' ? 'wss://' : 'ws://') + window.location.host + '/ws';
        this.accessToken = accessToken;
        this.ws = null;
        this.connected = false;
        this.peerID = null;
        this.eventHandlers = {};
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 5;
        this.reconnectDelay = 1000;
        this.tokenProvider = null;
        this.reconnecting = false;
    }

    setAccessToken(token) {
        this.accessToken = token;
    }

    setTokenProvider(provider) {
        this.tokenProvider = provider;
    }

    setPeerID(peerID) {
        this.peerID = peerID;
    }

    async resolveAccessToken() {
        if (this.tokenProvider) {
            const token = await this.tokenProvider();
            if (token) {
                this.accessToken = token;
            }
            return this.accessToken;
        }
        return this.accessToken;
    }

    async connect(peerID, token) {
        if (peerID) this.peerID = peerID;
        if (token) this.accessToken = token;

        if (!this.peerID || !this.accessToken) {
            throw new Error('Peer ID and access token are required');
        }

        return new Promise((resolve, reject) => {
            const url = `${this.wsURL}?peer_id=${encodeURIComponent(this.peerID)}&token=${encodeURIComponent(this.accessToken)}`;
            
            try {
                this.ws = new WebSocket(url);

                this.ws.onopen = () => {
                    this.connected = true;
                    this.reconnectAttempts = 0;
                    this.emit('connected');
                    resolve();
                };

                this.ws.onmessage = (event) => {
                    try {
                        const message = JSON.parse(event.data);
                        this.handleMessage(message);
                    } catch (error) {
                        console.error('Error parsing WebSocket message:', error);
                        this.emit('error', { type: 'parse_error', error: error.message });
                    }
                };

                this.ws.onerror = (error) => {
                    console.error('WebSocket error:', error);
                    this.emit('error', { type: 'connection_error', error });
                    reject(error);
                };

                this.ws.onclose = (event) => {
                    this.connected = false;
                    this.emit('disconnected', { code: event.code, reason: event.reason });

                    if (event.code === 1000 || this.reconnectAttempts >= this.maxReconnectAttempts) {
                        return;
                    }

                    this.reconnectAttempts++;
                    const delay = this.reconnectDelay * this.reconnectAttempts;
                    setTimeout(() => {
                        this.scheduleReconnect().catch(console.error);
                    }, delay);
                };

            } catch (error) {
                reject(error);
            }
        });
    }

    async scheduleReconnect() {
        if (this.reconnecting) {
            return;
        }
        this.reconnecting = true;
        try {
            const token = await this.resolveAccessToken();
            if (!token) {
                this.emit('auth_required', { message: 'Session expired, please log in again' });
                return;
            }
            await this.connect(this.peerID, token);
        } catch (error) {
            this.emit('auth_required', { message: error.message || 'Authentication failed' });
        } finally {
            this.reconnecting = false;
        }
    }

    disconnect() {
        this.reconnectAttempts = this.maxReconnectAttempts;
        if (this.ws) {
            this.ws.close(1000, 'Client disconnect');
            this.ws = null;
        }
        this.connected = false;
    }

    handleMessage(message) {
        switch (message.type) {
            case 'peers_list':
                this.emit('peers_list', message.peers || []);
                break;
            case 'offer':
                this.emit('offer', {
                    fromPeer: message.from_peer,
                    streamId: message.stream_id,
                    sdp: message.payload?.sdp,
                });
                break;
            case 'answer':
                this.emit('answer', {
                    fromPeer: message.from_peer,
                    streamId: message.stream_id,
                    sdp: message.payload?.sdp,
                });
                break;
            case 'ice_candidate':
                this.emit('ice_candidate', {
                    fromPeer: message.from_peer,
                    streamId: message.stream_id,
                    candidate: message.payload?.candidate,
                });
                break;
            case 'error':
                this.emit('error', {
                    type: 'server_error',
                    message: message.message || message.error,
                });
                break;
            default:
                this.emit('message', message);
        }
    }

    sendMessage(type, payload = {}) {
        if (!this.connected || !this.ws) {
            throw new Error('WebSocket is not connected');
        }

        const message = {
            type,
            peer_id: this.peerID,
            ...payload,
        };

        try {
            this.ws.send(JSON.stringify(message));
        } catch (error) {
            console.error('Error sending WebSocket message:', error);
            throw error;
        }
    }

    joinStream(streamId, isPublisher = false, capabilities = {}) {
        this.sendMessage('join_stream', {
            stream_id: streamId,
            payload: {
                stream_id: streamId,
                is_publisher: isPublisher,
                capabilities: {
                    max_bitrate: capabilities.maxBitrate || 2000,
                    codecs: capabilities.codecs || ['VP8', 'Opus'],
                },
            },
        });
    }

    sendOffer(sdp, targetPeer = null, streamId = null) {
        this.sendMessage('offer', {
            stream_id: streamId,
            payload: {
                sdp,
                target_peer: targetPeer,
                stream_id: streamId,
            },
        });
    }

    sendAnswer(sdp, targetPeer = null, streamId = null) {
        this.sendMessage('answer', {
            stream_id: streamId,
            payload: {
                sdp,
                target_peer: targetPeer,
                stream_id: streamId,
            },
        });
    }

    sendICECandidate(candidate, targetPeer = null, streamId = null) {
        this.sendMessage('ice_candidate', {
            stream_id: streamId,
            payload: {
                candidate,
                target_peer: targetPeer,
                stream_id: streamId,
            },
        });
    }

    sendMetricsUpdate(bandwidth, packetLoss, latency) {
        this.sendMessage('metrics_update', {
            payload: {
                bandwidth,
                packet_loss: packetLoss,
                latency,
            },
        });
    }

    on(event, handler) {
        if (!this.eventHandlers[event]) {
            this.eventHandlers[event] = [];
        }
        this.eventHandlers[event].push(handler);
    }

    off(event, handler) {
        if (this.eventHandlers[event]) {
            this.eventHandlers[event] = this.eventHandlers[event].filter(h => h !== handler);
        }
    }

    emit(event, data) {
        if (this.eventHandlers[event]) {
            this.eventHandlers[event].forEach(handler => {
                try {
                    handler(data);
                } catch (error) {
                    console.error(`Error in event handler for ${event}:`, error);
                }
            });
        }
    }
}
