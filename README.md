# QEMU Pentest Lab - Automated VM Isolation Setup

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Linux](https://img.shields.io/badge/Platform-Linux-blue.svg)](https://www.linux.org/)
[![Bash](https://img.shields.io/badge/Shell-Bash-green.svg)](https://www.gnu.org/software/bash/)
[![Version](https://img.shields.io/badge/Version-1.0.0-orange.svg)](https://github.com/0xb0rn3/qemu-pentest)

> **🎯 Professional VM Lab Setup for Penetration Testing**
> 
> Transform your Linux system into a sophisticated penetration testing laboratory with automated VM creation, network isolation, and USB WiFi adapter passthrough capabilities.

---

## 📋 Table of Contents

- [🤔 What is QEMU/KVM?](#-what-is-qemukvm)
- [🌟 What This Tool Does](#-what-this-tool-does)
- [🎓 Learning Objectives](#-learning-objectives)
- [🔧 Prerequisites](#-prerequisites)
- [⚡ Quick Start](#-quick-start)
- [📚 Detailed Installation](#-detailed-installation)
- [🎮 Usage Examples](#-usage-examples)
- [🌐 Network Architecture](#-network-architecture)
- [🔌 USB WiFi Adapter Support](#-usb-wifi-adapter-support)
- [⚙️ Advanced Configuration](#-advanced-configuration)
- [🛠️ Troubleshooting](#-troubleshooting)
- [🔒 Security Considerations](#-security-considerations)
- [🤝 Contributing](#-contributing)
- [📄 License](#-license)

---

## 🤔 What is QEMU/KVM?

If you're new to virtualization, here's what you need to know:

**QEMU (Quick Emulator)** is software that creates virtual computers inside your real computer. Think of it like running a completely separate computer as a program on your desktop - it has its own operating system, applications, and behaves just like a physical machine.

**KVM (Kernel-based Virtual Machine)** is Linux's built-in virtualization technology that makes QEMU run much faster by using your processor's hardware virtualization features. Without KVM, virtual machines would be extremely slow.

**Why Use QEMU/KVM for Security Testing?**
- **Isolation**: Test dangerous tools and techniques without risking your main system
- **Snapshots**: Save the exact state of a VM and restore it instantly if something breaks  
- **Multiple Systems**: Run different operating systems (Windows, Linux, etc.) simultaneously
- **Hardware Access**: Pass through real hardware like WiFi adapters for wireless testing
- **Cost Effective**: No need for multiple physical computers for your lab

**QEMU/KVM vs Other Virtualization:**
- **VirtualBox**: QEMU/KVM typically offers better performance and more advanced features
- **VMware**: QEMU/KVM is free and open-source with similar capabilities
- **Docker**: QEMU/KVM runs complete operating systems, while Docker runs applications

This tool automates the complex setup of QEMU/KVM specifically for penetration testing scenarios.

---

## 🌟 What This Tool Does

This script automates the complex process of setting up isolated virtual machine environments specifically designed for penetration testing and cybersecurity research. Think of it as your personal lab assistant that handles all the technical complexity while you focus on learning and testing.

### Core Capabilities

**Virtual Machine Management**
- Creates new VMs from ISO images with optimal resource allocation
- Configures existing VMs for penetration testing workflows
- Provides automated backup and restoration capabilities

**Network Isolation Architecture**
- Implements VLAN-based network segmentation for safe testing
- Creates bridged networking with NAT capabilities for internet access
- Isolates your testing environment from your host system and other networks

**USB Device Passthrough**
- Direct hardware access to USB WiFi adapters for wireless security testing
- Supports popular penetration testing adapters with monitor mode capabilities
- Maintains compatibility with tools like Aircrack-ng, Kismet, and Wireshark

**Security and Convenience Features**
- Automatic configuration backup before making changes
- Dry-run mode for testing configurations without applying them
- Persistent network configurations that survive system reboots
- Comprehensive logging for troubleshooting and audit purposes

---

## 🎓 Learning Objectives

By using this tool, you'll gain hands-on experience with several important technologies and concepts that are fundamental to both cybersecurity and systems administration:

### Virtualization Technologies
Understanding how QEMU/KVM virtualization works at a technical level, including how virtual machines interact with host hardware and how resources are allocated and managed.

### Network Engineering Concepts
Learning about advanced networking concepts including VLANs (Virtual Local Area Networks), bridge networking, NAT (Network Address Translation), and how network isolation protects both your testing environment and production systems.

### Linux System Administration
Gaining experience with system-level configuration, including network interface management, systemd services, and how different Linux distributions handle network configuration persistence.

### Hardware Virtualization and Passthrough
Understanding how USB passthrough works, why it's necessary for certain types of security testing, and how virtual machines can gain direct access to physical hardware.

### Security Laboratory Design
Learning the principles behind creating safe, isolated environments for security testing that prevent accidental damage to production systems while maintaining the ability to test real-world attack scenarios.

---

## 🔧 Prerequisites

### System Requirements

Your system needs to meet certain technical requirements to run this tool effectively. Let's break down what you need and why each component is important.

**Hardware Requirements**
- **CPU**: x86_64 processor with virtualization support (Intel VT-x or AMD-V)
  - Most modern processors include this, but you can verify with: `grep -E "(vmx|svm)" /proc/cpuinfo`
  - This hardware virtualization support is crucial for good VM performance
- **Memory**: Minimum 8GB RAM (16GB recommended for multiple VMs)
  - VMs will share your system's memory, so more RAM allows for better performance and multiple concurrent VMs
- **Storage**: At least 50GB free space for VM images
  - Virtual machine disk images can grow large, especially when installing multiple tools and datasets
- **Network**: Active internet connection for VM internet access
  - Required for downloading updates, tools, and testing scenarios that involve internet connectivity

**Software Requirements**
- **Operating System**: Linux distribution (Ubuntu 20.04+, Debian 11+, Arch Linux, or similar)
  - This tool is designed specifically for Linux and uses Linux-specific networking and virtualization features
- **User Privileges**: Root access (sudo) for system configuration
  - Required because the script modifies network interfaces, creates bridges, and configures system-level virtualization settings

### Understanding Virtualization Support

Before proceeding, it's important to understand what virtualization support means and how to verify it's available on your system.

Modern processors include special instructions that allow them to run virtual machines efficiently. Without these instructions, virtual machines run much slower because they have to emulate all processor operations in software. To check if your system supports hardware virtualization:

```bash
# Check for Intel VT-x support
grep -o vmx /proc/cpuinfo | wc -l

# Check for AMD-V support  
grep -o svm /proc/cpuinfo | wc -l
```

If either command returns a number greater than zero, your processor supports hardware virtualization. You also need to ensure it's enabled in your BIOS/UEFI settings, which is usually found under "Virtualization Technology," "Intel VT-x," or "AMD-V" in your system's firmware settings.

---

## ⚡ Quick Start

For users who want to get started immediately, here's the fastest path to setting up your first isolated VM lab.

### One-Command Setup

```bash
# Download, make executable, and run the setup
curl -O https://raw.githubusercontent.com/0xb0rn3/qemu-pentest/main/setup
chmod +x setup
sudo ./setup
```

This interactive approach will guide you through every step, explaining each choice and helping you understand what's being configured.

### What Happens During Interactive Setup

When you run the script without parameters, it enters an educational interactive mode that:

1. **Scans your system** to detect available resources, network interfaces, and USB WiFi adapters
2. **Explains each option** as it presents choices, helping you understand the implications
3. **Provides recommendations** based on your hardware capabilities and common use cases
4. **Validates your choices** to prevent configuration errors before applying changes
5. **Creates comprehensive backups** of existing configurations before making modifications

The interactive mode is designed to be educational, so even if you're new to virtualization or network configuration, you'll learn as you go.

---

## 📚 Detailed Installation

For users who want to understand each step of the installation process, or who need to customize the setup for their specific environment, this section provides comprehensive guidance.

### Step 1: System Preparation

Before running the setup script, you should prepare your system by installing the necessary virtualization and networking components.

**Ubuntu/Debian Systems:**
```bash
# Update package repositories
sudo apt update

# Install virtualization components
sudo apt install qemu-kvm libvirt-daemon-system libvirt-clients virtinst

# Install networking tools
sudo apt install bridge-utils iproute2 iptables-persistent

# Install XML processing tools (used for VM configuration)
sudo apt install libxml2-utils

# Add your user to the libvirt group for VM management
sudo usermod -a -G libvirt $USER

# Enable and start libvirt service
sudo systemctl enable libvirtd
sudo systemctl start libvirtd
```

**Arch Linux Systems:**
```bash
# Install virtualization stack
sudo pacman -S qemu libvirt virt-install

# Install networking utilities
sudo pacman -S bridge-utils iproute2 iptables

# Install XML tools
sudo pacman -S libxml2

# Enable libvirt service
sudo systemctl enable libvirtd.service
sudo systemctl start libvirtd.service

# Add user to libvirt group
sudo usermod -a -G libvirt $USER
```

**Understanding What These Packages Do:**
- **qemu-kvm/qemu**: The actual virtualization engine that runs your VMs
- **libvirt**: A management layer that provides consistent APIs for managing VMs
- **bridge-utils**: Tools for creating and managing network bridges
- **iproute2**: Modern Linux networking tools for interface and routing management
- **iptables**: Firewall tools used for creating NAT rules and network isolation

### Step 2: Download and Prepare the Script

```bash
# Create a dedicated directory for the project
mkdir -p ~/qemu-pentest
cd ~/qemu-pentest

# Download the setup script
curl -O https://raw.githubusercontent.com/0xb0rn3/qemu-pentest/main/setup

# Make the script executable
chmod +x setup

# Verify the script downloaded correctly
ls -la setup
```

### Step 3: Understanding Script Permissions

The setup script requires root privileges to modify system network configuration and virtualization settings. Let's understand why each privilege is needed:

- **Network Interface Management**: Creating bridges and VLAN interfaces requires root access
- **Firewall Configuration**: Setting up NAT rules and forwarding requires iptables access
- **VM Configuration**: Modifying libvirt VM definitions requires access to system virtualization resources
- **USB Device Passthrough**: Configuring USB device access requires hardware-level permissions

Always review scripts before running them with sudo. You can examine the script contents with:
```bash
less setup
```

---

## 🎮 Usage Examples

This section provides practical examples for different use cases, from simple setups to advanced configurations.

### Example 1: Interactive Setup (Recommended for Beginners)

```bash
sudo ./setup
```

This launches the full interactive mode where the script will:
- Detect your system capabilities and provide recommendations
- Walk you through VM selection or creation
- Guide you through network interface selection
- Help you configure USB WiFi adapter passthrough
- Explain each step and its purpose

### Example 2: Create a New Kali Linux Lab

```bash
# Download Kali Linux ISO first
wget https://cdimage.kali.org/kali-2024.1/kali-linux-2024.1-installer-amd64.iso

# Create new VM with optimal settings
sudo ./setup \
  --create-vm \
  --vm kali-pentest-lab \
  --iso kali-linux-2024.1-installer-amd64.iso \
  --memory 4096 \
  --cores 2 \
  --disk 60 \
  --adapter 0e8d:7612 \
  --persistent
```

**What This Does:**
- Creates a new VM named "kali-pentest-lab"
- Allocates 4GB RAM and 2 CPU cores
- Creates a 60GB virtual disk
- Sets up USB passthrough for a MediaTek MT7612U WiFi adapter
- Makes network configuration persistent across reboots

### Example 3: Configure Existing VM for Penetration Testing

```bash
# Configure an existing VM with network isolation and USB passthrough
sudo ./setup \
  --vm existing-vm-name \
  --adapter 0bda:8812 \
  --vlan 200 \
  --interface eth0 \
  --bridge br-pentest
```

**What This Does:**
- Configures an existing VM for isolated networking
- Adds USB passthrough for a Realtek RTL8812AU adapter
- Creates a custom VLAN (200) for network segmentation
- Uses eth0 as the host interface and creates bridge "br-pentest"

### Example 4: Dry Run (Test Configuration Without Applying)

```bash
# Test what the script would do without making changes
sudo ./setup \
  --create-vm \
  --vm test-vm \
  --iso /path/to/test.iso \
  --dry-run \
  --verbose
```

**What This Does:**
- Shows exactly what would be configured without applying changes
- Provides verbose output explaining each step
- Useful for understanding the script's behavior or troubleshooting

### Example 5: Restore VM from Backup

```bash
# List available backups
ls /tmp/vm-backups/

# Restore VM from specific backup
sudo ./setup --restore /tmp/vm-backups/kali-lab_backup_20240315_143022.xml
```

### Example 6: Advanced Custom Network Setup

```bash
# Create multiple isolated networks for complex testing scenarios
sudo ./setup \
  --vm target-network-vm \
  --vlan 300 \
  --interface enp3s0 \
  --bridge br-targets \
  --adapter 2357:0105 \
  --persistent
```

---

## 🌐 Network Architecture

Understanding the network architecture created by this tool is crucial for effective penetration testing and maintaining security isolation.

### Network Topology Overview

When you run this script, it creates a sophisticated network architecture that provides both isolation and functionality:

```
Internet
    │
    └── Host Physical Interface (eth0)
         │
         └── Bridge Interface (br0)
              │
              ├── VLAN Interface (br0.100) ── 192.168.100.1/24
              │    │
              │    └── VM Virtual Interface (eth0) ── 192.168.100.x/24
              │
              └── USB WiFi Adapter ────────────── VM WiFi Interface (wlan0)
                   (Direct Passthrough)
```

### Understanding Each Network Component

**Physical Host Interface**
Your computer's actual network interface (like eth0 or enp3s0) maintains its original configuration and continues to provide internet access to your host system.

**Bridge Interface**
Acts as a virtual network switch that connects multiple network interfaces together. Think of it like a network hub that allows different devices to communicate with each other and share internet access.

**VLAN Interface**
Creates a separate network segment with its own IP range (typically 192.168.100.0/24). This provides isolation between your testing environment and other networks while still allowing controlled internet access.

**VM Virtual Interface**
The network interface inside your virtual machine that connects to the VLAN. It receives an IP address in the VLAN range and can communicate with the internet through NAT.

**USB WiFi Interface**
When using USB passthrough, the WiFi adapter appears directly inside the VM as if it were physically connected to the virtual machine, allowing direct hardware access for wireless security testing.

### Network Security Benefits

**Isolation**: Your penetration testing activities are contained within the VLAN and cannot directly access your host system or other network devices.

**Controlled Internet Access**: VMs can access the internet for updates and tool downloads, but incoming connections are blocked by default.

**Multiple Network Interfaces**: Having both wired (bridged) and wireless (USB passthrough) interfaces allows testing of different network attack vectors.

**Easy Cleanup**: The entire testing network can be torn down without affecting your host system's networking.

### IP Address Allocation

The script automatically configures IP addressing:
- **Gateway**: 192.168.[VLAN_ID].1 (typically 192.168.100.1)
- **VM Range**: 192.168.[VLAN_ID].2-254
- **DNS**: Forwarded through the host system's DNS configuration

---

## 🔌 USB WiFi Adapter Support

USB WiFi adapter passthrough is one of the most powerful features of this tool, enabling direct hardware access for wireless security testing.

### Why USB Passthrough Matters

Traditional virtualization creates virtual network interfaces that cannot perform certain wireless security operations like:
- Monitor mode for packet capture
- Packet injection for testing network defenses  
- Access point creation for rogue AP testing
- Raw 802.11 frame manipulation

USB passthrough solves this by giving the VM direct, exclusive access to the physical USB WiFi adapter, as if it were plugged directly into the virtual machine.

### Supported Adapters Database

The script includes a curated database of WiFi adapters known to work well with penetration testing tools:

**Highly Recommended Adapters:**
- **MediaTek MT7612U** (0e8d:7612) - Excellent monitor mode and injection support
- **Realtek RTL8812AU** (0bda:8812) - Good dual-band performance
- **Atheros AR9271** (0cf3:9271) - Reliable with older tools

**Good Performance Adapters:**
- **TP-Link AC600** (2357:0105) - Good balance of features and price
- **Realtek RTL88x2bu** (0bda:b812) - Decent performance, limited driver support

**Basic Support Adapters:**
- **Ralink MT7601U** (148f:7601) - Monitor mode only, no injection

### Checking Your Current Adapter

```bash
# List all USB devices to find your WiFi adapter
lsusb

# Check if your adapter supports monitor mode (run on host)
iwconfig

# After VM setup, verify adapter in VM
sudo airmon-ng
```

### Adapter Selection Criteria

When choosing a USB WiFi adapter for penetration testing, consider these factors:

**Monitor Mode Support**: Essential for wireless reconnaissance and packet capture. This allows the adapter to listen to all wireless traffic in range, not just traffic directed to it.

**Packet Injection Capability**: Required for active testing techniques like deauthentication attacks, fake access point creation, and WPS testing.

**Dual-Band Support**: 2.4GHz and 5GHz capability allows testing of modern wireless networks that use both frequency bands.

**Driver Stability**: Some adapters have better Linux driver support than others, affecting reliability during extended testing sessions.

**Antenna Design**: External antennas or antenna connectors allow for better signal reception and the ability to use directional antennas for targeted testing.

### Troubleshooting USB Passthrough

**Adapter Not Detected in VM:**
```bash
# Check if adapter is attached to VM
virsh dumpxml your-vm-name | grep -A 5 -B 5 hostdev

# Verify USB device IDs
lsusb | grep -i wireless
```

**Driver Issues in VM:**
```bash
# Check if adapter is recognized in VM
lsusb
iwconfig
dmesg | grep -i firmware
```

**Permission Problems:**
```bash
# Ensure user is in libvirt group
groups $USER

# Check libvirt service status
sudo systemctl status libvirtd
```

---

## ⚙️ Advanced Configuration

For experienced users who need to customize the setup for specific environments or testing scenarios.

### Custom Network Configurations

**Multiple VLAN Setup:**
```bash
# Create isolated networks for different testing phases
sudo ./setup --vm reconnaissance-vm --vlan 101 --bridge br-recon
sudo ./setup --vm exploitation-vm --vlan 102 --bridge br-exploit  
sudo ./setup --vm post-exploit-vm --vlan 103 --bridge br-post
```

**Custom IP Ranges:**
```bash
# Modify the script to use custom IP ranges
# Edit the script and change the IP assignment section:
# ip addr add "10.0.${vlan}.1/24" dev "$vlan_interface"
```

### Performance Optimization

**CPU Pinning for Dedicated Cores:**
```bash
# Edit VM XML to pin specific CPU cores
virsh edit your-vm-name

# Add CPU pinning configuration:
# <vcpu placement='static' cpuset='2-3'>2</vcpu>
```

**Memory Hugepages:**
```bash
# Configure hugepages for better memory performance
echo 1024 > /proc/sys/vm/nr_hugepages

# Add to VM configuration:
# <memoryBacking><hugepages/></memoryBacking>
```

### Security Hardening

**Network Isolation Rules:**
```bash
# Create strict iptables rules for lab isolation
sudo iptables -I FORWARD -s 192.168.100.0/24 -d 192.168.1.0/24 -j DROP
sudo iptables -I FORWARD -s 192.168.100.0/24 -d 10.0.0.0/8 -j DROP
```

**VM Resource Limits:**
```bash
# Set CPU and memory limits to prevent resource exhaustion
virsh edit your-vm-name

# Add resource limits:
# <cputune><shares>512</shares></cputune>
# <memtune><hard_limit unit='KiB'>4194304</hard_limit></memtune>
```

### Persistent Configuration Files

The script can create persistent network configurations that survive reboots:

**Netplan Configuration (Ubuntu):**
```yaml
# /etc/netplan/99-vm-isolation.yaml
network:
  version: 2
  ethernets:
    eth0:
      dhcp4: false
  bridges:
    br0:
      interfaces: [eth0]
      dhcp4: true
      parameters:
        stp: false
  vlans:
    br0.100:
      id: 100
      link: br0
      addresses: [192.168.100.1/24]
```

**SystemD-Networkd Configuration:**
```ini
# /etc/systemd/network/99-br0.netdev
[NetDev]
Name=br0
Kind=bridge

# /etc/systemd/network/99-br0.network
[Match]
Name=br0

[Network]
DHCP=yes
IPForward=yes
```

---

## 🛠️ Troubleshooting

Common issues and their solutions, organized by category to help you quickly identify and resolve problems.

### Virtual Machine Issues

**Problem: VM Creation Fails**
```bash
# Check available disk space
df -h /var/lib/libvirt/images/

# Verify libvirt permissions
sudo ls -la /var/lib/libvirt/images/

# Check libvirt logs
sudo journalctl -u libvirtd
```

**Problem: VM Won't Start**
```bash
# Check VM configuration syntax
virsh dumpxml your-vm-name | xmllint --format -

# Verify VM state
virsh list --all

# Check for conflicting processes
ps aux | grep qemu
```

**Problem: Poor VM Performance**
```bash
# Check CPU virtualization extensions
grep -E "(vmx|svm)" /proc/cpuinfo

# Verify KVM is loaded
lsmod | grep kvm

# Check memory allocation
free -h
```

### Network Configuration Issues

**Problem: No Internet Access in VM**
```bash
# Verify bridge configuration
ip addr show br0

# Check routing table
ip route show

# Test NAT configuration
sudo iptables -t nat -L POSTROUTING
```

**Problem: Bridge Interface Not Created**
```bash
# Check network interface status
ip link show

# Verify bridge utilities
which brctl

# Check for conflicting network managers
systemctl status NetworkManager
systemctl status systemd-networkd
```

**Problem: VLAN Interface Issues**
```bash
# Verify VLAN module is loaded  
lsmod | grep 8021q

# Load VLAN module if needed
sudo modprobe 8021q

# Check VLAN interface configuration
ip addr show br0.100
```

### USB Passthrough Problems

**Problem: USB Adapter Not Detected**
```bash
# Verify adapter is connected
lsusb

# Check if adapter is bound to host driver
ls /sys/bus/usb/drivers/

# Force unbind from host (if needed)
echo "1-1.2" > /sys/bus/usb/drivers/usb/unbind
```

**Problem: Adapter Appears in VM But No Driver**
```bash
# Check VM logs for firmware messages
dmesg | grep -i firmware

# Install additional drivers in VM
sudo apt update
sudo apt install firmware-realtek firmware-atheros
```

**Problem: Multiple Adapters Conflict**
```bash
# List all USB devices with vendor:product IDs
lsusb | grep -E "[0-9a-f]{4}:[0-9a-f]{4}"

# Verify correct adapter is being passed through
virsh dumpxml your-vm-name | grep -A 3 -B 3 "vendor id"
```

### Permission and Access Issues

**Problem: Script Requires Root But Fails**
```bash
# Check if running with sudo
id

# Verify libvirt group membership
groups $USER

# Test libvirt access
virsh list
```

**Problem: Can't Access VM Console**
```bash
# Check VNC configuration
virsh dumpxml your-vm-name | grep vnc

# Test VNC connection
virt-viewer your-vm-name

# Alternative: Use serial console
virsh console your-vm-name
```

### Log Analysis and Debugging

**Enable Verbose Logging:**
```bash
# Run script with maximum verbosity
sudo ./setup --verbose --dry-run

# Check system logs
sudo journalctl -f

# Monitor libvirt logs
sudo tail -f /var/log/libvirt/libvirtd.log
```

**Configuration Backup and Recovery:**
```bash
# List available backups
ls -la /tmp/vm-backups/

# Restore from backup if needed
sudo ./setup --restore /tmp/vm-backups/vm-name_backup_timestamp.xml

# Manual backup of current config
virsh dumpxml your-vm-name > vm-backup.xml
```

---

## 🔒 Security Considerations

When setting up penetration testing laboratories, security should be a primary concern. This section covers important security considerations and best practices.

### Network Isolation Principles

**Understand What Isolation Means**: The VLAN-based isolation created by this script prevents direct network access between your testing VMs and your host system or other network devices. However, it's not perfect isolation – sophisticated attacks could potentially break out of this containment.

**Multiple Layers of Protection**: Consider the network isolation as one layer in a defense-in-depth strategy. Additional protections might include running VMs on dedicated hardware, using separate physical networks, or implementing additional firewall rules.

**Internet Access Considerations**: While VMs have internet access for tool updates and research, this also means they could potentially be used to attack external systems. Always ensure you have proper authorization before conducting any security testing that involves external networks.

### VM Security Best Practices

**Regular Snapshots**: Create VM snapshots before conducting potentially dangerous testing. This allows you to quickly restore to a clean state if something goes wrong.

**Dedicated Testing VMs**: Use separate VMs for different types of testing (web application testing, wireless testing, malware analysis) to prevent cross-contamination and maintain organization.

**Resource Monitoring**: Monitor your host system's resources during testing to ensure VMs don't consume all available CPU, memory, or disk space, potentially affecting host system stability.

### Legal and Ethical Considerations

**Authorization Requirements**: Only conduct penetration testing on systems you own or have explicit written permission to test. Unauthorized testing is illegal in most jurisdictions.

**Scope Limitations**: Ensure your testing activities remain within the authorized scope. The network isolation helps prevent accidental testing of unauthorized systems.

**Data Handling**: Be careful about what data you collect during testing and how you store and dispose of it. Consider privacy and confidentiality requirements.

### Host System Protection

**Backup Host Configuration**: Before running the script, create backups of important network configuration files on your host system.

**Monitor Resource Usage**: Keep an eye on CPU, memory, and disk usage to ensure VMs don't impact host system performance.

**Regular Security Updates**: Keep both your host system and VMs updated with the latest security patches.

### USB Security Considerations

**Device Authentication**: Be cautious about which USB devices you pass through to VMs. Only use devices you trust and have verified.

**Firmware Security**: Some USB WiFi adapters can have their firmware modified. Ensure you're using adapters from reputable sources.

**Physical Security**: Remember that USB passthrough gives the VM direct hardware access. Ensure physical security of your testing environment.

---

## 🤝 Contributing

We welcome contributions from the community to help improve this tool and make it more useful for penetration testers and security researchers.

### How to Contribute

**Reporting Issues**: If you encounter bugs or have feature requests, please open an issue on GitHub with detailed information about your system configuration and the problem you're experiencing.

**Code Contributions**: Fork the repository, make your changes, and submit a pull request. Please ensure your code follows the existing style and includes appropriate comments.

**Documentation Improvements**: Help improve this documentation by suggesting clarifications, additional examples, or corrections.

**Adapter Database**: If you've tested additional USB WiFi adapters, please contribute information about their compatibility and performance.

### Development Guidelines

**Code Style**: Follow the existing bash scripting style with clear variable names, comprehensive error checking, and detailed logging.

**Testing**: Test your changes on multiple Linux distributions if possible, and include both interactive and non-interactive usage scenarios.

**Documentation**: Update relevant documentation sections when adding new features or changing existing behavior.

**Backward Compatibility**: Try to maintain compatibility with existing configurations and usage patterns.

### Testing Environments

If you're contributing code, please test in these environments when possible:
- Ubuntu 20.04 LTS and 22.04 LTS
- Debian 11 and 12
- Arch Linux (current)
- Different hardware configurations (Intel/AMD, various memory sizes)

---

## 📄 License

This project is licensed under the MIT License, which means you're free to use, modify, and distribute it, even for commercial purposes, as long as you include the original license and copyright notice.

### MIT License

```
Copyright (c) 2024 0xbv1 | 0xb0rn3

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

## 🎯 Final Thoughts

This tool represents a sophisticated approach to creating isolated penetration testing environments that balance security, functionality, and ease of use. By automating the complex networking and virtualization configuration required for professional security testing, it allows you to focus on learning and improving your security skills rather than fighting with technical setup issues.

Remember that with great power comes great responsibility. The capabilities provided by this tool should be used ethically and legally, with proper authorization for any testing activities. The isolated environment helps protect against accidental damage, but it's not a substitute for proper planning, authorization, and responsible testing practices.

Whether you're a beginner learning the fundamentals of penetration testing or an experienced professional setting up a new lab environment, this tool is designed to grow with your needs and provide a solid foundation for security research and testing.

Happy testing, and remember to always test responsibly! 🔐

---

**Made with ❤️ by [0xb0rn3](https://github.com/0xb0rn3)**

[![Star this repo](https://img.shields.io/github/stars/0xb0rn3/fastdl?style=social)](https://github.com/0xb0rn3/fastdl)
[![Follow @theehiv3](https://img.shields.io/badge/Follow-@theehiv3-E4405F?style=social&logo=instagram)](https://instagram.com/theehiv3)

*If FastDL has been helpful to you, please consider giving it a ⭐ on GitHub!*

</div>
