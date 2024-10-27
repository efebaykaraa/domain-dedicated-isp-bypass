package proxy

import (
    "bufio"
    "errors"
    "io"

    "github.com/armon/go-socks5"
    "github.com/efebaykaraa/domain-dedicated-isp-bypass/server/config"
    "github.com/efebaykaraa/domain-dedicated-isp-bypass/server/logging"
    "github.com/efebaykaraa/domain-dedicated-isp-bypass/server/session"
)

// Custom errors for authentication
var (
    ErrAuthenticationFailed = errors.New("authentication failed")
)

// StartSOCKS5Proxy starts a SOCKS5 proxy server to handle TCP and UDP traffic.
func StartSOCKS5Proxy(cfg *config.Config, logging *logging.Logging, sessionStore *session.SessionStore) {
    logging.Logln("Starting SOCKS5 proxy on :1080")

    // Custom authentication method
    credChecker := &UserPassAuthenticator{
        Config:       cfg,
        SessionStore: sessionStore,
        Logging:       logging,
    }

    // Create a SOCKS5 server with custom authentication
    conf := &socks5.Config{
        AuthMethods: []socks5.Authenticator{credChecker},
    }
    server, err := socks5.New(conf)
    if err != nil {
        logging.Fatalf("Failed to create SOCKS5 server: %v", err)
    }

    // Start listening on port 1080
    if err := server.ListenAndServe("tcp", ":1080"); err != nil {
        logging.Fatalf("Failed to start SOCKS5 server: %v", err)
    }
}

// UserPassAuthenticator implements the SOCKS5 authentication interface.
type UserPassAuthenticator struct {
    Config       *config.Config
    SessionStore *session.SessionStore
    Logging       *logging.Logging
}

// GetCode returns the SOCKS5 authentication code for User/Password.
func (a *UserPassAuthenticator) GetCode() uint8 {
    // Typically, 0x02 represents Username/Password authentication in SOCKS5.
    return 0x02
}

// Authenticate verifies the username and password sent from the client.
func (a *UserPassAuthenticator) Authenticate(reader io.Reader, writer io.Writer) (*socks5.AuthContext, error) {
    bufReader := bufio.NewReader(reader)

    // Read username and password length and values (customize as needed)
    username, err := bufReader.ReadString('\n')
    if err != nil {
        return nil, ErrAuthenticationFailed
    }
    username = username[:len(username)-1] // Strip newline

    password, err := bufReader.ReadString('\n')
    if err != nil {
        return nil, ErrAuthenticationFailed
    }
    password = password[:len(password)-1] // Strip newline

    a.Logging.Logf("Attempting to authenticate user: %s", username)

    // Validate the username and password against the stored credentials.
    if a.Config.AuthenticateUser(username, password) {
        a.Logging.Logf("User '%s' authenticated successfully", username)
        // Create an AuthContext with a string map for Payload.
        return &socks5.AuthContext{Payload: map[string]string{"username": username}}, nil
    }

    a.Logging.Logf("Authentication failed for user: %s", username)
    return nil, ErrAuthenticationFailed
}