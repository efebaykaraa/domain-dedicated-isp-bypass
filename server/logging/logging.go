package logging

import (
	"log"
	"os"
)

type Logging struct {
    fileLogger *log.Logger
	consoleLogger *log.Logger
    logFile *os.File
	visitedLogger *log.Logger
    visitedFile *os.File
    isVerbose bool
}

// Initialize
func New(isVerbose bool) *Logging {
	l := &Logging{}
	l.consoleLogger = log.New(os.Stdout, "", log.LstdFlags)
	l.isVerbose = isVerbose
	return l
}

// SetPath sets the path for the log file
func (l *Logging) InitializeLogging(path string) {
	l.openLog(path)
	l.openVisited(path)
}

// openLog opens a log file for writing
func (l *Logging) openLog(path string) {
	var err error
	l.logFile, err = os.OpenFile(path + "/server.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		l.consoleLogger.Fatalf("Failed to open log file: %s", err)
	}
	l.fileLogger = log.New(l.logFile, "", 0)
}

// openVisited opens a file for visited addresses
func (l *Logging) openVisited(path string) {
	var err error
	l.visitedFile, err = os.OpenFile(path + "/visited.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		l.Fatalf("Failed to open visited file: %s", err)
	}
	l.visitedLogger = log.New(l.visitedFile, "", 0)
}

// Close closes the log and visited files
func (l *Logging) Close() {
	l.logFile.Close()
	l.visitedFile.Close()
}