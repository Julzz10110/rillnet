class Logger {
    constructor(prefix = 'App') {
        this.prefix = prefix;
        this.logs = [];
        this.autoScroll = true;
        this.filters = {
            info: true,
            success: true,
            warning: true,
            error: true
        };
    }

    log(level, message, data = null) {
        const timestamp = new Date().toLocaleTimeString();
        const logEntry = {
            timestamp,
            level,
            message,
            data,
            prefix: this.prefix
        };

        this.logs.push(logEntry);
        this.displayLog(logEntry);
    }

    info(message, data = null) {
        this.log('info', message, data);
    }

    success(message, data = null) {
        this.log('success', message, data);
    }

    warning(message, data = null) {
        this.log('warning', message, data);
    }

    error(message, data = null) {
        this.log('error', message, data);
    }

    displayLog(entry) {
        if (!this.filters[entry.level]) return;

        const logContainer = document.getElementById('connectionLog');
        if (!logContainer) return;

        const logElement = document.createElement('div');
        logElement.className = `log-entry log-level-${entry.level}`;
        logElement.innerHTML = `
            <span class="log-timestamp">[${entry.timestamp}]</span>
            <span class="log-prefix">${entry.prefix}:</span>
            <span class="log-message">${entry.message}</span>
        `;

        logContainer.appendChild(logElement);

        if (this.autoScroll) {
            logContainer.scrollTop = logContainer.scrollHeight;
        }
    }

    clear() {
        this.logs = [];
        const logContainer = document.getElementById('connectionLog');
        if (logContainer) {
            logContainer.innerHTML = '';
        }
    }

    setAutoScroll(enabled) {
        this.autoScroll = enabled;
        const button = document.getElementById('autoScroll');
        if (button) {
            button.classList.toggle('active', enabled);
        }
    }

    setFilters(filters) {
        this.filters = { ...this.filters, ...filters };
        this.refreshLogDisplay();
    }

    refreshLogDisplay() {
        const logContainer = document.getElementById('connectionLog');
        if (!logContainer) return;

        logContainer.innerHTML = '';
        this.logs.forEach(entry => this.displayLog(entry));
    }

    export() {
        const logText = this.logs.map(entry => 
            `[${entry.timestamp}] ${entry.prefix} [${entry.level.toUpperCase()}]: ${entry.message}`
        ).join('\n');

        const blob = new Blob([logText], { type: 'text/plain' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `rillnet-log-${new Date().toISOString().split('T')[0]}.txt`;
        a.click();
        URL.revokeObjectURL(url);
    }

    getLogs() {
        return this.logs;
    }
}