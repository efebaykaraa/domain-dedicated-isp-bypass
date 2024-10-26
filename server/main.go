package main

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "io/ioutil"
    "log"
    "net/url"
    "os"
    "strings"
    "sync"
    "time"

    "crypto/tls"

    "github.com/fsnotify/fsnotify"
    "github.com/valyala/fasthttp"
)

// Configuration structure
type Config struct {
    Username   string            `json:"username"`
    Password   string            `json:"password"`
    DomainMap  map[string]string `json:"domain_map"`
    ConfigPath string            // Path to the config file
    Mutex      sync.RWMutex      // Mutex to handle concurrent access
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
    logger.Println("Starting proxy server on :8080")
    if err := fasthttp.ListenAndServe(":8080", requestHandler); err != nil {
        logger.Fatalf("Error in ListenAndServe: %s", err)
    }
}

// requestHandler handles incoming client requests and proxies them to the target domain
func requestHandler(ctx *fasthttp.RequestCtx) {
    // Acquire read lock for configuration
    config.Mutex.RLock()
    defer config.Mutex.RUnlock()

    logger.Printf("Received request: %s %s", ctx.Method(), ctx.RequestURI())

    // Authentication
    authHash := ctx.Request.Header.Peek("Auth-Hash")
    if !authenticate(string(authHash), config.Username, config.Password) {
        logger.Println("Authentication failed")
        ctx.Error("Unauthorized", fasthttp.StatusUnauthorized)
        return
    }
    logger.Println("Authentication successful")

    // Get the domain name from the request
    domainName := string(ctx.Request.Header.Peek("Domain-Name"))
    targetDomain, exists := config.DomainMap[domainName]
    if !exists {
        logger.Printf("Domain not found: %s", domainName)
        ctx.Error("Domain not found", fasthttp.StatusNotFound)
        return
    }
    logger.Printf("Target domain: %s", targetDomain)

    // Log the visited subaddress
    subaddress := string(ctx.RequestURI())
    logger.Printf("Visiting subaddress: %s", subaddress)

    // Prepare the proxy request
    req := fasthttp.AcquireRequest()
    resp := fasthttp.AcquireResponse()
    defer fasthttp.ReleaseRequest(req)
    defer fasthttp.ReleaseResponse(resp)

    // Copy the original request to the proxy request
    ctx.Request.CopyTo(req)

    // Modify the request URI
    fullURL := joinURL(targetDomain, subaddress)
    logger.Printf("Full target URL: %s", fullURL)
    req.SetRequestURI(fullURL)

    // Set User-Agent header if not present
    if len(req.Header.Peek("User-Agent")) == 0 {
        req.Header.Set("User-Agent", "GoProxy/1.0")
        logger.Println("Set default User-Agent header")
    }

    // Create a client with TLS config
    client := &fasthttp.Client{
        ReadTimeout:  10 * time.Second,
        WriteTimeout: 10 * time.Second,
        TLSConfig: &tls.Config{
            InsecureSkipVerify: true, // Note: Insecure, only for testing
        },
    }

    // Follow redirects manually
    maxRedirects := 10
    redirectCount := 0
    for {
        logger.Printf("Making request to upstream server: %s", req.URI())

        // Perform the request
        if err := client.Do(req, resp); err != nil {
            logger.Printf("Error when proxying the request: %s", err)
            ctx.Error("Error when proxying the request", fasthttp.StatusBadGateway)
            return
        }

        statusCode := resp.StatusCode()
        logger.Printf("Received response with status code: %d", statusCode)

        // Check for redirect status codes
        if statusCode >= 300 && statusCode < 400 {
            if redirectCount >= maxRedirects {
                logger.Println("Too many redirects")
                ctx.Error("Too many redirects", fasthttp.StatusBadGateway)
                return
            }
            redirectCount++

            // Get the Location header
            location := resp.Header.Peek("Location")
            if len(location) == 0 {
                logger.Println("Redirect response missing Location header")
                ctx.Error("Redirect without Location header", fasthttp.StatusBadGateway)
                return
            }

            // Resolve the new URL
            newURL := string(location)
            logger.Printf("Redirecting to: %s", newURL)
            if !strings.HasPrefix(newURL, "http") {
                baseURL, err := url.Parse(req.URI().String())
                if err != nil {
                    logger.Printf("Invalid base URL: %s", err)
                    ctx.Error("Invalid redirect URL", fasthttp.StatusBadGateway)
                    return
                }
                resolvedURL := baseURL.ResolveReference(&url.URL{Path: newURL})
                newURL = resolvedURL.String()
                logger.Printf("Resolved new URL: %s", newURL)
            }

            // Update the request for the next iteration
            req.SetRequestURI(newURL)
            req.SetHost(string(req.URI().Host()))
            continue
        }

        // Not a redirect; break the loop
        break
    }

    // Copy the response headers and body to the client
    resp.Header.CopyTo(&ctx.Response.Header)
    ctx.SetStatusCode(resp.StatusCode())
    ctx.SetBody(resp.Body())

    logger.Printf("Response sent to client with status code: %d", resp.StatusCode())
}

// authenticate verifies the provided Auth-Hash against the expected hash
func authenticate(receivedHash, username, password string) bool {
    // Compute the SHA-256 hash of the username and password
    hasher := sha256.New()
    hasher.Write([]byte(username + password))
    expectedHash := hex.EncodeToString(hasher.Sum(nil))

    return expectedHash == receivedHash
}

// joinURL concatenates the base URL and subaddress into a full URL
func joinURL(baseURL, subaddress string) string {
    return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(subaddress, "/")
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
        Username  string            `json:"username"`
        Password  string            `json:"password"`
        DomainMap map[string]string `json:"domain_map"`
    }{}

    if err := json.Unmarshal(data, &tempConfig); err != nil {
        return err
    }

    c.Username = tempConfig.Username
    c.Password = tempConfig.Password
    c.DomainMap = tempConfig.DomainMap

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
