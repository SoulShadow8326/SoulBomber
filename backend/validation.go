package main

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	uuidRegex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	nameRegex = regexp.MustCompile(`^[a-zA-Z0-9\s\-_]{1,20}$`)
	directions = []string{"up", "down", "left", "right"}
	difficulties = []string{AI_EASY, AI_MEDIUM, AI_HARD, AI_CHOSEN_ONE}
)

func ValidateLobbyName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("lobby name cannot be empty")
	}

	if len(name) > 50 {
		return fmt.Errorf("lobby name too long (max 50 characters)")
	}

	if !nameRegex.MatchString(name) {
		return fmt.Errorf("lobby name contains invalid characters")
	}

	return nil
}

func ValidatePlayerName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("player name cannot be empty")
	}

	if len(name) > 20 {
		return fmt.Errorf("player name too long (max 20 characters)")
	}

	if !nameRegex.MatchString(name) {
		return fmt.Errorf("player name contains invalid characters")
	}

	return nil
}

func ValidateDirection(direction string) error {
	for _, validDir := range directions {
		if direction == validDir {
			return nil
		}
	}
	return fmt.Errorf("invalid direction: %s", direction)
}

func ValidateDifficulty(difficulty string) error {
	for _, validDiff := range difficulties {
		if difficulty == validDiff {
			return nil
		}
	}
	return fmt.Errorf("invalid difficulty: %s", difficulty)
}

func ValidateUUID(id string) error {
	if !uuidRegex.MatchString(id) {
		return fmt.Errorf("invalid UUID format")
	}
	return nil
}

func SanitizeString(input string) string {
	var result strings.Builder
	for _, r := range input {
		if r < 32 && r != 9 && r != 10 && r != 13 {
			continue
		}
		if r > 127 {
			continue
		}
		result.WriteRune(r)
	}
	return strings.TrimSpace(result.String())
}
