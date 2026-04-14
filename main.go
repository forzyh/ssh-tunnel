package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// ForwardRule defines a local-to-remote port forwarding rule.
type ForwardRule struct {
	LocalAddr  string `json:"local_addr"`  // e.g. "127.0.0.1:8080"
	RemoteAddr string `json:"remote_addr"` // e.g. "192.168.1.100:3306"
}

// Config holds SSH tunnel configuration.
type Config struct {
	SSHAddr   string        `json:"ssh_addr"`   // e.g. "jump.example.com:22"
	User      string        `json:"user"`       // e.g. "your-username"
	Password  string        `json:"password"`   // leave empty to prompt or use SSH key
	Forwards  []ForwardRule `json:"forwards"`   // port forwarding rules
	KeepAlive int           `json:"keep_alive"` // heartbeat interval in seconds, 0 = disabled
}

var logger *log.Logger

func main() {
	configPath := flag.String("config", "", "path to config file (json)")
	flag.Parse()

	// Default to ~/.ssh-tunnel/config.json if not specified
	if *configPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Cannot get home directory: %v", err)
		}
		*configPath = filepath.Join(home, ".ssh-tunnel", "config.json")
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Load config: %v\n\n", err)
		fmt.Fprintf(os.Stderr, "Usage: ssh-tunnel [-config path]\n")
		fmt.Fprintf(os.Stderr, "  Default config: ~/.ssh-tunnel/config.json\n\n")
		fmt.Fprintf(os.Stderr, "Config file format (JSON):\n")
		fmt.Fprintf(os.Stderr, `{
  "ssh_addr": "jump.example.com:22",
  "user": "your-username",
  "password": "",       // optional, leave empty to prompt or use SSH key
  "keep_alive": 30,     // optional, heartbeat interval in seconds
  "forwards": [
    {"local_addr": "127.0.0.1:3306", "remote_addr": "192.168.1.100:3306"}
  ]
}
`)
		os.Exit(1)
	}

	// Resolve auth: prefer SSH key, fallback to password prompt
	if cfg.Password == "" {
		if _, err := loadDefaultKey(); err != nil {
			// No local SSH key found
			printKeySetupGuide(cfg)
			os.Exit(1)
		}
		// Key exists, will attempt key auth
	}

	logger = log.New(os.Stdout, "", log.LstdFlags)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger.Println("=== SSH Tunnel ===")
	logger.Printf("Target: %s@%s\n", cfg.User, cfg.SSHAddr)
	for _, f := range cfg.Forwards {
		logger.Printf("Forward: %s -> %s", f.LocalAddr, f.RemoteAddr)
	}
	logger.Println("----------------")

	reconnect := true
	for {
		if err := runTunnel(ctx, cfg); err != nil {
			logger.Printf("Tunnel error: %v", err)
		}

		if !reconnect || ctx.Err() != nil {
			break
		}
		logger.Println("Reconnecting in 3s...")
		select {
		case <-time.After(3 * time.Second):
		case <-ctx.Done():
			break
		}
	}
	logger.Println("SSH tunnel stopped")
}

func printKeySetupGuide(cfg Config) {
	fmt.Fprintf(os.Stderr, `
SSH key not found. To enable password-free login, run:

    ssh-copy-id %s@%s

Then try again.

`, cfg.User, cfg.SSHAddr)
}

func loadConfig(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.SSHAddr == "" {
		return cfg, fmt.Errorf("ssh_addr is required")
	}
	if cfg.User == "" {
		return cfg, fmt.Errorf("user is required")
	}
	if len(cfg.Forwards) == 0 {
		return cfg, fmt.Errorf("forwards is required")
	}
	return cfg, nil
}

func runTunnel(ctx context.Context, cfg Config) error {
	client, err := dialSSH(ctx, cfg)
	if err != nil {
		return err
	}
	defer client.Close()
	logger.Printf("SSH connected to %s", cfg.SSHAddr)

	keepAliveSec := cfg.KeepAlive
	if keepAliveSec == 0 {
		keepAliveSec = 30
	}
	go keepAlive(ctx, client, time.Duration(keepAliveSec)*time.Second)

	errCh := make(chan error, len(cfg.Forwards))
	for _, rule := range cfg.Forwards {
		go func(r ForwardRule) {
			errCh <- startForward(ctx, client, r)
		}(rule)
	}

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

func dialSSH(ctx context.Context, cfg Config) (*ssh.Client, error) {
	authMethods := []ssh.AuthMethod{}

	if key, err := loadDefaultKey(); err == nil {
		authMethods = append(authMethods, ssh.PublicKeys(key))
	}
	if cfg.Password != "" {
		authMethods = append(authMethods, ssh.Password(cfg.Password))
	}

	sshConfig := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", cfg.SSHAddr)
	if err != nil {
		return nil, fmt.Errorf("dial SSH: %w", err)
	}

	c, chans, reqs, err := ssh.NewClientConn(conn, cfg.SSHAddr, sshConfig)
	if err != nil {
		conn.Close()
		// If key auth failed and no password provided, guide the user
		if cfg.Password == "" && isAuthError(err) {
			fmt.Fprintf(os.Stderr, "\nSSH key authentication failed. Target machine may not recognize your public key.\n")
			printKeySetupGuide(cfg)
		}
		return nil, fmt.Errorf("SSH handshake: %w", err)
	}
	return ssh.NewClient(c, chans, reqs), nil
}

func loadDefaultKey() (ssh.Signer, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	keyPath := home + "/.ssh/id_ed25519"
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		keyPath = home + "/.ssh/id_rsa"
	}
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	return ssh.ParsePrivateKey(data)
}

func startForward(ctx context.Context, client *ssh.Client, rule ForwardRule) error {
	listener, err := net.Listen("tcp", rule.LocalAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", rule.LocalAddr, err)
	}
	defer listener.Close()
	logger.Printf("Listening on %s -> %s", rule.LocalAddr, rule.RemoteAddr)

	connCount := 0
	for {
		local, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("accept: %w", err)
		}

		connCount++
		id := connCount
		logger.Printf("[%d] New connection on %s", id, rule.LocalAddr)

		go func() {
			remote, err := client.Dial("tcp", rule.RemoteAddr)
			if err != nil {
				logger.Printf("[%d] Dial remote %s: %v", id, rule.RemoteAddr, err)
				local.Close()
				return
			}

			go pipe(local, remote, fmt.Sprintf("[%d]", id))
			go pipe(remote, local, fmt.Sprintf("[%d]", id))
			logger.Printf("[%d] Pipe established", id)
		}()
	}
}

func keepAlive(ctx context.Context, client *ssh.Client, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
			if err != nil {
				logger.Printf("Keepalive failed: %v", err)
				return
			}
		}
	}
}

func pipe(a, b net.Conn, tag string) {
	defer a.Close()
	defer b.Close()
	buf := make([]byte, 64*1024)
	for {
		n, err := a.Read(buf)
		if n > 0 {
			if _, werr := b.Write(buf[:n]); werr != nil {
				logger.Printf("%s Write error: %v", tag, werr)
				return
			}
		}
		if err != nil {
			return
		}
	}
}

func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "unable to authenticate") || strings.Contains(s, "permission denied")
}

func init() {
	log.SetOutput(os.Stdout)
}
