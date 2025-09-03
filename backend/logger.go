package main

import (
	"fmt"
	"log"
	"os"
	"time"
)

var (
	logFile *os.File
	logger  *log.Logger
)

func initLogger() {
	if err := os.MkdirAll("logs", 0755); err != nil {
		log.Fatal("Failed to create logs directory:", err)
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("logs/soulbomber_%s.log", timestamp)

	var err error
	logFile, err = os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal("Failed to open log file:", err)
	}

	logger = log.New(logFile, "", log.LstdFlags)
	log.SetOutput(os.Stdout)
}

func logWebSocketEvent(eventType, playerID string, data interface{}) {
	message := fmt.Sprintf("[WS] %s | Player: %s | Data: %v",
		eventType, playerID, data)
	logger.Println(message)
	log.Println(message)
}

func logError(msg string, err error, fields ...string) {
	message := fmt.Sprintf("[ERROR] %s", msg)
	if err != nil {
		message += fmt.Sprintf(" | Error: %v", err)
	}
	if len(fields) > 0 {
		message += fmt.Sprintf(" | Fields: %v", fields)
	}
	logger.Println(message)
	log.Println(message)
}

func logInfo(msg string, fields ...string) {
	message := fmt.Sprintf("[INFO] %s", msg)
	if len(fields) > 0 {
		message += fmt.Sprintf(" | Fields: %v", fields)
	}
	logger.Println(message)
	log.Println(message)
}
