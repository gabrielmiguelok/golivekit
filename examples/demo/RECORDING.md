# Recording a Demo GIF

Instructions for creating a professional GIF demo for the README.

## Recommended Tools

### macOS
- **Kap** (free): https://getkap.co/
- **CleanShot X** (paid): https://cleanshot.com/

### Linux
- **Peek**: `sudo apt install peek`
- **Kazam**: `sudo apt install kazam`

### Cross-platform
- **OBS Studio** + convert to GIF
- **LICEcap**: https://www.cockos.com/licecap/

## Recording Steps

1. **Start the demo**:
   ```bash
   cd examples/demo && go run main.go
   ```

2. **Open browser** to `http://localhost:3000`

3. **Resize window** to ~800x600 for clean recording

4. **Record these actions** (10-15 seconds total):
   - Click "Increment" 3 times (show counter updating)
   - Click "Decrement" once
   - Click "Reset"
   - Pause briefly to show the live stats

5. **Export settings**:
   - Format: GIF
   - FPS: 15-20 (balance quality/size)
   - Width: 800px max
   - Target size: < 2MB for README

## Optimizing the GIF

Use `gifsicle` to optimize:

```bash
# Install
brew install gifsicle  # macOS
apt install gifsicle   # Linux

# Optimize
gifsicle -O3 --colors 128 --lossy=80 demo.gif -o demo-optimized.gif
```

## Adding to README

1. Save GIF to repository: `assets/demo.gif`

2. Add to README.md after the title:

```markdown
# GoliveKit

![GoliveKit Demo](assets/demo.gif)

**GoliveKit** is a LiveView framework for Go...
```

## Tips for a Great Demo

- Use a clean browser window (no extensions visible)
- Move mouse smoothly between clicks
- Pause briefly after each action so viewers can see the result
- Consider adding a subtle cursor highlight
