/**
 * GoliveKit - JavaScript Client
 * Handles WebSocket connections, DOM updates, and event handling.
 */

class GoliveKit {
    constructor(options = {}) {
        this.options = {
            url: this._defaultURL(),
            heartbeatInterval: 30000,
            reconnectMaxAttempts: 5,
            reconnectBaseDelay: 1000,
            reconnectMaxDelay: 30000,
            optimisticUpdates: true,
            eventDebounce: 16, // ~1 frame, prevents double-clicks but allows fast navigation
            ...options
        };

        this.socket = null;
        this.connected = false;
        this.joined = false;
        this.connecting = false;
        this.reconnecting = false;
        this.reconnectAttempts = 0;
        this.msgRef = 0;
        this.lastV = 0; // Version for diff ordering
        this.pendingReplies = new Map();
        this.hooks = new Map();
        this.eventListeners = new Map();
        this.heartbeatTimer = null;
        this.reconnectTimer = null;
        this.topic = null;

        // Optimistic UI state
        this.pendingOptimistic = new Map();
        this._lastEvents = new Map();

        this._onOpen = this._onOpen.bind(this);
        this._onClose = this._onClose.bind(this);
        this._onError = this._onError.bind(this);
        this._onMessage = this._onMessage.bind(this);

        // Inject styles for instant feedback
        this._injectStyles();
    }

    // Inject CSS for instant visual feedback
    _injectStyles() {
        if (document.getElementById('lv-styles')) return;
        const style = document.createElement('style');
        style.id = 'lv-styles';
        style.textContent = `
            /* Feedback inmediato al click */
            [lv-click] {
                cursor: pointer;
                transition: transform 0.08s ease, opacity 0.08s ease;
                user-select: none;
            }
            [lv-click]:active {
                transform: scale(0.97);
                opacity: 0.85;
            }
            [lv-click].lv-pending {
                opacity: 0.7;
            }

            /* Transición suave en slots */
            [data-slot] {
                transition: opacity 0.12s ease;
            }
            [data-slot].lv-updating {
                opacity: 0.7;
            }

            /* Tab activo optimista */
            .docs-nav-item.lv-optimistic-active {
                background: var(--accent-color, rgba(139,92,246,0.1)) !important;
                color: var(--color-primary, #8B5CF6) !important;
                font-weight: 600 !important;
            }
        `;
        document.head.appendChild(style);
    }

    // Apply optimistic update BEFORE sending to server
    _applyOptimisticNav(target, event, payload) {
        if (!this.options.optimisticUpdates) return;

        // Detect tab navigation
        if (event === 'nav' && payload.section) {
            // 1. Remove active class from all tabs
            document.querySelectorAll('.docs-nav-item').forEach(el => {
                el.classList.remove('docs-nav-item-active', 'lv-optimistic-active');
            });

            // 2. Mark clicked tab as active IMMEDIATELY
            target.classList.add('lv-optimistic-active');

            // 3. Scroll content to top IMMEDIATELY
            const contentSlot = document.querySelector('[data-slot="content"]');
            if (contentSlot) {
                contentSlot.scrollTop = 0;
            }
            // Also scroll the main window to top
            window.scrollTo({ top: 0, behavior: 'instant' });

            // 4. Save for revert if error
            this.pendingOptimistic.set('nav', {
                target,
                section: payload.section
            });
        }

        // Detect counter (increment/decrement)
        if (event === 'increment' || event === 'inc' || event === 'decrement' || event === 'dec') {
            const slotAttr = target.getAttribute('lv-slot');
            const slotEl = slotAttr
                ? document.querySelector(`[data-slot="${slotAttr}"]`)
                : target.closest('[data-live-view]')?.querySelector('[data-slot="count"]');

            if (slotEl) {
                const current = parseInt(slotEl.textContent) || 0;
                const delta = (event === 'increment' || event === 'inc') ? 1 : -1;
                this.pendingOptimistic.set(slotEl.dataset.slot, {
                    el: slotEl,
                    prev: slotEl.textContent
                });
                slotEl.textContent = String(current + delta);
                slotEl.classList.add('lv-updating');
            }
        }
    }

