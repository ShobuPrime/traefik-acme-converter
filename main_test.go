package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Test configuration loading from environment variables
func TestLoadConfig(t *testing.T) {
	// Set up test environment variables
	os.Setenv("WATCH_DIR", "/test/watch")
	os.Setenv("INTERVAL", "600")
	os.Setenv("PROVIDER", "TestProvider")
	os.Setenv("OUTPUT_DIR", "/test/output")
	os.Setenv("USER_UID", "2000")
	os.Setenv("USER_GID", "2000")
	os.Setenv("USE_WATCHER", "false")

	defer func() {
		// Clean up
		os.Unsetenv("WATCH_DIR")
		os.Unsetenv("INTERVAL")
		os.Unsetenv("PROVIDER")
		os.Unsetenv("OUTPUT_DIR")
		os.Unsetenv("USER_UID")
		os.Unsetenv("USER_GID")
		os.Unsetenv("USE_WATCHER")
	}()

	config, err := loadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.WatchDir != "/test/watch" {
		t.Errorf("Expected WatchDir '/test/watch', got '%s'", config.WatchDir)
	}

	if config.Interval != 600*time.Second {
		t.Errorf("Expected Interval 600s, got %v", config.Interval)
	}

	if config.Provider != "TestProvider" {
		t.Errorf("Expected Provider 'TestProvider', got '%s'", config.Provider)
	}

	if config.OutputDir != "/test/output" {
		t.Errorf("Expected OutputDir '/test/output', got '%s'", config.OutputDir)
	}

	if config.UserUID != 2000 {
		t.Errorf("Expected UserUID 2000, got %d", config.UserUID)
	}

	if config.UserGID != 2000 {
		t.Errorf("Expected UserGID 2000, got %d", config.UserGID)
	}

	if config.UseWatcher != false {
		t.Errorf("Expected UseWatcher false, got %v", config.UseWatcher)
	}
}

// Test default configuration values
func TestLoadConfigDefaults(t *testing.T) {
	// Clear any existing environment variables
	os.Clearenv()

	config, err := loadConfig()
	if err != nil {
		t.Fatalf("Failed to load config with defaults: %v", err)
	}

	if config.WatchDir != "/traefik/certs" {
		t.Errorf("Expected default WatchDir '/traefik/certs', got '%s'", config.WatchDir)
	}

	// Default interval is 0 for watcher mode (pure fsnotify, no periodic checks)
	if config.Interval != 0 {
		t.Errorf("Expected default Interval 0s (watcher mode), got %v", config.Interval)
	}

	if config.Provider != "ACME" {
		t.Errorf("Expected default Provider 'ACME', got '%s'", config.Provider)
	}

	if config.ACMEFile != "ACME.json" {
		t.Errorf("Expected default ACMEFile 'ACME.json', got '%s'", config.ACMEFile)
	}

	if config.UserUID != 1000 {
		t.Errorf("Expected default UserUID 1000, got %d", config.UserUID)
	}

	if config.UseWatcher != true {
		t.Errorf("Expected default UseWatcher true, got %v", config.UseWatcher)
	}
}

// Test polling mode default interval
func TestLoadConfigPollingMode(t *testing.T) {
	// Clear and set polling mode
	os.Clearenv()
	os.Setenv("USE_WATCHER", "false")
	defer os.Unsetenv("USE_WATCHER")

	config, err := loadConfig()
	if err != nil {
		t.Fatalf("Failed to load config for polling mode: %v", err)
	}

	if config.UseWatcher != false {
		t.Errorf("Expected UseWatcher false, got %v", config.UseWatcher)
	}

	// Default interval for polling mode should be 1800s
	if config.Interval != 1800*time.Second {
		t.Errorf("Expected default Interval 1800s for polling mode, got %v", config.Interval)
	}
}

