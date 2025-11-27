class StreamManager {
    constructor(signalClient) {
        this.signalClient = signalClient;
        this.localStream = null;
        this.remoteStream = null;
        this.peerConnection = null;
        this.dataChannel = null;
        
        this.currentStreamId = null;
        this.isPublisher = false;
        this.isSubscriber = false;
        
        this.videoQuality = 'medium';
        this.statsCollector = new WebRTCStats();
        
        this.eventHandlers = {};
    }

    on(event, handler) {
        if (!this.eventHandlers[event]) {
            this.eventHandlers[event] = [];
        }
        this.eventHandlers[event].push(handler);
    }

    emit(event, data) {
        if (this.eventHandlers[event]) {
            this.eventHandlers[event].forEach(handler => handler(data));
        }
    }

    async startPublisher() {
        try {
            // Get user media with quality settings
            const constraints = this.getVideoConstraints(this.videoQuality);
            this.localStream = await navigator.mediaDevices.getUserMedia({
                video: constraints,
                audio: {
                    echoCancellation: true,
                    noiseSuppression: true,
                    autoGainControl: true
                }
            });

            // Display local stream
            const localVideo = document.getElementById('localVideo');
            localVideo.srcObject = this.localStream;
            localVideo.play().catch(console.error);

            // Setup WebRTC connection
            await this.setupPeerConnection('publisher');
            
            this.isPublisher = true;
            this.emit('streamStarted', this.localStream);
            
            // Start stats collection
            this.startStatsCollection();
            
            return this.localStream;

        } catch (error) {
            console.error('Error starting publisher:', error);
            throw new Error(`Failed to access camera: ${error.message}`);
        }
    }

    async stopPublisher() {
        if (this.localStream) {
            this.localStream.getTracks().forEach(track => track.stop());
            this.localStream = null;
        }

        if (this.peerConnection) {
            this.peerConnection.close();
            this.peerConnection = null;
        }

        const localVideo = document.getElementById('localVideo');
        if (localVideo) {
            localVideo.srcObject = null;
        }

        this.isPublisher = false;
        this.emit('streamStopped');
        this.stopStatsCollection();
    }

    async switchCamera() {
        if (!this.localStream) return;

        try {
            const videoTrack = this.localStream.getVideoTracks()[0];
            const constraints = videoTrack.getConstraints();
            
            // Toggle facing mode
            constraints.facingMode = constraints.facingMode === 'user' ? 'environment' : 'user';
            
            const newStream = await navigator.mediaDevices.getUserMedia({
                video: constraints,
                audio: true
            });

            // Replace video track in existing stream
            const newVideoTrack = newStream.getVideoTracks()[0];
            const sender = this.getVideoSender();
            
            if (sender) {
                await sender.replaceTrack(newVideoTrack);
                // Stop the old video track
                this.localStream.getVideoTracks()[0].stop();
            }

            // Update local stream reference
            this.localStream.removeTrack(this.localStream.getVideoTracks()[0]);
            this.localStream.addTrack(newVideoTrack);

            // Update video element
            document.getElementById('localVideo').srcObject = this.localStream;

            this.emit('cameraSwitched');
            
        } catch (error) {
            console.error('Error switching camera:', error);
            throw new Error(`Camera switch failed: ${error.message}`);
        }
    }

    async joinStream(streamId) {
        try {
            this.currentStreamId = streamId;
            await this.setupPeerConnection('subscriber');
            this.isSubscriber = true;
            this.emit('streamJoined', streamId);
            
            this.startStatsCollection();
            
        } catch (error) {
            console.error('Error joining stream:', error);
            throw new Error(`Failed to join stream: ${error.message}`);
        }
    }

    async leaveStream() {
        if (this.peerConnection) {
            this.peerConnection.close();
            this.peerConnection = null;
        }

        if (this.remoteStream) {
            this.remoteStream.getTracks().forEach(track => track.stop());
            this.remoteStream = null;
        }

        const remoteVideo = document.getElementById('remoteVideo');
        if (remoteVideo) {
            remoteVideo.srcObject = null;
        }

        this.isSubscriber = false;
        this.currentStreamId = null;
        this.emit('streamLeft');
        this.stopStatsCollection();
    }

    async setupPeerConnection(role) {
        // WebRTC configuration for P2P streaming
        const configuration = {
            iceServers: [
                { urls: 'stun:stun.l.google.com:19302' },
                { urls: 'stun:stun1.l.google.com:19302' }
            ],
            sdpSemantics: 'unified-plan',
            bundlePolicy: 'max-bundle',
            rtcpMuxPolicy: 'require'
        };

        this.peerConnection = new RTCPeerConnection(configuration);
        this.setupPeerConnectionEvents();

        if (role === 'publisher' && this.localStream) {
            // Add all tracks from local stream
            this.localStream.getTracks().forEach(track => {
                this.peerConnection.addTrack(track, this.localStream);
            });

            // Create data channel for metadata
            this.dataChannel = this.peerConnection.createDataChannel('metadata', {
                ordered: false,
                maxRetransmits: 0
            });
            this.setupDataChannel();
        }

        if (role === 'subscriber') {
            // Setup data channel for incoming metadata
            this.peerConnection.ondatachannel = (event) => {
                this.dataChannel = event.channel;
                this.setupDataChannel();
            };
        }

        // For now, simulate connection - in real implementation,
        // this would involve signaling with the SFU
        await this.simulateConnection(role);
    }

    setupPeerConnectionEvents() {
        this.peerConnection.oniceconnectionstatechange = () => {
            const state = this.peerConnection.iceConnectionState;
            this.emit('iceConnectionStateChange', state);
            console.log('ICE connection state:', state);
        };

        this.peerConnection.onicegatheringstatechange = () => {
            console.log('ICE gathering state:', this.peerConnection.iceGatheringState);
        };

        this.peerConnection.onsignalingstatechange = () => {
            console.log('Signaling state:', this.peerConnection.signalingState);
        };

        this.peerConnection.onconnectionstatechange = () => {
            const state = this.peerConnection.connectionState;
            this.emit('connectionStateChange', state);
            console.log('Connection state:', state);
            
            if (state === 'connected') {
                this.emit('connectionEstablished');
            } else if (state === 'failed' || state === 'disconnected') {
                this.emit('connectionLost');
            }
        };

        this.peerConnection.ontrack = (event) => {
            console.log('Received remote track:', event.track.kind);
            this.remoteStream = event.streams[0];
            
            const remoteVideo = document.getElementById('remoteVideo');
            if (remoteVideo) {
                remoteVideo.srcObject = this.remoteStream;
                remoteVideo.play().catch(console.error);
            }
            
            this.emit('streamReceived', this.remoteStream);
        };

        this.peerConnection.onicecandidate = (event) => {
            if (event.candidate) {
                // In real implementation, send to signaling server
                console.log('New ICE candidate:', event.candidate);
            }
        };
    }

    setupDataChannel() {
        this.dataChannel.onopen = () => {
            console.log('Data channel opened');
            this.emit('dataChannelOpen');
        };

        this.dataChannel.onclose = () => {
            console.log('Data channel closed');
            this.emit('dataChannelClose');
        };

        this.dataChannel.onmessage = (event) => {
            try {
                const data = JSON.parse(event.data);
                this.emit('metadata', data);
            } catch (error) {
                console.error('Error parsing metadata:', error);
            }
        };
    }

    getVideoConstraints(quality) {
        const constraints = {
            low: {
                width: { ideal: 640 },
                height: { ideal: 360 },
                frameRate: { ideal: 15, max: 30 }
            },
            medium: {
                width: { ideal: 1280 },
                height: { ideal: 720 },
                frameRate: { ideal: 30, max: 60 }
            },
            high: {
                width: { ideal: 1920 },
                height: { ideal: 1080 },
                frameRate: { ideal: 30, max: 60 }
            }
        };
        
        return constraints[quality] || constraints.medium;
    }

    setVideoQuality(quality) {
        this.videoQuality = quality;
        
        if (this.isPublisher && this.localStream) {
            const videoTrack = this.localStream.getVideoTracks()[0];
            const constraints = this.getVideoConstraints(quality);
            
            videoTrack.applyConstraints(constraints).catch(error => {
                console.error('Error applying constraints:', error);
            });
        }
    }

    getVideoSender() {
        if (!this.peerConnection) return null;
        
        const senders = this.peerConnection.getSenders();
        return senders.find(sender => 
            sender.track && sender.track.kind === 'video'
        );
    }

    getLocalStats() {
        if (!this.isPublisher) return null;
        
        // Simulate stats - in real implementation, use getStats()
        return {
            resolution: this.getResolutionString(this.videoQuality),
            bitrate: Math.floor(Math.random() * 2000) + 500,
            fps: this.videoQuality === 'low' ? 15 : 30
        };
    }

    getRemoteStats() {
        if (!this.isSubscriber) return null;
        
        // Simulate stats
        return {
            resolution: '1280x720',
            bitrate: Math.floor(Math.random() * 1500) + 300,
            latency: Math.floor(Math.random() * 100) + 20
        };
    }

    getResolutionString(quality) {
        const resolutions = {
            low: '640x360',
            medium: '1280x720',
            high: '1920x1080'
        };
        return resolutions[quality] || '1280x720';
    }

    startStatsCollection() {
        this.statsInterval = setInterval(() => {
            this.emit('statsUpdate', {
                local: this.getLocalStats(),
                remote: this.getRemoteStats()
            });
        }, 1000);
    }

    stopStatsCollection() {
        if (this.statsInterval) {
            clearInterval(this.statsInterval);
            this.statsInterval = null;
        }
    }

    // Simulation methods for demo
    async simulateConnection(role) {
        return new Promise((resolve) => {
            setTimeout(() => {
                if (role === 'publisher') {
                    this.emit('connectionEstablished');
                }
                resolve();
            }, 1000);
        });
    }
}