    // Confirm optimistic when response arrives
    _confirmOptimistic() {
        this.pendingOptimistic.forEach((data, key) => {
            if (key === 'nav') {
                data.target.classList.remove('lv-optimistic-active');
                // Server diff already updated the real classes
            } else if (data.el) {
                data.el.classList.remove('lv-updating');
            }
        });
        this.pendingOptimistic.clear();
    }

    // Revert if error
    _revertOptimistic() {
        this.pendingOptimistic.forEach((data, key) => {
            if (key === 'nav') {
                data.target.classList.remove('lv-optimistic-active');
                // Restore previous active tab
                document.querySelector('.docs-nav-item-active')?.classList.add('docs-nav-item-active');
            } else if (data.el) {
                data.el.textContent = data.prev;
                data.el.classList.remove('lv-updating');
            }
        });
        this.pendingOptimistic.clear();
    }

    _defaultURL() {
        const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
        // Connect to current path - the router handles WebSocket upgrade
        return `${protocol}//${location.host}${location.pathname}${location.search}`;
    }

    connect() {
        if (this.socket && this.connected) return Promise.resolve();
        if (this.connecting) return Promise.resolve();

        this.connecting = true;

        return new Promise((resolve) => {
            try {
                this.socket = new WebSocket(this.options.url);
                this.socket.onopen = () => { this._onOpen(); resolve(); };
                this.socket.onclose = this._onClose;
                this.socket.onerror = this._onError;
                this.socket.onmessage = this._onMessage;
            } catch (e) {
                this.connecting = false;
                resolve();
            }
        });
    }

    disconnect() {
        this._clearTimers();
        if (this.socket) {
            try { this.socket.close(1000); } catch (e) {}
            this.socket = null;
        }
        this.connected = false;
        this.joined = false;
        this.connecting = false;
        this.reconnecting = false;
    }

    _onOpen() {
        this.connected = true;
        this.connecting = false;
        this.reconnecting = false;
        this.reconnectAttempts = 0;
        this._startHeartbeat();
        this._join();
    }

    _join() {
        this.topic = 'lv:' + this._getLiveViewId();
        const ref = String(++this.msgRef);

        this.pendingReplies.set(ref, (payload) => {
            if (payload && payload.status === 'ok') {
                this.joined = true;
                this._callHooks('mounted');
            }
        });

        this._send({
            ref,
            join_ref: ref,
            topic: this.topic,
            event: 'phx_join',
            payload: { join_ref: ref }
        });
    }

    _onClose(event) {
        this.connected = false;
        this.joined = false;
        this.connecting = false;
        this._clearTimers();
        if (event.code !== 1000 && !this.reconnecting) {
            this._scheduleReconnect();
        }
        this._callHooks('disconnected');
    }

    _onError() {
        this.connecting = false;
    }

    _onMessage(event) {
        try {
            const msg = JSON.parse(event.data);
            this._handleMessage(msg);
        } catch (e) {}
    }

    _scheduleReconnect() {
        if (this.reconnectAttempts >= this.options.reconnectMaxAttempts) return;

        this.reconnecting = true;
        this.reconnectAttempts++;

        const delay = Math.min(
            this.options.reconnectBaseDelay * Math.pow(2, this.reconnectAttempts - 1),
            this.options.reconnectMaxDelay
        );

        this.reconnectTimer = setTimeout(() => {
            this.connect();
        }, delay);
    }

    _startHeartbeat() {
        this._stopHeartbeat();
        this.heartbeatTimer = setInterval(() => {
            if (this.connected) {
                this._send({ topic: 'phoenix', event: 'heartbeat', payload: {} });
            }
        }, this.options.heartbeatInterval);
    }

    _stopHeartbeat() {
        if (this.heartbeatTimer) {
            clearInterval(this.heartbeatTimer);
            this.heartbeatTimer = null;
        }
    }

    _clearTimers() {
        this._stopHeartbeat();
        if (this.reconnectTimer) {
            clearTimeout(this.reconnectTimer);
            this.reconnectTimer = null;
        }
    }

