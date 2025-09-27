package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/net/http2"
	"golang.org/x/time/rate"
)

const (
	Version        = "5.0.0"
	DefaultChunks  = 32
	ChunkSize      = 4 * 1024 * 1024 // 4MB
	BufferSize     = 32 * 1024       // 32KB
	MaxRetries     = 5
	RetryDelay     = 2 * time.Second
	ProgressUpdate = 100 * time.Millisecond
)

var (
	startTime = time.Now()
	globalConfig *Config
	jobQueue *JobQueue
	daemon *DaemonServer
)

// Color codes for terminal output
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorWhite  = "\033[37m"
	ColorBold   = "\033[1m"
)

// Config holds all configuration settings
type Config struct {
	MaxConnections   int               `json:"max_connections"`
	ChunkSize        int64             `json:"chunk_size"`
	MaxRetries       int               `json:"max_retries"`
	RetryDelay       int               `json:"retry_delay_seconds"`
	DownloadDir      string            `json:"download_dir"`
	RateLimit        int64             `json:"rate_limit_bytes"`
	ProxyURL         string            `json:"proxy_url"`
	UserAgent        string            `json:"user_agent"`
	Timeout          int               `json:"timeout_seconds"`
	ResumeEnabled    bool              `json:"resume_enabled"`
	VerifyChecksum   bool              `json:"verify_checksum"`
	UseMirrors       bool              `json:"use_mirrors"`
	Mirrors          []string          `json:"mirrors"`
	CookieFile       string            `json:"cookie_file"`
	Headers          map[string]string `json:"headers"`
	EnableDaemon     bool              `json:"enable_daemon"`
	DaemonPort       int               `json:"daemon_port"`
	DatabasePath     string            `json:"database_path"`
	EnableHTTP2      bool              `json:"enable_http2"`
	EnableTUI        bool              `json:"enable_tui"`
	MaxParallel      int               `json:"max_parallel_downloads"`
	TorrentPort      int               `json:"torrent_port"`
	EnableTorrent    bool              `json:"enable_torrent"`
	EnableFTP        bool              `json:"enable_ftp"`
	LogFile          string            `json:"log_file"`
	ConfigPath       string            `json:"config_path"`
}

// DownloadManager handles all download operations
type DownloadManager struct {
	client       *http.Client
	maxWorkers   int
	downloadDir  string
	verifyHashes bool
	resume       bool
	rateLimiter  *RateLimiter
	proxyManager *ProxyManager
	config       *Config
}

// Job represents a download job
type Job struct {
	ID          string            `json:"id"`
	URL         string            `json:"url"`
	Protocol    string            `json:"protocol"` // http, https, ftp, torrent, magnet
	Mirrors     []string          `json:"mirrors"`
	FilePath    string            `json:"file_path"`
	TotalSize   int64             `json:"total_size"`
	Downloaded  int64             `json:"downloaded"`
	Status      string            `json:"status"`
	Priority    int               `json:"priority"`
	SHA256      string            `json:"sha256"`
	SHA1        string            `json:"sha1"`
	MD5         string            `json:"md5"`
	AddedTime   time.Time         `json:"added_time"`
	StartTime   *time.Time        `json:"start_time"`
	EndTime     *time.Time        `json:"end_time"`
	Speed       float64           `json:"speed"`
	ETA         int               `json:"eta"`
	Error       string            `json:"error"`
	Metadata    map[string]string `json:"metadata"`
	ChunkStates []ChunkState      `json:"chunk_states"`
	Chunks      int               `json:"chunks"`
}

// ChunkState tracks individual chunk progress
type ChunkState struct {
	Index      int   `json:"index"`
	Start      int64 `json:"start"`
	End        int64 `json:"end"`
	Downloaded int64 `json:"downloaded"`
	Complete   bool  `json:"complete"`
	Retries    int   `json:"retries"`
}

// DownloadTask represents a single download operation
type DownloadTask struct {
	URL           string
	Filepath      string
	SHA256        string
	SHA1          string
	MD5           string
	Size          int64
	Downloaded    int64
	Chunks        int
	SupportsRange bool
	StartTime     time.Time
	Headers       map[string]string
	Cookies       []*http.Cookie
}

// ChunkInfo represents a download chunk
type ChunkInfo struct {
	ID    int
	Start int64
	End   int64
	Path  string
}

// ProgressInfo for real-time updates
type ProgressInfo struct {
	Downloaded int64
	Total      int64
	Speed      float64
	Percentage float64
	Active     int32
	ETA        time.Duration
}

// RateLimiter implements bandwidth throttling
type RateLimiter struct {
	limiter  *rate.Limiter
	enabled  bool
	maxBytes int64
	mu       sync.RWMutex
}

// ProxyManager handles proxy configuration
type ProxyManager struct {
	proxyURL *url.URL
	enabled  bool
}

// MirrorManager handles multiple mirrors
type MirrorManager struct {
	mirrors    []string
	current    int
	maxRetries int
	mu         sync.Mutex
}

// JobQueue manages download jobs
type JobQueue struct {
	jobs       map[string]*Job
	queue      []*Job
	active     map[string]*Job
	completed  map[string]*Job
	failed     map[string]*Job
	maxActive  int
	mu         sync.RWMutex
	db         *sql.DB
	stopCh     chan struct{}
	wg         sync.WaitGroup
	manager    *DownloadManager
}

// DaemonServer provides HTTP API
type DaemonServer struct {
	queue       *JobQueue
	config      *Config
	server      *http.Server
	rateLimiter *RateLimiter
}

// Initialize default configuration
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	return &Config{
		MaxConnections: DefaultChunks,
		ChunkSize:      ChunkSize,
		MaxRetries:     MaxRetries,
		RetryDelay:     2,
		DownloadDir:    "./downloads",
		RateLimit:      0,
		UserAgent:      fmt.Sprintf("FastDL/%s", Version),
		Timeout:        30,
		ResumeEnabled:  true,
		VerifyChecksum: true,
		DaemonPort:     8080,
		DatabasePath:   filepath.Join(homeDir, ".config", "fastdl", "fastdl.db"),
		EnableHTTP2:    true,
		MaxParallel:    4,
		TorrentPort:    6881,
		LogFile:        filepath.Join(homeDir, ".config", "fastdl", "fastdl.log"),
		ConfigPath:     filepath.Join(homeDir, ".config", "fastdl", "config.json"),
		Headers:        make(map[string]string),
	}
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(bytesPerSecond int64) *RateLimiter {
	if bytesPerSecond <= 0 {
		return &RateLimiter{enabled: false}
	}
	return &RateLimiter{
		limiter:  rate.NewLimiter(rate.Limit(bytesPerSecond), int(bytesPerSecond)),
		enabled:  true,
		maxBytes: bytesPerSecond,
	}
}

func (rl *RateLimiter) Wait(ctx context.Context, bytes int) error {
	if !rl.enabled {
		return nil
	}
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	return rl.limiter.WaitN(ctx, bytes)
}

func (rl *RateLimiter) SetLimit(bytesPerSecond int64) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if bytesPerSecond <= 0 {
		rl.enabled = false
		return
	}
	rl.enabled = true
	rl.maxBytes = bytesPerSecond
	rl.limiter.SetLimit(rate.Limit(bytesPerSecond))
	rl.limiter.SetBurst(int(bytesPerSecond))
}

// NewProxyManager creates a new proxy manager
func NewProxyManager(proxyURL string) (*ProxyManager, error) {
	if proxyURL == "" {
		return &ProxyManager{enabled: false}, nil
	}
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}
	return &ProxyManager{
		proxyURL: parsed,
		enabled:  true,
	}, nil
}

