// Stream Manager for WebRTC streaming
class StreamManager {
    constructor(signalClient, apiClient) {
        this.signalClient = signalClient;
        this.apiClient = apiClient;
        this.localStream = null;
        this.remoteStream = null;
        this.peerConnection = null;
        
        this.currentStreamId = null;
        this.isPublisher = false;
        this.isSubscriber = false;
        this.videoQuality = 'medium';
        
        this.eventHandlers = {};
        this.targetPeerID = null;
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

    async startPublisher(streamId) {
        try {
            const constraints = this.getVideoConstraints(this.videoQuality);
            this.localStream = await navigator.mediaDevices.getUserMedia({
                video: constraints,
                audio: {
                    echoCancellation: true,
                    noiseSuppression: true,
                    autoGainControl: true
                }
            });

            const localVideo = document.getElementById('localVideo');
            if (localVideo) {
                localVideo.srcObject = this.localStream;
                await localVideo.play();
            }

            await this.setupPeerConnection('publisher', streamId);
            
            this.isPublisher = true;
            this.currentStreamId = streamId;
            this.emit('streamStarted', this.localStream);
            
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
        this.currentStreamId = null;
        this.emit('streamStopped');
    }

    async switchCamera() {
        if (!this.localStream) return;

        try {
            const videoTrack = this.localStream.getVideoTracks()[0];
            const constraints = videoTrack.getConstraints();
            
            constraints.facingMode = constraints.facingMode === 'user' ? 'environment' : 'user';
            
            const newStream = await navigator.mediaDevices.getUserMedia({
                video: constraints,
                audio: true
            });

            const newVideoTrack = newStream.getVideoTracks()[0];
            const sender = this.getVideoSender();
            
            if (sender) {
                await sender.replaceTrack(newVideoTrack);
                this.localStream.getVideoTracks()[0].stop();
            }

            this.localStream.removeTrack(this.localStream.getVideoTracks()[0]);
            this.localStream.addTrack(newVideoTrack);

            const localVideo = document.getElementById('localVideo');
            if (localVideo) {
                localVideo.srcObject = this.localStream;
            }

            this.emit('cameraSwitched');
            
        } catch (error) {
            console.error('Error switching camera:', error);
            throw new Error(`Camera switch failed: ${error.message}`);
        }
    }

    async joinStream(streamId) {
        try {
            this.currentStreamId = streamId;
            await this.setupPeerConnection('subscriber', streamId);
            this.isSubscriber = true;
            this.emit('streamJoined', streamId);
            
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
    }

    async setupPeerConnection(role, streamId) {
        // Get ICE servers from config (should be passed from backend)
        const configuration = {
            iceServers: [
                { urls: 'stun:stun.l.google.com:19302' },
                { urls: 'stun:stun1.l.google.com:19302' }
            ]
        };

        this.peerConnection = new RTCPeerConnection(configuration);
        this.setupPeerConnectionEvents();

        if (role === 'publisher' && this.localStream) {
            this.localStream.getTracks().forEach(track => {
                this.peerConnection.addTrack(track, this.localStream);
            });
        }

        // Create offer/answer based on role
        if (role === 'publisher') {
            await this.createPublisherOffer(streamId);
        } else {
            await this.createSubscriberOffer(streamId);
        }
    }

    async createPublisherOffer(streamId) {
        try {
            const offer = await this.peerConnection.createOffer();
            await this.peerConnection.setLocalDescription(offer);

            // Send offer via API
            const response = await this.apiClient.createPublisherOffer(streamId, offer.sdp);
            
            if (response.answer) {
                await this.peerConnection.setRemoteDescription({
                    type: 'answer',
                    sdp: response.answer
                });
            }

            // Also send via WebSocket for P2P
            this.signalClient.sendOffer(offer.sdp, null, streamId);

        } catch (error) {
            console.error('Error creating publisher offer:', error);
            throw error;
        }
    }

    async createSubscriberOffer(streamId) {
        try {
            const offer = await this.peerConnection.createOffer();
            await this.peerConnection.setLocalDescription(offer);

            // Send offer via API
            const response = await this.apiClient.createSubscriberOffer(streamId, offer.sdp);
            
            if (response.answer) {
                await this.peerConnection.setRemoteDescription({
                    type: 'answer',
                    sdp: response.answer
                });
            }

            // Also send via WebSocket for P2P
            this.signalClient.sendOffer(offer.sdp, null, streamId);

        } catch (error) {
            console.error('Error creating subscriber offer:', error);
            throw error;
        }
    }

    handleOffer(data) {
        if (!this.peerConnection) return;

        this.peerConnection.setRemoteDescription({
            type: 'offer',
            sdp: data.sdp
        }).then(() => {
            return this.peerConnection.createAnswer();
        }).then(answer => {
            return this.peerConnection.setLocalDescription(answer);
        }).then(() => {
            // Send answer via WebSocket
            this.signalClient.sendAnswer(this.peerConnection.localDescription.sdp, data.fromPeer, data.streamId);
        }).catch(error => {
            console.error('Error handling offer:', error);
        });
    }

    handleAnswer(data) {
        if (!this.peerConnection) return;

        this.peerConnection.setRemoteDescription({
            type: 'answer',
            sdp: data.sdp
        }).catch(error => {
            console.error('Error handling answer:', error);
        });
    }

    handleICECandidate(data) {
        if (!this.peerConnection || !data.candidate) return;

        const candidate = new RTCIceCandidate({
            candidate: data.candidate,
            sdpMLineIndex: 0,
            sdpMid: '0'
        });

        this.peerConnection.addIceCandidate(candidate).catch(error => {
            console.error('Error adding ICE candidate:', error);
        });
    }

    setupPeerConnectionEvents() {
        this.peerConnection.oniceconnectionstatechange = () => {
            const state = this.peerConnection.iceConnectionState;
            this.emit('iceConnectionStateChange', state);
        };

        this.peerConnection.onconnectionstatechange = () => {
            const state = this.peerConnection.connectionState;
            this.emit('connectionStateChange', state);
            
            if (state === 'connected') {
                this.emit('connectionEstablished');
            }
        };

        this.peerConnection.onicecandidate = (event) => {
            if (event.candidate) {
                this.signalClient.sendICECandidate(
                    event.candidate.candidate,
                    this.targetPeerID,
                    this.currentStreamId
                );
            }
        };

        this.peerConnection.ontrack = (event) => {
            this.remoteStream = event.streams[0];
            
            const remoteVideo = document.getElementById('remoteVideo');
            if (remoteVideo) {
                remoteVideo.srcObject = this.remoteStream;
                remoteVideo.play().catch(console.error);
            }
            
            this.emit('streamReceived', this.remoteStream);
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
            
            videoTrack.applyConstraints(constraints).catch(console.error);
        }
    }

    getVideoSender() {
        if (!this.peerConnection) return null;
        
        const senders = this.peerConnection.getSenders();
        return senders.find(sender => 
            sender.track && sender.track.kind === 'video'
        );
    }
}
