package logging

// Logln writes a message to the log file as a single line
func (l *Logging) Logln(message string) {
	l.fileLogger.Println(message)
	l.consoleLogger.Println(message)
}

// Logf writes a message to the log file and with formatting
func (l *Logging) Logf(format string, v ...interface{}) {
	l.fileLogger.Printf(format, v...)
	l.consoleLogger.Printf(format, v...)
}

// Visitedln writes a visited address to the visited file as a single line
func (l *Logging) Visitedln(address string) {
	l.visitedFile.WriteString(address + "\n")
}

// Visitedf writes a visited address to the visited file with formatting
func (l *Logging) Visitedf(format string, v ...interface{}) {
	l.visitedLogger.Printf(format, v...)
}

// Verboseln writes a verbose message to the log file as a single line
func (l *Logging) Verboseln(message string) {
	if l.isVerbose {
		l.fileLogger.Println(message)
		l.consoleLogger.Println(message)
	}
}

// Verbosef writes a verbose message to the log file with formatting
func (l *Logging) Verbosef(format string, v ...interface{}) {
	if l.isVerbose {
		l.fileLogger.Printf(format, v...)
		l.consoleLogger.Printf(format, v...)
	}
}

// Fatalln writes a fatal message to the log file as a single line
func (l *Logging) Fatalln(message string) {
	l.fileLogger.Fatalln(message)
	l.consoleLogger.Fatalln(message)
}

// Fatalf writes a fatal message to the log file with formatting
func (l *Logging) Fatalf(format string, v ...interface{}) {
	l.fileLogger.Fatalf(format, v...)
	l.consoleLogger.Fatalf(format, v...)
}

// Panic writes a panic message to the log file
func (l *Logging) Panicln(message string) {
	l.fileLogger.Panicln(message)
	l.consoleLogger.Panicln(message)
}

// Panicf writes a panic message to the log file with formatting
func (l *Logging) Panicf(format string, v ...interface{}) {
	l.fileLogger.Panicf(format, v...)
	l.consoleLogger.Panicf(format, v...)
}