// Test certificate extraction with valid data
func TestExtractCertificate(t *testing.T) {
	// Create temporary directory for output
	tempDir, err := os.MkdirTemp("", "acme-converter-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := &Config{
		OutputDir: tempDir,
		UserUID:   os.Getuid(),
		UserGID:   os.Getgid(),
	}

	converter := &CertificateConverter{
		config: config,
		logger: NewLogger("[Test] "),
	}

	// Create test certificate data
	testCert := "test-certificate-data"
	testKey := "test-key-data"

	certData := map[string]any{
		"domain": map[string]any{
			"main": "example.com",
		},
		"certificate": base64.StdEncoding.EncodeToString([]byte(testCert)),
		"key":         base64.StdEncoding.EncodeToString([]byte(testKey)),
	}

	// Extract certificate
	err = converter.extractCertificate(certData, 0)
	if err != nil {
		t.Fatalf("Failed to extract certificate: %v", err)
	}

	// Verify certificate file exists and has correct content
	certPath := filepath.Join(tempDir, "example.com", "fullchain.pem")
	certContent, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("Failed to read certificate file: %v", err)
	}

	if string(certContent) != testCert {
		t.Errorf("Certificate content mismatch. Expected '%s', got '%s'", testCert, string(certContent))
	}

	// Verify key file exists and has correct content
	keyPath := filepath.Join(tempDir, "example.com", "privkey.pem")
	keyContent, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("Failed to read key file: %v", err)
	}

	if string(keyContent) != testKey {
		t.Errorf("Key content mismatch. Expected '%s', got '%s'", testKey, string(keyContent))
	}

	// Verify file permissions
	certInfo, err := os.Stat(certPath)
	if err != nil {
		t.Fatalf("Failed to stat certificate file: %v", err)
	}

	if certInfo.Mode().Perm() != 0600 {
		t.Errorf("Expected certificate file permissions 0600, got %v", certInfo.Mode().Perm())
	}
}

// Test wildcard domain handling
func TestExtractCertificateWildcard(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "acme-converter-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := &Config{
		OutputDir: tempDir,
		UserUID:   os.Getuid(),
		UserGID:   os.Getgid(),
	}

	converter := &CertificateConverter{
		config: config,
		logger: NewLogger("[Test] "),
	}

	testCert := "test-certificate-data"
	testKey := "test-key-data"

	certData := map[string]any{
		"domain": map[string]any{
			"main": "*.example.com",
		},
		"certificate": base64.StdEncoding.EncodeToString([]byte(testCert)),
		"key":         base64.StdEncoding.EncodeToString([]byte(testKey)),
	}

	err = converter.extractCertificate(certData, 0)
	if err != nil {
		t.Fatalf("Failed to extract wildcard certificate: %v", err)
	}

	// Verify that wildcard prefix was stripped
	certPath := filepath.Join(tempDir, "example.com", "fullchain.pem")
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		t.Errorf("Certificate file not found at expected path (wildcard should be stripped): %s", certPath)
	}
}

// Test invalid certificate data handling
func TestExtractCertificateInvalidData(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "acme-converter-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := &Config{
		OutputDir: tempDir,
		UserUID:   os.Getuid(),
		UserGID:   os.Getgid(),
	}

	converter := &CertificateConverter{
		config: config,
		logger: NewLogger("[Test] "),
	}

	tests := []struct {
		name     string
		certData any
	}{
		{
			name:     "Invalid structure",
			certData: "not a map",
		},
		{
			name: "Missing domain",
			certData: map[string]any{
				"certificate": base64.StdEncoding.EncodeToString([]byte("test")),
				"key":         base64.StdEncoding.EncodeToString([]byte("test")),
			},
		},
		{
			name: "Missing certificate",
			certData: map[string]any{
				"domain": map[string]any{
					"main": "example.com",
				},
				"key": base64.StdEncoding.EncodeToString([]byte("test")),
			},
		},
		{
			name: "Missing key",
			certData: map[string]any{
				"domain": map[string]any{
					"main": "example.com",
				},
				"certificate": base64.StdEncoding.EncodeToString([]byte("test")),
			},
		},
		{
			name: "Invalid base64 certificate",
			certData: map[string]any{
				"domain": map[string]any{
					"main": "example.com",
				},
				"certificate": "not-valid-base64!!!",
				"key":         base64.StdEncoding.EncodeToString([]byte("test")),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := converter.extractCertificate(tt.certData, 0)
			if err == nil {
				t.Errorf("Expected error for %s, got nil", tt.name)
			}
		})
	}
}