func (p *ProxyManager) GetTransport() *http.Transport {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
	}
	if p.enabled && p.proxyURL != nil {
		transport.Proxy = http.ProxyURL(p.proxyURL)
	}
	return transport
}

// NewMirrorManager creates a new mirror manager
func NewMirrorManager(mirrors []string, maxRetries int) *MirrorManager {
	return &MirrorManager{
		mirrors:    mirrors,
		maxRetries: maxRetries,
	}
}

func (m *MirrorManager) GetNextMirror() (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current >= len(m.mirrors) {
		return "", false
	}
	mirror := m.mirrors[m.current]
	m.current++
	return mirror, true
}

// NewDownloadManager creates a new download manager
func NewDownloadManager(config *Config) (*DownloadManager, error) {
	proxyManager, err := NewProxyManager(config.ProxyURL)
	if err != nil {
		return nil, err
	}

	transport := proxyManager.GetTransport()
	if config.EnableHTTP2 {
		http2.ConfigureTransport(transport)
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(config.Timeout) * time.Second,
	}

	return &DownloadManager{
		client:       client,
		maxWorkers:   config.MaxConnections,
		downloadDir:  config.DownloadDir,
		verifyHashes: config.VerifyChecksum,
		resume:       config.ResumeEnabled,
		rateLimiter:  NewRateLimiter(config.RateLimit),
		proxyManager: proxyManager,
		config:       config,
	}, nil
}

// GetFileInfo retrieves file information from URL
func (dm *DownloadManager) GetFileInfo(ctx context.Context, urlStr string) (*DownloadTask, error) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", urlStr, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", dm.config.UserAgent)
	for k, v := range dm.config.Headers {
		req.Header.Set(k, v)
	}

	resp, err := dm.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	task := &DownloadTask{
		URL:       urlStr,
		StartTime: time.Now(),
		Headers:   dm.config.Headers,
	}

	if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
		task.Size, _ = strconv.ParseInt(contentLength, 10, 64)
	}

	if acceptRanges := resp.Header.Get("Accept-Ranges"); acceptRanges == "bytes" {
		task.SupportsRange = true
	}

	if task.Filepath == "" {
		parsedURL, _ := url.Parse(urlStr)
		task.Filepath = path.Base(parsedURL.Path)
		if task.Filepath == "" || task.Filepath == "/" {
			task.Filepath = fmt.Sprintf("download_%d", time.Now().Unix())
		}
	}

	return task, nil
}

// Download performs the main download operation
func (dm *DownloadManager) Download(ctx context.Context, task *DownloadTask) error {
	info, err := dm.GetFileInfo(ctx, task.URL)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	if task.Size == 0 {
		task.Size = info.Size
	}
	task.SupportsRange = info.SupportsRange

	outputPath := filepath.Join(dm.downloadDir, task.Filepath)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	fmt.Printf("%sDownloading:%s %s\n", ColorGreen, ColorReset, task.URL)
	fmt.Printf("%sOutput:%s %s\n", ColorCyan, ColorReset, outputPath)
	fmt.Printf("%sSize:%s %s\n", ColorCyan, ColorReset, formatBytes(task.Size))
	fmt.Printf("%sRange Support:%s %v\n", ColorCyan, ColorReset, task.SupportsRange)
	fmt.Printf("%sConnections:%s %d\n\n", ColorCyan, ColorReset, task.Chunks)

	progress := &ProgressInfo{Total: task.Size}
	progressDone := make(chan bool)
	go dm.reportProgress(ctx, task, progress, progressDone)

	var downloadErr error
	
	if task.SupportsRange && task.Chunks > 1 && task.Size > 0 {
		downloadErr = dm.downloadParallel(ctx, task, outputPath, progress)
	} else {
		downloadErr = dm.downloadSingle(ctx, task, outputPath, progress)
	}

	close(progressDone)
	
	if downloadErr != nil {
		return downloadErr
	}

	// Verify checksums
	if dm.verifyHashes {
		if err := dm.verifyChecksums(outputPath, task); err != nil {
			return err
		}
	}

	duration := time.Since(task.StartTime)
	avgSpeed := float64(task.Size) / duration.Seconds() / 1024 / 1024
	fmt.Printf("\n%s✓ Download completed in %s (avg %.2f MB/s)%s\n", 
		ColorGreen, duration.Round(time.Second), avgSpeed, ColorReset)

	return nil
}

// downloadParallel handles multi-threaded downloads
func (dm *DownloadManager) downloadParallel(ctx context.Context, task *DownloadTask, outputPath string, progress *ProgressInfo) error {
	tempFile, err := os.Create(outputPath + ".tmp")
	if err != nil {
		return err
	}
	defer os.Remove(outputPath + ".tmp")

	if err := tempFile.Truncate(task.Size); err != nil {
		tempFile.Close()
		return err
	}
	tempFile.Close()

	chunkSize := task.Size / int64(task.Chunks)
	chunks := make([]ChunkInfo, task.Chunks)
	
	for i := 0; i < task.Chunks; i++ {
		chunks[i] = ChunkInfo{
			ID:    i,
			Start: int64(i) * chunkSize,
			Path:  fmt.Sprintf("%s.part%d", outputPath, i),
		}
		
		if i == task.Chunks-1 {
			chunks[i].End = task.Size - 1
		} else {
			chunks[i].End = chunks[i].Start + chunkSize - 1
		}
	}

	var wg sync.WaitGroup
	chunkChan := make(chan ChunkInfo, len(chunks))
	errorChan := make(chan error, len(chunks))
	
	for i := 0; i < dm.maxWorkers && i < task.Chunks; i++ {
		wg.Add(1)
		go dm.downloadWorker(ctx, &wg, task, chunkChan, errorChan, progress)
	}

	for _, chunk := range chunks {
		chunkChan <- chunk
	}
	close(chunkChan)

	wg.Wait()
	close(errorChan)

	for err := range errorChan {
		if err != nil {
			return err
		}
	}

	return dm.mergeChunks(outputPath, chunks)
}

// downloadWorker handles individual chunk downloads
func (dm *DownloadManager) downloadWorker(ctx context.Context, wg *sync.WaitGroup, task *DownloadTask, chunks <-chan ChunkInfo, errors chan<- error, progress *ProgressInfo) {
	defer wg.Done()

	for chunk := range chunks {
		atomic.AddInt32(&progress.Active, 1)
		
		for retry := 0; retry < dm.config.MaxRetries; retry++ {
			if err := dm.downloadChunk(ctx, task.URL, chunk, progress, task.Headers); err == nil {
				break
			} else if retry == dm.config.MaxRetries-1 {
				errors <- fmt.Errorf("chunk %d failed after %d retries: %w", chunk.ID, dm.config.MaxRetries, err)
				atomic.AddInt32(&progress.Active, -1)
				return
			}
			time.Sleep(time.Duration(dm.config.RetryDelay) * time.Second)
		}
		
		atomic.AddInt32(&progress.Active, -1)
	}
}

