package config

import (
    "encoding/json"
    "io/ioutil"
    "sync"

    "github.com/fsnotify/fsnotify"
    "github.com/efebaykaraa/domain-dedicated-isp-bypass/client/logging"
)

type Config struct {
    Username   string `json:"username"`
    Password   string `json:"password"`
    DomainName string `json:"domain_name"`
    ConfigPath string
    Mutex      sync.RWMutex
    Logger     *logging.Logger
}

func LoadConfig(path string, logger *logging.Logger) *Config {
    cfg := &Config{
        ConfigPath: path,
        Logger:     logger,
    }
    cfg.loadConfig()
    return cfg
}

func (c *Config) loadConfig() {
    c.Mutex.Lock()
    defer c.Mutex.Unlock()

    data, err := ioutil.ReadFile(c.ConfigPath)
    if err != nil {
        c.Logger.Fatalf("Failed to read config file: %s", err)
    }

    tempConfig := struct {
        Username   string `json:"username"`
        Password   string `json:"password"`
        DomainName string `json:"domain_name"`
    }{}

    if err := json.Unmarshal(data, &tempConfig); err != nil {
        c.Logger.Fatalf("Failed to parse config: %s", err)
    }

    c.Username = tempConfig.Username
    c.Password = tempConfig.Password
    c.DomainName = tempConfig.DomainName

    c.Logger.Logln("Configuration loaded")
}

func (c *Config) WatchConfig() {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        c.Logger.Fatalf("Failed to create file watcher: %s", err)
    }
    defer watcher.Close()

    err = watcher.Add(".")
    if err != nil {
        c.Logger.Fatalf("Failed to add directory to watcher: %s", err)
    }

    for {
        select {
        case event, ok := <-watcher.Events:
            if !ok {
                return
            }
            if event.Op&(fsnotify.Write|fsnotify.Create) != 0 && event.Name == c.ConfigPath {
                c.Logger.Logln("Config file changed, reloading...")
                c.loadConfig()
            }
        case err, ok := <-watcher.Errors:
            if !ok {
                return
            }
            c.Logger.Logf("Watcher error: %s", err)
        }
    }
}
