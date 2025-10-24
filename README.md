# Cascade 🌊

**Cascade** is a high-performance HTTP/HTTPS caching proxy written in Go, designed specifically for caching package repository mirrors (apt, yum, etc.) and general HTTP traffic. It features intelligent caching, LRU eviction, and proper handling of repository metadata.

## Features

✨ **Core Features:**
- **Universal HTTP/HTTPS Proxy** - Works as a standard HTTP proxy for any client
- **HTTPS CONNECT Tunneling** - Supports CONNECT method for HTTPS passthrough
- **Intelligent Caching** - Smart cache management with configurable TTLs
- **LRU Eviction** - Automatic cleanup when cache size limit is reached
- **Repository-Aware** - Special handling for InRelease, Release, and Packages files
- **Buffered I/O** - Memory-efficient streaming with configurable buffer sizes
- **File Locking** - Proper concurrent access control to prevent corruption
- **Egress Proxy Support** - HTTP and SOCKS5 upstream proxy support
- **Passthrough Rules** - Configurable patterns to bypass caching
- **Header Respect** - Honors Cache-Control headers when configured

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/amirmatini/cascade.git
cd cascade

# Build
make build

# Run
./build/cascade -config config.yaml
```

### Pre-built Binaries

Download from releases page or use the install script:

```bash
# Download latest release
wget https://github.com/amirmatini/cascade/releases/latest/download/cascade-linux-amd64
chmod +x cascade-linux-amd64
sudo mv cascade-linux-amd64 /usr/local/bin/cascade

# Or use install script
curl -fsSL https://raw.githubusercontent.com/amirmatini/cascade/main/install.sh | sudo bash
```

### Using Go Install

```bash
go install github.com/amirmatini/cascade/cmd/cascade@latest
```

## Configuration

Create a `config.yaml` file:

```yaml
server:
  host: "0.0.0.0"
  port: 3142

cache:
  directory: "./cache"
  max_size_gb: 50
  min_file_size_kb: 1       # Don't cache files < 1KB
  max_file_size_mb: 10240   # Don't cache files > 10GB
  default_ttl: 24h
  buffer_size_kb: 32
  respect_headers: true

egress:
  enabled: false
  proxy_type: "http"  # http or socks5
  proxy_url: ""       # e.g., http://proxy.example.com:3128

rules:
  passthrough:
    - "*login*"
    - "*auth*"
    
  special_ttl:
    "*InRelease*": "5m"
    "*Release.gpg*": "5m"
    "*/Release": "30m"
    "*/Packages*": "1h"
    "*.deb": "720h"  # 30 days
    "*.rpm": "720h"
```

## Usage

### As APT Proxy

Configure APT to use Cascade:

```bash
# /etc/apt/apt.conf.d/01proxy
Acquire::http::Proxy "http://localhost:3142";
```

### As YUM Proxy

Configure YUM to use Cascade:

```bash
# /etc/yum.conf
proxy=http://localhost:3142
```

### As General HTTP Proxy

Set environment variables:

```bash
export http_proxy=http://localhost:3142
export https_proxy=http://localhost:3142
```

Or configure your application to use `localhost:3142` as HTTP proxy.

## Advanced Configuration

### Using Upstream Proxy

If you're behind a corporate proxy:

```yaml
egress:
  enabled: true
  proxy_type: "http"
  proxy_url: "http://corporate-proxy:3128"
```

Or with SOCKS5:

```yaml
egress:
  enabled: true
  proxy_type: "socks5"
  proxy_url: "socks5://127.0.0.1:1080"
```

### Custom TTL Rules

Define custom cache durations for specific patterns:

```yaml
rules:
  special_ttl:
    "*/docker/*": "168h"      # Docker images: 7 days
    "*.iso": "720h"           # ISO files: 30 days
    "*api*": "5m"             # API calls: 5 minutes
    "*.tar.gz": "168h"        # Archives: 7 days
```

### Passthrough Patterns

Skip caching for specific URLs:

```yaml
rules:
  passthrough:
    - "*login*"
    - "*auth*"
    - "*.cgi"
  
  https_passthrough:
    - "download.docker.com"
```

**Note:** `passthrough` skips caching for HTTP, `https_passthrough` allows HTTPS CONNECT tunneling for specific hosts.

## Architecture

### How It Works

1. **Request Reception**: Client sends HTTP request through Cascade
2. **Cache Lookup**: Cascade checks if the resource is cached and valid
3. **Cache Hit**: Serves from cache, updates access time
4. **Cache Miss**: Fetches from origin, streams to client while caching
5. **LRU Eviction**: When cache is full, automatically removes least recently used items

## Performance

- **Fast Hashing**: Uses FNV-1a hash (not SHA256) for cache keys - ~10x faster
- **File Size Filtering**: Configurable limits to skip tiny and huge files
- **Buffered I/O**: Configurable buffer size (default 32KB) prevents memory bloat
- **Streaming**: Files are streamed during caching, no full memory load
- **Concurrent Safe**: File locking prevents corruption during concurrent access
- **Efficient Storage**: Uses directory sharding (first 2 chars of hash)

## Development

### Building

```bash
make build
```

### Running Tests

```bash
make test
```

### Cleaning

```bash
make clean
```

## Repository-Specific Behaviors

### Debian/Ubuntu (APT)

- **InRelease**: 5 minutes (signature verification)
- **Release.gpg**: 5 minutes (signature)
- **Release**: 30 minutes (metadata)
- **Packages/Sources**: 1 hour (package lists)
- **.deb files**: 30 days (actual packages)

### RedHat/CentOS (YUM/DNF)

- **repodata/repomd.xml**: Low TTL recommended
- **.rpm files**: 30 days (actual packages)

## Use Cases

- **CI/CD Pipelines**: Reduce build times by caching dependencies
- **Development Teams**: Share package cache across team
- **Air-Gapped Networks**: Pre-populate cache for offline use
- **Bandwidth Optimization**: Reduce external bandwidth usage
- **Mirror Servers**: Build custom caching mirror infrastructure

## License

MIT License - See LICENSE file for details

---

**Cascade** - Cache And Storage Content ADaptively Engine