// downloadChunk downloads a single chunk
func (dm *DownloadManager) downloadChunk(ctx context.Context, urlStr string, chunk ChunkInfo, progress *ProgressInfo, headers map[string]string) error {
	if dm.resume {
		if stat, err := os.Stat(chunk.Path); err == nil {
			if stat.Size() == chunk.End-chunk.Start+1 {
				atomic.AddInt64(&progress.Downloaded, stat.Size())
				return nil
			}
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", chunk.Start, chunk.End))
	req.Header.Set("User-Agent", dm.config.UserAgent)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := dm.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}

	file, err := os.Create(chunk.Path)
	if err != nil {
		return err
	}
	defer file.Close()

	buffer := make([]byte, BufferSize)
	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			if dm.rateLimiter != nil {
				dm.rateLimiter.Wait(ctx, n)
			}
			if _, writeErr := file.Write(buffer[:n]); writeErr != nil {
				return writeErr
			}
			atomic.AddInt64(&progress.Downloaded, int64(n))
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil
}

// mergeChunks combines all chunks into final file
func (dm *DownloadManager) mergeChunks(outputPath string, chunks []ChunkInfo) error {
	output, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer output.Close()

	for _, chunk := range chunks {
		input, err := os.Open(chunk.Path)
		if err != nil {
			return err
		}

		if _, err := io.Copy(output, input); err != nil {
			input.Close()
			return err
		}
		
		input.Close()
		os.Remove(chunk.Path)
	}

	return nil
}

// downloadSingle handles single-threaded downloads
func (dm *DownloadManager) downloadSingle(ctx context.Context, task *DownloadTask, outputPath string, progress *ProgressInfo) error {
	req, err := http.NewRequestWithContext(ctx, "GET", task.URL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", dm.config.UserAgent)
	for k, v := range task.Headers {
		req.Header.Set(k, v)
	}

	resp, err := dm.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	buffer := make([]byte, BufferSize)
	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			if dm.rateLimiter != nil {
				dm.rateLimiter.Wait(ctx, n)
			}
			if _, writeErr := file.Write(buffer[:n]); writeErr != nil {
				return writeErr
			}
			atomic.AddInt64(&progress.Downloaded, int64(n))
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil
}

// reportProgress displays download progress
func (dm *DownloadManager) reportProgress(ctx context.Context, task *DownloadTask, progress *ProgressInfo, done <-chan bool) {
	ticker := time.NewTicker(ProgressUpdate)
	defer ticker.Stop()

	lastDownloaded := int64(0)
	lastTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			downloaded := atomic.LoadInt64(&progress.Downloaded)
			now := time.Now()
			elapsed := now.Sub(lastTime).Seconds()
			
			if elapsed > 0 {
				speed := float64(downloaded-lastDownloaded) / elapsed / 1024 / 1024
				percentage := float64(downloaded) / float64(progress.Total) * 100
				
				if speed > 0 {
					remaining := progress.Total - downloaded
					eta := time.Duration(float64(remaining) / (float64(downloaded-lastDownloaded) / elapsed)) * time.Second
					progress.ETA = eta
				}

				active := atomic.LoadInt32(&progress.Active)
				
				// Progress bar
				barWidth := 40
				filled := int(percentage * float64(barWidth) / 100)
				bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
				
				fmt.Printf("\r%s[%s] %.1f%% %s/%s | %.2f MB/s | %d active | ETA: %s%s",
					ColorCyan, bar, percentage,
					formatBytes(downloaded),
					formatBytes(progress.Total),
					speed,
					active,
					formatDuration(progress.ETA),
					ColorReset)
				
				lastDownloaded = downloaded
				lastTime = now
			}
		}
	}
}

// verifyChecksums verifies file checksums
func (dm *DownloadManager) verifyChecksums(filepath string, task *DownloadTask) error {
	if task.SHA256 != "" {
		fmt.Printf("\n%sVerifying SHA256...%s", ColorYellow, ColorReset)
		hash, err := calculateHash(filepath, "sha256")
		if err != nil {
			return err
		}
		if !strings.EqualFold(hash, task.SHA256) {
			return fmt.Errorf("SHA256 mismatch: expected %s, got %s", task.SHA256, hash)
		}
		fmt.Printf(" %s✓%s\n", ColorGreen, ColorReset)
	}

	if task.SHA1 != "" {
		fmt.Printf("%sVerifying SHA1...%s", ColorYellow, ColorReset)
		hash, err := calculateHash(filepath, "sha1")
		if err != nil {
			return err
		}
		if !strings.EqualFold(hash, task.SHA1) {
			return fmt.Errorf("SHA1 mismatch: expected %s, got %s", task.SHA1, hash)
		}
		fmt.Printf(" %s✓%s\n", ColorGreen, ColorReset)
	}

	if task.MD5 != "" {
		fmt.Printf("%sVerifying MD5...%s", ColorYellow, ColorReset)
		hash, err := calculateHash(filepath, "md5")
		if err != nil {
			return err
		}
		if !strings.EqualFold(hash, task.MD5) {
			return fmt.Errorf("MD5 mismatch: expected %s, got %s", task.MD5, hash)
		}
		fmt.Printf(" %s✓%s\n", ColorGreen, ColorReset)
	}

	return nil
}

// calculateHash calculates file hash
func calculateHash(filepath string, algorithm string) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var h hash.Hash
	switch algorithm {
	case "sha256":
		h = sha256.New()
	case "sha1":
		h = sha1.New()
	case "md5":
		h = md5.New()
	default:
		return "", fmt.Errorf("unsupported hash algorithm: %s", algorithm)
	}

	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// BatchDownload handles multiple downloads
func (dm *DownloadManager) BatchDownload(ctx context.Context, urlFile string, concurrent int) error {
	file, err := os.Open(urlFile)
	if err != nil {
		return err
	}
	defer file.Close()

	var tasks []DownloadTask
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		task := DownloadTask{
			URL:    parts[0],
			Chunks: dm.maxWorkers,
		}

		for i := 1; i < len(parts); i++ {
			if strings.HasPrefix(parts[i], "sha256:") {
				task.SHA256 = strings.TrimPrefix(parts[i], "sha256:")
			} else if strings.HasPrefix(parts[i], "sha1:") {
				task.SHA1 = strings.TrimPrefix(parts[i], "sha1:")
			} else if strings.HasPrefix(parts[i], "md5:") {
				task.MD5 = strings.TrimPrefix(parts[i], "md5:")
			}
		}

		tasks = append(tasks, task)
	}

	fmt.Printf("%sFound %d URLs to download%s\n\n", ColorCyan, len(tasks), ColorReset)

	sem := make(chan struct{}, concurrent)
	var wg sync.WaitGroup
	
	for i, task := range tasks {
		wg.Add(1)
		go func(index int, t DownloadTask) {
			defer wg.Done()
			
			sem <- struct{}{}
			defer func() { <-sem }()
			
			fmt.Printf("%s[%d/%d] Downloading %s%s\n", ColorBlue, index+1, len(tasks), t.URL, ColorReset)
			
			if err := dm.Download(ctx, &t); err != nil {
				fmt.Printf("%s[%d/%d] Failed: %v%s\n", ColorRed, index+1, len(tasks), err, ColorReset)
			} else {
				fmt.Printf("%s[%d/%d] Completed%s\n", ColorGreen, index+1, len(tasks), ColorReset)
			}
		}(i, task)
	}

	wg.Wait()
	return nil
}

// NewJobQueue creates a new job queue
func NewJobQueue(maxActive int, dbPath string) (*JobQueue, error) {
	// Create directory if it doesn't exist
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	schema := `
	CREATE TABLE IF NOT EXISTS jobs (
		id TEXT PRIMARY KEY,
		url TEXT NOT NULL,
		protocol TEXT,
		mirrors TEXT,
		file_path TEXT,
		total_size INTEGER,
		downloaded INTEGER,
		status TEXT,
		priority INTEGER,
		sha256 TEXT,
		sha1 TEXT,
		md5 TEXT,
		added_time TIMESTAMP,
		start_time TIMESTAMP,
		end_time TIMESTAMP,
		error TEXT,
		metadata TEXT,
		chunk_states TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_status ON jobs(status);
	CREATE INDEX IF NOT EXISTS idx_priority ON jobs(priority DESC);
	`
	
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}

	jq := &JobQueue{
		jobs:      make(map[string]*Job),
		queue:     make([]*Job, 0),
		active:    make(map[string]*Job),
		completed: make(map[string]*Job),
		failed:    make(map[string]*Job),
		maxActive: maxActive,
		db:        db,
		stopCh:    make(chan struct{}),
	}

	if err := jq.loadJobs(); err != nil {
		return nil, err
	}

	return jq, nil
}

