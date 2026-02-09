# GoliveKit Demo

Interactive demo showcasing GoliveKit's real-time capabilities.

## Features

- **Live Counter**: Click buttons to see instant WebSocket updates
- **Live Stats**: Shows active visitors and total clicks across all sessions
- **Sample Code**: Displays the simplicity of GoliveKit components

## Run Locally

```bash
cd examples/demo
go run main.go
# Open http://localhost:3000
```

## Deploy to Fly.io

1. Install Fly CLI: https://fly.io/docs/hands-on/install-flyctl/

2. Login to Fly:
```bash
fly auth login
```

3. Deploy from repository root:
```bash
cd /path/to/golivekit
fly launch --name golivekit-demo --dockerfile examples/demo/Dockerfile
```

4. Your demo will be live at `https://golivekit-demo.fly.dev`

## Deploy to Render

1. Create a new Web Service on [Render](https://render.com)
2. Connect your GitHub repository
3. Configure:
   - **Build Command**: `go build -o demo ./examples/demo`
   - **Start Command**: `./demo`
   - **Environment**: `PORT=10000`

## Deploy to Railway

```bash
# From repository root
railway init
railway up
```

## Docker

```bash
# Build from repo root
docker build -f examples/demo/Dockerfile -t golivekit-demo .

# Run
docker run -p 3000:3000 golivekit-demo
```
