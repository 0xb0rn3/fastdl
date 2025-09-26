// FastDL Core Engine - High-performance cross-platform downloader
// Fixed version with proper error handling for thread safety

use std::env;
use std::fs::File;
use std::io::{self, Write, Seek, SeekFrom};
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Arc;
use std::time::{Duration, Instant};
use std::collections::HashMap;

use reqwest::Client;
use serde::{Deserialize, Serialize};
use tokio::fs::File as AsyncFile;
use tokio::io::{AsyncSeekExt, AsyncWriteExt};
use tokio::sync::{Semaphore, Mutex};
use tokio::time::sleep;
use futures_util::StreamExt;

// Custom error type that implements Send + Sync
#[derive(Debug)]
pub struct DownloadError {
    message: String,
}

impl std::fmt::Display for DownloadError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", self.message)
    }
}

impl std::error::Error for DownloadError {}

impl From<reqwest::Error> for DownloadError {
    fn from(err: reqwest::Error) -> Self {
        DownloadError {
            message: format!("Network error: {}", err),
        }
    }
}

impl From<std::io::Error> for DownloadError {
    fn from(err: std::io::Error) -> Self {
        DownloadError {
            message: format!("IO error: {}", err),
        }
    }
}

impl From<serde_json::Error> for DownloadError {
    fn from(err: serde_json::Error) -> Self {
        DownloadError {
            message: format!("JSON error: {}", err),
        }
    }
}

impl From<url::ParseError> for DownloadError {
    fn from(err: url::ParseError) -> Self {
        DownloadError {
            message: format!("URL parse error: {}", err),
        }
    }
}

impl From<tokio::time::error::Elapsed> for DownloadError {
    fn from(err: tokio::time::error::Elapsed) -> Self {
        DownloadError {
            message: format!("Timeout error: {}", err),
        }
    }
}

impl From<String> for DownloadError {
    fn from(message: String) -> Self {
        DownloadError { message }
    }
}

impl From<&str> for DownloadError {
    fn from(message: &str) -> Self {
        DownloadError {
            message: message.to_string(),
        }
    }
}

// Configuration structure - received from wrapper scripts
#[derive(Debug, Deserialize)]
pub struct DownloadConfig {
    pub urls: Vec<String>,
    pub output_dir: String,
    pub connections: usize,
    pub chunk_size_mb: usize,
    pub timeout_seconds: u64,
    pub retries: usize,
    pub max_concurrent: usize,
    pub url_file: Option<String>,
    pub verbose: bool,
}

// Progress information sent back to wrapper
#[derive(Debug, Serialize)]
pub struct DownloadProgress {
    pub url: String,
    pub filename: String,
    pub total_size: u64,
    pub downloaded: u64,
    pub speed_mbps: f64,
    pub eta_seconds: u64,
    pub status: String,
}

// Final download result
#[derive(Debug, Serialize)]
pub struct DownloadResult {
    pub url: String,
    pub filename: String,
    pub success: bool,
    pub error: Option<String>,
    pub total_time_seconds: f64,
    pub average_speed_mbps: f64,
    pub file_size: u64,
}

// Chunk information for multi-threaded downloading
#[derive(Debug, Clone)]
pub struct ChunkInfo {
    pub start: u64,
    pub end: u64,
    pub size: u64,
    pub completed: bool,
    pub retries: usize,
}

// Statistics tracking for each download with thread-safe updates
pub struct DownloadStats {
    pub total_size: AtomicU64,
    pub downloaded: AtomicU64,
    pub start_time: Instant,
    pub chunks_completed: AtomicU64,
    pub chunks_total: AtomicU64,
}

impl DownloadStats {
    pub fn new() -> Self {
        Self {
            total_size: AtomicU64::new(0),
            downloaded: AtomicU64::new(0),
            start_time: Instant::now(),
            chunks_completed: AtomicU64::new(0),
            chunks_total: AtomicU64::new(0),
        }
    }

