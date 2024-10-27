package proxy

import (
    "crypto/tls"
    "strings"
    "time"

    "github.com/efebaykaraa/domain-dedicated-isp-bypass/server/config"
    "github.com/efebaykaraa/domain-dedicated-isp-bypass/server/logging"
    "github.com/efebaykaraa/domain-dedicated-isp-bypass/server/session"
    "github.com/valyala/fasthttp"
)

func StartHTTPProxy(cfg *config.Config, logger *logging.Logging, sessionStore *session.SessionStore) {
    logger.Logln("Starting HTTP proxy on :8080")
    if err := fasthttp.ListenAndServe(":8080", func(ctx *fasthttp.RequestCtx) {
        requestHandler(ctx, cfg, logger, sessionStore)
    }); err != nil {
        logger.Fatalf("Error in ListenAndServe: %s", err)
    }
}

func requestHandler(ctx *fasthttp.RequestCtx, cfg *config.Config, logger *logging.Logging, sessionStore *session.SessionStore) {
    path := string(ctx.Path())
    if path == "/handshake" {
        handleHandshake(ctx, cfg, logger, sessionStore)
    } else {
        handleProxyRequest(ctx, cfg, logger, sessionStore)
    }
}

func handleHandshake(ctx *fasthttp.RequestCtx, cfg *config.Config, logger *logging.Logging, sessionStore *session.SessionStore) {
    // Read authentication info from the request
    username := string(ctx.Request.Header.Peek("Username"))
    password := string(ctx.Request.Header.Peek("Password"))
    domainName := string(ctx.Request.Header.Peek("Domain-Name"))

    // Authenticate user
    if !cfg.AuthenticateUser(username, password) {
        logger.Logln("Authentication failed during handshake")
        ctx.Error("Unauthorized", fasthttp.StatusUnauthorized)
        return
    }
    logger.Logf("User '%s' authenticated successfully", username)

    // Get target domain for the domain name
    targetDomain, exists := cfg.GetTargetDomain(domainName)
    if !exists {
        logger.Logf("Domain not found during handshake: %s", domainName)
        ctx.Error("Domain not found", fasthttp.StatusNotFound)
        return
    }
    logger.Logf("Target domain for handshake: %s", targetDomain)

    // Create a new session
    sessionToken := sessionStore.CreateSession(username, targetDomain)
    logger.Logf("Session created with token: %s", sessionToken)

    // Return the session token to the client
    ctx.Response.Header.Set("Session-Token", sessionToken)
    ctx.SetStatusCode(fasthttp.StatusOK)
}

func handleProxyRequest(ctx *fasthttp.RequestCtx, cfg *config.Config, logger *logging.Logger, sessionStore *session.SessionStore) {
    // Get session token from request header
    sessionToken := string(ctx.Request.Header.Peek("Session-Token"))
    if sessionToken == "" {
        logger.Logln("Session token missing in request")
        ctx.Error("Unauthorized", fasthttp.StatusUnauthorized)
        return
    }

    // Retrieve session
    session, exists := sessionStore.GetSession(sessionToken)
    if !exists {
        logger.Logf("Invalid or expired session token: %s", sessionToken)
        ctx.Error("Session not found or expired", fasthttp.StatusUnauthorized)
        return
    }

    // Validate client IP
    clientIP := ctx.RemoteIP().String()
    if clientIP != session.ClientIP {
        logger.Logf("Request from IP '%s' does not match session IP '%s'", clientIP, session.ClientIP)
        ctx.Error("Unauthorized", fasthttp.StatusUnauthorized)
        return
    }

    // Update session last active time
    session.LastActive = time.Now()
    logger.Logf("Session '%s' accessed by user '%s'", sessionToken, session.Username)

    // Handle keep-alive messages
    if string(ctx.Request.Header.Peek("Keep-Alive")) == "true" {
        ctx.SetStatusCode(fasthttp.StatusOK)
        ctx.SetBodyString("Keep-alive acknowledged")
        logger.Logf("Keep-alive message received from session '%s'", sessionToken)
        return
    }

    // Handle data requests
    // Extract the sub-URL from the header
    subURL := string(ctx.Request.Header.Peek("Sub-URL"))
    if subURL == "" {
        logger.Logln("Sub-URL missing in request")
        ctx.Error("Bad Request: Sub-URL missing", fasthttp.StatusBadRequest)
        return
    }

    // Construct the full target URL
    fullURL := joinURL(session.TargetDomain, subURL)
    logger.Printf("Proxying request to: %s", fullURL)

    // Prepare the proxy request
    req := fasthttp.AcquireRequest()
    resp := fasthttp.AcquireResponse()
    defer fasthttp.ReleaseRequest(req)
    defer fasthttp.ReleaseResponse(resp)

    // Copy the method and headers from the client request
    req.Header.SetMethodBytes(ctx.Method())
    ctx.Request.Header.CopyTo(&req.Header)
    req.SetRequestURI(fullURL)

    // Create a client for proxying the request
    client := &fasthttp.Client{
        ReadTimeout:  10 * time.Second,
        WriteTimeout: 10 * time.Second,
        TLSConfig: &tls.Config{
            InsecureSkipVerify: true, // Note: For testing purposes only
        },
    }

    // Perform the request to the target server
    if err := client.Do(req, resp); err != nil {
        logger.Logf("Error when proxying the request: %s", err)
        ctx.Error("Error when proxying the request", fasthttp.StatusBadGateway)
        return
    }

    // Copy the response from the target server to the client
    resp.Header.CopyTo(&ctx.Response.Header)
    ctx.SetStatusCode(resp.StatusCode())
    ctx.SetBody(resp.Body())
    logger.Logf("Response sent to client with status code: %d", resp.StatusCode())
}


func joinURL(baseURL, subaddress string) string {
    return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(subaddress, "/")
}