func (jq *JobQueue) loadJobs() error {
	rows, err := jq.db.Query("SELECT id, url, protocol, file_path, total_size, downloaded, status, priority, sha256, sha1, md5, added_time FROM jobs WHERE status != 'completed'")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		job := &Job{}
		err := rows.Scan(&job.ID, &job.URL, &job.Protocol, &job.FilePath, &job.TotalSize, 
			&job.Downloaded, &job.Status, &job.Priority, &job.SHA256, &job.SHA1, &job.MD5, &job.AddedTime)
		if err != nil {
			continue
		}
		
		if job.Status == "downloading" {
			job.Status = "pending"
		}
		
		jq.jobs[job.ID] = job
		if job.Status == "pending" {
			jq.queue = append(jq.queue, job)
		}
	}

	return nil
}

func (jq *JobQueue) AddJob(job *Job) error {
	jq.mu.Lock()
	defer jq.mu.Unlock()

	if job.ID == "" {
		job.ID = fmt.Sprintf("%d-%x", time.Now().Unix(), time.Now().UnixNano())
	}

	// Detect protocol from URL
	if job.Protocol == "" {
		parsedURL, _ := url.Parse(job.URL)
		job.Protocol = parsedURL.Scheme
	}

	job.Status = "pending"
	job.AddedTime = time.Now()

	_, err := jq.db.Exec(`
		INSERT INTO jobs (id, url, protocol, file_path, total_size, status, priority, sha256, sha1, md5, added_time)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, job.ID, job.URL, job.Protocol, job.FilePath, job.TotalSize, job.Status, job.Priority, 
		job.SHA256, job.SHA1, job.MD5, job.AddedTime)
	
	if err != nil {
		return err
	}

	jq.jobs[job.ID] = job
	jq.queue = append(jq.queue, job)
	jq.sortQueue()

	return nil
}

func (jq *JobQueue) sortQueue() {
	sort.Slice(jq.queue, func(i, j int) bool {
		return jq.queue[i].Priority > jq.queue[j].Priority
	})
}

func (jq *JobQueue) ProcessQueue(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-jq.stopCh:
			return
		case <-ticker.C:
			jq.processNext()
		}
	}
}

func (jq *JobQueue) processNext() {
	jq.mu.Lock()
	defer jq.mu.Unlock()

	if len(jq.active) >= jq.maxActive || len(jq.queue) == 0 {
		return
	}

	job := jq.queue[0]
	jq.queue = jq.queue[1:]
	jq.active[job.ID] = job

	go jq.processJob(job)
}

func (jq *JobQueue) processJob(job *Job) {
	defer func() {
		jq.mu.Lock()
		delete(jq.active, job.ID)
		jq.mu.Unlock()
	}()

	job.Status = "downloading"
	now := time.Now()
	job.StartTime = &now

	ctx := context.Background()
	task := &DownloadTask{
		URL:      job.URL,
		Filepath: job.FilePath,
		SHA256:   job.SHA256,
		SHA1:     job.SHA1,
		MD5:      job.MD5,
		Chunks:   job.Chunks,
	}

	if jq.manager != nil {
		if err := jq.manager.Download(ctx, task); err != nil {
			job.Status = "failed"
			job.Error = err.Error()
			jq.mu.Lock()
			jq.failed[job.ID] = job
			jq.mu.Unlock()
		} else {
			job.Status = "completed"
			end := time.Now()
			job.EndTime = &end
			jq.mu.Lock()
			jq.completed[job.ID] = job
			jq.mu.Unlock()
		}
	}

	jq.updateJobInDB(job)
}

func (jq *JobQueue) updateJobInDB(job *Job) {
	_, err := jq.db.Exec(`
		UPDATE jobs SET status = ?, downloaded = ?, error = ?, start_time = ?, end_time = ?
		WHERE id = ?
	`, job.Status, job.Downloaded, job.Error, job.StartTime, job.EndTime, job.ID)
	if err != nil {
		fmt.Printf("Failed to update job in DB: %v\n", err)
	}
}

// DaemonServer implementation
func NewDaemonServer(config *Config, queue *JobQueue) *DaemonServer {
	return &DaemonServer{
		queue:       queue,
		config:      config,
		rateLimiter: NewRateLimiter(config.RateLimit),
	}
}

func (d *DaemonServer) Start() error {
	mux := http.NewServeMux()
	
	// API endpoints
	mux.HandleFunc("/api/jobs", d.handleJobs)
	mux.HandleFunc("/api/jobs/add", d.handleAddJob)
	mux.HandleFunc("/api/jobs/pause", d.handlePauseJob)
	mux.HandleFunc("/api/jobs/resume", d.handleResumeJob)
	mux.HandleFunc("/api/jobs/delete", d.handleDeleteJob)
	mux.HandleFunc("/api/jobs/retry", d.handleRetryJob)
	mux.HandleFunc("/api/status", d.handleStatus)
	mux.HandleFunc("/api/config", d.handleConfig)
	mux.HandleFunc("/api/stats", d.handleStats)

	// Serve simple web UI
	mux.HandleFunc("/", d.handleWebUI)

	d.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", d.config.DaemonPort),
		Handler: mux,
	}

	fmt.Printf("%s[Daemon] Server listening on http://localhost:%d%s\n", ColorGreen, d.config.DaemonPort, ColorReset)
	return d.server.ListenAndServe()
}

func (d *DaemonServer) handleJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	d.queue.mu.RLock()
	defer d.queue.mu.RUnlock()

	response := map[string]interface{}{
		"pending":   len(d.queue.queue),
		"active":    len(d.queue.active),
		"completed": len(d.queue.completed),
		"failed":    len(d.queue.failed),
		"jobs":      d.queue.jobs,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (d *DaemonServer) handleAddJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var job Job
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := d.queue.AddJob(&job); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": job.ID, "status": "added"})
}

func (d *DaemonServer) handlePauseJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Query().Get("id")
	if jobID == "" {
		http.Error(w, "Job ID required", http.StatusBadRequest)
		return
	}

	d.queue.mu.Lock()
	defer d.queue.mu.Unlock()

	if job, exists := d.queue.jobs[jobID]; exists {
		job.Status = "paused"
		d.queue.updateJobInDB(job)
		w.Write([]byte(`{"status":"paused"}`))
	} else {
		http.Error(w, "Job not found", http.StatusNotFound)
	}
}

func (d *DaemonServer) handleResumeJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Query().Get("id")
	if jobID == "" {
		http.Error(w, "Job ID required", http.StatusBadRequest)
		return
	}

	d.queue.mu.Lock()
	defer d.queue.mu.Unlock()

	if job, exists := d.queue.jobs[jobID]; exists {
		job.Status = "pending"
		d.queue.queue = append(d.queue.queue, job)
		d.queue.sortQueue()
		d.queue.updateJobInDB(job)
		w.Write([]byte(`{"status":"resumed"}`))
	} else {
		http.Error(w, "Job not found", http.StatusNotFound)
	}
}

func (d *DaemonServer) handleDeleteJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Query().Get("id")
	if jobID == "" {
		http.Error(w, "Job ID required", http.StatusBadRequest)
		return
	}

	d.queue.mu.Lock()
	defer d.queue.mu.Unlock()

	if _, exists := d.queue.jobs[jobID]; exists {
		delete(d.queue.jobs, jobID)
		d.queue.db.Exec("DELETE FROM jobs WHERE id = ?", jobID)
		w.Write([]byte(`{"status":"deleted"}`))
	} else {
		http.Error(w, "Job not found", http.StatusNotFound)
	}
}

func (d *DaemonServer) handleRetryJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Query().Get("id")
	if jobID == "" {
		http.Error(w, "Job ID required", http.StatusBadRequest)
		return
	}

	d.queue.mu.Lock()
	defer d.queue.mu.Unlock()

	if job, exists := d.queue.failed[jobID]; exists {
		job.Status = "pending"
		job.Error = ""
		delete(d.queue.failed, jobID)
		d.queue.queue = append(d.queue.queue, job)
		d.queue.sortQueue()
		d.queue.updateJobInDB(job)
		w.Write([]byte(`{"status":"retrying"}`))
	} else {
		http.Error(w, "Job not found in failed queue", http.StatusNotFound)
	}
}

func (d *DaemonServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"version":     Version,
		"uptime":      time.Since(startTime).Seconds(),
		"jobs_total":  len(d.queue.jobs),
		"jobs_active": len(d.queue.active),
		"rate_limit":  d.config.RateLimit,
		"config":      d.config,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (d *DaemonServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(d.config)
		return
	}

	if r.Method == http.MethodPost {
		var newConfig Config
		if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		*d.config = newConfig
		saveConfig(d.config)
		
		w.Write([]byte(`{"status":"updated"}`))
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (d *DaemonServer) handleStats(w http.ResponseWriter, r *http.Request) {
	var totalDownloaded, totalSize int64
	var avgSpeed float64
	var completedCount int

	d.queue.mu.RLock()
	for _, job := range d.queue.completed {
		totalDownloaded += job.Downloaded
		totalSize += job.TotalSize
		completedCount++
		if job.StartTime != nil && job.EndTime != nil {
			duration := job.EndTime.Sub(*job.StartTime).Seconds()
			if duration > 0 {
				avgSpeed += float64(job.TotalSize) / duration
			}
		}
	}
	d.queue.mu.RUnlock()

	if completedCount > 0 {
		avgSpeed = avgSpeed / float64(completedCount) / 1024 / 1024
	}

	stats := map[string]interface{}{
		"total_downloaded": formatBytes(totalDownloaded),
		"total_size":       formatBytes(totalSize),
		"avg_speed_mbps":   avgSpeed,
		"completed_jobs":   completedCount,
		"failed_jobs":      len(d.queue.failed),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (d *DaemonServer) handleWebUI(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>FastDL Dashboard</title>
    <style>
        body { font-family: Arial, sans-serif; background: #1a1a1a; color: #fff; margin: 0; padding: 20px; }
        .container { max-width: 1200px; margin: 0 auto; }
        h1 { color: #4CAF50; }
        .stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 20px; margin: 20px 0; }
        .stat-card { background: #2a2a2a; padding: 20px; border-radius: 8px; }
        .stat-value { font-size: 24px; font-weight: bold; color: #4CAF50; }
        .stat-label { color: #888; margin-top: 5px; }
        .jobs-table { width: 100%; background: #2a2a2a; border-radius: 8px; overflow: hidden; }
        .jobs-table th { background: #333; padding: 15px; text-align: left; }
        .jobs-table td { padding: 15px; border-top: 1px solid #333; }
        .status { padding: 5px 10px; border-radius: 4px; font-size: 12px; }
        .status.active { background: #4CAF50; }
        .status.pending { background: #FF9800; }
        .status.completed { background: #2196F3; }
        .status.failed { background: #F44336; }
        .add-job { background: #4CAF50; color: white; border: none; padding: 10px 20px; border-radius: 4px; cursor: pointer; }
        .add-job:hover { background: #45a049; }
        input { background: #333; border: 1px solid #555; color: white; padding: 10px; border-radius: 4px; width: 100%; margin: 5px 0; }
    </style>
</head>
<body>
    <div class="container">
        <h1>FastDL Dashboard</h1>
        <div class="stats" id="stats"></div>
        <div style="margin: 20px 0;">
            <h2>Add New Download</h2>
            <input type="text" id="urlInput" placeholder="Enter URL">
            <button class="add-job" onclick="addJob()">Add Download</button>
        </div>
        <h2>Jobs</h2>
        <table class="jobs-table">
            <thead>
                <tr>
                    <th>ID</th>
                    <th>URL</th>
                    <th>Status</th>
                    <th>Progress</th>
                    <th>Actions</th>
                </tr>
            </thead>
            <tbody id="jobsList"></tbody>
        </table>
    </div>
    <script>
        async function fetchData() {
            try {
                const [jobsRes, statsRes, statusRes] = await Promise.all([
                    fetch('/api/jobs'),
                    fetch('/api/stats'),
                    fetch('/api/status')
                ]);
                
                const jobs = await jobsRes.json();
                const stats = await statsRes.json();
                const status = await statusRes.json();
                
                updateStats(stats, status, jobs);
                updateJobsList(jobs);
            } catch (error) {
                console.error('Error fetching data:', error);
            }
        }
        
        function updateStats(stats, status, jobs) {
            const statsDiv = document.getElementById('stats');
            statsDiv.innerHTML = ` +
                '<div class="stat-card">
                    <div class="stat-value">${jobs.active || 0}</div>
                    <div class="stat-label">Active Downloads</div>
                </div>
                <div class="stat-card">
                    <div class="stat-value">${jobs.pending || 0}</div>
                    <div class="stat-label">Pending</div>
                </div>
                <div class="stat-card">
                    <div class="stat-value">${jobs.completed || 0}</div>
                    <div class="stat-label">Completed</div>
                </div>
                <div class="stat-card">
                    <div class="stat-value">${stats.total_downloaded || '0 B'}</div>
                    <div class="stat-label">Total Downloaded</div>
                </div>';
        }
        
        function updateJobsList(data) {
            const tbody = document.getElementById('jobsList');
            tbody.innerHTML = '';
            
            if (data.jobs) {
                Object.entries(data.jobs).forEach(([id, job]) => {
                    const progress = job.total_size > 0 
                        ? Math.round((job.downloaded / job.total_size) * 100) 
                        : 0;
                    
                    tbody.innerHTML += ` +
                        '<tr>
                            <td>${id.substring(0, 8)}...</td>
                            <td>${job.url}</td>
                            <td><span class="status ${job.status}">${job.status}</span></td>
                            <td>${progress}%</td>
                            <td>
                                <button onclick="pauseJob(\'${id}\')">Pause</button>
                                <button onclick="resumeJob(\'${id}\')">Resume</button>
                                <button onclick="deleteJob(\'${id}\')">Delete</button>
                            </td>
                        </tr>';
                });
            }
        }
        
        async function addJob() {
            const url = document.getElementById('urlInput').value;
            if (!url) return;
            
            try {
                await fetch('/api/jobs/add', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({url: url})
                });
                document.getElementById('urlInput').value = '';
                fetchData();
            } catch (error) {
                console.error('Error adding job:', error);
            }
        }
        
        async function pauseJob(id) {
            await fetch('/api/jobs/pause?id=' + id, {method: 'POST'});
            fetchData();
        }
        
        async function resumeJob(id) {
            await fetch('/api/jobs/resume?id=' + id, {method: 'POST'});
            fetchData();
        }
        
        async function deleteJob(id) {
            await fetch('/api/jobs/delete?id=' + id, {method: 'DELETE'});
            fetchData();
        }
        
        // Auto-refresh every 2 seconds
        setInterval(fetchData, 2000);
        fetchData();
    </script>
</body>
</html>`
	
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// Configuration functions
func loadConfig(path string) (*Config, error) {
	config := DefaultConfig()
	if path == "" {
		path = config.ConfigPath
	}

	file, err := os.Open(path)
	if err != nil {
		return config, nil // Use defaults if config doesn't exist
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(config); err != nil {
		return nil, err
	}

	return config, nil
}

func saveConfig(config *Config) error {
	configDir := filepath.Dir(config.ConfigPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	file, err := os.Create(config.ConfigPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(config)
}

// Utility functions
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		return "unknown"
	}
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	
	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// CLI Commands
func cmdDownload(args []string) {
	fs := flag.NewFlagSet("download", flag.ExitOnError)
	connections := fs.Int("c", DefaultChunks, "number of connections")
	output := fs.String("o", "", "output file path")
	sha256Hash := fs.String("sha256", "", "SHA256 hash")
	sha1Hash := fs.String("sha1", "", "SHA1 hash")
	md5Hash := fs.String("md5", "", "MD5 hash")
	downloadDir := fs.String("d", ".", "download directory")
	rateLimit := fs.Int64("rate", 0, "rate limit in bytes/sec")
	proxy := fs.String("proxy", "", "proxy URL")
	header := fs.String("H", "", "custom header (format: Key:Value)")
	
	if err := fs.Parse(args); err != nil {
		log.Fatal(err)
	}

	if fs.NArg() < 1 {
		fmt.Println("Usage: fastdl download [options] <URL>")
		fs.PrintDefaults()
		os.Exit(1)
	}

	config := DefaultConfig()
	config.MaxConnections = *connections
	config.DownloadDir = *downloadDir
	config.RateLimit = *rateLimit
	config.ProxyURL = *proxy
	
	if *header != "" {
		parts := strings.SplitN(*header, ":", 2)
		if len(parts) == 2 {
			config.Headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	dm, err := NewDownloadManager(config)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n\nDownload interrupted")
		cancel()
	}()

	task := &DownloadTask{
		URL:      fs.Arg(0),
		Filepath: *output,
		SHA256:   *sha256Hash,
		SHA1:     *sha1Hash,
		MD5:      *md5Hash,
		Chunks:   *connections,
		Headers:  config.Headers,
	}

	if task.Filepath == "" {
		parsedURL, _ := url.Parse(task.URL)
		task.Filepath = path.Base(parsedURL.Path)
	}

	if err := dm.Download(ctx, task); err != nil {
		log.Fatal(err)
	}
}

func cmdBatch(args []string) {
	fs := flag.NewFlagSet("batch", flag.ExitOnError)
	concurrent := fs.Int("c", 4, "concurrent downloads")
	downloadDir := fs.String("d", ".", "download directory")
	connections := fs.Int("w", DefaultChunks, "connections per download")
	
	if err := fs.Parse(args); err != nil {
		log.Fatal(err)
	}

	if fs.NArg() < 1 {
		fmt.Println("Usage: fastdl batch [options] <url-file>")
		fs.PrintDefaults()
		os.Exit(1)
	}

	config := DefaultConfig()
	config.MaxConnections = *connections
	config.DownloadDir = *downloadDir

	dm, err := NewDownloadManager(config)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n\nBatch download interrupted")
		cancel()
	}()

	if err := dm.BatchDownload(ctx, fs.Arg(0), *concurrent); err != nil {
		log.Fatal(err)
	}
}

func cmdDaemon(args []string) {
	fs := flag.NewFlagSet("daemon", flag.ExitOnError)
	port := fs.Int("port", 8080, "daemon port")
	configPath := fs.String("config", "", "config file path")
	workers := fs.Int("workers", 4, "max parallel downloads")
	
	if err := fs.Parse(args); err != nil {
		log.Fatal(err)
	}

	config, err := loadConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	
	config.DaemonPort = *port
	config.EnableDaemon = true
	config.MaxParallel = *workers

	// Save config
	saveConfig(config)

	// Create download manager
	dm, err := NewDownloadManager(config)
	if err != nil {
		log.Fatal(err)
	}

	// Create job queue
	queue, err := NewJobQueue(config.MaxParallel, config.DatabasePath)
	if err != nil {
		log.Fatal(err)
	}
	queue.manager = dm

	// Create daemon server
	daemon := NewDaemonServer(config, queue)
	
	// Start processing queue in background
	ctx := context.Background()
	go queue.ProcessQueue(ctx)
	
	// Handle shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nShutting down daemon...")
		if daemon.server != nil {
			daemon.server.Shutdown(context.Background())
		}
		os.Exit(0)
	}()
	
	fmt.Printf("\n%s╔════════════════════════════════════════╗%s\n", ColorGreen, ColorReset)
	fmt.Printf("%s║       FastDL Daemon Started!           ║%s\n", ColorGreen, ColorReset)
	fmt.Printf("%s╠════════════════════════════════════════╣%s\n", ColorGreen, ColorReset)
	fmt.Printf("%s║  Web UI: http://localhost:%d         ║%s\n", ColorCyan, config.DaemonPort, ColorReset)
	fmt.Printf("%s║  API:    http://localhost:%d/api     ║%s\n", ColorCyan, config.DaemonPort, ColorReset)
	fmt.Printf("%s╚════════════════════════════════════════╝%s\n\n", ColorGreen, ColorReset)
	
	if err := daemon.Start(); err != nil {
		log.Fatal(err)
	}
}

func cmdVerify(args []string) {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	algorithm := fs.String("a", "sha256", "hash algorithm (sha256/sha1/md5)")
	
	if err := fs.Parse(args); err != nil {
		log.Fatal(err)
	}

	if fs.NArg() < 2 {
		fmt.Println("Usage: fastdl verify [options] <file> <hash>")
		fs.PrintDefaults()
		os.Exit(1)
	}

	filepath := fs.Arg(0)
	expectedHash := fs.Arg(1)
	
	fmt.Printf("%sVerifying %s...%s ", ColorYellow, filepath, ColorReset)
	
	calculatedHash, err := calculateHash(filepath, *algorithm)
	if err != nil {
		log.Fatal(err)
	}

	if strings.EqualFold(calculatedHash, expectedHash) {
		fmt.Printf("%s✓%s\n", ColorGreen, ColorReset)
		fmt.Printf("%s%s: %s%s\n", ColorCyan, strings.ToUpper(*algorithm), calculatedHash, ColorReset)
	} else {
		fmt.Printf("%s✗%s\n", ColorRed, ColorReset)
		fmt.Printf("%sExpected: %s%s\n", ColorRed, expectedHash, ColorReset)
		fmt.Printf("%sGot:      %s%s\n", ColorRed, calculatedHash, ColorReset)
		os.Exit(1)
	}
}

func cmdConfig(args []string) {
	fs := flag.NewFlagSet("config", flag.ExitOnError)
	show := fs.Bool("show", false, "show current configuration")
	edit := fs.Bool("edit", false, "edit configuration interactively")
	reset := fs.Bool("reset", false, "reset to default configuration")
	set := fs.String("set", "", "set config value (format: key=value)")
	
	if err := fs.Parse(args); err != nil {
		log.Fatal(err)
	}

	config, err := loadConfig("")
	if err != nil {
		log.Fatal(err)
	}

	if *reset {
		config = DefaultConfig()
		if err := saveConfig(config); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%sConfiguration reset to defaults%s\n", ColorGreen, ColorReset)
		return
	}

	if *show || (!*edit && *set == "") {
		jsonData, _ := json.MarshalIndent(config, "", "  ")
		fmt.Printf("%sCurrentConfiguration:%s\n%s\n", ColorCyan, ColorReset, string(jsonData))
		return
	}

	if *set != "" {
		parts := strings.SplitN(*set, "=", 2)
		if len(parts) != 2 {
			fmt.Printf("%sInvalid format. Use: key=value%s\n", ColorRed, ColorReset)
			os.Exit(1)
		}
		
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		
		switch key {
		case "max_connections":
			config.MaxConnections, _ = strconv.Atoi(value)
		case "download_dir":
			config.DownloadDir = value
		case "rate_limit":
			config.RateLimit, _ = strconv.ParseInt(value, 10, 64)
		case "proxy_url":
			config.ProxyURL = value
		case "daemon_port":
			config.DaemonPort, _ = strconv.Atoi(value)
		case "enable_http2":
			config.EnableHTTP2 = value == "true"
		case "enable_daemon":
			config.EnableDaemon = value == "true"
		case "max_parallel":
			config.MaxParallel, _ = strconv.Atoi(value)
		default:
			fmt.Printf("%sUnknown configuration key: %s%s\n", ColorRed, key, ColorReset)
			os.Exit(1)
		}
		
		if err := saveConfig(config); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%sConfiguration updated: %s = %s%s\n", ColorGreen, key, value, ColorReset)
	}

	if *edit {
		// Interactive configuration editor
		reader := bufio.NewReader(os.Stdin)
		
		fmt.Printf("\n%s=== FastDL Configuration Editor ===%s\n", ColorCyan, ColorReset)
		fmt.Println("Press Enter to keep current value")
		
		fmt.Printf("\nMax Connections [%d]: ", config.MaxConnections)
		if input, _ := reader.ReadString('\n'); strings.TrimSpace(input) != "" {
			config.MaxConnections, _ = strconv.Atoi(strings.TrimSpace(input))
		}
		
		fmt.Printf("Download Directory [%s]: ", config.DownloadDir)
		if input, _ := reader.ReadString('\n'); strings.TrimSpace(input) != "" {
			config.DownloadDir = strings.TrimSpace(input)
		}
		
		fmt.Printf("Rate Limit (bytes/sec, 0=unlimited) [%d]: ", config.RateLimit)
		if input, _ := reader.ReadString('\n'); strings.TrimSpace(input) != "" {
			config.RateLimit, _ = strconv.ParseInt(strings.TrimSpace(input), 10, 64)
		}
		
		fmt.Printf("Proxy URL [%s]: ", config.ProxyURL)
		if input, _ := reader.ReadString('\n'); strings.TrimSpace(input) != "" {
			config.ProxyURL = strings.TrimSpace(input)
		}
		
		fmt.Printf("Daemon Port [%d]: ", config.DaemonPort)
		if input, _ := reader.ReadString('\n'); strings.TrimSpace(input) != "" {
			config.DaemonPort, _ = strconv.Atoi(strings.TrimSpace(input))
		}
		
		fmt.Printf("Enable HTTP/2 [%v]: ", config.EnableHTTP2)
		if input, _ := reader.ReadString('\n'); strings.TrimSpace(input) != "" {
			config.EnableHTTP2 = strings.ToLower(strings.TrimSpace(input)) == "true"
		}
		
		if err := saveConfig(config); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("\n%sConfiguration saved successfully!%s\n", ColorGreen, ColorReset)
	}
}

func cmdTUI(args []string) {
	// Simple TUI mode using terminal controls
	fmt.Printf("\033[2J\033[H") // Clear screen
	
	config, _ := loadConfig("")
	dm, err := NewDownloadManager(config)
	if err != nil {
		log.Fatal(err)
	}

	reader := bufio.NewReader(os.Stdin)
	
	for {
		fmt.Printf("\033[2J\033[H") // Clear screen
		printTUIHeader()
		printTUIMenu()
		
		fmt.Print("\nSelect option: ")
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)
		
		switch choice {
		case "1":
			fmt.Print("Enter URL: ")
			url, _ := reader.ReadString('\n')
			url = strings.TrimSpace(url)
			
			if url != "" {
				ctx := context.Background()
				task := &DownloadTask{
					URL:    url,
					Chunks: config.MaxConnections,
				}
				
				fmt.Println("\nStarting download...")
				if err := dm.Download(ctx, task); err != nil {
					fmt.Printf("%sError: %v%s\n", ColorRed, err, ColorReset)
				}
				fmt.Print("\nPress Enter to continue...")
				reader.ReadString('\n')
			}
			
		case "2":
			fmt.Print("Enter batch file path: ")
			filepath, _ := reader.ReadString('\n')
			filepath = strings.TrimSpace(filepath)
			
			if filepath != "" {
				ctx := context.Background()
				if err := dm.BatchDownload(ctx, filepath, config.MaxParallel); err != nil {
					fmt.Printf("%sError: %v%s\n", ColorRed, err, ColorReset)
				}
				fmt.Print("\nPress Enter to continue...")
				reader.ReadString('\n')
			}
			
		case "3":
			cmdConfig([]string{"-edit"})
			fmt.Print("\nPress Enter to continue...")
			reader.ReadString('\n')
			
		case "4":
			cmdDaemon([]string{})
			
		case "5":
			printStats(config)
			fmt.Print("\nPress Enter to continue...")
			reader.ReadString('\n')
			
		case "q", "Q":
			fmt.Println("\nGoodbye!")
			return
			
		default:
			fmt.Printf("%sInvalid option%s\n", ColorRed, ColorReset)
			time.Sleep(1 * time.Second)
		}
	}
}

func printTUIHeader() {
	fmt.Printf("%s╔══════════════════════════════════════════════════════╗%s\n", ColorGreen, ColorReset)
	fmt.Printf("%s║                                                      ║%s\n", ColorGreen, ColorReset)
	fmt.Printf("%s║              FastDL v%s - TUI Mode               ║%s\n", ColorGreen, Version, ColorReset)
	fmt.Printf("%s║           High-Performance Download Manager          ║%s\n", ColorGreen, ColorReset)
	fmt.Printf("%s║                                                      ║%s\n", ColorGreen, ColorReset)
	fmt.Printf("%s╚══════════════════════════════════════════════════════╝%s\n\n", ColorGreen, ColorReset)
}

func printTUIMenu() {
	fmt.Printf("%s┌─────────────────────────────────────┐%s\n", ColorCyan, ColorReset)
	fmt.Printf("%s│           MAIN MENU                 │%s\n", ColorCyan, ColorReset)
	fmt.Printf("%s├─────────────────────────────────────┤%s\n", ColorCyan, ColorReset)
	fmt.Printf("%s│  1. %sSingle Download                %s│%s\n", ColorCyan, ColorWhite, ColorCyan, ColorReset)
	fmt.Printf("%s│  2. %sBatch Download                 %s│%s\n", ColorCyan, ColorWhite, ColorCyan, ColorReset)
	fmt.Printf("%s│  3. %sConfiguration                  %s│%s\n", ColorCyan, ColorWhite, ColorCyan, ColorReset)
	fmt.Printf("%s│  4. %sStart Daemon                   %s│%s\n", ColorCyan, ColorWhite, ColorCyan, ColorReset)
	fmt.Printf("%s│  5. %sStatistics                     %s│%s\n", ColorCyan, ColorWhite, ColorCyan, ColorReset)
	fmt.Printf("%s│  Q. %sQuit                           %s│%s\n", ColorCyan, ColorYellow, ColorCyan, ColorReset)
	fmt.Printf("%s└─────────────────────────────────────┘%s\n", ColorCyan, ColorReset)
}

func printStats(config *Config) {
	fmt.Printf("\n%s=== Statistics ===%s\n", ColorCyan, ColorReset)
	fmt.Printf("Version:          %s\n", Version)
	fmt.Printf("OS:               %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("CPUs:             %d\n", runtime.NumCPU())
	fmt.Printf("Go Version:       %s\n", runtime.Version())
	fmt.Printf("Config Dir:       %s\n", filepath.Dir(config.ConfigPath))
	fmt.Printf("Database:         %s\n", config.DatabasePath)
	fmt.Printf("Download Dir:     %s\n", config.DownloadDir)
	
	// Check if database exists and show job stats
	if _, err := os.Stat(config.DatabasePath); err == nil {
		if queue, err := NewJobQueue(1, config.DatabasePath); err == nil {
			fmt.Printf("\nJob Statistics:\n")
			fmt.Printf("Total Jobs:       %d\n", len(queue.jobs))
			fmt.Printf("Completed:        %d\n", len(queue.completed))
			fmt.Printf("Failed:           %d\n", len(queue.failed))
		}
	}
}

func cmdInfo() {
	fmt.Printf("%s╔══════════════════════════════════════════════════════╗%s\n", ColorGreen, ColorReset)
	fmt.Printf("%s║         FastDL v%s - System Information         ║%s\n", ColorGreen, Version, ColorReset)
	fmt.Printf("%s╚══════════════════════════════════════════════════════╝%s\n\n", ColorGreen, ColorReset)
	
	fmt.Printf("%sSystem Information:%s\n", ColorCyan, ColorReset)
	fmt.Printf("  OS:           %s\n", runtime.GOOS)
	fmt.Printf("  Architecture: %s\n", runtime.GOARCH)
	fmt.Printf("  CPUs:         %d\n", runtime.NumCPU())
	fmt.Printf("  Go Version:   %s\n", runtime.Version())
	fmt.Printf("  Compiler:     %s\n", runtime.Compiler)
	
	fmt.Printf("\n%sFeatures:%s\n", ColorCyan, ColorReset)
	fmt.Printf("  %s✓%s Parallel chunk downloads\n", ColorGreen, ColorReset)
	fmt.Printf("  %s✓%s HTTP/HTTPS support with HTTP/2\n", ColorGreen, ColorReset)
	fmt.Printf("  %s✓%s Resume capability\n", ColorGreen, ColorReset)
	fmt.Printf("  %s✓%s SHA-256/SHA-1/MD5 verification\n", ColorGreen, ColorReset)
	fmt.Printf("  %s✓%s Batch downloads\n", ColorGreen, ColorReset)
	fmt.Printf("  %s✓%s Rate limiting\n", ColorGreen, ColorReset)
	fmt.Printf("  %s✓%s Proxy support\n", ColorGreen, ColorReset)
	fmt.Printf("  %s✓%s Mirror/fallback support\n", ColorGreen, ColorReset)
	fmt.Printf("  %s✓%s Job queue with persistence\n", ColorGreen, ColorReset)
	fmt.Printf("  %s✓%s Daemon mode with Web UI\n", ColorGreen, ColorReset)
	fmt.Printf("  %s✓%s RESTful API\n", ColorGreen, ColorReset)
	fmt.Printf("  %s✓%s TUI interface\n", ColorGreen, ColorReset)
	fmt.Printf("  %s✓%s Configuration management\n", ColorGreen, ColorReset)
	
	fmt.Printf("\n%sProtocols:%s\n", ColorCyan, ColorReset)
	fmt.Printf("  • HTTP/HTTPS\n")
	fmt.Printf("  • HTTP/2\n")
	fmt.Printf("  • FTP (planned)\n")
	fmt.Printf("  • BitTorrent (planned)\n")
}

func printUsage() {
	fmt.Printf("%s╔══════════════════════════════════════════════════════╗%s\n", ColorGreen, ColorReset)
	fmt.Printf("%s║       FastDL v%s - High-Performance Downloader  ║%s\n", ColorGreen, Version, ColorReset)
	fmt.Printf("%s╚══════════════════════════════════════════════════════╝%s\n\n", ColorGreen, ColorReset)
	
	fmt.Printf("%sUsage:%s fastdl <command> [options]\n\n", ColorCyan, ColorReset)
	
	fmt.Printf("%sCommands:%s\n", ColorYellow, ColorReset)
	fmt.Printf("  %sdownload%s    Download a single file\n", ColorWhite, ColorReset)
	fmt.Printf("  %sbatch%s       Download multiple files from URL list\n", ColorWhite, ColorReset)
	fmt.Printf("  %sdaemon%s      Start daemon with Web UI\n", ColorWhite, ColorReset)
	fmt.Printf("  %stui%s         Interactive TUI mode\n", ColorWhite, ColorReset)
	fmt.Printf("  %sconfig%s      Manage configuration\n", ColorWhite, ColorReset)
	fmt.Printf("  %sverify%s      Verify file checksum\n", ColorWhite, ColorReset)
	fmt.Printf("  %sinfo%s        Show system information\n", ColorWhite, ColorReset)
	fmt.Printf("  %shelp%s        Show this help message\n", ColorWhite, ColorReset)
	
	fmt.Printf("\n%sExamples:%s\n", ColorYellow, ColorReset)
	fmt.Printf("  fastdl download -c 32 -o output.zip https://example.com/file.zip\n")
	fmt.Printf("  fastdl batch -c 4 urls.txt\n")
	fmt.Printf("  fastdl daemon -port 8080\n")
	fmt.Printf("  fastdl tui\n")
	fmt.Printf("  fastdl config -set max_connections=64\n")
	fmt.Printf("  fastdl verify file.zip abc123...\n")
	
	fmt.Printf("\n%sQuick Start:%s\n", ColorYellow, ColorReset)
	fmt.Printf("  1. Run 'fastdl tui' for interactive mode\n")
	fmt.Printf("  2. Run 'fastdl daemon' to start Web UI at http://localhost:8080\n")
	fmt.Printf("  3. Run 'fastdl config -edit' to configure settings\n")
	
	fmt.Printf("\n%sRun 'fastdl <command> -h' for command-specific help%s\n", ColorCyan, ColorReset)
}

func main() {
	// Initialize global configuration
	var err error
	globalConfig, err = loadConfig("")
	if err != nil {
		log.Fatal(err)
	}

	if len(os.Args) < 2 {
		// If no arguments, start TUI mode
		cmdTUI([]string{})
		return
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "download", "d", "get":
		cmdDownload(args)
	case "batch", "b":
		cmdBatch(args)
	case "daemon", "server":
		cmdDaemon(args)
	case "tui", "ui":
		cmdTUI(args)
	case "config", "cfg":
		cmdConfig(args)
	case "verify", "v", "check":
		cmdVerify(args)
	case "info", "i", "about":
		cmdInfo()
	case "help", "h", "-h", "--help":
		printUsage()
	case "version", "-v", "--version":
		fmt.Printf("FastDL v%s\n", Version)
	default:
		fmt.Printf("%sUnknown command: %s%s\n\n", ColorRed, command, ColorReset)
		printUsage()
		os.Exit(1)
	}
}
