# FastDL - High-Performance Multi-Connection Downloader

<div align="center">

```
╔══════════════════════════════════════════════════════════════════════════════╗
║                                 FastDL v1.0                                  ║
║                     High-Performance Multi-Connection Downloader             ║
║                                                                              ║
║                            Developed by 0xb0rn3 | 0xbv1                     ║
║                                                                              ║
║  Discord: oxbv1  │  X: oxbv1  │  Instagram: theehiv3  │  Email: q4n0@proton.me║
╚══════════════════════════════════════════════════════════════════════════════╝
```

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Bash](https://img.shields.io/badge/bash-4.0+-orange.svg)](https://www.gnu.org/software/bash/)
[![Platform](https://img.shields.io/badge/platform-Linux-lightgrey.svg)](https://github.com/0xb0rn3/fastdl)
[![Version](https://img.shields.io/badge/version-1.0.0-brightgreen.svg)](https://github.com/0xb0rn3/fastdl)

**Extremely fast, pure bash downloader designed for massive ISO images, torrents, and concurrent file downloads with zero external dependencies.**

</div>

---

## Features

**Extreme Performance:**
- Multi-connection downloads (up to 64 concurrent connections)
- Automatic system optimization based on hardware
- Intelligent tool selection (aria2c, axel, curl, wget)
- Concurrent batch downloading with job management
- Advanced buffer management and memory optimization

**Advanced Capabilities:**
- Real-time storage device detection and analysis
- Torrent support via aria2c integration
- Automatic resume of interrupted downloads
- Performance testing and optimization
- Real-time monitoring dashboard
- Cross-architecture Linux compatibility

**Zero Dependencies:**
- Pure bash implementation
- Works with standard Linux utilities
- Auto-detects available download tools
- Compatible with all major Linux distributions

---

## Quick Start

### Installation

**Method 1: Direct Download & Execute**
```bash
# Download and make executable
wget https://raw.githubusercontent.com/0xb0rn3/fastdl/main/fastdl
chmod +x fastdl
./fastdl
```

**Method 2: Clone Repository**
```bash
git clone https://github.com/0xb0rn3/fastdl.git
cd fastdl
chmod +x fastdl
./fastdl
```

**Method 3: System-wide Installation**
```bash
# Download script
wget https://raw.githubusercontent.com/0xb0rn3/fastdl/main/fastdl

# Make executable
chmod +x fastdl

# Install system-wide (requires sudo)
sudo cp fastdl /usr/bin/fastdl

# Now you can run from anywhere
fastdl
```

### First Run

```bash
# Interactive mode (recommended for first-time users)
./fastdl

# Quick download
./fastdl --download https://releases.ubuntu.com/22.04/ubuntu-22.04.3-desktop-amd64.iso

# Batch download
./fastdl --batch urls.txt
```

---

## Usage

### Interactive Mode

Launch the beautiful interactive interface:
```bash
./fastdl
```

The interactive menu provides:
1. **Single File Download** - Download individual files with custom settings
2. **Batch Download** - Process multiple URLs from a file
3. **Torrent Download** - Full torrent support with aria2c
4. **System Analysis** - Analyze your system for optimal settings
5. **Storage Detection** - Real-time storage device detection
6. **Real-time Dashboard** - Monitor active downloads and system stats
7. **Configuration** - Customize download parameters
8. **Performance Test** - Benchmark optimal connection settings
9. **Help & About** - Documentation and contact information

### Command Line Usage

```bash
# Quick single download
./fastdl --download <url>

# Batch download from file
./fastdl --batch <file>

# Analyze URL capabilities
./fastdl --analyze <url>

# Show real-time dashboard
./fastdl --dashboard

# Configure settings
./fastdl --config

# Display storage information
./fastdl --storage

# Show help
./fastdl --help

# Show version
./fastdl --version
```

### Batch Download File Format

Create a text file with URLs (one per line):

```txt
# High-priority ISO downloads
https://releases.ubuntu.com/22.04/ubuntu-22.04.3-desktop-amd64.iso
https://cdimage.debian.org/debian-cd/current/amd64/iso-cd/debian-12.2.0-amd64-netinst.iso

# Software downloads
https://download.fedoraproject.org/pub/fedora/linux/releases/38/Workstation/x86_64/iso/Fedora-Workstation-Live-x86_64-38-1.6.iso

# Comments start with # and are ignored
https://archlinux.org/iso/2023.10.14/archlinux-2023.10.14-x86_64.iso
```

---

## Configuration

FastDL automatically optimizes based on your system but allows full customization:

### Automatic Optimization

**High-end Systems (16+ cores):**
- 64 connections per download
- 16 concurrent downloads
- 16MB buffer size

**Mid-range Systems (8-16 cores):**
- 32 connections per download
- 8 concurrent downloads
- 8MB buffer size

**Entry-level Systems (4-8 cores):**
- 16 connections per download
- 4 concurrent downloads
- 4MB buffer size

### Manual Configuration

Access via interactive menu (option 7) or edit `~/.fastdl/config`:

```bash
# FastDL Configuration
CONNECTIONS=32                    # Connections per download
CHUNK_SIZE=1M                    # Chunk size for splitting
TIMEOUT=30                       # Connection timeout (seconds)
RETRIES=5                        # Retry attempts
MAX_CONCURRENT=8                 # Max concurrent downloads
BUFFER_SIZE=8M                   # Buffer size
AUTO_RESUME=true                 # Resume interrupted downloads
VERIFY_SSL=true                  # Verify SSL certificates
```

---

## System Requirements

### Minimum Requirements
- **OS:** Any Linux distribution
- **Shell:** Bash 4.0+
- **Tools:** At least one of: curl, wget, aria2c, axel
- **Memory:** 1GB RAM
- **Storage:** 10MB for FastDL + space for downloads

### Recommended Requirements
- **OS:** Modern Linux distribution
- **CPU:** 4+ cores
- **Memory:** 4GB+ RAM
- **Tools:** aria2c + axel + curl + wget
- **Network:** High-speed internet connection

### Supported Distributions

FastDL is tested and compatible with:
- **Debian/Ubuntu** (apt)
- **RHEL/CentOS/Fedora** (yum/dnf)
- **Arch Linux** (pacman)
- **SUSE/openSUSE** (zypper)
- **Alpine Linux** (apk)
- **And virtually any Linux distribution**

---

## Performance Examples

### Single Large File (10GB ISO)
```bash
# Standard download
wget https://example.com/large.iso  # ~45 minutes

# FastDL with 32 connections
./fastdl --download https://example.com/large.iso  # ~8 minutes
```

### Batch Download (50 files)
```bash
# Sequential downloads
for url in $(cat urls.txt); do wget "$url"; done  # ~3 hours

# FastDL concurrent batch (8 concurrent)
./fastdl --batch urls.txt  # ~25 minutes
```

### Torrent Download
```bash
# FastDL with optimized settings
./fastdl  # Select option 3 for torrent download
# Automatic DHT, PEX, and peer optimization
```

---

## Advanced Features

### Real-time Monitoring

Access the dashboard for live statistics:
```bash
./fastdl --dashboard
```

Features:
- **System Stats:** CPU load, memory usage, disk usage
- **Active Downloads:** Live progress of all downloads
- **Network Usage:** Real-time bandwidth monitoring
- **Recent Completions:** History of completed downloads

### Storage Intelligence

Automatic detection of:
- **Physical Devices:** HDDs, SSDs, NVMe drives
- **Mount Points:** All accessible storage locations
- **Network Mounts:** NFS, CIFS, SSHFS detection
- **Available Space:** Real-time capacity monitoring
- **I/O Schedulers:** Disk optimization recommendations

### Performance Testing

Built-in benchmarking to find optimal settings:
```bash
./fastdl  # Select option 8 for performance test
```

Tests different connection counts and recommends optimal settings for your system and network.

---

## Troubleshooting

### Common Issues

**Downloads fail with SSL errors:**
```bash
# Disable SSL verification (not recommended for security)
# In configuration, set: VERIFY_SSL=false
```

**Low download speeds:**
```bash
# Run performance test
./fastdl  # Option 8
# Or manually increase connections
# In configuration, set: CONNECTIONS=64
```

**Permission denied errors:**
```bash
# Ensure download directory is writable
chmod 755 ~/Downloads/FastDL
# Or change download directory in config
```

**Missing download tools:**
```bash
# Debian/Ubuntu
sudo apt install aria2 axel curl wget

# RHEL/CentOS/Fedora
sudo yum install aria2 axel curl wget

# Arch Linux
sudo pacman -S aria2 axel curl wget

# Alpine Linux
sudo apk add aria2 axel curl wget
```

### Debug Mode

Enable debug output for troubleshooting:
```bash
DEBUG=1 ./fastdl --download <url>
```

### Log Files

FastDL maintains detailed logs:
- **Main log:** `~/.fastdl/logs/fastdl.log`
- **Configuration:** `~/.fastdl/config`
- **Temporary files:** `/tmp/fastdl-*`

---

## Security Considerations

### SSL/TLS Verification
FastDL verifies SSL certificates by default. Only disable for trusted internal networks.

### File Integrity
For critical downloads, verify checksums manually:
```bash
# After download
sha256sum downloaded_file.iso
# Compare with provided checksum
```

### Network Security
FastDL respects system proxy settings and network configurations.

---

## Contributing

We welcome contributions! Here's how to get involved:

### Reporting Issues
1. Check existing issues on GitHub
2. Include system information (OS, bash version)
3. Provide detailed steps to reproduce
4. Include relevant log files

### Feature Requests
1. Search existing feature requests
2. Describe the use case clearly
3. Explain expected behavior
4. Consider implementation complexity

### Code Contributions
1. Fork the repository
2. Create a feature branch
3. Test thoroughly on multiple distributions
4. Submit a pull request with clear description

### Testing
Help us test on different distributions:
- Test installation methods
- Verify compatibility
- Report performance results
- Document edge cases

---

## Support & Contact

### Developer Information
- **Author:** 0xb0rn3 | 0xbv1
- **Discord:** oxbv1
- **X (Twitter):** oxbv1
- **Instagram:** theehiv3
- **Email:** q4n0@proton.me

### Community Support
- **GitHub Issues:** [Report bugs and request features](https://github.com/0xb0rn3/fastdl/issues)
- **GitHub Discussions:** [Community support and questions](https://github.com/0xb0rn3/fastdl/discussions)

### Professional Support
For enterprise or professional support, contact: q4n0@proton.me

---

## License

FastDL is released under the MIT License. See [LICENSE](LICENSE) file for details.

```
MIT License

Copyright (c) 2024 0xb0rn3

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

---

## Changelog

### v1.0.0 (2024)
- Initial release
- Pure bash implementation
- Multi-connection download support
- Torrent support via aria2c
- Real-time monitoring dashboard
- Automatic system optimization
- Cross-distribution compatibility
- Interactive and CLI interfaces

---

<div align="center">

**Made with ⚡ by [0xb0rn3](https://github.com/0xb0rn3)**

[![GitHub](https://img.shields.io/badge/GitHub-0xb0rn3-181717?style=for-the-badge&logo=github)](https://github.com/0xb0rn3/fastdl)
[![Discord](https://img.shields.io/badge/Discord-oxbv1-5865F2?style=for-the-badge&logo=discord&logoColor=white)](https://discord.com)
[![Instagram](https://img.shields.io/badge/Instagram-theehiv3-E4405F?style=for-the-badge&logo=instagram&logoColor=white)](https://instagram.com/theehiv3)

*If FastDL has been helpful, please consider giving it a ⭐ on GitHub!*

</div>
