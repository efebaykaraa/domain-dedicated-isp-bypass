package main

import (
    "bufio"
    "fmt"
    "io/ioutil"
    "net/http"
    "os"
    "strings"
    "time"

    "github.com/efebaykaraa/domain-dedicated-isp-bypass/client/config"
    "github.com/efebaykaraa/domain-dedicated-isp-bypass/client/logging"
    "golang.org/x/net/proxy"
)

var (
    cfg    *config.Config
    logger *logging.Logger
)

func init() {
    logger = logging.New(true)

    // Load configuration
    configPath := "config.json"
    cfg = config.LoadConfig(configPath, logger)

    // Watch configuration changes
    go cfg.WatchConfig()
}

func main() {
    // Perform handshake to get session token
    sessionToken, err := performHandshake()
    if err != nil {
        logger.Fatalf("Handshake failed: %s", err)
    }
    logger.Printf("Received Session-Token: %s", sessionToken)

    // Create a SOCKS5 dialer
    socksProxy := "localhost:1080"
    dialer, err := proxy.SOCKS5("tcp", socksProxy, nil, proxy.Direct)
    if err != nil {
        logger.Fatalf("Failed to create SOCKS5 dialer: %s", err)
    }

    // Create HTTP client with transport using SOCKS5 dialer
    httpTransport := &http.Transport{
        Dial: dialer.Dial,
    }
    client := &http.Client{
        Transport: httpTransport,
        Timeout:   30 * time.Second,
    }

    // Begin user interaction loop for entering sub-URLs
    reader := bufio.NewReader(os.Stdin)
    for {
        fmt.Print("Enter sub-URL (or type 'exit' to quit): ")
        subURL, err := reader.ReadString('\n')
        if err != nil {
            logger.Fatalf("Error reading input: %s", err)
        }
        subURL = strings.TrimSpace(subURL)

        if subURL == "exit" {
            fmt.Println("Exiting...")
            break
        }

        // Prepare the request to the proxy server (always to localhost:8080)
        fullURL := "http://localhost:8080"
        logger.Printf("Sending request to proxy for sub-URL: %s", subURL)

        // Prepare the request
        req, err := http.NewRequest("GET", fullURL, nil)
        if err != nil {
            logger.Fatalf("Failed to create request: %s", err)
        }

        // Set headers: Session-Token, Domain-Name, and Sub-URL
        req.Header.Set("Session-Token", sessionToken)
        req.Header.Set("Domain-Name", cfg.DomainName)
        req.Header.Set("Sub-URL", subURL)

        // Perform the request
        resp, err := client.Do(req)
        if err != nil {
            logger.Fatalf("Request failed: %s", err)
        }
        defer resp.Body.Close()

        // Check response status
        if resp.StatusCode != http.StatusOK {
            body, _ := ioutil.ReadAll(resp.Body)
            logger.Fatalf("Server returned non-OK status: %d\n%s", resp.StatusCode, body)
        }

        // Read and print response body
        body, err := ioutil.ReadAll(resp.Body)
        if err != nil {
            logger.Fatalf("Failed to read response body: %s", err)
        }
        fmt.Println("Response:")
        fmt.Println(string(body))
    }
}

func performHandshake() (string, error) {
    // Server handshake URL
    handshakeURL := "http://localhost:8080/handshake"

    // Prepare the request
    req, err := http.NewRequest("GET", handshakeURL, nil)
    if err != nil {
        return "", err
    }

    // Set authentication headers
    req.Header.Set("Username", cfg.Username)
    req.Header.Set("Password", cfg.Password)
    req.Header.Set("Domain-Name", cfg.DomainName)

    // Perform the request
    client := &http.Client{
        Timeout: 10 * time.Second,
    }
    resp, err := client.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    // Check response status
    if resp.StatusCode != http.StatusOK {
        body, _ := ioutil.ReadAll(resp.Body)
        return "", fmt.Errorf("Handshake failed with status %d: %s", resp.StatusCode, body)
    }

    // Get Session-Token from response header
    sessionToken := resp.Header.Get("Session-Token")
    if sessionToken == "" {
        return "", fmt.Errorf("Session-Token not received in handshake response")
    }

    // Channel to signal when the client should stop
    done := make(chan struct{})

    // Start keep-alive ticker
    go startKeepAlive(client, sessionToken, done)

    // Begin user interaction loop for entering sub-URLs
    reader := bufio.NewReader(os.Stdin)
    for {
        fmt.Print("Enter sub-URL (or type 'exit' to quit): ")
        subURL, err := reader.ReadString('\n')
        if err != nil {
            logger.Fatalf("Error reading input: %s", err)
        }
        subURL = strings.TrimSpace(subURL)

        if subURL == "exit" {
            fmt.Println("Exiting...")
            close(done)
            break
        }

        // Prepare the request to the proxy server
        fullURL := "http://localhost:8080"
        logger.Printf("Sending request to proxy for sub-URL: %s", subURL)

        // Prepare the request
        req, err := http.NewRequest("GET", fullURL, nil)
        if err != nil {
            logger.Fatalf("Failed to create request: %s", err)
        }

        // Set headers: Session-Token and Sub-URL
        req.Header.Set("Session-Token", sessionToken)
        req.Header.Set("Sub-URL", subURL)

        // Perform the request with a timeout context
        ctxReq, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        req = req.WithContext(ctxReq)

        resp, err := client.Do(req)
        if err != nil {
            logger.Fatalf("Request failed: %s", err)
        }
        defer resp.Body.Close()

        // Read and print response body
        body, err := ioutil.ReadAll(resp.Body)
        if err != nil {
            logger.Fatalf("Failed to read response body: %s", err)
        }
        fmt.Println("Response:")
        fmt.Println(string(body))
    }
}

func startKeepAlive(client *http.Client, sessionToken string, done <-chan struct{}) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-done:
            return
        case <-ticker.C:
            // Send keep-alive message
            sendKeepAlive(client, sessionToken)
        }
    }
}

func sendKeepAlive(client *http.Client, sessionToken string) {
    // Full URL for the proxy server
    fullURL := "http://localhost:8080"

    // Prepare the keep-alive request
    req, err := http.NewRequest("GET", fullURL, nil)
    if err != nil {
        logger.Printf("Failed to create keep-alive request: %s", err)
        return
    }

    // Set the headers
    req.Header.Set("Session-Token", sessionToken)
    req.Header.Set("Keep-Alive", "true")

    // Perform the request with a timeout context
    ctxReq, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    req = req.WithContext(ctxReq)

    resp, err := client.Do(req)
    if err != nil {
        logger.Printf("Keep-alive request failed: %s", err)
        // Handle connection shutdown if the keep-alive fails
        logger.Println("Server did not respond to keep-alive. Shutting down.")
        os.Exit(1)
    }
    defer resp.Body.Close()

    // Check response status
    if resp.StatusCode != http.StatusOK {
        body, _ := ioutil.ReadAll(resp.Body)
        logger.Printf("Keep-alive failed with status %d: %s", resp.StatusCode, body)
        return
    }

    // Read and print keep-alive response (optional)
    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        logger.Printf("Failed to read keep-alive response: %s", err)
        return
    }
    logger.Printf("Keep-alive response: %s", string(body))
}