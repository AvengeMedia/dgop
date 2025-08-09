# DankGop

System monitoring tool with CLI and REST API.

Can be used standalone, or as a  companion for [DankMaterialShell](https://github.com/AvengeMedia/DankMaterialShell) to unlock system information functionality.

## Quick Start

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
# See all your shit at once
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

Because I got tired of parsing htop output and wanted something that just works without XML configuration files or enterprise bullshit.