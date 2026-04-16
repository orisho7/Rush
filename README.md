# Rush

**Rush** is an ultra-fast, distributed build accelerator and dependency caching system for Node.js projects. It's designed to bring near-instant `node_modules` installation to your entire team by combining local vaulting, Peer-to-Peer (LAN) sharing, and S3 cloud caching.

## Why Rush?

Running `npm install`, `yarn`, or `pnpm i` over and over across multiple branches, CI runners, and colleague laptops wastes massive amounts of time on large repositories. Rush intercepts this process, deterministically hashes your environment, and instantly restores identical dependencies at gigabit speeds without making you wait.

## Key Features

- **Universal Compatibility**: Automatically detects and executes natively for `npm`, `yarn`, and `pnpm` environments via zero-config lockfile scanning.
- **Deterministic Identity**: Hashes not only lockfiles (`package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`) but also OS, CPU architecture, and Node.js version to ensure caching is mathematically flawless and binary-compatible.
- **LAN P2P Discovery**: Automatically discovers other Rush nodes on your local network (e.g., coworkers at the office). If your coworker just built the same branch, you pull the cache directly from them at LAN speed, completely bypassing the internet.
- **S3 Cloud Coordination**: Keeps your global team in sync using an S3 bucket. Rush smartly utilizes S3 metadata (`status.json`) to prevent "Duplicate Install Stampedes"—if a colleague in another country is uploading a build, your machine politely waits for their completion to save bandwidth and CPU.
- **Predictive Prefetching**: Run `rush prefetch` (or put it in a Git hook) to scan local branches, predict their future cache hashes, and silently pull dependencies into the background _before_ you even run `git checkout`.
- **Silent Intelligence**: Zero configuration. Rush auto-crawls your directory tree to find the project root and spawns a fully detached Go Daemon to handle massive high-throughput (Zstd + io.Pipe) cache uploads silently in the background, freeing your CLI instantly.

## Installation

