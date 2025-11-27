class SignalClient {
    constructor() {
        this.connected = false;
        this.ws = null;
        this.eventHandlers = {};
    }

    async connect() {
        return new Promise((resolve) => {
            // Simulate connection
            setTimeout(() => {
                this.connected = true;
                this.emit('connected');
                resolve();
            }, 500);
        });
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

    async publishStream(stream) {
        return `stream-${Date.now()}`;
    }

    async unpublishStream(streamId) {
        return true;
    }

    async getAvailableStreams() {
        // Simulate available streams
        return [
            { id: 'stream-1', name: 'Main Stream', viewers: 5 },
            { id: 'stream-2', name: 'Demo Stream', viewers: 2 }
        ];
    }

    async getServiceStatus() {
        return {
            signal: 'connected',
            ingest: 'connected',
            web: 'connected'
        };
    }
}