package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Config holds the application configuration
type Config struct {
	WatchDir    string
	Interval    time.Duration
	Provider    string
	ACMEFile    string
	OutputDir   string
	UserUID     int
	UserGID     int
	UseWatcher  bool // Use file watcher instead of polling
}

// ACMEData represents the structure of the ACME JSON file
type ACMEData struct {
	Providers map[string]Provider
}

// Provider represents a certificate provider
type Provider struct {
	Certificates []Certificate `json:"Certificates"`
}

// Certificate represents a single certificate entry
type Certificate struct {
	Domain      Domain `json:"domain"`
	Certificate string `json:"certificate"`
	Key         string `json:"key"`
}

// Domain represents domain information
type Domain struct {
	Main string   `json:"main"`
	SANs []string `json:"SANs"`
}

// Logger provides structured logging
type Logger struct {
	prefix string
}

func NewLogger(prefix string) *Logger {
	return &Logger{prefix: prefix}
}

func (l *Logger) Info(format string, v ...interface{}) {
	log.Printf("[INFO] "+l.prefix+format, v...)
}

func (l *Logger) Error(format string, v ...interface{}) {
	log.Printf("[ERROR] "+l.prefix+format, v...)
}

func (l *Logger) Warn(format string, v ...interface{}) {
	log.Printf("[WARN] "+l.prefix+format, v...)
}

func (l *Logger) Debug(format string, v ...interface{}) {
	if os.Getenv("DEBUG") == "true" {
		log.Printf("[DEBUG] "+l.prefix+format, v...)
	}
}

func main() {
	logger := NewLogger("")
	logger.Info("Starting Traefik ACME Certificate Converter (Go Edition)")

	config, err := loadConfig()
	if err != nil {
		logger.Error("Failed to load configuration: %v", err)
		os.Exit(1)
	}

	logger.Info("Configuration loaded successfully")
	logger.Info("  Watch Directory: %s", config.WatchDir)
	logger.Info("  ACME File: %s", config.ACMEFile)
	logger.Info("  Provider: %s", config.Provider)
	logger.Info("  Output Directory: %s", config.OutputDir)
	logger.Info("  Check Interval: %v", config.Interval)
	logger.Info("  User UID:GID: %d:%d", config.UserUID, config.UserGID)
	logger.Info("  Use File Watcher: %v", config.UseWatcher)

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		logger.Error("Failed to create output directory: %v", err)
		os.Exit(1)
	}

	converter := &CertificateConverter{
		config: config,
		logger: NewLogger("[Converter] "),
	}

	if config.UseWatcher {
		logger.Info("Starting file watcher mode...")
		if err := converter.WatchAndExtract(); err != nil {
			logger.Error("File watcher failed: %v", err)
			os.Exit(1)
		}
	} else {
		logger.Info("Starting polling mode...")
		converter.PollAndExtract()
	}
}

// loadConfig loads configuration from environment variables
func loadConfig() (*Config, error) {
	watchDir := getEnv("WATCH_DIR", "/traefik/certs")
	provider := getEnv("PROVIDER", "ACME")
	acmeFile := getEnv("ACME_FILE_NAME", provider+".json")
	outputDir := getEnv("OUTPUT_DIR", "/app/output")
	userUID := getEnvInt("USER_UID", 1000)
	userGID := getEnvInt("USER_GID", 1000)
	useWatcher := getEnv("USE_WATCHER", "true") == "true"

	// Interval: 0 = disabled (pure fsnotify in watcher mode), default 1800 for polling mode
	defaultInterval := 0
	if !useWatcher {
		defaultInterval = 1800 // Polling mode requires an interval
	}
	interval := getEnvInt("INTERVAL", defaultInterval)

	return &Config{
		WatchDir:   watchDir,
		Interval:   time.Duration(interval) * time.Second,
		Provider:   provider,
		ACMEFile:   acmeFile,
		OutputDir:  outputDir,
		UserUID:    userUID,
		UserGID:    userGID,
		UseWatcher: useWatcher,
	}, nil
}

// CertificateConverter handles the extraction of certificates
type CertificateConverter struct {
	config *Config
	logger *Logger
}

// PollAndExtract polls the ACME file at regular intervals
func (c *CertificateConverter) PollAndExtract() {
	ticker := time.NewTicker(c.config.Interval)
	defer ticker.Stop()

	// Process immediately on start
	c.processACMEFile()

	for range ticker.C {
		c.processACMEFile()
	}
}

// WatchAndExtract uses fsnotify to watch for file changes
func (c *CertificateConverter) WatchAndExtract() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Close()

	// Process immediately on start
	c.processACMEFile()

	// Add watch directory
	if err := watcher.Add(c.config.WatchDir); err != nil {
		return fmt.Errorf("failed to watch directory %s: %w", c.config.WatchDir, err)
	}

	c.logger.Info("Watching directory: %s", c.config.WatchDir)

	// Optional: periodic fallback check for resiliency (catches missed events)
	// Only enabled if INTERVAL is explicitly set
	var ticker *time.Ticker
	var tickerChan <-chan time.Time
	if c.config.Interval > 0 {
		ticker = time.NewTicker(c.config.Interval)
		tickerChan = ticker.C
		defer ticker.Stop()
		c.logger.Info("Periodic fallback check enabled every %v (for resiliency)", c.config.Interval)
	} else {
		c.logger.Info("Relying purely on fsnotify events (no periodic fallback)")
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return fmt.Errorf("watcher channel closed")
			}

			c.logger.Debug("File event: %s %s", event.Op, event.Name)

			// Check if the event is for our ACME file
			if filepath.Base(event.Name) == c.config.ACMEFile {
				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
					c.logger.Info("ACME file modified, processing...")
					// Small delay to ensure file is fully written
					time.Sleep(500 * time.Millisecond)
					c.processACMEFile()
				}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return fmt.Errorf("watcher error channel closed")
			}
			c.logger.Error("Watcher error: %v", err)

		case <-tickerChan:
			if tickerChan != nil {
				c.logger.Debug("Periodic fallback check triggered")
				c.processACMEFile()
			}
		}
	}
}

