package main

import (
    "os"

    "github.com/efebaykaraa/domain-dedicated-isp-bypass/server/config"
    "github.com/efebaykaraa/domain-dedicated-isp-bypass/server/logging"
    "github.com/efebaykaraa/domain-dedicated-isp-bypass/server/proxy"
    "github.com/efebaykaraa/domain-dedicated-isp-bypass/server/session"
)

var (
    cfg          *config.Config
    logg         *logging.Logging
    sessionStore *session.SessionStore
)

func init() {
    configPath := "config.json"

    // Check if a config file path is provided as an argument
    if len(os.Args) > 1 {
        configPath = os.Args[1]
    }

    // Initialize logging
    logg = logging.New(true)

    if len(os.Args) > 2 {
        logg.InitializeLogging(os.Args[2])
    } else {
        logg.InitializeLogging(".")
    }

    cfg = config.LoadConfig(configPath, logg)

    // Initialize session store
    sessionStore = session.NewSessionStore()

    // Watch configuration changes
    go cfg.WatchConfig()
}

func main() {
    // Start HTTP proxy (for HTTP/HTTPS traffic)
    go proxy.StartHTTPProxy(cfg, logg, sessionStore)

    // Start SOCKS5 proxy (for TCP and UDP traffic)
    go proxy.StartSOCKS5Proxy(cfg, logg, sessionStore)

    // Block main goroutine
    select {}
}