    // Calculate current download speed in MB/s
    pub fn speed_mbps(&self) -> f64 {
        let elapsed = self.start_time.elapsed().as_secs_f64();
        if elapsed > 0.0 {
            let downloaded_mb = self.downloaded.load(Ordering::Relaxed) as f64 / (1024.0 * 1024.0);
            downloaded_mb / elapsed
        } else {
            0.0
        }
    }

    // Estimate time remaining in seconds
    pub fn eta_seconds(&self) -> u64 {
        let speed = self.speed_mbps();
        let remaining_mb = (self.total_size.load(Ordering::Relaxed) - self.downloaded.load(Ordering::Relaxed)) as f64 / (1024.0 * 1024.0);
        if speed > 0.0 {
            (remaining_mb / speed) as u64
        } else {
            0
        }
    }

    // Get completion percentage
    pub fn completion_percentage(&self) -> f64 {
        let total = self.total_size.load(Ordering::Relaxed);
        if total > 0 {
            (self.downloaded.load(Ordering::Relaxed) as f64 / total as f64) * 100.0
        } else {
            0.0
        }
    }
}

pub struct FastDownloader {
    client: Client,
    config: DownloadConfig,
    semaphore: Arc<Semaphore>, // Controls concurrent connections
}

impl FastDownloader {
    pub fn new(config: DownloadConfig) -> Result<Self, DownloadError> {
        // Create optimized HTTP client with connection pooling
        let client = Client::builder()
            .timeout(Duration::from_secs(config.timeout_seconds))
            .user_agent("FastDL-Core/1.0 (High-Performance Downloader)")
            .pool_max_idle_per_host(config.connections)
            .pool_idle_timeout(Duration::from_secs(30))
            .tcp_keepalive(Duration::from_secs(60))
            .build()?;

        // Create semaphore to limit concurrent connections
        let semaphore = Arc::new(Semaphore::new(config.connections));

        Ok(Self { client, config, semaphore })
    }

    // Extract filename from URL with better handling
    fn extract_filename(&self, url: &str) -> String {
        if let Ok(parsed_url) = url::Url::parse(url) {
            if let Some(segments) = parsed_url.path_segments() {
                if let Some(last_segment) = segments.last() {
                    if !last_segment.is_empty() {
                        let decoded = urlencoding::decode(last_segment).unwrap_or_default();
                        let filename = decoded.to_string();
                        // Remove query parameters if present
                        if let Some(clean_name) = filename.split('?').next() {
                            if !clean_name.is_empty() {
                                return clean_name.to_string();
                            }
                        }
                    }
                }
            }
        }

        // Generate a meaningful name based on URL and timestamp
        let url_hash = url.chars().fold(0u32, |acc, c| acc.wrapping_add(c as u32));
        format!("download_{}_{}", url_hash, std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap_or_default()
            .as_secs())
    }

    // Get file information with enhanced error handling
    async fn get_file_info(&self, url: &str) -> Result<(u64, String, bool), DownloadError> {
        let mut retries = 0;
        let max_retries = 3;

        loop {
            match self.client.head(url).send().await {
                Ok(response) => {
                    if !response.status().is_success() {
                        return Err(format!("HTTP error: {}", response.status()).into());
                    }

                    // Get file size
                    let file_size = response
                        .headers()
                        .get("content-length")
                        .and_then(|v| v.to_str().ok())
                        .and_then(|v| v.parse::<u64>().ok())
                        .unwrap_or(0);

                    // Extract filename
                    let filename = self.extract_filename(url);

                    // Check range support
                    let supports_ranges = response.headers()
                        .get("accept-ranges")
                        .and_then(|v| v.to_str().ok())
                        .map(|v| v.to_lowercase())
                        .as_deref() == Some("bytes");

                    return Ok((file_size, filename, supports_ranges));
                }
                Err(e) => {
                    retries += 1;
                    if retries >= max_retries {
                        return Err(format!("Failed to get file info after {} retries: {}", max_retries, e).into());
                    }
                    
                    if self.config.verbose {
                        println!("Retry {}/{} for file info: {}", retries, max_retries, e);
                    }
                    
                    // Exponential backoff
                    sleep(Duration::from_millis(1000 * (1 << retries))).await;
                }
            }
        }
    }