// Test processing of complete ACME file
func TestProcessACMEFile(t *testing.T) {
	// Create temporary directories
	watchDir, err := os.MkdirTemp("", "acme-watch-*")
	if err != nil {
		t.Fatalf("Failed to create watch dir: %v", err)
	}
	defer os.RemoveAll(watchDir)

	outputDir, err := os.MkdirTemp("", "acme-output-*")
	if err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	// Create test ACME file
	acmeData := map[string]any{
		"ACME": map[string]any{
			"Certificates": []any{
				map[string]any{
					"domain": map[string]any{
						"main": "example.com",
					},
					"certificate": base64.StdEncoding.EncodeToString([]byte("cert1")),
					"key":         base64.StdEncoding.EncodeToString([]byte("key1")),
				},
				map[string]any{
					"domain": map[string]any{
						"main": "test.com",
					},
					"certificate": base64.StdEncoding.EncodeToString([]byte("cert2")),
					"key":         base64.StdEncoding.EncodeToString([]byte("key2")),
				},
			},
		},
	}

	acmeJSON, err := json.Marshal(acmeData)
	if err != nil {
		t.Fatalf("Failed to marshal ACME data: %v", err)
	}

	acmeFilePath := filepath.Join(watchDir, "ACME.json")
	err = os.WriteFile(acmeFilePath, acmeJSON, 0644)
	if err != nil {
		t.Fatalf("Failed to write ACME file: %v", err)
	}

	// Create converter
	config := &Config{
		WatchDir:  watchDir,
		ACMEFile:  "ACME.json",
		Provider:  "ACME",
		OutputDir: outputDir,
		UserUID:   os.Getuid(),
		UserGID:   os.Getgid(),
	}

	converter := &CertificateConverter{
		config: config,
		logger: NewLogger("[Test] "),
	}

	// Process the file
	converter.processACMEFile()

	// Verify both certificates were extracted
	domains := []string{"example.com", "test.com"}
	for _, domain := range domains {
		certPath := filepath.Join(outputDir, domain, "fullchain.pem")
		if _, err := os.Stat(certPath); os.IsNotExist(err) {
			t.Errorf("Certificate file not found for domain %s", domain)
		}

		keyPath := filepath.Join(outputDir, domain, "privkey.pem")
		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			t.Errorf("Key file not found for domain %s", domain)
		}
	}
}

// Test processing when ACME file doesn't exist
func TestProcessACMEFileNotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "acme-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := &Config{
		WatchDir:  tempDir,
		ACMEFile:  "nonexistent.json",
		Provider:  "ACME",
		OutputDir: tempDir,
		UserUID:   os.Getuid(),
		UserGID:   os.Getgid(),
	}

	converter := &CertificateConverter{
		config: config,
		logger: NewLogger("[Test] "),
	}

	// This should not panic or return an error, just log a warning
	converter.processACMEFile()
}

// Test real-world wildcard certificate with mock data
func TestExtractCertificateRealWildcard(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "acme-converter-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := &Config{
		OutputDir: tempDir,
		UserUID:   os.Getuid(),
		UserGID:   os.Getgid(),
	}

	converter := &CertificateConverter{
		config: config,
		logger: NewLogger("[Test] "),
	}

	// Mock certificate data for *.shobu.prime (a play on shobuprime - "show be prime")
	// Using realistic base64-encoded PEM certificates for testing
	mockCertPEM := `-----BEGIN CERTIFICATE-----
MIIDsTCCApmgAwIBAgIUTestCertificateForShobuPrimeOnly123wDQYJKoZIhvcNAQEL
BQAwaDELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAkNBMRYwFAYDVQQHDA1TYW4gRnJh
bmNpc2NvMRMwEQYDVQQKDApTaG9idVByaW1lMR8wHQYDVQQDDBYqLnNob2J1LnBy
aW1lLTNkLmxvY2FsMB4XDTI1MTAxMzAwMDAwMFoXDTI2MTAxMzAwMDAwMFowaDEL
MAkGA1UEBhMCVVMxCzAJBgNVBAgMAkNBMRYwFAYDVQQHDA1TYW4gRnJhbmNpc2Nv
MRMwEQYDVQQKDApTaG9idVByaW1lMR8wHQYDVQQDDBYqLnNob2J1LnByaW1lLTNk
LmxvY2FsMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAy123Test456789
MockCertificateDataForTestingPurposesOnlyNotRealProductionCertData
-----END CERTIFICATE-----`

	mockKeyPEM := `-----BEGIN EC PRIVATE KEY-----
MIGkAgEBBDCMockPrivateKey123456789TestDataOnlyNotRealForShobuPrime3D
TestingMockData1234567890NotForProductionUseThisIsATestKeyOnly
-----END EC PRIVATE KEY-----`

	certData := map[string]any{
		"domain": map[string]any{
			"main": "*.shobu.prime-3d.local", // Pun: "show be prime" + 3D
		},
		"certificate": base64.StdEncoding.EncodeToString([]byte(mockCertPEM)),
		"key":         base64.StdEncoding.EncodeToString([]byte(mockKeyPEM)),
		"Store":       "default",
	}

	err = converter.extractCertificate(certData, 0)
	if err != nil {
		t.Fatalf("Failed to extract wildcard certificate: %v", err)
	}

	// Verify that wildcard prefix was stripped
	certPath := filepath.Join(tempDir, "shobu.prime-3d.local", "fullchain.pem")
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		t.Errorf("Certificate file not found at expected path (wildcard should be stripped): %s", certPath)
	}

	// Verify certificate content
	certContent, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("Failed to read certificate: %v", err)
	}

	if !strings.Contains(string(certContent), "BEGIN CERTIFICATE") {
		t.Errorf("Certificate does not contain expected PEM header")
	}

	if !strings.Contains(string(certContent), "ShobuPrime") {
		t.Errorf("Certificate does not contain expected test data")
	}

	// Verify key content
	keyPath := filepath.Join(tempDir, "shobu.prime-3d.local", "privkey.pem")
	keyContent, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("Failed to read key: %v", err)
	}

	if !strings.Contains(string(keyContent), "BEGIN EC PRIVATE KEY") {
		t.Errorf("Private key does not contain expected PEM header")
	}

	if !strings.Contains(string(keyContent), "MockPrivateKey") {
		t.Errorf("Private key does not contain expected test data")
	}
}

