## **Rush**

Rush is a fast, cross-platform dependency caching tool for Node.js projects.
It avoids repeating `npm install` by reusing dependencies across machines.

---

## **What it does**

When you run:

```bash
rush
```

Rush:

1. Checks your local cache (L1)
2. If not found, checks a shared remote cache (L2)
3. If not found, runs the install once
4. Saves the result for future use

---

## **Why it matters**

Installing dependencies repeatedly is slow:

* switching branches
* fresh clones
* CI pipelines
* multiple developers

Rush removes this repetition.

Same dependencies → same result → reuse instead of reinstall.

---

## **Example**

```text
npm install   → 2m 30s
rush          → 8.5s (remote cache)
rush          → <0.1s (local cache)
```

---

## **How it works**

Rush creates a deterministic identity for your environment using:

* lockfile (`package-lock.json`, `yarn.lock`, etc.)
* Node.js version
* OS and CPU architecture

If two environments match, Rush safely reuses the same dependencies.

---

## **Features**

* **Cross-platform**: Native support for Windows, Linux, and macOS.
* **Multi-manager**: Works with `npm`, `yarn`, and `pnpm`.
* **Zero-config**: Automatically finds your project root and lockfiles.
* **L1 Cache**: Ultra-fast local restores using system junctions/symlinks.
* **L2 Cache**: Distributed team and CI reuse via S3-compatible storage.
* **P2P Streaming**: Ultra-fast LAN cache sharing between local nodes.
* **Atomic**: Integrated locking prevents corrupted states.

---

## **Getting started**

1. Build the binary:
```bash
git clone https://github.com/orisho7/Rush.git
cd Rush
go build -o rush cmd/rush/main.go
```

2. Run inside your project:
```bash
./rush
```
OR

```bash
rush
```

---

## **Optional: Remote cache (team use)**

Create a `.env` file in your project root:

```env
RUSH_S3_ENDPOINT="http://your-s3-endpoint:9000"
RUSH_S3_BUCKET="your-bucket"
AWS_ACCESS_KEY_ID="..."
AWS_SECRET_ACCESS_KEY="..."
```

---

## **When to use Rush**

* large `node_modules`
* slow installs
* multiple developers
* CI pipelines

---

## **Notes**

* First run behaves like a normal install.
* Later runs reuse cached results.
* Cache is based on environment identity, not guesses.

---

## **Summary**

Rush replaces repeated installs with cached restores.

Less waiting. Same result.
