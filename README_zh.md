# ssh-tunnel

轻量级 SSH 端口转发工具 — 纯 Go 实现，不依赖系统 `ssh` 命令。

[English](README.md)

## 快速安装

```bash
go install github.com/forzyh/ssh-tunnel@latest
```

需要安装 [Go](https://go.dev/) 1.19+。二进制文件会安装到 `$HOME/go/bin`。

### 确保 `ssh-tunnel` 在 PATH 中

如果安装后提示 `command not found`，将 `$HOME/go/bin` 加入 shell 配置即可：

```bash
# 一键命令：写入配置并立即生效
echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.zshrc && source ~/.zshrc
# bash 用户：
# echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.bashrc && source ~/.bashrc
```

## 使用方式

### 1. 创建配置文件

创建 `~/.ssh-tunnel/config.json`：

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

| 字段 | 说明 |
|---|---|
| `ssh_addr` | SSH 服务器地址（host:port） |
| `user` | SSH 用户名 |
| `password` | 留空则使用 SSH key 认证，也可填入密码 |
| `keep_alive` | 心跳间隔（秒），`0` 表示关闭 |
| `forwards` | 端口转发规则数组 |
| `forwards[].local_addr` | 本地监听地址 |
| `forwards[].remote_addr` | 远程目标地址（从 SSH 服务器可达） |

### 2. 启动

```bash
ssh-tunnel
```

就这样。隧道在前台运行，按 `Ctrl+C` 停止。

### 指定配置文件路径

```bash
ssh-tunnel -config /path/to/my-config.json
```

## 使用场景

### 转发内网 MySQL 到本地

```json
{
  "ssh_addr": "jump.example.com:22",
  "user": "dev",
  "forwards": [
    {"local_addr": "127.0.0.1:3306", "remote_addr": "10.0.0.100:3306"}
  ]
}
```

启动后，你的应用直接连接 `127.0.0.1:3306` 即可访问内网数据库。

### 同时转发多个服务

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

### 使用密码认证代替 SSH key

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

## SSH Key 配置

如果没有 SSH 密钥对或目标机器不识别你的公钥：

```bash
# 生成密钥（如果没有的话）
ssh-keygen -t ed25519

# 将公钥推送到 SSH 服务器
ssh-copy-id your-username@jump.example.com
```

之后 `ssh-tunnel` 就能免密连接了。

## 运行效果

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

## 功能特性

- **纯 Go** — 不依赖系统 `ssh`，编译后单个静态二进制文件
- **自动重连** — 断开后 3 秒自动重连
- **心跳保活** — 可配置的心跳间隔，防止连接因空闲被断开
- **密码提示** — 无 key 且未配置密码时，安全提示输入
- **多端口转发** — 一个隧道，转发多个服务

## 源码编译

```bash
git clone https://github.com/forzyh/ssh-tunnel.git
cd ssh-tunnel
go build -o ssh-tunnel .
```

## 与系统 ssh -L 的对比

| | 系统 `ssh -L` | ssh-tunnel |
|---|---|---|
| 依赖 | 需要系统安装 OpenSSH | 纯 Go，无外部依赖 |
| 密码输入 | 需要终端交互或配 key | 配置文件可填密码，也可用 key |
| 断线重连 | 需要额外脚本 | 内置自动重连 |
| 心跳保活 | 需配置 `ServerAliveInterval` | 内置 `keep_alive` |
| 跨平台 | Windows 可能需要 Git Bash | 一次编译，到处运行 |
| 多端口 | 命令行参数越来越长 | 配置文件数组添加即可 |
