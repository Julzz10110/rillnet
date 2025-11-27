class StreamManager {
    constructor(signalClient) {
        this.signalClient = signalClient;
        this.localStream = null;
        this.remoteStream = null;
        this.peerConnection = null;
        this.isPublisher = false;
        this.isSubscriber = false;
        
        this.eventHandlers = {};
        
        this.initializeWebRTC();
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

    initializeWebRTC() {
        // WebRTC configuration for P2P streaming
        this.rtcConfig = {
            iceServers: [
                { urls: 'stun:stun.l.google.com:19302' },
                { urls: 'stun:stun1.l.google.com:19302' }
            ],
            sdpSemantics: 'unified-plan'
        };
    }

    async startPublisher() {
        try {
            // Get user media
            this.localStream = await navigator.mediaDevices.getUserMedia({
                video: {
                    width: { ideal: 1280 },
                    height: { ideal: 720 },
                    frameRate: { ideal: 30 }
                },
                audio: true
            });

            // Display local stream
            const localVideo = document.getElementById('localVideo');
            localVideo.srcObject = this.localStream;

            // Connect to SFU as publisher
            await this.connectToSFU('publisher');

            this.isPublisher = true;
            this.emit('streamStarted');
            
            return this.localStream;
        } catch (error) {
            console.error('Error starting publisher:', error);
            throw error;
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
    }

    async switchCamera() {
        if (!this.localStream) return;

        const videoTrack = this.localStream.getVideoTracks()[0];
        const constraints = videoTrack.getConstraints();

        // Toggle facing mode
        constraints.facingMode = constraints.facingMode === 'user' ? 'environment' : 'user';

        try {
            const newStream = await navigator.mediaDevices.getUserMedia({
                video: constraints,
                audio: true
            });

            // Replace video track
            const newVideoTrack = newStream.getVideoTracks()[0];
            const sender = this.getVideoSender();
            
            if (sender) {
                await sender.replaceTrack(newVideoTrack);
            }

            // Stop old tracks
            this.localStream.getTracks().forEach(track => track.stop());
            this.localStream = newStream;

            // Update video element
            document.getElementById('localVideo').srcObject = this.localStream;

            this.emit('cameraSwitched');
        } catch (error) {
            console.error('Error switching camera:', error);
            throw error;
        }
    }

    getVideoSender() {
        if (!this.peerConnection) return null;
        
        const senders = this.peerConnection.getSenders();
        return senders.find(sender => 
            sender.track && sender.track.kind === 'video'
        );
    }

    async connectToSFU(role) {
        // Create peer connection
        this.peerConnection = new RTCPeerConnection(this.rtcConfig);

        // Add local tracks if publisher
        if (role === 'publisher' && this.localStream) {
            this.localStream.getTracks().forEach(track => {
                this.peerConnection.addTrack(track, this.localStream);
            });
        }

        // Set up event handlers
        this.setupPeerConnectionEvents();

        // Create and set local description
        const offer = await this.peerConnection.createOffer();
        await this.peerConnection.setLocalDescription(offer);

        // Send offer to SFU via signal server
        await this.signalClient.sendOffer(offer, role);

        // Wait for answer
        // This would typically be handled via the signal server
    }

    setupPeerConnectionEvents() {
        if (!this.peerConnection) return;

        this.peerConnection.oniceconnectionstatechange = () => {
            console.log('ICE connection state:', this.peerConnection.iceConnectionState);
            this.emit('iceStateChange', this.peerConnection.iceConnectionState);
        };

        this.peerConnection.onicecandidate = (event) => {
            if (event.candidate) {
                // Send ICE candidate to signal server
                this.signalClient.sendICECandidate(event.candidate);
            }
        };

        this.peerConnection.ontrack = (event) => {
            console.log('Received remote track:', event.track.kind);
            this.remoteStream = event.streams[0];
            
            const remoteVideo = document.getElementById('remoteVideo');
            if (remoteVideo) {
                remoteVideo.srcObject = this.remoteStream;
            }
            
            this.emit('streamReceived', this.remoteStream);
        };

        this.peerConnection.onconnectionstatechange = () => {
            console.log('Connection state:', this.peerConnection.connectionState);
            this.emit('connectionStateChange', this.peerConnection.connectionState);
        };
    }

    async joinStream() {
        try {
            await this.connectToSFU('subscriber');
            this.isSubscriber = true;
            this.emit('streamJoined');
        } catch (error) {
            console.error('Error joining stream:', error);
            throw error;
        }
    }

    async leaveStream() {
        if (this.peerConnection) {
            this.peerConnection.close();
            this.peerConnection = null;
        }

        const remoteVideo = document.getElementById('remoteVideo');
        if (remoteVideo) {
            remoteVideo.srcObject = null;
        }

        this.remoteStream = null;
        this.isSubscriber = false;
        this.emit('streamLeft');
    }

    async handleRemoteOffer(offer) {
        if (!this.peerConnection) {
            this.peerConnection = new RTCPeerConnection(this.rtcConfig);
            this.setupPeerConnectionEvents();
        }

        await this.peerConnection.setRemoteDescription(offer);
        const answer = await this.peerConnection.createAnswer();
        await this.peerConnection.setLocalDescription(answer);

        // Send answer back via signal server
        await this.signalClient.sendAnswer(answer);
    }

    async handleRemoteAnswer(answer) {
        if (this.peerConnection) {
            await this.peerConnection.setRemoteDescription(answer);
        }
    }

    async addICECandidate(candidate) {
        if (this.peerConnection) {
            await this.peerConnection.addIceCandidate(candidate);
        }
    }
}