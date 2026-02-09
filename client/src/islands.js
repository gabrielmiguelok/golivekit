/**
 * GoliveKit Islands - Partial Hydration Manager
 *
 * This module handles the hydration of island components
 * using various strategies (load, visible, idle, interaction).
 */

class IslandManager {
    constructor() {
        this.islands = new Map();
        this.observers = new Map();
        this.hydrationQueue = [];
        this.isHydrating = false;
    }

    init() {
        // Find all islands in the page
        document.querySelectorAll('golive-island').forEach(el => {
            this.registerIsland(el);
        });

        // Process hydration queue
        this.processQueue();

        // Watch for dynamically added islands
        this._observeMutations();
    }

    registerIsland(element) {
        const island = {
            id: element.id,
            component: element.getAttribute('component'),
            hydrate: element.getAttribute('hydrate') || 'load',
            priority: parseInt(element.getAttribute('priority')) || 2,
            props: this._parseProps(element.getAttribute('props')),
            element: element,
            hydrated: false,
            socket: null
        };

        this.islands.set(island.id, island);

        // Configure hydration strategy
        switch (island.hydrate) {
            case 'load':
                this.queueHydration(island, true);
                break;

            case 'visible':
                this._observeVisibility(island);
                break;

            case 'idle':
                this._scheduleIdle(island);
                break;

            case 'interaction':
                this._observeInteraction(island);
                break;

            case 'media':
                this._observeMedia(island);
                break;

            case 'none':
                // Never hydrate
                break;
        }
    }

    _parseProps(propsAttr) {
        if (!propsAttr) return {};
        try {
            return JSON.parse(propsAttr);
        } catch {
            return {};
        }
    }

    _observeVisibility(island) {
        const observer = new IntersectionObserver((entries) => {
            entries.forEach(entry => {
                if (entry.isIntersecting) {
                    this.queueHydration(island);
                    observer.disconnect();
                    this.observers.delete(island.id);
                }
            });
        }, {
            threshold: 0.1,
            rootMargin: '50px'
        });

        observer.observe(island.element);
        this.observers.set(island.id, observer);
    }

    _scheduleIdle(island) {
        if ('requestIdleCallback' in window) {
            requestIdleCallback(() => {
                this.queueHydration(island);
            }, { timeout: 2000 });
        } else {
            // Fallback for Safari
            setTimeout(() => this.queueHydration(island), 200);
        }
    }

    _observeInteraction(island) {
        const handler = (e) => {
            this.queueHydration(island, true);
            cleanup();
        };

        const cleanup = () => {
            island.element.removeEventListener('click', handler);
            island.element.removeEventListener('focus', handler);
            island.element.removeEventListener('mouseenter', handler);
        };

        island.element.addEventListener('click', handler, { once: true, passive: true });
        island.element.addEventListener('focus', handler, { once: true, passive: true });
        island.element.addEventListener('mouseenter', handler, { once: true, passive: true });
    }

    _observeMedia(island) {
        const mediaQuery = island.element.getAttribute('media');
        if (!mediaQuery) {
            this.queueHydration(island);
            return;
        }

        const mql = window.matchMedia(mediaQuery);
        const handler = (e) => {
            if (e.matches) {
                this.queueHydration(island);
                mql.removeEventListener('change', handler);
            }
        };

        if (mql.matches) {
            this.queueHydration(island);
        } else {
            mql.addEventListener('change', handler);
        }
    }

    _observeMutations() {
        const observer = new MutationObserver((mutations) => {
            mutations.forEach(mutation => {
                mutation.addedNodes.forEach(node => {
                    if (node.nodeType === Node.ELEMENT_NODE) {
                        if (node.tagName === 'GOLIVE-ISLAND') {
                            this.registerIsland(node);
                        }
                        node.querySelectorAll('golive-island').forEach(el => {
                            if (!this.islands.has(el.id)) {
                                this.registerIsland(el);
                            }
                        });
                    }
                });
            });
        });

        observer.observe(document.body, {
            childList: true,
            subtree: true
        });
    }

    queueHydration(island, immediate = false) {
        if (island.hydrated) return;

        this.hydrationQueue.push(island);

        // Sort by priority (higher first)
        this.hydrationQueue.sort((a, b) => b.priority - a.priority);

        if (immediate) {
            this.processQueue();
        } else {
            // Use microtask to batch hydrations
            queueMicrotask(() => this.processQueue());
        }
    }

    async processQueue() {
        if (this.isHydrating || this.hydrationQueue.length === 0) return;

        this.isHydrating = true;

        while (this.hydrationQueue.length > 0) {
            const island = this.hydrationQueue.shift();
            await this.hydrateIsland(island);
        }

        this.isHydrating = false;
    }

    async hydrateIsland(island) {
        if (island.hydrated) return;

        console.debug(`[GoliveKit] Hydrating island: ${island.id}`);

        try {
            // Connect WebSocket for this island
            if (window.liveView && window.liveView.connected) {
                // Use existing connection
                await window.liveView.pushEvent('island:mount', {
                    id: island.id,
                    component: island.component,
                    props: island.props
                });
            }

            // Mark as hydrated
            island.hydrated = true;
            island.element.setAttribute('hydrated', 'true');
            island.element.classList.add('hydrated');

            // Dispatch event
            island.element.dispatchEvent(new CustomEvent('golive:hydrated', {
                bubbles: true,
                detail: { island }
            }));

            // Call any mounted callbacks
            const hookName = island.element.getAttribute('lv-hook');
            if (hookName && window.liveView.hooks.has(hookName)) {
                const hook = window.liveView.hooks.get(hookName);
                if (hook.mounted) {
                    hook.mounted.call(island.element);
                }
            }

        } catch (err) {
            console.error(`[GoliveKit] Failed to hydrate island ${island.id}:`, err);
            island.element.classList.add('hydration-error');
        }
    }

    dehydrateIsland(id) {
        const island = this.islands.get(id);
        if (!island) return;

        island.hydrated = false;
        island.element.removeAttribute('hydrated');
        island.element.classList.remove('hydrated');

        if (island.socket) {
            island.socket.disconnect();
            island.socket = null;
        }

        this.islands.delete(id);

        // Cleanup observer if exists
        const observer = this.observers.get(id);
        if (observer) {
            observer.disconnect();
            this.observers.delete(id);
        }
    }

    getIsland(id) {
        return this.islands.get(id);
    }

    getAllIslands() {
        return Array.from(this.islands.values());
    }

    getHydratedIslands() {
        return this.getAllIslands().filter(i => i.hydrated);
    }

    getPendingIslands() {
        return this.getAllIslands().filter(i => !i.hydrated);
    }
}

// Create global instance
window.islandManager = new IslandManager();

// Auto-init on DOMContentLoaded
document.addEventListener('DOMContentLoaded', () => {
    window.islandManager.init();
});

// Export for module systems
if (typeof module !== 'undefined' && module.exports) {
    module.exports = IslandManager;
}