    // Create chunks for multi-threaded downloading
    fn create_chunks(&self, file_size: u64, supports_ranges: bool) -> Vec<ChunkInfo> {
        if !supports_ranges || file_size == 0 {
            // Single chunk for servers that don't support ranges
            return vec![ChunkInfo {
                start: 0,
                end: file_size.saturating_sub(1),
                size: file_size,
                completed: false,
                retries: 0,
            }];
        }

        let chunk_size = (self.config.chunk_size_mb * 1024 * 1024) as u64;
        let num_chunks = std::cmp::min(
            self.config.connections as u64,
            (file_size + chunk_size - 1) / chunk_size
        );

        let mut chunks = Vec::new();
        let chunk_size_actual = file_size / num_chunks;
        let remainder = file_size % num_chunks;

        for i in 0..num_chunks {
            let start = i * chunk_size_actual;
            let mut end = start + chunk_size_actual - 1;
            
            // Add remainder to the last chunk
            if i == num_chunks - 1 {
                end += remainder;
            }

            chunks.push(ChunkInfo {
                start,
                end,
                size: end - start + 1,
                completed: false,
                retries: 0,
            });
        }

        chunks
    }

    // Download a single chunk with retry logic
    async fn download_chunk(
        &self,
        url: &str,
        chunk: ChunkInfo,
        file_path: &PathBuf,
        stats: Arc<DownloadStats>,
    ) -> Result<ChunkInfo, DownloadError> {
        let mut current_chunk = chunk;
        
        // Retry loop for this chunk
        while current_chunk.retries < self.config.retries {
            // Acquire semaphore permit to limit concurrent connections
            let _permit = self.semaphore.acquire().await
                .map_err(|e| DownloadError::from(format!("Semaphore error: {}", e)))?;
            
            match self.download_chunk_attempt(url, &current_chunk, file_path, stats.clone()).await {
                Ok(_) => {
                    current_chunk.completed = true;
                    stats.chunks_completed.fetch_add(1, Ordering::Relaxed);
                    return Ok(current_chunk);
                }
                Err(e) => {
                    current_chunk.retries += 1;
                    if self.config.verbose {
                        println!("Chunk {}-{} failed (attempt {}): {}", 
                            current_chunk.start, current_chunk.end, current_chunk.retries, e);
                    }
                    
                    if current_chunk.retries < self.config.retries {
                        // Exponential backoff with jitter
                        let delay = Duration::from_millis(500 * (1 << current_chunk.retries) + 
                            (fastrand::u64(0..1000)));
                        sleep(delay).await;
                    }
                }
            }
        }

        Err(format!("Chunk {}-{} failed after {} retries", 
            current_chunk.start, current_chunk.end, self.config.retries).into())
    }

