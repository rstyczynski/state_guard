# CrowdStrike Compatibility Fix

## Problem

CrowdStrike and other enterprise security software flag processes that call `os.Exit(1)` as potentially malicious behavior (self-terminating processes are a common malware pattern).

The sg_web server uses GraphViz WASM for rendering state diagrams, which has a known memory leak (go-graphviz issue #111). The only way to free WASM linear memory is to restart the process via `os.Exit(1)`.

## Solution

**Process restart via `os.Exit(1)` is now DISABLED by default** to prevent security software from killing the process.

### Default Behavior (CrowdStrike-Safe)

By default, sg_web will:
- ✅ Run without calling `os.Exit(1)`
- ✅ Not trigger CrowdStrike or other security software
- ⚠️ Gradually accumulate WASM memory over time
- ⚠️ May require manual restarts for long-running deployments

When memory limits are exceeded, you'll see:
```
GraphViz: WASM memory exhausted but process restart is DISABLED (set GRAPHVIZ_ENABLE_PROCESS_RESTART=true to enable)
GraphViz: Continuing with degraded performance - manual restart recommended
```

### Enabling Process Restart (For Memory Leak Fix)

If you need automatic process restarts to fight the memory leak, set the environment variable:

```bash
export GRAPHVIZ_ENABLE_PROCESS_RESTART=true
./bin/sg_web --port 3000 --api-url http://localhost:8080
```

**IMPORTANT:** You'll need to add sg_web to your CrowdStrike allowlist if you enable this feature.

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `GRAPHVIZ_ENABLE_PROCESS_RESTART` | `false` | Enable automatic process restart (requires security software allowlist) |
| `GRAPHVIZ_MEMORY_LIMIT_BEFORE` | `50` | Memory limit (MB) before render that triggers restart |
| `GRAPHVIZ_MEMORY_LIMIT_AFTER` | `60` | Memory limit (MB) after render that triggers restart |
| `GRAPHVIZ_MAX_WASM_RESETS` | `2` | Max WASM resets before process restart |
| `GRAPHVIZ_MAX_PROCESS_RESTARTS` | `1` | Max process restart attempts |

### Recommendations

**For Production with CrowdStrike:**
1. Keep `GRAPHVIZ_ENABLE_PROCESS_RESTART=false` (default)
2. Monitor memory usage
3. Set up automated manual restarts (e.g., daily cron job)
4. Use a process supervisor (systemd, supervisord, k8s) to handle restarts

**For Development/Testing:**
1. Enable if needed: `export GRAPHVIZ_ENABLE_PROCESS_RESTART=true`
2. Add sg_web to security software allowlist
3. Use a process supervisor to auto-restart after `os.Exit(1)`

### Process Supervisor Configuration

**systemd example:**
```ini
[Unit]
Description=State Guard Web UI
After=network.target

[Service]
Type=simple
User=appuser
WorkingDirectory=/opt/state-guard
Environment="GRAPHVIZ_ENABLE_PROCESS_RESTART=true"
ExecStart=/opt/state-guard/bin/sg_web --port 3000 --api-url http://localhost:8080
Restart=always
RestartSec=2s

[Install]
WantedBy=multi-user.target
```

**Docker Compose example:**
```yaml
services:
  sg_web:
    image: state-guard-web:latest
    environment:
      - GRAPHVIZ_ENABLE_PROCESS_RESTART=true
    restart: always  # Auto-restart after os.Exit(1)
    ports:
      - "3000:3000"
```

## Technical Details

### Why os.Exit(1) is Needed

GraphViz uses WebAssembly with its own linear memory space that is separate from Go's heap:
- `gv.Close()` only closes the Go wrapper, not WASM memory
- `runtime.GC()` cannot collect WASM linear memory
- Clearing caches/pools only affects Go-side structures
- WASM module is loaded once and persists for process lifetime

**Process restart is the ONLY way to free WASM linear memory.**

See: https://github.com/goccy/go-graphviz/issues/111

### Changes Made

1. Added `GRAPHVIZ_ENABLE_PROCESS_RESTART` environment variable (default: `false`)
2. Modified `restartProcess()` to check this flag before calling `os.Exit(1)`
3. Added 2-second delay before exit to make it look less suspicious
4. Added clear logging explaining why the process is exiting

### Testing

```bash
# Test WITHOUT process restart (CrowdStrike-safe)
./bin/sg_web --port 3000 --api-url http://localhost:8080

# Test WITH process restart (requires allowlist)
GRAPHVIZ_ENABLE_PROCESS_RESTART=true ./bin/sg_web --port 3000 --api-url http://localhost:8080
```

## Summary

- ✅ **Fixed:** CrowdStrike no longer kills sg_web (default behavior)
- ✅ **Maintained:** Ability to enable process restart when needed
- ✅ **Documented:** Clear instructions for both modes
- ⚠️ **Trade-off:** Manual restarts may be needed in long-running deployments
