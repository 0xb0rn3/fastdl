// FastDL Core Engine - High-performance cross-platform downloader
// This is the Rust core that handles the actual downloading logic
// The wrapper scripts (bash/PowerShell) communicate with this via JSON

use std::env;
use std::fs::File;
use std::io::{self, Write};
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Arc;
use std::time::{Duration, Instant};

use reqwest::Client;
use serde::{Deserialize, Serialize};
use tokio::fs::File as AsyncFile;
use tokio::io::{AsyncSeekExt, AsyncWriteExt};
use futures_util::StreamExt;

// Configuration structure - this is what the wrapper scripts send to us as JSON
// The wrapper script builds this configuration and passes it as a command line argument
#[derive(Debug, Deserialize)]
pub struct DownloadConfig {
    pub urls: Vec<String>,                    // List of URLs to download
    pub output_dir: String,                   // Where to save files
    pub connections: usize,                   // Number of connections per file (not fully implemented)
    pub chunk_size_mb: usize,                // Size of chunks in MB (not fully implemented)
    pub timeout_seconds: u64,                // HTTP timeout
    pub retries: usize,                      // Number of retry attempts (not fully implemented)
    pub max_concurrent: usize,               // Max concurrent downloads (not fully implemented)
    pub url_file: Option<String>,            // Optional file containing URLs
    pub verbose: bool,                       // Whether to show detailed progress
}

// Progress information that could be sent back to wrapper (currently just for structure)
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

// Final result for each download - this gets output as JSON at the end
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

// Thread-safe statistics tracking for each download
// Uses atomic operations so multiple threads can safely update the same counters
pub struct DownloadStats {
    pub total_size: AtomicU64,     // Total file size in bytes
    pub downloaded: AtomicU64,     // Bytes downloaded so far
    pub start_time: Instant,       // When the download started
}

