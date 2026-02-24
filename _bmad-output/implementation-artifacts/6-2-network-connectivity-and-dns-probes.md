# Story 6.2: Network Connectivity and DNS Probes

Status: done

## Story

As an AI agent,
I want to test TCP connectivity and DNS resolution from within the cluster using ephemeral pods,
so that I can diagnose network reachability and name resolution issues between services and namespaces.

## Acceptance Criteria

1. The probe_connectivity tool deploys an ephemeral pod running `nc -z` to test TCP connectivity to a target host and port
2. The probe_dns tool deploys an ephemeral pod running `nslookup` to resolve a hostname with a specified record type
3. Input validation rejects hostnames with invalid characters using a regex whitelist
4. DNS record types are validated against a whitelist (A, AAAA, SRV, CNAME, MX, TXT, NS, PTR)
5. Findings use SeverityOK on success and SeverityCritical on failure with actionable suggestions
6. Both tools are always registered (not conditional on CRD discovery)

## Tasks / Subtasks

- [x] Task 1: Implement input validation helpers (AC: 3, 4)
  - [x] Define `validHostname` regex: `^[a-zA-Z0-9._-]+$` matching DNS names, IPs, and K8s service FQDNs
  - [x] Define `validRecordTypes` whitelist map: A, AAAA, SRV, CNAME, MX, TXT, NS, PTR
  - [x] Define `containsShellMeta` helper checking for shell metacharacters
- [x] Task 2: Implement ProbeConnectivityTool (AC: 1, 3, 5)
  - [x] Create struct with BaseTool and ProbeManager fields
  - [x] Define InputSchema: source_namespace, target_host (required), target_port (required), timeout_seconds
  - [x] Validate target_host against validHostname regex, return MCPError on mismatch
  - [x] Cap timeout_seconds at 30
  - [x] Build probe command: `nc -z -w {timeout} {host} {port} && echo 'CONNECTION_SUCCESS' || echo 'CONNECTION_FAILED'`
  - [x] Check output for CONNECTION_SUCCESS to determine finding severity (OK vs Critical)
  - [x] Include duration and suggestion text for failed probes
- [x] Task 3: Implement ProbeDNSTool (AC: 2, 3, 4, 5)
  - [x] Create struct with BaseTool and ProbeManager fields
  - [x] Define InputSchema: hostname (required), source_namespace, record_type (default: A)
  - [x] Validate hostname against validHostname regex
  - [x] Validate record_type against whitelist, fallback to "A" on invalid
  - [x] Build probe command: `nslookup -type={recordType} {hostname} 2>&1; echo EXIT_CODE=$?`
  - [x] Detect failure via "server can't find" or "NXDOMAIN" in output
  - [x] Include CoreDNS troubleshooting suggestion for failed lookups
- [x] Task 4: Register tools in main.go (AC: 6)
  - [x] Register ProbeConnectivityTool and ProbeDNSTool with ProbeManager reference
  - [x] Both registered unconditionally (always available)

## Dev Notes

### Key Design Decisions

- **Hostname regex validation**: `^[a-zA-Z0-9._-]+$` allows standard DNS names and K8s service FQDNs (e.g., `my-service.default.svc.cluster.local`) while blocking shell injection. This is deliberately restrictive rather than permissive.
- **Record type whitelist**: Invalid record types silently fall back to "A" rather than returning an error, making the tool more forgiving for agents.
- **nc -z for connectivity**: Uses netcat's zero-I/O mode which tests TCP connection without sending data. The exit code combined with echo markers makes success/failure parsing reliable.
- **nslookup over dig**: `nslookup` is more universally available in minimal container images. Output parsing checks for both "server can't find" and "NXDOMAIN" strings.
- **Category constants**: Uses `types.CategoryConnectivity` and `types.CategoryDNS` for finding categorization.
- **Source namespace defaults to config.ProbeNamespace**: When not specified, probes run in the configured probe namespace rather than requiring the agent to always specify one.

### Files Modified

| File | Action |
|---|---|
| `pkg/tools/probes.go` | Created ProbeConnectivityTool and ProbeDNSTool with validation helpers |
| `cmd/server/main.go` | Registered both tools with ProbeManager |

## Dev Agent Record

### Agent Model Used
claude-opus-4-6

### Completion Notes List
- Both tools use ProbeManager.Execute for the full ephemeral pod lifecycle
- Timeout is capped at 30 seconds to prevent long-running probes
- Connection success/failure is distinguished by output markers, not just exit code
- DNS failure detection uses string matching on nslookup-specific error messages

### File List
- pkg/tools/probes.go
- cmd/server/main.go
