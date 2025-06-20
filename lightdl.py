#!/usr/bin/env python3
"""
Lightning Fast CLI Downloader
A high-performance downloader with concurrent connections, resume capability, and batch processing.
"""

import asyncio
import aiohttp
import aiofiles
import argparse
import sys
import os
import time
from pathlib import Path
from urllib.parse import urlparse, unquote
from typing import List, Optional, Tuple
import json
from dataclasses import dataclass
from concurrent.futures import ThreadPoolExecutor
import signal
import readline  # For better input handling

@dataclass
class DownloadStats:
    """Track download statistics for progress reporting"""
    total_size: int = 0
    downloaded: int = 0
    start_time: float = 0
    chunks_completed: int = 0
    
    @property
    def speed(self) -> float:
        """Calculate current download speed in MB/s"""
        elapsed = time.time() - self.start_time
        if elapsed > 0:
            return (self.downloaded / (1024 * 1024)) / elapsed
        return 0
    
    @property
    def progress_percent(self) -> float:
        """Calculate download progress percentage"""
        if self.total_size > 0:
            return (self.downloaded / self.total_size) * 100
        return 0

class InteractiveMenu:
    """Interactive CLI menu system for configuring downloads without command-line complexity"""
    
    def __init__(self):
        self.config = {
            'urls': [],
            'output_dir': str(Path.home() / 'Downloads'),  # Default to user's Downloads folder
            'connections': 8,
            'chunk_size': 1,
            'timeout': 30,
            'retries': 3,
            'max_concurrent': 3,
            'url_file': None
        }
        self.history_file = Path.home() / '.fastdl_history'
        self._load_history()
    
    def _load_history(self):
        """Load previous download settings for convenience"""
        try:
            if self.history_file.exists():
                with open(self.history_file, 'r') as f:
                    saved_config = json.load(f)
                    # Only load non-URL settings to avoid accidentally redownloading
                    for key in ['output_dir', 'connections', 'chunk_size', 'timeout', 'retries', 'max_concurrent']:
                        if key in saved_config:
                            self.config[key] = saved_config[key]
        except Exception:
            pass  # Ignore errors loading history
    
    def _save_history(self):
        """Save current settings for next time (excluding URLs for privacy)"""
        try:
            save_config = {k: v for k, v in self.config.items() if k != 'urls' and k != 'url_file'}
            with open(self.history_file, 'w') as f:
                json.dump(save_config, f, indent=2)
        except Exception:
            pass  # Ignore errors saving history
    
    def _clear_screen(self):
        """Clear screen for cleaner navigation experience"""
        os.system('clear' if os.name == 'posix' else 'cls')
    
    def _print_header(self):
        """Display the application header with current configuration"""
        print("=" * 80)
        print("🚀 LIGHTNING FAST CLI DOWNLOADER".center(80))
        print("=" * 80)
        print(f"📁 Output Directory: {self.config['output_dir']}")
        print(f"🔗 Connections per file: {self.config['connections']} | Chunk size: {self.config['chunk_size']}MB")
        print(f"⏱️  Timeout: {self.config['timeout']}s | Retries: {self.config['retries']} | Concurrent: {self.config['max_concurrent']}")
        
        if self.config['urls']:
            print(f"📋 URLs queued: {len(self.config['urls'])}")
        elif self.config['url_file']:
            print(f"📄 URL file: {self.config['url_file']}")
        
        print("-" * 80)
    
    def _get_input(self, prompt: str, default: str = "") -> str:
        """Get user input with default value handling"""
        if default:
            full_prompt = f"{prompt} [{default}]: "
        else:
            full_prompt = f"{prompt}: "
        
        try:
            user_input = input(full_prompt).strip()
            return user_input if user_input else default
        except (EOFError, KeyboardInterrupt):
            return ""
    
    def _validate_path(self, path: str) -> bool:
        """Validate if a path can be created or accessed"""
        try:
            Path(path).mkdir(parents=True, exist_ok=True)
            return True
        except Exception:
            return False
    
    def _validate_url(self, url: str) -> bool:
        """Basic URL validation"""
        return url.startswith(('http://', 'https://')) and '.' in url
    
    def add_urls_menu(self):
        """Interactive menu for adding download URLs"""
        while True:
            self._clear_screen()
            self._print_header()
            print("📝 ADD DOWNLOAD URLS")
            print("-" * 40)
            print("1. Add single URL")
            print("2. Add multiple URLs (paste/type)")
            print("3. Load URLs from file")
            print("4. Clear all URLs")
            print("5. View current URLs")
            print("0. Back to main menu")
            
            choice = self._get_input("\nSelect option")
            
            if choice == '1':
                url = self._get_input("Enter URL")
                if url and self._validate_url(url):
                    self.config['urls'].append(url)
                    print(f"✅ Added: {url}")
                    input("\nPress Enter to continue...")
                elif url:
                    print("❌ Invalid URL format. URLs must start with http:// or https://")
                    input("\nPress Enter to continue...")
            
            elif choice == '2':
                print("Enter URLs (one per line, empty line to finish):")
                urls = []
                while True:
                    url = input("URL: ").strip()
                    if not url:
                        break
                    if self._validate_url(url):
                        urls.append(url)
                        print(f"✅ Added")
                    else:
                        print("❌ Invalid URL format, skipped")
                
                self.config['urls'].extend(urls)
                print(f"✅ Added {len(urls)} URLs total")
                input("\nPress Enter to continue...")
            
            elif choice == '3':
                file_path = self._get_input("Enter path to URL file")
                if file_path and Path(file_path).exists():
                    self.config['url_file'] = file_path
                    self.config['urls'] = []  # Clear individual URLs when using file
                    print(f"✅ URL file set to: {file_path}")
                elif file_path:
                    print("❌ File not found")
                input("\nPress Enter to continue...")
            
            elif choice == '4':
                self.config['urls'] = []
                self.config['url_file'] = None
                print("✅ All URLs cleared")
                input("\nPress Enter to continue...")
            
            elif choice == '5':
                if self.config['urls']:
                    print(f"\nCurrent URLs ({len(self.config['urls'])}):")
                    for i, url in enumerate(self.config['urls'], 1):
                        print(f"{i:2d}. {url}")
                elif self.config['url_file']:
                    print(f"\nURL file: {self.config['url_file']}")
                else:
                    print("\nNo URLs configured")
                input("\nPress Enter to continue...")
            
            elif choice == '0':
                break
    
    def settings_menu(self):
        """Interactive menu for configuring download settings"""
        while True:
            self._clear_screen()
            self._print_header()
            print("⚙️  DOWNLOAD SETTINGS")
            print("-" * 40)
            print(f"1. Output directory [{self.config['output_dir']}]")
            print(f"2. Connections per file [{self.config['connections']}]")
            print(f"3. Chunk size in MB [{self.config['chunk_size']}]")
            print(f"4. Connection timeout [{self.config['timeout']}s]")
            print(f"5. Retry attempts [{self.config['retries']}]")
            print(f"6. Max concurrent downloads [{self.config['max_concurrent']}]")
            print("7. Reset to defaults")
            print("0. Back to main menu")
            
            choice = self._get_input("\nSelect setting to change")
            
            if choice == '1':
                new_dir = self._get_input("Enter output directory", self.config['output_dir'])
                if new_dir and self._validate_path(new_dir):
                    self.config['output_dir'] = new_dir
                    print(f"✅ Output directory set to: {new_dir}")
                elif new_dir:
                    print("❌ Cannot create or access directory")
                input("\nPress Enter to continue...")
            
            elif choice == '2':
                try:
                    connections = int(self._get_input("Connections per file (1-32)", str(self.config['connections'])))
                    if 1 <= connections <= 32:
                        self.config['connections'] = connections
                        print(f"✅ Connections set to: {connections}")
                    else:
                        print("❌ Must be between 1 and 32")
                except ValueError:
                    print("❌ Must be a number")
                input("\nPress Enter to continue...")
            
            elif choice == '3':
                try:
                    chunk_size = int(self._get_input("Chunk size in MB (1-10)", str(self.config['chunk_size'])))
                    if 1 <= chunk_size <= 10:
                        self.config['chunk_size'] = chunk_size
                        print(f"✅ Chunk size set to: {chunk_size}MB")
                    else:
                        print("❌ Must be between 1 and 10 MB")
                except ValueError:
                    print("❌ Must be a number")
                input("\nPress Enter to continue...")
            
            elif choice == '4':
                try:
                    timeout = int(self._get_input("Timeout in seconds (10-300)", str(self.config['timeout'])))
                    if 10 <= timeout <= 300:
                        self.config['timeout'] = timeout
                        print(f"✅ Timeout set to: {timeout}s")
                    else:
                        print("❌ Must be between 10 and 300 seconds")
                except ValueError:
                    print("❌ Must be a number")
                input("\nPress Enter to continue...")
            
            elif choice == '5':
                try:
                    retries = int(self._get_input("Retry attempts (1-10)", str(self.config['retries'])))
                    if 1 <= retries <= 10:
                        self.config['retries'] = retries
                        print(f"✅ Retries set to: {retries}")
                    else:
                        print("❌ Must be between 1 and 10")
                except ValueError:
                    print("❌ Must be a number")
                input("\nPress Enter to continue...")
            
            elif choice == '6':
                try:
                    concurrent = int(self._get_input("Max concurrent downloads (1-10)", str(self.config['max_concurrent'])))
                    if 1 <= concurrent <= 10:
                        self.config['max_concurrent'] = concurrent
                        print(f"✅ Max concurrent set to: {concurrent}")
                    else:
                        print("❌ Must be between 1 and 10")
                except ValueError:
                    print("❌ Must be a number")
                input("\nPress Enter to continue...")
            
            elif choice == '7':
                self.config.update({
                    'output_dir': str(Path.home() / 'Downloads'),
                    'connections': 8,
                    'chunk_size': 1,
                    'timeout': 30,
                    'retries': 3,
                    'max_concurrent': 3
                })
                print("✅ Settings reset to defaults")
                input("\nPress Enter to continue...")
            
            elif choice == '0':
                break
    
    def quick_setup_menu(self):
        """Quick setup for common download scenarios"""
        self._clear_screen()
        self._print_header()
        print("⚡ QUICK SETUP PRESETS")
        print("-" * 40)
        print("1. 🏠 Home user (balanced speed & stability)")
        print("2. 🚀 High-speed (maximum performance)")
        print("3. 🐌 Conservative (slow/unreliable connection)")
        print("4. 📦 Batch download (multiple files)")
        print("0. Back to main menu")
        
        choice = self._get_input("\nSelect preset")
        
        if choice == '1':
            self.config.update({
                'connections': 6,
                'chunk_size': 1,
                'timeout': 30,
                'retries': 3,
                'max_concurrent': 2
            })
            print("✅ Home user preset applied")
        
        elif choice == '2':
            self.config.update({
                'connections': 16,
                'chunk_size': 2,
                'timeout': 60,
                'retries': 5,
                'max_concurrent': 1
            })
            print("✅ High-speed preset applied")
        
        elif choice == '3':
            self.config.update({
                'connections': 2,
                'chunk_size': 1,
                'timeout': 60,
                'retries': 5,
                'max_concurrent': 1
            })
            print("✅ Conservative preset applied")
        
        elif choice == '4':
            self.config.update({
                'connections': 4,
                'chunk_size': 1,
                'timeout': 30,
                'retries': 3,
                'max_concurrent': 5
            })
            print("✅ Batch download preset applied")
        
        if choice in ['1', '2', '3', '4']:
            input("\nPress Enter to continue...")
    
    def start_download_menu(self):
        """Review settings and start download"""
        self._clear_screen()
        self._print_header()
        
        # Check if we have something to download
        has_urls = bool(self.config['urls'] or self.config['url_file'])
        
        if not has_urls:
            print("❌ No URLs configured! Please add URLs first.")
            input("\nPress Enter to continue...")
            return False
        
        print("🎯 READY TO DOWNLOAD")
        print("-" * 40)
        
        if self.config['urls']:
            print(f"📋 {len(self.config['urls'])} URLs queued")
            if len(self.config['urls']) <= 5:
                for i, url in enumerate(self.config['urls'], 1):
                    print(f"  {i}. {url}")
            else:
                for i, url in enumerate(self.config['urls'][:3], 1):
                    print(f"  {i}. {url}")
                print(f"  ... and {len(self.config['urls']) - 3} more")
        
        if self.config['url_file']:
            print(f"📄 URL file: {self.config['url_file']}")
        
        print(f"\n📁 Destination: {self.config['output_dir']}")
        print(f"⚙️  Settings: {self.config['connections']} connections, {self.config['chunk_size']}MB chunks")
        
        confirm = self._get_input("\n🚀 Start download? (y/N)", "n").lower()
        
        if confirm in ['y', 'yes']:
            self._save_history()  # Save settings for next time
            return True
        
        return False
    
    def run_interactive_mode(self):
        """Main interactive menu loop"""
        while True:
            self._clear_screen()
            self._print_header()
            print("🎛️  MAIN MENU")
            print("-" * 40)
            print("1. 📝 Add/Manage URLs")
            print("2. ⚙️  Download Settings")
            print("3. ⚡ Quick Setup Presets")
            print("4. 🚀 Start Download")
            print("0. Exit")
            
            choice = self._get_input("\nSelect option")
            
            if choice == '1':
                self.add_urls_menu()
            elif choice == '2':
                self.settings_menu()
            elif choice == '3':
                self.quick_setup_menu()
            elif choice == '4':
                if self.start_download_menu():
                    return self.config  # Return config to start download
            elif choice == '0':
                print("\n👋 Goodbye!")
                sys.exit(0)
            else:
                print("❌ Invalid option")
                input("\nPress Enter to continue...")
        
        return None

