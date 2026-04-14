# ssh-tunnel

Lightweight SSH port forwarder — no dependency on system `ssh`, pure Go binary.

## Quick Install

```bash
go install github.com/forzyh/ssh-tunnel@latest
```

Requires [Go](https://go.dev/) 1.19+. The binary will be placed in your `$GOPATH/bin` (or `$HOME/go/bin`).

## Usage

### 1. Configure

Create `~/.ssh-tunnel/config.json`:

```json
{
  "ssh_addr": "jump.example.com:22",
  "user": "your-username",
  "password": "",
  "keep_alive": 30,
  "forwards": [
    {
      "local_addr": "127.0.0.1:3306",
      "remote_addr": "192.168.1.100:3306"
    }
  ]
}
```

| Field | Description |
|---|---|
| `ssh_addr` | SSH server address (host:port) |
| `user` | SSH username |
| `password` | Leave empty to use SSH key, or fill to use password auth |
| `keep_alive` | Heartbeat interval in seconds, `0` to disable |
| `forwards` | Array of port forwarding rules |
| `forwards[].local_addr` | Local listen address |
| `forwards[].remote_addr` | Remote target address (reachable from the SSH server) |

### 2. Run

```bash
ssh-tunnel
```

That's it. The tunnel runs in the foreground. Press `Ctrl+C` to stop.

### Custom config path

```bash
ssh-tunnel -config /path/to/my-config.json
```

## Examples

### Forward inner MySQL to local

```json
{
  "ssh_addr": "jump.example.com:22",
  "user": "dev",
  "forwards": [
    {"local_addr": "127.0.0.1:3306", "remote_addr": "10.0.0.100:3306"}
  ]
}
```

Then connect your app to `127.0.0.1:3306`.

### Multiple forwards at once

```json
{
  "ssh_addr": "jump.example.com:22",
  "user": "dev",
  "forwards": [
    {"local_addr": "127.0.0.1:3306", "remote_addr": "10.0.0.100:3306"},
    {"local_addr": "127.0.0.1:6379", "remote_addr": "10.0.0.100:6379"},
    {"local_addr": "127.0.0.1:8080", "remote_addr": "10.0.0.200:443"}
  ]
}
```

### Use password auth instead of SSH key

```json
{
  "ssh_addr": "jump.example.com:22",
  "user": "dev",
  "password": "your-password",
  "forwards": [
    {"local_addr": "127.0.0.1:3306", "remote_addr": "10.0.0.100:3306"}
  ]
}
```

## SSH Key Setup

If you don't have an SSH key pair or the target doesn't recognize it:

```bash
# Generate a key (if you don't have one)
ssh-keygen -t ed25519

# Push your public key to the SSH server
ssh-copy-id your-username@jump.example.com
```

After that, `ssh-tunnel` will authenticate without a password.

## Output

```
2026/04/14 10:30:00 === SSH Tunnel ===
2026/04/14 10:30:00 Target: dev@jump.example.com:22
2026/04/14 10:30:00 Forward: 127.0.0.1:3306 -> 10.0.0.100:3306
2026/04/14 10:30:00 ----------------
2026/04/14 10:30:02 SSH connected to jump.example.com:22
2026/04/14 10:30:02 Listening on 127.0.0.1:3306 -> 10.0.0.100:3306
2026/04/14 10:31:15 [1] New connection on 127.0.0.1:3306
2026/04/14 10:31:15 [1] Pipe established
```

## Features

- **Pure Go** — no system `ssh` dependency, single static binary
- **Auto reconnect** — reconnects 3 seconds after disconnect
- **Keepalive** — configurable heartbeat to prevent idle timeout
- **Password prompt** — if no key and no password in config, prompts securely
- **Multiple forwards** — one tunnel, many services

## Build from source

```bash
git clone https://github.com/forzyh/ssh-tunnel.git
cd ssh-tunnel
go build -o ssh-tunnel .
```