// processACMEFile extracts and saves certificates from the ACME file
func (c *CertificateConverter) processACMEFile() {
	acmeFilePath := filepath.Join(c.config.WatchDir, c.config.ACMEFile)

	c.logger.Info("Checking for ACME file: %s", acmeFilePath)

	if _, err := os.Stat(acmeFilePath); os.IsNotExist(err) {
		c.logger.Warn("ACME file not found: %s", acmeFilePath)
		return
	}

	// Read the ACME file
	data, err := os.ReadFile(acmeFilePath)
	if err != nil {
		c.logger.Error("Failed to read ACME file: %v", err)
		return
	}

	if len(data) == 0 {
		c.logger.Warn("ACME file is empty")
		return
	}

	// Parse JSON
	var acmeData map[string]any
	if err := json.Unmarshal(data, &acmeData); err != nil {
		c.logger.Error("Failed to parse ACME JSON: %v", err)
		return
	}

	// Navigate to provider's certificates
	providerData, ok := acmeData[c.config.Provider]
	if !ok {
		c.logger.Error("Provider '%s' not found in ACME file", c.config.Provider)
		return
	}

	providerMap, ok := providerData.(map[string]any)
	if !ok {
		c.logger.Error("Invalid provider data structure")
		return
	}

	certificatesData, ok := providerMap["Certificates"]
	if !ok {
		c.logger.Warn("No certificates found for provider '%s'", c.config.Provider)
		return
	}

	certificates, ok := certificatesData.([]any)
	if !ok {
		c.logger.Error("Invalid certificates data structure")
		return
	}

	c.logger.Info("Found %d certificate(s)", len(certificates))

	successCount := 0
	errorCount := 0

	for i, cert := range certificates {
		if err := c.extractCertificate(cert, i); err != nil {
			c.logger.Error("Failed to extract certificate %d: %v", i, err)
			errorCount++
		} else {
			successCount++
		}
	}

	c.logger.Info("Extraction complete: %d successful, %d failed", successCount, errorCount)
}

// extractCertificate extracts and saves a single certificate
func (c *CertificateConverter) extractCertificate(certData any, index int) error {
	certMap, ok := certData.(map[string]any)
	if !ok {
		return fmt.Errorf("invalid certificate structure at index %d", index)
	}

	// Extract domain
	domainData, ok := certMap["domain"]
	if !ok {
		return fmt.Errorf("missing domain field")
	}

	domainMap, ok := domainData.(map[string]any)
	if !ok {
		return fmt.Errorf("invalid domain structure")
	}

	mainDomain, ok := domainMap["main"].(string)
	if !ok || mainDomain == "" {
		return fmt.Errorf("missing or invalid main domain")
	}

	// Strip wildcard prefix if present
	domain := strings.TrimPrefix(mainDomain, "*.")

	c.logger.Info("Processing certificate for domain: %s", domain)

	// Extract certificate and key
	certB64, ok := certMap["certificate"].(string)
	if !ok || certB64 == "" {
		return fmt.Errorf("missing or invalid certificate for domain %s", domain)
	}

	keyB64, ok := certMap["key"].(string)
	if !ok || keyB64 == "" {
		return fmt.Errorf("missing or invalid key for domain %s", domain)
	}

	// Decode base64
	certPEM, err := base64.StdEncoding.DecodeString(certB64)
	if err != nil {
		return fmt.Errorf("failed to decode certificate: %w", err)
	}

	keyPEM, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return fmt.Errorf("failed to decode key: %w", err)
	}

	// Create domain directory
	domainDir := filepath.Join(c.config.OutputDir, domain)
	if err := os.MkdirAll(domainDir, 0755); err != nil {
		return fmt.Errorf("failed to create domain directory: %w", err)
	}

	// Write certificate file
	certPath := filepath.Join(domainDir, "fullchain.pem")
	if err := os.WriteFile(certPath, certPEM, 0600); err != nil {
		return fmt.Errorf("failed to write certificate: %w", err)
	}

	// Write key file
	keyPath := filepath.Join(domainDir, "privkey.pem")
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		return fmt.Errorf("failed to write key: %w", err)
	}

	// Set ownership
	if err := c.setOwnership(certPath); err != nil {
		c.logger.Warn("Failed to set ownership for %s: %v", certPath, err)
	}
	if err := c.setOwnership(keyPath); err != nil {
		c.logger.Warn("Failed to set ownership for %s: %v", keyPath, err)
	}

	c.logger.Info("Successfully extracted certificates for domain: %s", domain)
	return nil
}

// setOwnership sets the file ownership
// Uses sudo if running as non-root user and needs elevated privileges
func (c *CertificateConverter) setOwnership(path string) error {
	err := os.Chown(path, c.config.UserUID, c.config.UserGID)
	if err != nil && os.Geteuid() != 0 {
		// Try with sudo if we're not running as root
		cmd := exec.Command("sudo", "chown",
			fmt.Sprintf("%d:%d", c.config.UserUID, c.config.UserGID),
			path)
		if sudoErr := cmd.Run(); sudoErr != nil {
			return fmt.Errorf("chown failed: %w (sudo also failed: %v)", err, sudoErr)
		}
		return nil
	}
	return err
}

// Helper functions for environment variables
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// Ensure we handle signals gracefully
func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
}