    _handleMessage(msg) {
        if (msg.ref && this.pendingReplies.has(msg.ref)) {
            const cb = this.pendingReplies.get(msg.ref);
            this.pendingReplies.delete(msg.ref);
            try { cb(msg.payload); } catch (e) {}
            return;
        }

        switch (msg.event) {
            case 'diff':
                this._applyDiff(msg.payload);
                // Confirm optimistic updates after diff is applied
                this._confirmOptimistic();
                break;
            case 'phx_reply':
                if (msg.payload && msg.payload.status === 'ok') {
                    this.joined = true;
                    const r = msg.payload.response;
                    if (r && r.rendered && r.rendered.s && r.rendered.s[0]) {
                        this._applyDiff({ f: r.rendered.s[0] });
                    }
                } else if (msg.payload && msg.payload.status === 'error') {
                    // Revert optimistic updates on error
                    this._revertOptimistic();
                }
                break;
        }
    }

    _applyDiff(diff) {
        // Version check for ordering (skip out-of-order updates)
        if (diff.v && diff.v <= this.lastV) return;
        if (diff.v) this.lastV = diff.v;

        // Full render (fallback)
        if (diff.f) {
            const container = document.querySelector('[data-live-view]');
            if (container) {
                const temp = document.createElement('div');
                temp.innerHTML = diff.f;
                while (container.firstChild) container.removeChild(container.firstChild);
                while (temp.firstChild) container.appendChild(temp.firstChild);
            }
            this._callHooks('updated');
            return;
        }

        // Text slots (fast path - textContent only)
        if (diff.s) {
            for (const [slotId, content] of Object.entries(diff.s)) {
                const slot = document.querySelector(`[data-slot="${slotId}"]`);
                if (slot) slot.textContent = content;
            }
        }

        // HTML slots (innerHTML with focus protection)
        if (diff.h) {
            const active = document.activeElement;
            for (const [slotId, content] of Object.entries(diff.h)) {
                const slot = document.querySelector(`[data-slot="${slotId}"]`);
                if (slot && !slot.contains(active)) {
                    slot.innerHTML = content;
                }
            }
        }

        // List operations (insert/delete/move/update)
        if (diff.l) {
            for (const [listId, ops] of Object.entries(diff.l)) {
                this._applyListOps(listId, ops);
            }
        }

        this._callHooks('updated');
    }

    _applyListOps(listId, ops) {
        const container = document.querySelector(`[data-list="${listId}"]`);
        if (!container) return;

        for (const op of ops) {
            switch (op.o) {
                case 'i': { // Insert
                    const template = document.createElement('template');
                    template.innerHTML = op.c;
                    const node = template.content.firstElementChild;
                    if (node) {
                        node.dataset.key = op.k;
                        const ref = container.children[op.i];
                        ref ? container.insertBefore(node, ref) : container.appendChild(node);
                    }
                    break;
                }
                case 'd': { // Delete
                    const el = container.querySelector(`[data-key="${op.k}"]`);
                    if (el) el.remove();
                    break;
                }
                case 'm': { // Move
                    const el = container.querySelector(`[data-key="${op.k}"]`);
                    if (el) {
                        const ref = container.children[op.i];
                        container.insertBefore(el, ref || null);
                    }
                    break;
                }
                case 'u': { // Update
                    const el = container.querySelector(`[data-key="${op.k}"]`);
                    if (el) el.outerHTML = op.c;
                    break;
                }
            }
        }
    }

    _send(msg) {
        if (!this.socket || !this.connected) return;
        try {
            this.socket.send(JSON.stringify(msg));
        } catch (e) {}
    }

    pushEvent(event, payload = {}) {
        // Lazy connect on first interaction
        if (!this.connected && !this.connecting) {
            this.connect().then(() => {
                setTimeout(() => this._pushEvent(event, payload), 100);
            });
            return Promise.resolve();
        }
        return this._pushEvent(event, payload);
    }