    // Single attempt to download a chunk
    async fn download_chunk_attempt(
        &self,
        url: &str,
        chunk: &ChunkInfo,
        file_path: &PathBuf,
        stats: Arc<DownloadStats>,
    ) -> Result<(), DownloadError> {
        // Create range request
        let mut request = self.client.get(url);
        
        if chunk.start > 0 || chunk.end < chunk.start + chunk.size {
            request = request.header("Range", format!("bytes={}-{}", chunk.start, chunk.end));
        }

        // Send request with per-chunk timeout
        let response = tokio::time::timeout(
            Duration::from_secs(self.config.timeout_seconds),
            request.send()
        ).await??;

        if !response.status().is_success() && response.status().as_u16() != 206 {
            return Err(format!("HTTP error: {}", response.status()).into());
        }

        // Open file for writing at the specific position
        let mut file = std::fs::OpenOptions::new()
            .create(true)
            .write(true)
            .open(file_path)?;
        
        file.seek(SeekFrom::Start(chunk.start))?;

        // Stream the chunk data
        let mut stream = response.bytes_stream();
        let mut chunk_downloaded = 0u64;

        while let Some(chunk_result) = stream.next().await {
            let data = chunk_result?;
            file.write_all(&data)?;
            
            let bytes_written = data.len() as u64;
            chunk_downloaded += bytes_written;
            stats.downloaded.fetch_add(bytes_written, Ordering::Relaxed);

            // Progress reporting for verbose mode
            if self.config.verbose && chunk_downloaded % (256 * 1024) == 0 {
                let progress = stats.completion_percentage();
                let speed = stats.speed_mbps();
                let eta = stats.eta_seconds();
                print!("\rProgress: {:.1}% | Speed: {:.2} MB/s | ETA: {}s", 
                    progress, speed, eta);
                io::stdout().flush().ok();
            }
        }

        file.flush()?;
        Ok(())
    }

    // Multi-threaded download with proper chunk management
    async fn download_multithread(
        &self,
        url: &str,
        file_path: &PathBuf,
        file_size: u64,
        supports_ranges: bool,
        stats: Arc<DownloadStats>,
    ) -> Result<(), DownloadError> {
        // Create chunks for parallel downloading
        let chunks = self.create_chunks(file_size, supports_ranges);
        stats.chunks_total.store(chunks.len() as u64, Ordering::Relaxed);

        if self.config.verbose {
            println!("Using {} chunks for parallel download", chunks.len());
        }

        // Create the output file
        if file_size > 0 {
            let file = std::fs::File::create(file_path)?;
            file.set_len(file_size)?;
        }

        // Download chunks concurrently
        let mut handles = Vec::new();
        
        for chunk in chunks {
            let url = url.to_string();
            let file_path = file_path.clone();
            let stats = stats.clone();
            let downloader = self.clone();

            let handle = tokio::spawn(async move {
                downloader.download_chunk(&url, chunk, &file_path, stats).await
            });
            
            handles.push(handle);
        }

        // Wait for all chunks to complete
        let mut all_success = true;
        let mut error_messages = Vec::new();

        for handle in handles {
            match handle.await {
                Ok(Ok(_)) => {
                    // Chunk completed successfully
                }
                Ok(Err(e)) => {
                    all_success = false;
                    error_messages.push(e.to_string());
                }
                Err(e) => {
                    all_success = false;
                    error_messages.push(format!("Task error: {}", e));
                }
            }
        }

        if !all_success {
            return Err(format!("Some chunks failed: {}", error_messages.join("; ")).into());
        }

        Ok(())
    }

    // Fallback single-stream download for servers without range support
    async fn download_single_stream(
        &self,
        url: &str,
        file_path: &PathBuf,
        stats: Arc<DownloadStats>,
    ) -> Result<(), DownloadError> {
        let response = self.client.get(url).send().await?;
        
        if !response.status().is_success() {
            return Err(format!("HTTP error: {}", response.status()).into());
        }

        let mut file = std::fs::File::create(file_path)?;
        let mut stream = response.bytes_stream();

        while let Some(chunk_result) = stream.next().await {
            let chunk = chunk_result?;
            file.write_all(&chunk)?;
            stats.downloaded.fetch_add(chunk.len() as u64, Ordering::Relaxed);

            // Progress reporting
            if self.config.verbose {
                let downloaded = stats.downloaded.load(Ordering::Relaxed);
                let total = stats.total_size.load(Ordering::Relaxed);
                if total > 0 {
                    let percent = (downloaded as f64 / total as f64) * 100.0;
                    let speed = stats.speed_mbps();
                    print!("\rProgress: {:.1}% | Speed: {:.2} MB/s", percent, speed);
                    io::stdout().flush().ok();
                }
            }
        }

        file.flush()?;
        if self.config.verbose {
            println!(); // New line after progress
        }
        Ok(())
    }

