class MetricsCollector {
    constructor() {
        this.metrics = {
            peers: 0,
            streams: 0,
            bandwidth: 0,
            latency: 0,
            packetLoss: '0%',
            connections: 0
        };
        this.eventHandlers = {};
    }

    collect() {
        // Simulate metrics collection
        this.metrics = {
            peers: Math.floor(Math.random() * 10),
            streams: Math.floor(Math.random() * 3) + 1,
            bandwidth: Math.floor(Math.random() * 5000) + 1000,
            latency: Math.floor(Math.random() * 50) + 10,
            packetLoss: (Math.random() * 2).toFixed(1) + '%',
            connections: Math.floor(Math.random() * 8) + 2
        };

        this.emit('metricsUpdate', this.metrics);
        return this.metrics;
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
}