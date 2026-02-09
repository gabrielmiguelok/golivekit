# JavaScript Client

GoliveKit includes a lightweight JavaScript client that handles WebSocket communication, DOM updates, and user interactions.

## Setup

Include the client script in your HTML:

```html
<script src="/_live/golivekit.js"></script>
```

The client automatically:
- Connects to the WebSocket endpoint
- Handles reconnection with exponential backoff
- Processes DOM diffs
- Captures and sends user events

## HTML Attributes

GoliveKit uses special `lv-*` attributes to bind events:

### lv-click

Triggered on click events:

```html
<button lv-click="increment">+1</button>
<button lv-click="delete" lv-value-id="123">Delete</button>
```

### lv-change

Triggered on change events (select, checkbox, radio):

```html
<select lv-change="update_category">
    <option value="tech">Technology</option>
    <option value="sports">Sports</option>
</select>

<input type="checkbox" lv-change="toggle_active" lv-value-id="456">
```

### lv-input

Triggered on input events (real-time typing):

```html
<input type="text" lv-input="search" lv-debounce="300" placeholder="Search...">
```

### lv-submit

Triggered on form submission:

```html
<form lv-submit="save_user">
    <input type="text" name="username">
    <input type="email" name="email">
    <button type="submit">Save</button>
</form>
```

Form data is automatically serialized and sent as the event payload.

### lv-hook

Attach JavaScript hooks to elements:

```html
<div lv-hook="Chart" data-values="[1,2,3,4,5]">
    <!-- Chart will be rendered here -->
</div>
```

### lv-debounce

Delay event sending (useful for search inputs):

```html
<input lv-input="search" lv-debounce="300">  <!-- 300ms delay -->
<input lv-input="validate" lv-debounce="blur">  <!-- On blur only -->
```

### lv-value-*

Send additional data with events:

```html
<button lv-click="delete" lv-value-id="123" lv-value-type="user">
    Delete User #123
</button>
```

This sends: `{id: "123", type: "user"}`

## Slots

Slots enable efficient partial updates without full re-renders:

### Text Slots

For simple text content updates:

```html
<div data-live-view="counter">
    <p>Count: <span data-slot="count">0</span></p>
</div>
```

When `count` changes, only the span content is updated.

### List Slots

For dynamic lists with keyed items:

```html
<ul data-list="items">
    <li data-key="1">Item 1</li>
    <li data-key="2">Item 2</li>
    <li data-key="3">Item 3</li>
</ul>
```

- `data-list` marks the list container
- `data-key` provides stable identity for each item
- Enables efficient add/remove/reorder operations

## JavaScript Hooks

Hooks let you run custom JavaScript when elements are mounted, updated, or destroyed.

### Defining Hooks

```javascript
window.liveView.registerHook('Chart', {
    mounted() {
        // Called when element is added to DOM
        this.chart = new Chart(this.el, {
            data: JSON.parse(this.el.dataset.values)
        })
    },

    updated() {
        // Called when element is updated via diff
        const newData = JSON.parse(this.el.dataset.values)
        this.chart.update(newData)
    },

    destroyed() {
        // Called when element is removed from DOM
        this.chart.destroy()
    }
})
```

### Hook Context

Inside hook callbacks, `this` provides:

| Property | Description |
|----------|-------------|
| `this.el` | The DOM element |
| `this.pushEvent(event, payload)` | Send event to server |
| `this.pushEventTo(selector, event, payload)` | Send to specific component |

### Example: Chart Hook

```javascript
window.liveView.registerHook('Chart', {
    mounted() {
        this.chart = new Chart(this.el.getContext('2d'), {
            type: 'line',
            data: {
                labels: [],
                datasets: [{
                    label: 'Values',
                    data: []
                }]
            }
        })
    },

    updated() {
        const values = JSON.parse(this.el.dataset.values)
        this.chart.data.datasets[0].data = values
        this.chart.update()
    },

    destroyed() {
        this.chart.destroy()
    }
})
```

## Programmatic API

### Sending Events

```javascript
// Send event to current LiveView
window.liveView.pushEvent('my_event', {key: 'value'})

// Send event to specific component
window.liveView.pushEventTo('#user-form', 'validate', {field: 'email'})
```

### Listening to Client Events

```javascript
// Connection status
window.liveView.on('connected', () => {
    console.log('Connected to server')
})

window.liveView.on('disconnected', () => {
    showReconnectingBanner()
})

window.liveView.on('reconnected', () => {
    hideReconnectingBanner()
})

// Server errors
window.liveView.on('error', (error) => {
    console.error('Server error:', error)
})
```

### Removing Listeners

```javascript
const handler = () => console.log('connected')
window.liveView.on('connected', handler)
window.liveView.off('connected', handler)
```

## Islands Architecture

For partial hydration, use `<golive-island>` elements:

```html
<golive-island
    id="weather-widget"
    component="WeatherWidget"
    hydrate="visible"
    props='{"city":"NYC"}'>

    <!-- Static placeholder shown until hydration -->
    <div class="skeleton">Loading weather...</div>
</golive-island>
```

### Hydration Strategies

| Strategy | Behavior |
|----------|----------|
| `load` | Hydrate immediately on page load |
| `visible` | Hydrate when element enters viewport |
| `idle` | Hydrate when browser is idle (`requestIdleCallback`) |
| `interaction` | Hydrate on first user interaction (click, focus) |
| `media` | Hydrate when media query matches |
| `none` | Never hydrate (static content) |

### Island JavaScript

```html
<script src="/_live/islands.js"></script>
```

The islands manager observes the DOM and hydrates components based on their strategy.

## Optimistic UI

GoliveKit provides instant feedback through CSS:

### Automatic States

When an event is triggered:
1. Element gets `.lv-loading` class
2. After server confirms, class is removed
3. If error occurs, `.lv-error` class is added

### CSS Example

```css
/* Loading state */
button.lv-loading {
    opacity: 0.7;
    cursor: wait;
}

/* Error state */
.lv-error {
    outline: 2px solid red;
}

/* Custom loading indicator */
.lv-loading::after {
    content: '';
    display: inline-block;
    width: 12px;
    height: 12px;
    border: 2px solid #ccc;
    border-top-color: #333;
    border-radius: 50%;
    animation: spin 0.6s linear infinite;
}

@keyframes spin {
    to { transform: rotate(360deg); }
}
```

## Configuration

Configure the client before connection:

```javascript
window.liveViewConfig = {
    // WebSocket URL (default: auto-detected)
    socketUrl: '/ws',

    // Reconnection settings
    reconnectAfterMs: [1000, 2000, 5000, 10000],

    // Heartbeat interval (default: 30000)
    heartbeatIntervalMs: 30000,

    // Debug mode
    debug: true
}
```

## Debugging

Enable debug mode to see WebSocket traffic:

```javascript
window.liveViewConfig = { debug: true }
```

This logs:
- Connection events
- Sent/received messages
- Diff operations
- Hook lifecycle

## Browser Support

- Chrome 80+
- Firefox 75+
- Safari 13.1+
- Edge 80+

For older browsers, the client automatically falls back to SSE or long-polling.