class FastDownloader:
    def __init__(self, max_connections: int = 8, chunk_size: int = 1024*1024, 
                 timeout: int = 30, max_retries: int = 3):
        """
        Initialize the downloader with performance-optimized settings.
        
        max_connections: Number of concurrent connections per file (default: 8)
        chunk_size: Size of each download chunk in bytes (default: 1MB)
        timeout: Connection timeout in seconds
        max_retries: Maximum retry attempts for failed chunks
        """
        self.max_connections = max_connections
        self.chunk_size = chunk_size
        self.timeout = aiohttp.ClientTimeout(total=timeout)
        self.max_retries = max_retries
        self.session = None
        self.stats = {}  # Track stats per URL
        
        # Handle Ctrl+C gracefully
        signal.signal(signal.SIGINT, self._signal_handler)
    
    def _signal_handler(self, signum, frame):
        """Handle interrupt signals gracefully"""
        print("\n\n🛑 Download interrupted by user")
        if self.session:
            asyncio.create_task(self.session.close())
        sys.exit(0)
    
    async def _create_session(self):
        """Create an optimized aiohttp session with connection pooling"""
        connector = aiohttp.TCPConnector(
            limit=100,  # Total connection pool size
            limit_per_host=self.max_connections,  # Connections per host
            keepalive_timeout=60,  # Keep connections alive
            enable_cleanup_closed=True  # Clean up closed connections
        )
        
        self.session = aiohttp.ClientSession(
            connector=connector,
            timeout=self.timeout,
            headers={
                'User-Agent': 'FastCLI-Downloader/1.0'
            }
        )
    
    async def _get_file_info(self, url: str) -> Tuple[int, bool, str]:
        """
        Get file size and check if server supports range requests.
        Returns: (file_size, supports_ranges, filename)
        """
        try:
            async with self.session.head(url, allow_redirects=True) as response:
                # Get file size
                content_length = response.headers.get('Content-Length')
                file_size = int(content_length) if content_length else 0
                
                # Check if server supports partial content (range requests)
                supports_ranges = response.headers.get('Accept-Ranges') == 'bytes'
                
                # Extract filename from URL or Content-Disposition header
                filename = self._extract_filename(url, response.headers)
                
                return file_size, supports_ranges, filename
                
        except Exception as e:
            print(f"❌ Error getting file info for {url}: {e}")
            return 0, False, self._extract_filename(url, {})
    
    def _extract_filename(self, url: str, headers: dict) -> str:
        """Extract filename from URL or headers"""
        # Try Content-Disposition header first
        content_disp = headers.get('Content-Disposition', '')
        if 'filename=' in content_disp:
            filename = content_disp.split('filename=')[1].strip('"\'')
            return unquote(filename)
        
        # Fall back to URL path
        parsed_url = urlparse(url)
        filename = os.path.basename(parsed_url.path)
        if filename:
            return unquote(filename)
        
        # Last resort: generate a name
        return f"download_{int(time.time())}"
    
    async def _download_chunk(self, url: str, start: int, end: int, 
                            file_path: Path, chunk_id: int) -> bool:
        """
        Download a specific byte range of the file.
        Returns True on success, False on failure.
        """
        headers = {'Range': f'bytes={start}-{end}'}
        
        for attempt in range(self.max_retries):
            try:
                async with self.session.get(url, headers=headers) as response:
                    if response.status in [206, 200]:  # Partial or full content
                        # Open file in binary append mode for this chunk
                        async with aiofiles.open(file_path, 'r+b') as f:
                            await f.seek(start)
                            
                            # Read and write data in smaller sub-chunks for better progress tracking
                            downloaded_chunk = 0
                            async for data in response.content.iter_chunked(8192):  # 8KB sub-chunks
                                await f.write(data)
                                downloaded_chunk += len(data)
                                
                                # Update progress
                                if url in self.stats:
                                    self.stats[url].downloaded += len(data)
                        
                        self.stats[url].chunks_completed += 1
                        return True
                    else:
                        print(f"⚠️  Chunk {chunk_id}: HTTP {response.status}")
                        
            except Exception as e:
                if attempt < self.max_retries - 1:
                    print(f"⚠️  Chunk {chunk_id} attempt {attempt + 1} failed: {e}")
                    await asyncio.sleep(1)  # Brief pause before retry
                else:
                    print(f"❌ Chunk {chunk_id} failed after {self.max_retries} attempts: {e}")
        
        return False
    
    async def _download_single_stream(self, url: str, file_path: Path) -> bool:
        """Fallback method for servers that don't support range requests"""
        try:
            async with self.session.get(url) as response:
                if response.status == 200:
                    async with aiofiles.open(file_path, 'wb') as f:
                        async for chunk in response.content.iter_chunked(self.chunk_size):
                            await f.write(chunk)
                            if url in self.stats:
                                self.stats[url].downloaded += len(chunk)
                    return True
                else:
                    print(f"❌ HTTP {response.status} for {url}")
                    return False
                    
        except Exception as e:
            print(f"❌ Single stream download failed: {e}")
            return False
    
    def _print_progress(self, url: str, filename: str):
        """Display real-time download progress"""
        stats = self.stats.get(url)
        if not stats:
            return
        
        # Calculate display values
        progress = stats.progress_percent
        speed = stats.speed
        downloaded_mb = stats.downloaded / (1024 * 1024)
        total_mb = stats.total_size / (1024 * 1024) if stats.total_size > 0 else 0
        
        # Create progress bar
        bar_length = 40
        filled_length = int(bar_length * progress / 100)
        bar = '█' * filled_length + '░' * (bar_length - filled_length)
        
        # Format the progress line
        progress_line = f"\r📁 {filename[:30]:<30} [{bar}] {progress:6.1f}% "
        progress_line += f"{downloaded_mb:8.1f}MB"
        if total_mb > 0:
            progress_line += f"/{total_mb:.1f}MB"
        progress_line += f" @ {speed:6.1f}MB/s"
        
        print(progress_line, end='', flush=True)
    
    async def download_file(self, url: str, output_dir: str = ".", 
                          custom_filename: Optional[str] = None) -> bool:
        """
        Download a single file with maximum speed using concurrent connections.
        
        url: The URL to download
        output_dir: Directory to save the file
        custom_filename: Optional custom filename (otherwise extracted from URL)
        """
        print(f"\n🔍 Analyzing: {url}")
        
        # Get file information
        file_size, supports_ranges, filename = await self._get_file_info(url)
        
        if custom_filename:
            filename = custom_filename
        
        file_path = Path(output_dir) / filename
        file_path.parent.mkdir(parents=True, exist_ok=True)
        
        # Initialize stats tracking
        self.stats[url] = DownloadStats(
            total_size=file_size,
            start_time=time.time()
        )
        
        print(f"📄 File: {filename}")
        print(f"📏 Size: {file_size / (1024*1024):.1f} MB" if file_size > 0 else "📏 Size: Unknown")
        print(f"🔗 Range support: {'Yes' if supports_ranges else 'No'}")
        
        # Create empty file with correct size for range downloads
        if supports_ranges and file_size > 0:
            # Use fallocate for instant file allocation on Linux
            try:
                # Create empty file first
                file_path.touch()
                # Use os.posix_fallocate for instant space allocation (Linux-specific optimization)
                with open(file_path, 'r+b') as f:
                    os.posix_fallocate(f.fileno(), 0, file_size)
            except (OSError, AttributeError):
                # Fallback for non-Linux systems or if fallocate fails
                with open(file_path, 'wb') as f:
                    f.seek(file_size - 1)
                    f.write(b'\0')
        
        success = False
        
        if supports_ranges and file_size > self.chunk_size:
            # Use concurrent chunk downloading for maximum speed
            print(f"🚀 Starting concurrent download with {self.max_connections} connections...")
            
            # Calculate chunk ranges
            chunk_ranges = []
            for i in range(self.max_connections):
                start = i * (file_size // self.max_connections)
                end = start + (file_size // self.max_connections) - 1
                if i == self.max_connections - 1:  # Last chunk gets remainder
                    end = file_size - 1
                chunk_ranges.append((start, end, i))
            
            # Start progress monitoring
            progress_task = asyncio.create_task(self._monitor_progress(url, filename))
            
            # Download all chunks concurrently
            tasks = [
                self._download_chunk(url, start, end, file_path, chunk_id)
                for start, end, chunk_id in chunk_ranges
            ]
            
            results = await asyncio.gather(*tasks, return_exceptions=True)
            progress_task.cancel()
            
            success = all(isinstance(r, bool) and r for r in results)
            
        else:
            # Single stream download for small files or unsupported servers
            print("📡 Starting single-stream download...")
            progress_task = asyncio.create_task(self._monitor_progress(url, filename))
            success = await self._download_single_stream(url, file_path)
            progress_task.cancel()
        
        # Final progress update
        self._print_progress(url, filename)
        print()  # New line after progress
        
        if success:
            final_stats = self.stats[url]
            total_time = time.time() - final_stats.start_time
            avg_speed = (final_stats.downloaded / (1024 * 1024)) / total_time if total_time > 0 else 0
            print(f"✅ {filename} downloaded successfully!")
            print(f"⏱️  Total time: {total_time:.1f}s | Average speed: {avg_speed:.1f} MB/s")
        else:
            print(f"❌ Failed to download {filename}")
            # Clean up partial file
            if file_path.exists():
                file_path.unlink()
        
        return success
    
    async def _monitor_progress(self, url: str, filename: str):
        """Monitor and display download progress in real-time"""
        while True:
            try:
                self._print_progress(url, filename)
                await asyncio.sleep(0.5)  # Update every 500ms
            except asyncio.CancelledError:
                break
    
    async def download_batch(self, urls: List[str], output_dir: str = ".", 
                           max_concurrent: int = 3) -> List[bool]:
        """
        Download multiple files concurrently with controlled parallelism.
        
        urls: List of URLs to download
        output_dir: Directory to save files
        max_concurrent: Maximum number of simultaneous file downloads
        """
        print(f"\n📦 Starting batch download of {len(urls)} files...")
        print(f"🎛️  Max concurrent downloads: {max_concurrent}")
        
        # Create semaphore to limit concurrent downloads
        semaphore = asyncio.Semaphore(max_concurrent)
        
        async def download_with_semaphore(url):
            async with semaphore:
                return await self.download_file(url, output_dir)
        
        # Execute downloads with controlled concurrency
        results = await asyncio.gather(
            *[download_with_semaphore(url) for url in urls],
            return_exceptions=True
        )
        
        # Process results
        success_count = sum(1 for r in results if isinstance(r, bool) and r)
        print(f"\n📊 Batch download complete: {success_count}/{len(urls)} successful")
        
        return [r if isinstance(r, bool) else False for r in results]
    
    async def download_from_file(self, file_path: str, output_dir: str = ".", 
                               max_concurrent: int = 3) -> List[bool]:
        """Download URLs from a text file (one URL per line)"""
        try:
            with open(file_path, 'r') as f:
                urls = [line.strip() for line in f if line.strip() and not line.startswith('#')]
            
            if not urls:
                print("❌ No valid URLs found in file")
                return []
            
            print(f"📋 Found {len(urls)} URLs in file")
            return await self.download_batch(urls, output_dir, max_concurrent)
            
        except FileNotFoundError:
            print(f"❌ File not found: {file_path}")
            return []
        except Exception as e:
            print(f"❌ Error reading file: {e}")
            return []
    
    async def close(self):
        """Clean up resources"""
        if self.session:
            await self.session.close()

async def main():
    # Check if user wants interactive mode or command-line mode
    if len(sys.argv) == 1 or (len(sys.argv) == 2 and sys.argv[1] in ['-i', '--interactive']):
        # Interactive mode - much more user-friendly for regular use
        print("🚀 Starting Interactive Mode...\n")
        menu = InteractiveMenu()
        config = menu.run_interactive_mode()
        
        if not config:
            return  # User exited without starting download
        
        # Create downloader with interactive settings
        downloader = FastDownloader(
            max_connections=config['connections'],
            chunk_size=config['chunk_size'] * 1024 * 1024,
            timeout=config['timeout'],
            max_retries=config['retries']
        )
        
        try:
            await downloader._create_session()
            
            if config['url_file']:
                await downloader.download_from_file(
                    config['url_file'], 
                    config['output_dir'], 
                    config['max_concurrent']
                )
            elif len(config['urls']) == 1:
                await downloader.download_file(config['urls'][0], config['output_dir'])
            else:
                await downloader.download_batch(
                    config['urls'], 
                    config['output_dir'], 
                    config['max_concurrent']
                )
        
        finally:
            await downloader.close()
        
        return
    
    # Command-line mode for scripting and power users
    parser = argparse.ArgumentParser(
        description="Lightning Fast CLI Downloader - High-performance concurrent file downloader",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s                                    # Interactive mode (default)
  %(prog)s -i                                 # Force interactive mode
  %(prog)s https://example.com/file.zip       # Quick download
  %(prog)s -o ~/Downloads https://example.com/file.mp4
  %(prog)s -c 16 -s 2 https://example.com/largefile.tar.gz
  %(prog)s -f urls.txt -o ~/Downloads --max-concurrent 5
  %(prog)s https://site1.com/file1.zip https://site2.com/file2.mp4
        """
    )
    
    parser.add_argument('urls', nargs='*', help='URLs to download')
    parser.add_argument('-i', '--interactive', action='store_true',
                       help='Start in interactive mode')
    parser.add_argument('-o', '--output', default='.', 
                       help='Output directory (default: current directory)')
    parser.add_argument('-c', '--connections', type=int, default=8,
                       help='Max connections per file (default: 8)')
    parser.add_argument('-s', '--chunk-size', type=int, default=1,
                       help='Chunk size in MB (default: 1)')
    parser.add_argument('-t', '--timeout', type=int, default=30,
                       help='Connection timeout in seconds (default: 30)')
    parser.add_argument('-r', '--retries', type=int, default=3,
                       help='Max retry attempts (default: 3)')
    parser.add_argument('-f', '--file', 
                       help='Download URLs from file (one per line)')
    parser.add_argument('--max-concurrent', type=int, default=3,
                       help='Max concurrent file downloads for batch (default: 3)')
    
    args = parser.parse_args()
    
    if not args.urls and not args.file:
        # No URLs provided, start interactive mode
        print("🚀 No URLs provided, starting Interactive Mode...\n")
        menu = InteractiveMenu()
        config = menu.run_interactive_mode()
        
        if not config:
            return
        
        downloader = FastDownloader(
            max_connections=config['connections'],
            chunk_size=config['chunk_size'] * 1024 * 1024,
            timeout=config['timeout'],
            max_retries=config['retries']
        )
        
        try:
            await downloader._create_session()
            
            if config['url_file']:
                await downloader.download_from_file(
                    config['url_file'], 
                    config['output_dir'], 
                    config['max_concurrent']
                )
            elif len(config['urls']) == 1:
                await downloader.download_file(config['urls'][0], config['output_dir'])
            else:
                await downloader.download_batch(
                    config['urls'], 
                    config['output_dir'], 
                    config['max_concurrent']
                )
        
        finally:
            await downloader.close()
        
        return
    
    # Standard command-line execution with provided arguments
    output_path = Path(args.output)
    output_path.mkdir(parents=True, exist_ok=True)
    
    downloader = FastDownloader(
        max_connections=args.connections,
        chunk_size=args.chunk_size * 1024 * 1024,
        timeout=args.timeout,
        max_retries=args.retries
    )
    
    try:
        await downloader._create_session()
        
        if args.file:
            await downloader.download_from_file(args.file, args.output, args.max_concurrent)
        elif len(args.urls) == 1:
            await downloader.download_file(args.urls[0], args.output)
        else:
            await downloader.download_batch(args.urls, args.output, args.max_concurrent)
    
    finally:
        await downloader.close()

if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        print("\n\n🛑 Download interrupted by user")
        sys.exit(0)
