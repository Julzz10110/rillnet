// Stream Manager for WebRTC streaming (SFU: server offer → browser answer)
class StreamManager {
    constructor(signalClient, apiClient, peerId) {
        this.signalClient = signalClient;
        this.apiClient = apiClient;
        this.peerId = peerId;
        this.localStream = null;
        this.remoteStream = null;
        this.peerConnection = null;

        this.currentStreamId = null;
        this.isPublisher = false;
        this.isSubscriber = false;
        this.videoQuality = 'medium';

        this.eventHandlers = {};
        this.targetPeerID = null;
        this.signalingMode = 'http';
        this._joinInProgress = false;
    }

    setPeerId(peerId) {
        this.peerId = peerId;
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

    getIceConfiguration() {
        return {
            iceServers: [
                { urls: 'stun:stun.l.google.com:19302' },
                { urls: 'stun:stun1.l.google.com:19302' },
            ],
        };
    }

    async startPublisher(streamId, peerId) {
        if (peerId) this.peerId = peerId;
        if (!this.peerId) {
            throw new Error('peer_id is required');
        }

        try {
            const constraints = this.getVideoConstraints(this.videoQuality);
            this.localStream = await navigator.mediaDevices.getUserMedia({
                video: constraints,
                audio: {
                    echoCancellation: true,
                    noiseSuppression: true,
                    autoGainControl: true,
                },
            });

            const localVideo = document.getElementById('localVideo');
            if (localVideo) {
                localVideo.srcObject = this.localStream;
                await localVideo.play();
            }

            this.peerConnection = new RTCPeerConnection(this.getIceConfiguration());
            this.setupPeerConnectionEvents();
            this.localStream.getTracks().forEach((track) => {
                this.peerConnection.addTrack(track, this.localStream);
            });

            const serverOffer = await this.apiClient.createPublisherOffer(streamId, this.peerId);
            await this.peerConnection.setRemoteDescription({
                type: 'offer',
                sdp: serverOffer.sdp,
            });

            const answer = await this.peerConnection.createAnswer();
            await this.peerConnection.setLocalDescription(answer);
            await this.apiClient.handlePublisherAnswer(streamId, this.peerId, answer);

            this.isPublisher = true;
            this.currentStreamId = streamId;
            this.emit('streamStarted', this.localStream);

            return this.localStream;
        } catch (error) {
            console.error('Error starting publisher:', error);
            if (error.name === 'NotAllowedError' || error.name === 'PermissionDeniedError') {
                throw new Error(
                    'Camera/microphone blocked. Allow access for this site in browser settings and retry.'
                );
            }
            throw new Error(`Failed to start publisher: ${error.message}`);
        }
    }

    async stopPublisher() {
        if (this.localStream) {
            this.localStream.getTracks().forEach((track) => track.stop());
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
                audio: true,
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

    async joinStream(streamId, peerId, sourcePeers = []) {
        if (this._joinInProgress) {
            return;
        }
        if (peerId) this.peerId = peerId;
        if (!this.peerId) {
            throw new Error('peer_id is required');
        }

        this._joinInProgress = true;
        try {
            await this.leaveStream();

            this.currentStreamId = streamId;
            this.peerConnection = new RTCPeerConnection(this.getIceConfiguration());
            this.setupPeerConnectionEvents();

            this.peerConnection.addTransceiver('video', { direction: 'recvonly' });
            this.peerConnection.addTransceiver('audio', { direction: 'recvonly' });

            const serverOffer = await this.apiClient.createSubscriberOffer(
                streamId,
                this.peerId,
                sourcePeers
            );
            await this.peerConnection.setRemoteDescription({
                type: 'offer',
                sdp: serverOffer.sdp,
            });

            const answer = await this.peerConnection.createAnswer();
            await this.peerConnection.setLocalDescription(answer);
            await this.apiClient.handleSubscriberAnswer(streamId, this.peerId, answer);

            this.isSubscriber = true;
            this.emit('streamJoined', streamId);
        } catch (error) {
            console.error('Error joining stream:', error);
            await this.leaveStream();
            throw new Error(`Failed to join stream: ${error.message}`);
        } finally {
            this._joinInProgress = false;
        }
    }

    async leaveStream() {
        if (this.peerConnection) {
            this.peerConnection.close();
            this.peerConnection = null;
        }

        if (this.remoteStream) {
            this.remoteStream.getTracks().forEach((track) => track.stop());
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

    handleOffer(data) {
        if (this.signalingMode === 'http' || !this.peerConnection) return;

        this.peerConnection
            .setRemoteDescription({
                type: 'offer',
                sdp: data.sdp,
            })
            .then(() => this.peerConnection.createAnswer())
            .then((answer) => this.peerConnection.setLocalDescription(answer))
            .then(() => {
                this.signalClient.sendAnswer(
                    this.peerConnection.localDescription.sdp,
                    data.fromPeer,
                    data.streamId
                );
            })
            .catch((error) => {
                console.error('Error handling offer:', error);
            });
    }

    handleAnswer(data) {
        if (this.signalingMode === 'http' || !this.peerConnection) return;

        this.peerConnection
            .setRemoteDescription({
                type: 'answer',
                sdp: data.sdp,
            })
            .catch((error) => {
                console.error('Error handling answer:', error);
            });
    }

    handleICECandidate(data) {
        if (this.signalingMode === 'http' || !this.peerConnection || !data.candidate) return;

        const candidate = new RTCIceCandidate({
            candidate: data.candidate,
            sdpMLineIndex: 0,
            sdpMid: '0',
        });

        this.peerConnection.addIceCandidate(candidate).catch((error) => {
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
            if (
                event.candidate &&
                this.signalingMode === 'websocket' &&
                this.signalClient?.connected
            ) {
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
                const playPromise = remoteVideo.play();
                if (playPromise) {
                    playPromise.catch((err) => {
                        if (err?.name !== 'AbortError') {
                            console.error('remoteVideo.play failed:', err);
                        }
                    });
                }
            }

            this.emit('streamReceived', this.remoteStream);
        };
    }

    getVideoConstraints(quality) {
        const constraints = {
            low: {
                width: { ideal: 640 },
                height: { ideal: 360 },
                frameRate: { ideal: 15, max: 30 },
            },
            medium: {
                width: { ideal: 1280 },
                height: { ideal: 720 },
                frameRate: { ideal: 30, max: 60 },
            },
            high: {
                width: { ideal: 1920 },
                height: { ideal: 1080 },
                frameRate: { ideal: 30, max: 60 },
            },
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
        return senders.find((sender) => sender.track && sender.track.kind === 'video');
    }
}