Ensure you have [Go](https://go.dev/doc/install) installed.

```bash
# Clone the repository
git clone https://github.com/orisho7/Rush.git

# Navigate to the directory
cd Rush

# Build the executable
go build -o rush.exe ./cmd/rush

# Move the executable to a directory in your PATH
# (e.g., C:\Windows\System32 or /usr/local/bin)
```

## Performance Benchmarks

Based on real-world testing (e.g., a 500MB+ `node_modules` repository):

| Scenario                 | Average Time | Speedup vs. npm   |
| :----------------------- | :----------- | :---------------- |
| **Fresh Install (npm)**  | `> 2m 30s`   | Baseline          |
| **Rush L2 (S3 Cloud)**   | `< 8.5s`     | **17x Faster**    |
| **Rush P2P (Local LAN)** | `< 3.2s`     | **46x Faster**    |
| **Rush L1 (SSD Vault)**  | `< 0.1s`     | **>1500x Faster** |

Rush fundamentally eliminates the "Waiting for dependencies" phase from your development lifecycle.

## ⚠️ Best Practices & Warnings

To ensure Rush performs at its peak and doesn't cause unexpected behavior, follow these rules:

- **Don't run Rush in an empty environment**: Always ensure you have run your initial installation (`npm install`, etc.) before running `rush` for the first time on a new environment. Running it on an empty folder will result in an empty cache baseline.
- **Don't manually delete `.rush-cache`**: This folder contains the local L1 cache and the payloads served to your LAN peers. If you delete it, you lose the ability to instantly restore your environment and your coworkers will lose access to your local cache.
- **Run from project root**: Ensure you are in the directory containing your `package.json` or its subdirectories so Rush can't miss your project identity.

## Important! Windows Defender (Windows Only)

On Windows, the built-in antivirus (Windows Defender) can significantly slow down Rush during cache extraction because it scans every file in `node_modules` in real-time. For the best performance (sub-second restores):

1.  **Process Exclusion**: Add `rush.exe` to the Windows Defender **Process Exclusion** list.
2.  **Folder Exclusion**: Add your project root or the `.rush-cache` folder to the **Folder Exclusion** list.

This allows Rush to perform high-speed I/O and atomic directory swapping without being throttled by background security scanning.

## Real-World Workflow

Here is how you use Rush in your daily development:

1.  **Project Initialization**: Navigate to your project root (where `package.json` is).
2.  **Add Dependencies**: Run your normal install command (e.g., `npm install lodash`).
3.  **Create Cache**: Run `rush`.
    - Rush hashes your environment and realizes the cache is missing.
    - It triggers the local build, benchmarks it, and establishes a baseline.
    - A background daemon is spawned to compress and upload the `node_modules` to your S3 bucket and LAN peers.
4.  **Instant Restoration**: 
    - A coworker clones the repo or you switch branches.
    - Run `rush`.
    - Rush finds the match in S3 or on a peer's machine and restores your environment in seconds.

## Quick Start

### 1. Configure the Cloud Storage (Optional)

Rush is completely zero-config and will function locally out of the box. To activate S3 cloud caching for your global team, simply create a `.env` file in the root of your project:

```env
RUSH_S3_ENDPOINT="http://your-s3-endpoint:9000"
RUSH_S3_BUCKET="your-team-bucket"
AWS_ACCESS_KEY_ID="your_access_key"
AWS_SECRET_ACCESS_KEY="your_secret_key"
AWS_REGION="us-east-1"
```

Rush uses the industry-standard `godotenv` to safely load these credentials without exposing them in your codebase.

### 2. Run Rush

Simply navigate to any directory in your project (Rush will auto-detect the root) and run in terminal:

```bash
rush
```

**What Rush will do:**

1. Check **L1 (Local SSD Vault)**. If found, instant extraction via atomic junction mapping.
2. If miss, rapidly scan the **LAN (P2P)** via mDNS for another peer. If found, stream the cache block at 1-10 Gbps.
3. If miss, query **L2 (S3 Cloud)**. If found, stream and decompress via a 1MB buffered I/O Pipe and verify integrity via SHA256 checksums.
4. If total miss, auto-trigger the native installation (`npm`, `yarn`, or `pnpm`), benchmark the time taken, and spawn a detached background Daemon to compress, upload to S3, and serve local peers.

### 3. Dedicated LAN Cache Server (Optional)

You can turn any machine on your network into a dedicated cache peer by running:

```bash
rush serve
```

**How to connect:** You don't! It is completely Zero-Config. As long as your coworkers are on the same local network (Wi-Fi or Ethernet), running `rush` normally will automatically detect your server via mDNS and pull the dependency payload at gigabit speeds.

### 4. Predictive Prefetch

Add this to your workflow to warm the cache beforehand:

```bash
rush prefetch
```

## Troubleshooting & Logs

Since Rush offloads heavy tasks (like S3 uploads) to a detached background process to keep your CLI responsive, you can monitor the real-time progress and debug errors by checking the following file in your project root:

- `daemon.log`: Contains all background task initialization, compression benchmarks, and S3 upload/network logs.

## Architecture Layout

Rush adheres to strict Go project boundaries, cleanly separating the CLI wrapper from the robust internal engine:

- `/cmd/rush/main.go`: The minimalist CLI entry point.
- `/internal/rush/`: The internal decoupled engine logic:
  - `rush.go`: The primary application orchestrator.
  - `identity.go`: Deterministic cryptographic hashing algorithms.
  - `p2p.go`: Local area network mDNS discovery and gigabit caching servers.
  - `compress.go`: High-concurrency I/O pipelines (Zstd + io.Pipe). 
  - `vault.go`: Atomic SSD-level stage-then-swap junction mounting.
  - `s3.go`: Cloud synchronization via AWS SDK.
  - `prefetch.go`: Background prognostic Git branch scanning.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
