package main

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "os"
    "strings"
    "sync"
    "time"

    "github.com/fsnotify/fsnotify"
    "github.com/valyala/fasthttp"
)

// Configuration structure
type Config struct {
    Username   string `json:"username"`
    Password   string `json:"password"`
    DomainName string `json:"domain_name"`
    ConfigPath string // Path to the config file
    Mutex      sync.RWMutex
}

var (
    config Config
    logger *log.Logger
)

func init() {
    // Initialize the logger to output to console
    logger = log.New(os.Stdout, "", log.LstdFlags)

    // Load initial configuration
    config.ConfigPath = "config.json"
    if err := config.loadConfig(); err != nil {
        logger.Fatalf("Failed to load config: %s", err)
    }

    // Start watching the config file for changes
    go config.watchConfig()
}

func main() {
    if len(os.Args) < 2 {
        fmt.Println("Usage: go run client.go <path>")
        return
    }

    path := os.Args[1]
    logger.Printf("Requesting path: %s", path)

    // Compute Auth-Hash
    config.Mutex.RLock()
    username := config.Username
    password := config.Password
    domainName := config.DomainName
    config.Mutex.RUnlock()

    authHash := computeAuthHash(username, password)
    logger.Printf("Computed Auth-Hash: %s", authHash)

    // Server address
    serverAddr := "http://localhost:8080"
    fullURL := serverAddr + path
    logger.Printf("Full request URL: %s", fullURL)

    // Prepare the request
    req := fasthttp.AcquireRequest()
    resp := fasthttp.AcquireResponse()
    defer fasthttp.ReleaseRequest(req)
    defer fasthttp.ReleaseResponse(resp)

    req.SetRequestURI(fullURL)
    req.Header.SetMethod("GET")
    req.Header.Set("Auth-Hash", authHash)
    req.Header.Set("Domain-Name", domainName)

    // Set User-Agent header
    req.Header.Set("User-Agent", "GoClient/1.0")

    // Create a client
    client := &fasthttp.Client{
        ReadTimeout:  10 * time.Second,
        WriteTimeout: 10 * time.Second,
    }

    logger.Println("Sending request to proxy server")
    // Perform the request
    if err := client.Do(req, resp); err != nil {
        logger.Fatalf("Error making request: %s", err)
    }

    // Check the response status code
    statusCode := resp.StatusCode()
    logger.Printf("Received response with status code: %d", statusCode)
    if statusCode != fasthttp.StatusOK {
        logger.Fatalf("Server returned non-OK status: %d\n%s", statusCode, resp.Body())
    }

    // Print the response body
    body := resp.Body()
    fmt.Println(string(body))
    logger.Println("Request completed successfully")
}

// computeAuthHash calculates the SHA-256 hash of username and password
func computeAuthHash(username, password string) string {
    hasher := sha256.New()
    hasher.Write([]byte(username + password))
    return hex.EncodeToString(hasher.Sum(nil))
}

// loadConfig reads the configuration from the JSON file
func (c *Config) loadConfig() error {
    c.Mutex.Lock()
    defer c.Mutex.Unlock()

    data, err := ioutil.ReadFile(c.ConfigPath)
    if err != nil {
        return err
    }

    tempConfig := struct {
        Username   string `json:"username"`
        Password   string `json:"password"`
        DomainName string `json:"domain_name"`
    }{}

    if err := json.Unmarshal(data, &tempConfig); err != nil {
        return err
    }

    c.Username = tempConfig.Username
    c.Password = tempConfig.Password
    c.DomainName = tempConfig.DomainName

    logger.Println("Configuration reloaded")
    return nil
}

// watchConfig monitors the configuration file for changes and reloads it
func (c *Config) watchConfig() {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        logger.Fatalf("Failed to create file watcher: %s", err)
    }
    defer watcher.Close()

    configDir := "."
    if idx := strings.LastIndex(c.ConfigPath, "/"); idx != -1 {
        configDir = c.ConfigPath[:idx]
    }

    err = watcher.Add(configDir)
    if err != nil {
        logger.Fatalf("Failed to add directory to watcher: %s", err)
    }

    for {
        select {
        case event, ok := <-watcher.Events:
            if !ok {
                return
            }
            if event.Op&(fsnotify.Write|fsnotify.Create) != 0 && strings.HasSuffix(event.Name, c.ConfigPath) {
                logger.Println("Config file changed, reloading...")
                if err := c.loadConfig(); err != nil {
                    logger.Printf("Failed to reload config: %s", err)
                }
            }
        case err, ok := <-watcher.Errors:
            if !ok {
                return
            }
            logger.Printf("Watcher error: %s", err)
        }
    }
}
