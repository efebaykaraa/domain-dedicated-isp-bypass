package config

import (
    "encoding/json"
    "io/ioutil"
    "sync"

    "github.com/fsnotify/fsnotify"
    "github.com/efebaykaraa/domain-dedicated-isp-bypass/server/logging"
)

type UserCredential struct {
    Username string `json:"username"`
    Password string `json:"password"`
}

type DomainMapping struct {
    From string `json:"from"`
    To   string `json:"to"`
}

type Config struct {
    UserCredentials []UserCredential `json:"user_credentials"`
    DomainMappings  []DomainMapping  `json:"domain_mappings"`
    ConfigPath      string
    Mutex           sync.RWMutex
    Logging         *logging.Logging
}

func LoadConfig(path string, logging *logging.Logging) *Config {
    cfg := &Config{
        ConfigPath: path,
        Logging:    logging,
    }
    cfg.loadConfig()
    return cfg
}

func (c *Config) loadConfig() {
    c.Mutex.Lock()
    defer c.Mutex.Unlock()

    data, err := ioutil.ReadFile(c.ConfigPath)
    if err != nil {
        c.Logging.Fatalf("Failed to read config file: %s", err)
    }

    tempConfig := struct {
        UserCredentials []UserCredential `json:"user_credentials"`
        DomainMappings  []DomainMapping  `json:"domain_mappings"`
    }{}

    if err := json.Unmarshal(data, &tempConfig); err != nil {
        c.Logging.Fatalf("Failed to parse config: %s", err)
    }

    c.UserCredentials = tempConfig.UserCredentials
    c.DomainMappings = tempConfig.DomainMappings

    c.Logging.Logln("Configuration loaded")
}

func (c *Config) WatchConfig() {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        c.Logging.Fatalf("Failed to create file watcher: %s", err)
    }
    defer watcher.Close()

    err = watcher.Add(".")
    if err != nil {
        c.Logging.Fatalf("Failed to add directory to watcher: %s", err)
    }

    for {
        select {
        case event, ok := <-watcher.Events:
            if !ok {
                return
            }
            if event.Op&(fsnotify.Write|fsnotify.Create) != 0 && event.Name == c.ConfigPath {
                c.Logging.Logln("Config file changed, reloading...")
                c.loadConfig()
            }
        case err, ok := <-watcher.Errors:
            if !ok {
                return
            }
            c.Logging.Logf("Watcher error: %s", err)
        }
    }
}

func (c *Config) GetTargetDomain(domainName string) (string, bool) {
    c.Mutex.RLock()
    defer c.Mutex.RUnlock()
    for _, mapping := range c.DomainMappings {
        if mapping.From == domainName {
            return mapping.To, true
        }
    }
    return "", false
}

func (c *Config) AuthenticateUser(username, password string) bool {
    c.Mutex.RLock()
    defer c.Mutex.RUnlock()
    for _, cred := range c.UserCredentials {
        if cred.Username == username && cred.Password == password {
            return true
        }
    }
    return false
}
