# Story 6.3: HTTP Service Probes

Status: done

## Story

As an AI agent,
I want to perform HTTP/HTTPS requests against services from within the cluster using ephemeral pods,
so that I can diagnose service health, response codes, TLS issues, and response times.

## Acceptance Criteria

1. The probe_http tool deploys an ephemeral pod running `curl` with write-out format to capture status code, timing, and SSL verification result
2. URL input is validated against shell metacharacters to prevent injection
3. HTTP method is validated against a whitelist (GET, POST, HEAD, PUT, DELETE, PATCH, OPTIONS)
4. Custom headers are sanitized, rejecting any containing shell metacharacters
5. Status code is parsed to integer for severity mapping: 2xx=OK, 4xx=Warning, 5xx=Critical
6. Response body snippet is truncated to 1KB
7. The tool is always registered (not conditional on CRD discovery)

## Tasks / Subtasks

- [x] Task 1: Implement ProbeHTTPTool (AC: 1, 2, 3, 4, 5, 6)
  - [x] Create struct with BaseTool and ProbeManager fields
  - [x] Define InputSchema: url (required), method (default: GET), headers, source_namespace, timeout_seconds
  - [x] Validate URL with containsShellMeta check, return MCPError on shell characters
  - [x] Validate method against probeAllowedMethods whitelist, fallback to "GET" on invalid
  - [x] Cap timeout_seconds at 30
  - [x] Build curl command with `-s -o /tmp/body -w '%{http_code}|%{time_total}|%{ssl_verify_result}'`
  - [x] Add `-X {method}`, `--max-time {timeout}`, `-L` (follow redirects)
  - [x] Parse semicolon-separated headers, sanitize each with containsShellMeta, add as `-H` flags
  - [x] Append body extraction: `head -c 1024 /tmp/body` for 1KB truncation
- [x] Task 2: Implement response parsing (AC: 5, 6)
  - [x] Split curl output on `|` to extract status_code, time_total, ssl_verify_result
  - [x] Extract body snippet from `---BODY---` marker
  - [x] Truncate body snippet to 1024 bytes with "...(truncated)" suffix
  - [x] Parse status code to int: >= 500 = SeverityCritical, >= 400 = SeverityWarning, else SeverityOK
  - [x] Handle connection errors (status "000") as SeverityCritical
- [x] Task 3: Register tool in main.go (AC: 7)
  - [x] Register ProbeHTTPTool with ProbeManager reference
  - [x] Registered unconditionally (always available)

## Dev Notes

### Key Design Decisions

- **curl write-out format**: `-w '%{http_code}|%{time_total}|%{ssl_verify_result}'` captures the three most useful diagnostic signals in a parseable pipe-delimited format. Writing body to `/tmp/body` separates status metadata from response content.
- **Shell metacharacter validation**: `containsShellMeta` checks for `'"` `` ` `` `;|&$(){}[]<>!\#~` in URLs and headers. This prevents shell injection since the curl command is assembled as a shell string. Individual headers are also validated.
- **Method whitelist with silent fallback**: Invalid methods silently become GET rather than erroring, matching the DNS tool pattern of forgiving input handling.
- **Body truncation at two levels**: First via `head -c 1024` in the probe pod (network-side), then via string slicing in Go (safety net). The `---BODY---` marker separates curl metadata from body content.
- **Status code 000 as failure**: curl returns 000 when the connection itself fails (DNS error, connection refused, timeout). This is treated as Critical severity with connection troubleshooting suggestions.
- **Follow redirects**: `-L` flag ensures the probe follows HTTP redirects, giving the final destination status rather than 301/302.

### Files Modified

| File | Action |
|---|---|
| `pkg/tools/probes.go` | Added ProbeHTTPTool implementation (HTTP section) |
| `cmd/server/main.go` | Registered ProbeHTTPTool with ProbeManager |

## Dev Agent Record

### Agent Model Used
claude-opus-4-6

### Completion Notes List
- All three probe tools (connectivity, DNS, HTTP) share the same probes.go file
- probeAllowedMethods and containsShellMeta are shared validation helpers
- The curl command writes body to /tmp/body because readOnlyRootFilesystem is true in the pod spec; /tmp is typically a tmpfs mount
- SSL verify result is captured but not yet used for specific TLS diagnostics

### File List
- pkg/tools/probes.go
- cmd/server/main.go
