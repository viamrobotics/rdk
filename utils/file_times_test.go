package utils

import (
	"os"
	"testing"
	"time"
)

func TestGetFileTimes(t *testing.T) {
	// Create a temporary test file
	tmpFile, err := os.CreateTemp("", "test_file_times_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Write some content
	_, err = tmpFile.WriteString("test content")
	if err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	err = tmpFile.Sync()
	if err != nil {
		t.Fatalf("Failed to sync temp file: %v", err)
	}

	// Get file times
	fileTimes, err := GetFileTimes(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to get file times: %v", err)
	}

	// Verify times are reasonable (within the last minute)
	now := time.Now()
	oneMinuteAgo := now.Add(-1 * time.Minute)

	if fileTimes.CreateTime.Before(oneMinuteAgo) || fileTimes.CreateTime.After(now) {
		t.Errorf("CreateTime %v is not within the last minute", fileTimes.CreateTime)
	}

	if fileTimes.ModifyTime.Before(oneMinuteAgo) || fileTimes.ModifyTime.After(now) {
		t.Errorf("ModifyTime %v is not within the last minute", fileTimes.ModifyTime)
	}

	// Verify ModifyTime is not zero
	if fileTimes.ModifyTime.IsZero() {
		t.Error("ModifyTime should not be zero")
	}

	// Verify CreateTime is not zero
	if fileTimes.CreateTime.IsZero() {
		t.Error("CreateTime should not be zero")
	}
}

func TestGetFileTimesNonexistent(t *testing.T) {
	_, err := GetFileTimes("/nonexistent/file/path")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}