// Test certificate with SANs
func TestExtractCertificateWithSANs(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "acme-converter-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := &Config{
		OutputDir: tempDir,
		UserUID:   os.Getuid(),
		UserGID:   os.Getgid(),
	}

	converter := &CertificateConverter{
		config: config,
		logger: NewLogger("[Test] "),
	}

	testCert := "test-certificate-with-sans"
	testKey := "test-key-with-sans"

	// Certificate with main domain and SANs
	certData := map[string]any{
		"domain": map[string]any{
			"main": "example.com",
			"SANs": []any{"www.example.com", "api.example.com", "*.example.com"},
		},
		"certificate": base64.StdEncoding.EncodeToString([]byte(testCert)),
		"key":         base64.StdEncoding.EncodeToString([]byte(testKey)),
	}

	err = converter.extractCertificate(certData, 0)
	if err != nil {
		t.Fatalf("Failed to extract certificate with SANs: %v", err)
	}

	// The certificate should be stored under the main domain
	certPath := filepath.Join(tempDir, "example.com", "fullchain.pem")
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		t.Errorf("Certificate file not found at expected path: %s", certPath)
	}

	certContent, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("Failed to read certificate: %v", err)
	}

	if string(certContent) != testCert {
		t.Errorf("Certificate content mismatch")
	}
}

// Test processing with empty ACME file
func TestProcessACMEFileEmpty(t *testing.T) {
	watchDir, err := os.MkdirTemp("", "acme-watch-*")
	if err != nil {
		t.Fatalf("Failed to create watch dir: %v", err)
	}
	defer os.RemoveAll(watchDir)

	acmeFilePath := filepath.Join(watchDir, "ACME.json")
	err = os.WriteFile(acmeFilePath, []byte{}, 0644)
	if err != nil {
		t.Fatalf("Failed to write empty ACME file: %v", err)
	}

	config := &Config{
		WatchDir:  watchDir,
		ACMEFile:  "ACME.json",
		Provider:  "ACME",
		OutputDir: watchDir,
		UserUID:   os.Getuid(),
		UserGID:   os.Getgid(),
	}

	converter := &CertificateConverter{
		config: config,
		logger: NewLogger("[Test] "),
	}

	// Should handle empty file gracefully
	converter.processACMEFile()
}

// Test logger functionality
func TestLogger(t *testing.T) {
	logger := NewLogger("[Test] ")

	// These should not panic
	logger.Info("Test info message: %s", "info")
	logger.Error("Test error message: %s", "error")
	logger.Warn("Test warning message: %s", "warning")
	logger.Debug("Test debug message: %s", "debug")
}

// Benchmark certificate extraction
func BenchmarkExtractCertificate(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "acme-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := &Config{
		OutputDir: tempDir,
		UserUID:   os.Getuid(),
		UserGID:   os.Getgid(),
	}

	converter := &CertificateConverter{
		config: config,
		logger: NewLogger("[Bench] "),
	}

	certData := map[string]any{
		"domain": map[string]any{
			"main": "example.com",
		},
		"certificate": base64.StdEncoding.EncodeToString([]byte("test-certificate-data")),
		"key":         base64.StdEncoding.EncodeToString([]byte("test-key-data")),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		converter.extractCertificate(certData, 0)
	}
}
