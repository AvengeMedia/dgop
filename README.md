# dankgop

<div align=center>

![GitHub last commit](https://img.shields.io/github/last-commit/AvengeMedia/dankgop?style=for-the-badge&labelColor=101418&color=9ccbfb)
![GitHub License](https://img.shields.io/github/license/AvengeMedia/dankgop?style=for-the-badge&labelColor=101418&color=b9c8da)
![GitHub Release](https://img.shields.io/github/v/release/AvengeMedia/dankgop?style=for-the-badge&labelColor=101418&color=a6da95)
[![AUR](https://img.shields.io/aur/version/dankgop?style=for-the-badge&labelColor=101418&color=f5a97f)](https://aur.archlinux.org/packages/dankgop)

</div>

<div align="center">
<img src="https://github.com/user-attachments/assets/397d5bb3-cac3-4c09-9ebc-afc7533c433b" width="600" alt="dankgop" />
</div>

System monitoring tool with CLI and REST API.

Can be used standalone, or as a  companion for [DankMaterialShell](https://github.com/AvengeMedia/DankMaterialShell) to unlock system information functionality.

## Installation

### Latest Release
Download the latest binary from [GitHub Releases](https://github.com/AvengeMedia/dankgop/releases/latest)

### Arch Linux (AUR)
```bash
# Using yay
yay -S dankgop

# Using paru  
paru -S dankgop
```

### Build from Source
```bash
# Build it
make

# Install system-wide
sudo make install

# Or just run locally
go run ./cmd/cli [command]
```

## Basic Commands

```bash
# See all at once
dankgop all

# Just CPU info
dankgop cpu

# Memory usage
dankgop memory

# Network interfaces
dankgop network

# Disk usage and mounts
dankgop disk

# Running processes (sorted by CPU usage)
dankgop processes

# System load and uptime
dankgop system

# Hardware info (BIOS, motherboard, etc)
dankgop hardware

# GPU information
dankgop gpu

# Get temperature for specific GPU
dankgop gpu-temp --pci-id 10de:2684

# List available modules
dankgop modules
```

## Meta Command

Mix and match any modules you want:

```bash
# Just CPU and memory
dankgop meta --modules cpu,memory

# Everything except processes
dankgop meta --modules cpu,memory,network,disk,system,hardware,gpu

# GPU with temperatures
dankgop meta --modules gpu --gpu-pci-ids 10de:2684

# Multiple GPU temperatures
dankgop meta --modules gpu --gpu-pci-ids 10de:2684,1002:164e

# Everything (same as 'dankgop all')
dankgop meta --modules all
```

## JSON Output

Add `--json` to any command:

```bash
dankgop cpu --json
dankgop meta --modules gpu,memory --json
```

## Process Options

```bash
# Sort by memory instead of CPU
dankgop processes --sort memory

# Limit to top 10
dankgop processes --limit 10

# Skip CPU calculation for faster results
dankgop processes --no-cpu

# Combine options
dankgop meta --modules processes --sort memory --limit 20 --no-cpu
```

## API Server

Start the REST API:

```bash
dankgop server
```

Then hit these endpoints:

- **GET** `/gops/cpu` - CPU info
- **GET** `/gops/memory` - Memory usage  
- **GET** `/gops/network` - Network interfaces
- **GET** `/gops/disk` - Disk usage
- **GET** `/gops/processes?sort_by=memory&limit=10` - Top 10 processes by memory
- **GET** `/gops/system` - System load and uptime
- **GET** `/gops/hardware` - Hardware info
- **GET** `/gops/gpu` - GPU information
- **GET** `/gops/gpu/temp?pciId=10de:2684` - GPU temperature
- **GET** `/gops/modules` - List available modules
- **GET** `/gops/meta?modules=cpu,memory&gpu_pci_ids=10de:2684` - Dynamic modules

API docs: http://localhost:63484/docs

## Examples

### Get GPU temps for both your cards
```bash
dankgop meta --modules gpu --gpu-pci-ids 10de:2684,1002:164e
```

### Monitor system without slow CPU calculations
```bash
dankgop meta --modules cpu,memory,network --no-cpu
```

### API: Get CPU and memory as JSON
```bash
curl http://localhost:63484/gops/meta?modules=cpu,memory
```

### API: Get GPU with temperature
```bash
curl "http://localhost:63484/gops/meta?modules=gpu&gpu_pci_ids=10de:2684"
```

## Real-time Monitoring with Sampling

dankgop supports cursor-based sampling for building real-time monitoring tools like htop. Instead of relying on instantaneous snapshots, you can track system state changes over time for more accurate CPU usage calculations.

The sampling system works by:
- Taking an initial measurement that establishes baseline CPU times and process ticks
- Returning a cursor object containing the current state and timestamp
- Using that cursor data in subsequent calls to calculate precise usage percentages over the sampling interval

This approach accounts for the actual time elapsed between measurements, making it ideal for monitoring tools that poll every few seconds. Process CPU usage is normalized per single core, and system CPU usage reflects the overall load across all cores.

```bash
# First call - establishes baseline
dankgop meta --modules cpu,processes --json > baseline.json

# Wait 5 seconds, then use cursor data for accurate measurements
sleep 5
dankgop meta --modules cpu,processes --json \
  --cpu-sample '{"previousTotal":[1690,1,391,53692,23,233,44,0],"timestamp":1754784779057}' \
  --proc-sample '[{"pid":1234,"previousTicks":93,"timestamp":1754784779057}]'
```

## Development

```bash
# Build
make

# Run tests
make test

# Format code
make fmt

# Build and install
make && sudo make install

# Clean build artifacts
make clean
```

## Requirements

- Go 1.22+
- Linux (uses `/proc`, `/sys`, and system commands)
- Optional: `nvidia-smi` for NVIDIA GPU temperatures
- Optional: `lspci` for GPU detection

## Why Another Monitoring Tool?

Because nothing did what i wanted, i didnt want to run a metrics server, I wanted GO because its fast and compiles to a single binary, bash scripts got too messy.

TL;DR single binary cli and server with json output, openapi spec, and a bunch of data.
