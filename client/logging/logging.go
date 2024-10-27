package logging

import (
    "log"
    "os"
)

type Logger struct {
    *log.Logger
}

func New(debug bool) *Logger {
    return &Logger{
        Logger: log.New(os.Stdout, "", log.LstdFlags),
    }
}

func (l *Logger) Logf(format string, v ...interface{}) {
    l.Printf(format, v...)
}

func (l *Logger) Logln(v ...interface{}) {
    l.Println(v...)
}
