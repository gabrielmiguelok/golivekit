module.exports = {
  apps: [{
    name: 'golivekit-demo',
    script: './golivekit-demo',
    cwd: '/root/servidores/go/golivekit/examples/demo',
    env: {
      PORT: 24772
    },
    interpreter: 'none',
    exec_mode: 'fork'
  }]
}