    // Main download function for a single file
    pub async fn download_file(&self, url: &str) -> DownloadResult {
        let start_time = Instant::now();
        
        if self.config.verbose {
            println!("Analyzing: {}", url);
        }

        // Get file information
        let (file_size, filename, supports_ranges) = match self.get_file_info(url).await {
            Ok(info) => info,
            Err(e) => {
                return DownloadResult {
                    url: url.to_string(),
                    filename: "unknown".to_string(),
                    success: false,
                    error: Some(format!("Failed to get file info: {}", e)),
                    total_time_seconds: start_time.elapsed().as_secs_f64(),
                    average_speed_mbps: 0.0,
                    file_size: 0,
                };
            }
        };

        let output_path = PathBuf::from(&self.config.output_dir).join(&filename);
        
        // Create output directory if needed
        if let Some(parent) = output_path.parent() {
            if let Err(e) = std::fs::create_dir_all(parent) {
                return DownloadResult {
                    url: url.to_string(),
                    filename,
                    success: false,
                    error: Some(format!("Failed to create output directory: {}", e)),
                    total_time_seconds: start_time.elapsed().as_secs_f64(),
                    average_speed_mbps: 0.0,
                    file_size,
                };
            }
        }

        let stats = Arc::new(DownloadStats::new());
        stats.total_size.store(file_size, Ordering::Relaxed);

        if self.config.verbose {
            println!("Downloading: {} -> {}", filename, output_path.display());
            if file_size > 0 {
                println!("File size: {} bytes", file_size);
            }
            println!("Range support: {}", if supports_ranges { "Yes" } else { "No" });
        }

        // Choose download strategy based on range support and file size
        let result = if supports_ranges && file_size > 1024 * 1024 && self.config.connections > 1 {
            // Multi-threaded download for large files with range support
            self.download_multithread(url, &output_path, file_size, supports_ranges, stats.clone()).await
        } else {
            // Single-stream download for small files or servers without range support
            self.download_single_stream(url, &output_path, stats.clone()).await
        };

        let total_time = start_time.elapsed().as_secs_f64();
        let downloaded = stats.downloaded.load(Ordering::Relaxed);
        let avg_speed = if total_time > 0.0 {
            (downloaded as f64 / (1024.0 * 1024.0)) / total_time
        } else {
            0.0
        };

        match result {
            Ok(_) => {
                if self.config.verbose {
                    println!("✓ Download completed: {}", filename);
                    println!("  Time: {:.2}s | Speed: {:.2} MB/s", total_time, avg_speed);
                }
                DownloadResult {
                    url: url.to_string(),
                    filename,
                    success: true,
                    error: None,
                    total_time_seconds: total_time,
                    average_speed_mbps: avg_speed,
                    file_size: downloaded,
                }
            }
            Err(e) => {
                if self.config.verbose {
                    println!("✗ Download failed: {}", e);
                }
                // Clean up partial file
                let _ = std::fs::remove_file(&output_path);
                DownloadResult {
                    url: url.to_string(),
                    filename,
                    success: false,
                    error: Some(e.to_string()),
                    total_time_seconds: total_time,
                    average_speed_mbps: avg_speed,
                    file_size: downloaded,
                }
            }
        }
    }