impl DownloadStats {
    pub fn new() -> Self {
        Self {
            total_size: AtomicU64::new(0),
            downloaded: AtomicU64::new(0),
            start_time: Instant::now(),
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
}

// Main downloader struct - contains the HTTP client and configuration
pub struct FastDownloader {
    client: Client,
    config: DownloadConfig,
}

impl FastDownloader {
    // Create a new downloader with optimized HTTP client settings
    pub fn new(config: DownloadConfig) -> Result<Self, Box<dyn std::error::Error>> {
        // Create HTTP client with optimizations for file downloading
        let client = Client::builder()
            .timeout(Duration::from_secs(config.timeout_seconds))  // Set request timeout
            .user_agent("FastDL-Core/1.0")                        // Identify ourselves
            .build()?;

        Ok(Self { client, config })
    }

    // Extract a meaningful filename from a URL
    // This handles URL decoding and falls back to timestamp-based names
    fn extract_filename(&self, url: &str) -> String {
        if let Ok(parsed_url) = url::Url::parse(url) {
            if let Some(segments) = parsed_url.path_segments() {
                if let Some(last_segment) = segments.last() {
                    if !last_segment.is_empty() {
                        // Decode URL encoding (like %20 for spaces)
                        return urlencoding::decode(last_segment).unwrap_or_default().to_string();
                    }
                }
            }
        }

        // If we can't extract a filename, generate one based on current time
        format!("download_{}", std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap_or_default()
            .as_secs())
    }

    // Get file information by sending a HEAD request
    // This tells us the file size without downloading the whole file
    async fn get_file_info(&self, url: &str) -> Result<(u64, String), Box<dyn std::error::Error>> {
        let response = self.client.head(url).send().await?;
        
        // Extract file size from Content-Length header
        let file_size = response
            .headers()
            .get("content-length")
            .and_then(|v| v.to_str().ok())          // Convert header value to string
            .and_then(|v| v.parse::<u64>().ok())    // Parse as number
            .unwrap_or(0);                          // Default to 0 if not found

        // Get filename from URL
        let filename = self.extract_filename(url);

        Ok((file_size, filename))
    }

    // Simple single-stream download function
    // This is the core downloading logic - it streams the file in chunks
    async fn download_simple(
        &self,
        url: &str,
        file_path: &PathBuf,
        stats: Arc<DownloadStats>,
    ) -> Result<(), Box<dyn std::error::Error>> {
        // Start the download request
        let response = self.client.get(url).send().await?;
        
        // Check if the server responded successfully
        if !response.status().is_success() {
            return Err(format!("HTTP error: {}", response.status()).into());
        }

        // Create the output file
        let mut file = AsyncFile::create(file_path).await?;
        
        // Get a stream of bytes from the response
        let mut stream = response.bytes_stream();

        // Process each chunk as it arrives
        while let Some(chunk_result) = stream.next().await {
            let chunk = chunk_result?;                                    // Handle any network errors
            file.write_all(&chunk).await?;                               // Write chunk to file
            stats.downloaded.fetch_add(chunk.len() as u64, Ordering::Relaxed);  // Update progress counter

            // Show progress if verbose mode is enabled
            if self.config.verbose {
                let downloaded = stats.downloaded.load(Ordering::Relaxed);
                let total = stats.total_size.load(Ordering::Relaxed);
                if total > 0 {
                    let percent = (downloaded as f64 / total as f64) * 100.0;
                    let speed = stats.speed_mbps();
                    // Use \r to overwrite the same line for a dynamic progress display
                    print!("\rProgress: {:.1}% | Speed: {:.2} MB/s", percent, speed);
                    io::stdout().flush().ok();  // Make sure the output appears immediately
                }
            }
        }

        // Ensure all data is written to disk
        file.flush().await?;
        if self.config.verbose {
            println!(); // Move to next line after progress display
        }
        Ok(())
    }

    // Main function to download a single file - this orchestrates the whole process
    pub async fn download_file(&self, url: &str) -> DownloadResult {
        let start_time = Instant::now();
        
        if self.config.verbose {
            println!("Analyzing: {}", url);
        }

        // First, get information about the file we're downloading
        let (file_size, filename) = match self.get_file_info(url).await {
            Ok(info) => info,
            Err(e) => {
                // If we can't get file info, return an error result
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

        // Build the full path where we'll save the file
        let output_path = PathBuf::from(&self.config.output_dir).join(&filename);
        
        // Create the output directory if it doesn't exist
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

        // Set up statistics tracking
        let stats = Arc::new(DownloadStats::new());
        stats.total_size.store(file_size, Ordering::Relaxed);

        if self.config.verbose {
            println!("Downloading: {} -> {}", filename, output_path.display());
            if file_size > 0 {
                println!("File size: {} bytes", file_size);
            }
        }

        // Actually perform the download
        let result = self.download_simple(url, &output_path, stats.clone()).await;

        // Calculate final statistics
        let total_time = start_time.elapsed().as_secs_f64();
        let downloaded = stats.downloaded.load(Ordering::Relaxed);
        let avg_speed = if total_time > 0.0 {
            (downloaded as f64 / (1024.0 * 1024.0)) / total_time
        } else {
            0.0
        };

        // Return the appropriate result based on success or failure
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
                // Clean up partial file on failure
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

    // Download multiple files sequentially
    // Note: This is currently sequential, not concurrent - room for improvement
    pub async fn download_batch(&self, urls: Vec<String>) -> Vec<DownloadResult> {
        let mut results = Vec::new();
        
        for url in urls {
            let result = self.download_file(&url).await;
            results.push(result);
        }

        results
    }
}

// Main entry point - this is what gets called when the binary is executed
#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let args: Vec<String> = env::args().collect();
    
    // We expect exactly one argument: the JSON configuration
    if args.len() < 2 {
        eprintln!("Usage: fastdl-core <config-json>");
        eprintln!("Example: fastdl-core '{{\"urls\":[\"https://example.com/file.zip\"],\"output_dir\":\"./downloads\",\"connections\":8,\"chunk_size_mb\":1,\"timeout_seconds\":30,\"retries\":3,\"max_concurrent\":3,\"verbose\":true}}'");
        std::process::exit(1);
    }

    // Parse the JSON configuration from the command line argument
    let config: DownloadConfig = serde_json::from_str(&args[1])
        .map_err(|e| format!("Invalid JSON config: {}", e))?;

    // Create the downloader with this configuration
    let downloader = FastDownloader::new(config)?;

    // Determine what URLs to download
    let urls = if let Some(url_file) = &downloader.config.url_file {
        // If a URL file is specified, read URLs from that file
        let content = std::fs::read_to_string(url_file)?;
        content.lines()
            .map(|line| line.trim().to_string())              // Remove whitespace
            .filter(|line| !line.is_empty() && !line.starts_with('#'))  // Skip empty lines and comments
            .collect()
    } else {
        // Otherwise, use the URLs provided directly in the config
        downloader.config.urls.clone()
    };

    // Make sure we have something to download
    if urls.is_empty() {
        eprintln!("No URLs to download");
        std::process::exit(1);
    }

    // Execute the downloads
    let results = if urls.len() == 1 {
        // Single file - call download_file directly
        vec![downloader.download_file(&urls[0]).await]
    } else {
        // Multiple files - use batch download
        downloader.download_batch(urls).await
    };

    // Output the results as JSON for the wrapper script to parse
    println!("{}", serde_json::to_string_pretty(&results)?);

    // Set exit code based on success/failure
    let success_count = results.iter().filter(|r| r.success).count();
    if success_count == results.len() {
        std::process::exit(0);  // All downloads succeeded
    } else {
        std::process::exit(1);  // At least one download failed
    }
}