    _pushEvent(event, payload) {
        if (!this.connected || !this.joined) return Promise.resolve();

        const ref = String(++this.msgRef);

        return new Promise((resolve) => {
            this.pendingReplies.set(ref, resolve);
            this._send({ ref, topic: this.topic, event, payload });
            setTimeout(() => {
                if (this.pendingReplies.has(ref)) {
                    this.pendingReplies.delete(ref);
                    resolve();
                }
            }, 10000);
        });
    }

    _getLiveViewId() {
        const el = document.querySelector('[data-live-view]');
        return el ? el.dataset.liveView : 'main';
    }

    bindEvents() {
        document.addEventListener('click', (e) => {
            const target = e.target.closest('[lv-click]');
            if (!target) return;

            e.preventDefault();

            const event = target.getAttribute('lv-click');
            const payload = this._getPayload(target);

            // Debounce rapid clicks
            const key = event + JSON.stringify(payload);
            const now = Date.now();
            if (this._lastEvents.get(key) && (now - this._lastEvents.get(key)) < this.options.eventDebounce) {
                return;
            }
            this._lastEvents.set(key, now);

            // 1. Instant visual feedback
            target.classList.add('lv-pending');

            // 2. Optimistic update BEFORE sending
            this._applyOptimisticNav(target, event, payload);

            // 3. Send event (confirmation happens when diff arrives)
            this.pushEvent(event, payload)
                .then(() => {
                    target.classList.remove('lv-pending');
                    // Note: _confirmOptimistic is called when diff arrives in _handleMessage
                })
                .catch(() => {
                    target.classList.remove('lv-pending');
                    this._revertOptimistic();
                });
        });

        document.addEventListener('submit', (e) => {
            const form = e.target.closest('[lv-submit]');
            if (form) {
                e.preventDefault();
                this.pushEvent(form.getAttribute('lv-submit'), Object.fromEntries(new FormData(form)));
            }
        });

        document.addEventListener('change', (e) => {
            const target = e.target.closest('[lv-change]');
            if (target) {
                const debounce = parseInt(target.getAttribute('lv-debounce') || '0');
                const payload = { value: target.value, ...this._getPayload(target) };
                if (debounce > 0) {
                    clearTimeout(target._dt);
                    target._dt = setTimeout(() => this.pushEvent(target.getAttribute('lv-change'), payload), debounce);
                } else {
                    this.pushEvent(target.getAttribute('lv-change'), payload);
                }
            }
        });

        document.addEventListener('input', (e) => {
            const target = e.target.closest('[lv-input]');
            if (target) {
                const debounce = parseInt(target.getAttribute('lv-debounce') || '300');
                clearTimeout(target._dt);
                target._dt = setTimeout(() => {
                    this.pushEvent(target.getAttribute('lv-input'), { value: target.value, ...this._getPayload(target) });
                }, debounce);
            }
        });
    }

    _getPayload(el) {
        const p = {};
        for (const attr of el.attributes) {
            if (attr.name.startsWith('lv-value-')) p[attr.name.slice(9)] = attr.value;
        }
        return p;
    }

    registerHook(name, callbacks) { this.hooks.set(name, callbacks); }

    _callHooks(event) {
        try {
            document.querySelectorAll('[lv-hook]').forEach(el => {
                const hook = this.hooks.get(el.getAttribute('lv-hook'));
                if (hook && hook[event]) try { hook[event].call(el); } catch (e) {}
            });
        } catch (e) {}
    }

    on(event, cb) {
        if (!this.eventListeners.has(event)) this.eventListeners.set(event, []);
        this.eventListeners.get(event).push(cb);
    }

    off(event, cb) {
        if (this.eventListeners.has(event)) {
            const l = this.eventListeners.get(event);
            const i = l.indexOf(cb);
            if (i > -1) l.splice(i, 1);
        }
    }
}

// Create instance and bind events only
window.liveView = new GoliveKit();
document.addEventListener('DOMContentLoaded', () => {
    if (document.querySelector('[data-live-view]')) {
        window.liveView.bindEvents();
    }
});

if (typeof module !== 'undefined' && module.exports) module.exports = GoliveKit;