    // Download multiple files with controlled concurrency
    pub async fn download_batch(&self, urls: Vec<String>) -> Vec<DownloadResult> {
        let semaphore = Arc::new(Semaphore::new(self.config.max_concurrent));
        let mut handles = Vec::new();
        
        for url in urls {
            let semaphore = semaphore.clone();
            let downloader = self.clone();
            
            let handle = tokio::spawn(async move {
                let _permit = semaphore.acquire().await.unwrap();
                downloader.download_file(&url).await
            });
            
            handles.push(handle);
        }

        let mut results = Vec::new();
        for handle in handles {
            match handle.await {
                Ok(result) => results.push(result),
                Err(e) => {
                    results.push(DownloadResult {
                        url: "unknown".to_string(),
                        filename: "unknown".to_string(),
                        success: false,
                        error: Some(format!("Task error: {}", e)),
                        total_time_seconds: 0.0,
                        average_speed_mbps: 0.0,
                        file_size: 0,
                    });
                }
            }
        }

        results
    }
}

// Implement Clone for FastDownloader to enable sharing across tasks
impl Clone for FastDownloader {
    fn clone(&self) -> Self {
        Self {
            client: self.client.clone(),
            config: DownloadConfig {
                urls: self.config.urls.clone(),
                output_dir: self.config.output_dir.clone(),
                connections: self.config.connections,
                chunk_size_mb: self.config.chunk_size_mb,
                timeout_seconds: self.config.timeout_seconds,
                retries: self.config.retries,
                max_concurrent: self.config.max_concurrent,
                url_file: self.config.url_file.clone(),
                verbose: self.config.verbose,
            },
            semaphore: self.semaphore.clone(),
        }
    }
}

// Simple random number generator for jitter in retry delays
mod fastrand {
    use std::collections::hash_map::DefaultHasher;
    use std::hash::{Hash, Hasher};
    use std::sync::atomic::{AtomicU64, Ordering};
    
    static SEED: AtomicU64 = AtomicU64::new(1);
    
    pub fn u64(range: std::ops::Range<u64>) -> u64 {
        let mut hasher = DefaultHasher::new();
        std::thread::current().id().hash(&mut hasher);
        std::time::SystemTime::now().duration_since(std::time::UNIX_EPOCH)
            .unwrap_or_default().as_nanos().hash(&mut hasher);
        SEED.fetch_add(1, Ordering::Relaxed).hash(&mut hasher);
        
        let hash = hasher.finish();
        range.start + (hash % (range.end - range.start))
    }
}

// Main entry point
#[tokio::main]
async fn main() -> Result<(), DownloadError> {
    let args: Vec<String> = env::args().collect();
    
    if args.len() < 2 {
        eprintln!("Usage: fastdl-core <config-json>");
        eprintln!("Example: fastdl-core '{{\"urls\":[\"https://example.com/file.zip\"],\"output_dir\":\"./downloads\",\"connections\":8,\"chunk_size_mb\":1,\"timeout_seconds\":30,\"retries\":3,\"max_concurrent\":3,\"verbose\":true}}'");
        std::process::exit(1);
    }

    // Parse configuration from JSON argument
    let config: DownloadConfig = serde_json::from_str(&args[1])?;

    let downloader = FastDownloader::new(config)?;

    // Determine what to download
    let urls = if let Some(url_file) = &downloader.config.url_file {
        // Read URLs from file
        let content = std::fs::read_to_string(url_file)
            .map_err(|e| DownloadError::from(format!("Failed to read URL file: {}", e)))?;
        content.lines()
            .map(|line| line.trim().to_string())
            .filter(|line| !line.is_empty() && !line.starts_with('#'))
            .collect()
    } else {
        downloader.config.urls.clone()
    };

    if urls.is_empty() {
        eprintln!("No URLs to download");
        std::process::exit(1);
    }

    // Execute downloads
    let results = if urls.len() == 1 {
        vec![downloader.download_file(&urls[0]).await]
    } else {
        downloader.download_batch(urls).await
    };

    // Output final results as JSON
    println!("{}", serde_json::to_string_pretty(&results)?);

    // Exit with appropriate code
    let success_count = results.iter().filter(|r| r.success).count();
    if success_count == results.len() {
        std::process::exit(0);
    } else {
        std::process::exit(1);
    }
}
