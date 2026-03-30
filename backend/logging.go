package main

import (
	"io"
	"log"
	"os"
)

const StandardLogTimestampLayout = "2006/01/02 15:04:05"

func setupLogging(logFilePath string) (io.Writer, func() error, error) {
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, err
	}

	output := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(output)
	log.SetFlags(log.Ldate | log.Ltime)
	log.SetPrefix("")

	cleanup := func() error {
		return logFile.Close()
	}

	return output, cleanup, nil
}
