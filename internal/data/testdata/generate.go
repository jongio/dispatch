//go:build ignore

// generate.go creates a fake session-store SQLite database for demos and tests.
// Run with: go run ./internal/data/testdata/generate.go
package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// Schema — mirrors the production Copilot CLI session-store exactly.
// ---------------------------------------------------------------------------

const ddl = `
CREATE TABLE sessions (
	id TEXT PRIMARY KEY,
	cwd TEXT,
	repository TEXT,
	branch TEXT,
	summary TEXT,
	created_at TEXT DEFAULT (datetime('now')),
	updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE turns (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id TEXT NOT NULL REFERENCES sessions(id),
	turn_index INTEGER NOT NULL,
	user_message TEXT,
	assistant_response TEXT,
	timestamp TEXT DEFAULT (datetime('now')),
	UNIQUE(session_id, turn_index)
);

CREATE TABLE checkpoints (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id TEXT NOT NULL REFERENCES sessions(id),
	checkpoint_number INTEGER NOT NULL,
	title TEXT,
	overview TEXT,
	history TEXT,
	work_done TEXT,
	technical_details TEXT,
	important_files TEXT,
	next_steps TEXT,
	created_at TEXT DEFAULT (datetime('now')),
	UNIQUE(session_id, checkpoint_number)
);

CREATE TABLE session_files (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id TEXT NOT NULL REFERENCES sessions(id),
	file_path TEXT NOT NULL,
	tool_name TEXT,
	turn_index INTEGER,
	first_seen_at TEXT DEFAULT (datetime('now')),
	UNIQUE(session_id, file_path)
);

CREATE TABLE session_refs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id TEXT NOT NULL REFERENCES sessions(id),
	ref_type TEXT NOT NULL,
	ref_value TEXT NOT NULL,
	turn_index INTEGER,
	created_at TEXT DEFAULT (datetime('now')),
	UNIQUE(session_id, ref_type, ref_value)
);

CREATE VIRTUAL TABLE search_index USING fts5(
	content,
	session_id UNINDEXED,
	source_type UNINDEXED,
	source_id UNINDEXED
);

CREATE TABLE schema_version (version INTEGER NOT NULL);
INSERT INTO schema_version (version) VALUES (1);

CREATE INDEX idx_sessions_cwd ON sessions(cwd);
CREATE INDEX idx_sessions_repo ON sessions(repository);
CREATE INDEX idx_turns_session ON turns(session_id);
CREATE INDEX idx_checkpoints_session ON checkpoints(session_id);
CREATE INDEX idx_session_files_path ON session_files(file_path);
CREATE INDEX idx_session_refs_type_value ON session_refs(ref_type, ref_value);
`

// ---------------------------------------------------------------------------
// Fake data — 35 sessions across ~14 repos, video-optimized.
// Timestamps are relative to now so sessions populate every time filter.
// ---------------------------------------------------------------------------

type session struct {
	id, cwd, repo, branch, summary string
	ageMinutes                     int // minutes before now
	durationMinutes                int // session length
}

type turn struct {
	sessionID string
	idx       int
	user, bot string
	offsetMin int // minutes after session start
}

type checkpoint struct {
	sessionID                                                         string
	num                                                               int
	title, overview, history, workDone, techDetails, files, nextSteps string
	offsetMin                                                         int
}

type file struct {
	sessionID, path, tool string
	turnIdx               int
	offsetMin             int
}

type ref struct {
	sessionID, refType, refValue string
	turnIdx                      int
	offsetMin                    int
}

var sessions = []session{
	// ── 0–15 minutes ago (2 sessions) ───────────────────────────────────────
	// ses-001 uses a real GUID so we can create a launchable Copilot session folder.
	{"fa800b7b-3a24-4e3b-9f2d-a414198b27ab", `D:\code\project-alpha\api`, "empire/death-star-api", "feature/auth",
		"Added authentication middleware for Death Star API with JWT validation and rate limiting",
		5, 30},
	{"ses-026", `D:\code\project-alpha\dashboard`, "starfleet/ops-dashboard", "main",
		"Deployed real-time fleet status widget to the ops dashboard homepage",
		12, 20},

	// ── 15–60 minutes ago (3 sessions) ──────────────────────────────────────
	{"ses-002", `D:\code\project-alpha\api`, "empire/death-star-api", "main",
		"Refactored superlaser charging sequence to use worker pool pattern",
		22, 45},
	{"ses-003", `D:\code\project-alpha\dashboard`, "starfleet/ops-dashboard", "develop",
		"Added real-time warp factor metrics with Grafana dashboard and alerting",
		38, 40},
	{"ses-004", `C:\Users\developer\projects\aperture-science`, "aperture/test-chamber-framework", "feat/cake-promise-api",
		"Added cake promise API endpoint with fulfillment tracking (the cake is NOT a lie)",
		52, 25},

	// ── 1–6 hours ago (4 sessions) ────────────────────────────────────────
	{"ses-005", `D:\code\project-alpha\api`, "empire/death-star-api", "fix/login",
		"Fixed bug in Sith login flow causing 500 errors when dark-side tokens expire mid-session",
		95, 60},
	{"ses-006", `D:\code\project-beta\enrollment`, "hogwarts/enrollment-system", "feature/auth",
		"Replaced sorting hat rule engine with GPT-4 fine-tuned model for authentication and house assignment",
		175, 120},
	{"ses-027", `C:\Users\developer\projects\webapp`, "mycompany/webapp", "feature/auth",
		"Implemented comprehensive authentication middleware with JWT token validation and refresh token rotation",
		250, 90},
	{"ses-007", `D:\code\vibranium-tracker`, "wakanda/vibranium-supply-chain", "feat/kimoyo-bead-sync",
		"Built Kimoyo bead mesh network sync protocol for real-time vibranium inventory",
		340, 90},

	// ── 6–24 hours ago (5 sessions) ───────────────────────────────────────
	{"ses-008", `D:\code\project-beta\platformer`, "nintendo/mushroom-kingdom", "feat/double-jump",
		"Implemented double jump with coyote time and input buffering for tight controls",
		420, 60},
	{"ses-009", `D:\code\project-alpha\dashboard`, "starfleet/ops-dashboard", "main",
		"Set up Prometheus and Grafana monitoring for fleet operations dashboard",
		600, 120},
	{"ses-028", `C:\Users\developer\projects\webapp`, "mycompany/webapp", "main",
		"Refactored database migration scripts for the user management module",
		780, 60},
	{"ses-029", `D:\code\project-alpha\api`, "empire/death-star-api", "develop",
		"Added API endpoint for crew roster CRUD operations with role-based access control",
		960, 90},
	{"ses-010", `C:\Users\developer\projects\dotfiles`, "user/dotfiles", "main",
		"Updated shell configuration and dotfiles with new aliases and PATH entries",
		1200, 30},

	// ── 1–3 days ago (6 sessions) ────────────────────────────────────────
	{"ses-011", `D:\code\project-alpha\dashboard`, "starfleet/ops-dashboard", "feat/replicator-menu",
		"Implemented crew replicator menu with dietary preference filtering (Vulcan-safe mode)",
		1560, 90},
	{"ses-012", `D:\code\project-alpha\dashboard`, "starfleet/ops-dashboard", "fix/memory-leak",
		"Debugged holodeck memory leak — Moriarty kept spawning background goroutines",
		1920, 60},
	{"ses-030", `D:\code\project-alpha\api`, "empire/death-star-api", "fix/memory-leak",
		"Refactored database connection pooling to support read replicas and automatic failover with health checks",
		2400, 120},
	{"ses-013", `D:\code\project-beta\infra`, "mordor/terraform-middle-earth", "main",
		"Added auto-scaling for Mount Doom forge workers based on ring production demand",
		2880, 150},
	{"ses-031", `C:\Users\developer\projects\webapp`, "mycompany/webapp", "develop",
		"Set up deploy pipeline with blue-green deployment strategy and automatic rollback on health check failure",
		3360, 90},
	{"ses-014", `C:\Users\developer\projects\aperture-science`, "aperture/test-chamber-framework", "feat/portal-gun-v2",
		"Upgraded portal rendering to use dual-framebuffer technique for seamless recursion",
		3840, 120},

	// ── 3–7 days ago (5 sessions) ────────────────────────────────────────
	{"ses-015", `D:\code\project-beta\enrollment`, "hogwarts/enrollment-system", "fix/owl-delivery-retry",
		"Fixed owl mail delivery retries — owls were stuck in exponential backoff loops",
		4680, 90},
	{"ses-016", `D:\code\hyrule-inventory`, "hyrule/item-management", "feat/weapon-durability",
		"Implemented weapon durability system with break warnings and auto-swap to next best weapon",
		5760, 90},
	{"ses-017", `D:\code\matrix-sim`, "zion/matrix-simulation", "feat/red-pill-blue-pill",
		"Added red-pill/blue-pill A/B testing framework with reality divergence metrics",
		7200, 120},
	{"ses-032", `D:\code\project-beta\testing`, "aperture/test-chamber-framework", "main",
		"Fixed CSS styling regression in test chamber control panel after the Bootstrap 5 migration",
		8640, 60},
	{"ses-033", `D:\code\project-alpha\api`, "empire/death-star-api", "fix/targeting-query",
		"Performance optimization of superlaser targeting query reducing p99 latency from 2s to 200ms",
		9360, 90},

	// ── 1–2 weeks ago (4 sessions) ───────────────────────────────────────
	{"ses-018", `D:\code\project-beta\platformer`, "nintendo/mushroom-kingdom", "fix/pipe-collision",
		"Fixed pipe collision detection — Mario kept clipping through warp pipes at high velocity",
		10800, 60},
	{"ses-019", `C:\Users\developer\projects\aperture-science`, "aperture/test-chamber-framework", "fix/companion-cube-gc",
		"Fixed companion cube garbage collection — cubes were surviving past test chamber lifecycle",
		12960, 90},
	{"ses-020", `D:\code\project-beta\infra`, "mordor/terraform-middle-earth", "fix/nazgul-dns",
		"Fixed Nazgul service discovery — wraiths kept resolving to wrong IP in the Shire",
		15120, 60},
	{"ses-034", `C:\Users\developer\projects\dotfiles`, "user/dotfiles", "main",
		"Updated terminal themes and Neovim config for improved developer experience",
		18720, 45},

	// ── 2–4 weeks ago (3 sessions) ───────────────────────────────────────
	{"ses-021", `D:\code\jarvis-homelab`, "stark-industries/jarvis-v2", "feat/arc-reactor-monitoring",
		"Fixed heart-shaped herb dosage calculator — was prescribing amounts for Hulk-sized patients",
		21600, 60},
	{"ses-022", `D:\code\hyrule-inventory`, "hyrule/item-management", "fix/korok-seed-overflow",
		"Fixed korok seed counter overflow — collecting seed #32768 caused inventory wipe (int16 moment)",
		28800, 60},
	{"ses-023", `D:\code\you-shall-not-pass`, "middle-earth/firewall-rules", "feat/balrog-blocklist",
		"Implemented Balrog-class threat blocklist for the Khazad-dum gateway firewall",
		34560, 60},

	// ── Older than 4 weeks (3 sessions) ─────────────────────────────────────
	{"ses-024", `D:\code\matrix-sim`, "zion/matrix-simulation", "main",
		"Built bullet-time CPU profiler that visualizes hot paths in slow motion",
		43200, 120},
	{"ses-025", `D:\code\logic-engine`, "starfleet/vulcan-logic-engine", "fix/emotion-leak",
		"Patched emotion leak in logic engine — Pon Farr edge case was bypassing sentiment filters",
		50400, 90},
	{"ses-035", `D:\code\project-beta\api`, "mycompany/api", "develop",
		"Added API endpoint for arc reactor telemetry streaming with WebSocket support",
		60480, 90},
}

var turns = []turn{
	// ── ses-001: authentication middleware (5min ago, 3 turns) ───────────
	{"fa800b7b-3a24-4e3b-9f2d-a414198b27ab", 0, "Add authentication middleware to the Death Star API with JWT validation",
		"I'll create the middleware in `middleware/auth.go` with JWT token validation. Let me first check the existing router setup and identify which endpoints need protection.",
		0},
	{"fa800b7b-3a24-4e3b-9f2d-a414198b27ab", 1, "Make sure the token expiry is enforced — we don't want expired Sith credentials getting through",
		"Good call. I've added strict expiry validation and a custom error response for expired tokens. Also added unit tests covering boundary cases around the expiry window.",
		10},
	{"fa800b7b-3a24-4e3b-9f2d-a414198b27ab", 2, "Add rate limiting too, the rebels keep fuzzing our API",
		"Added token bucket rate limiter at 100 req/s per IP. Also added a `/health` endpoint while I was in the router.",
		25},

	// ── ses-026: fleet status widget (12min ago, 1 turn) ──────────────
	{"ses-026", 0, "Deploy the real-time fleet status widget to the ops dashboard homepage",
		"Done. Widget shows ship count, average warp factor, and active alerts. Uses SSE for live updates. Deployed to staging first, ran smoke tests, then promoted to production.",
		0},

	// ── ses-002: superlaser refactor (22min ago, 2 turns) ─────────────
	{"ses-002", 0, "The superlaser charging sequence is a single 2000-line function. Help me refactor it",
		"I'll extract the charging stages into a pipeline with 3 worker pools: crystal alignment, energy accumulation, and beam focusing. Each pool will use buffered channels.",
		0},
	{"ses-002", 1, "Add a progress callback so the bridge can show charging percentage",
		"Added a `ProgressReporter` interface with real-time percentage updates via SSE. The bridge UI can subscribe to `/superlaser/status` for live updates.",
		30},

	// ── ses-003: warp metrics (38min ago, 2 turns) ──────────────────
	{"ses-003", 0, "Build a Grafana dashboard that shows real-time warp factor for all ships",
		"I'll set up Prometheus scraping from each ship's warp core telemetry endpoint and create a Grafana dashboard with fleet-wide warp factor panels and dilithium crystal consumption.",
		0},
	{"ses-003", 1, "Add an alert for when any ship drops below Warp 1 unexpectedly",
		"Added alert rule: `warp_factor < 1 AND status != 'docked'` with PagerDuty integration. Also added a 'Scotty Mode' button that pages engineering directly.",
		25},

	// ── ses-004: cake promise API (52min ago, 1 turn) ─────────────────
	{"ses-004", 0, "Create a cake promise API endpoint — test subjects should check if their cake promise will be fulfilled",
		"Built `/api/v1/cake/promise` with status tracking: `pending`, `baking`, `fulfilled`, `was_a_lie`. Each promise has a UUID and webhook notification on status change.",
		0},

	// ── ses-005: login bug fix (95min ago, 2 turns) ─────────────────
	{"ses-005", 0, "The Sith login flow is throwing 500 errors when tokens expire mid-session",
		"Looking at the logs, `AuthService.ValidateToken()` panics on nil pointer when the token expiry field is missing. I'll add nil checks and proper error handling for expired tokens.",
		0},
	{"ses-005", 1, "Also add retry with backoff for token refresh",
		"Done. Added exponential backoff with jitter (initial: 500ms, max: 10s, factor: 2). The login flow now gracefully handles expired tokens and auto-refreshes.",
		40},

	// ── ses-006: sorting hat ML (175min ago, 3 turns) ─────────────────
	{"ses-006", 0, "Replace the sorting hat's rule engine with a fine-tuned GPT-4 model for house assignment",
		"I'll create a `SortingService` that sends questionnaire responses to the fine-tuned model. The prompt includes house descriptions and historical sorting data for few-shot context.",
		0},
	{"ses-006", 1, "Add fallback to rule engine if the API is down",
		"Added a fallback chain: GPT-4 → local ONNX model → rule engine. Each tier logs which sorter was used for audit trail compliance (Wizengamot requirement).",
		60},
	{"ses-006", 2, "Can you add a bias check? Last year 60% went to Hufflepuff",
		"Created a `HouseBalancer` that tracks distribution and adjusts the system prompt to favor underrepresented houses. Added Prometheus metrics for house distribution ratios.",
		100},

	// ── ses-027: auth middleware — LONG CONVERSATION (250min ago, 8 turns) ──
	{"ses-027", 0, "I need to build authentication middleware for our webapp. We're using Go with chi router. Should support JWT with refresh tokens",
		"I'll start by creating the middleware structure. We need: JWT validation middleware, token refresh endpoint, and a token store. Let me scaffold the files first.",
		0},
	{"ses-027", 1, "Looks good. Can you implement the JWT validation part first?",
		"Created `middleware/auth.go` with JWT validation. It extracts the Bearer token from the Authorization header, validates the signature using RS256, checks expiry, and injects claims into the request context.",
		10},
	{"ses-027", 2, "Now add the refresh token endpoint. Refresh tokens should be stored in an HTTP-only cookie",
		"Added `POST /auth/refresh` endpoint. The refresh token is stored in an HTTP-only, Secure, SameSite=Strict cookie. On refresh, it validates the refresh token, generates a new access token (15min TTL) and rotates the refresh token (7 day TTL).",
		25},
	{"ses-027", 3, "What about token revocation? If a user logs out, both tokens should be invalidated",
		"Added a token blacklist using Redis with TTL matching the token's remaining lifetime. The `POST /auth/logout` endpoint adds both tokens to the blacklist. The JWT middleware checks the blacklist before accepting any token.",
		35},
	{"ses-027", 4, "We need role-based access control too. Admin, editor, and viewer roles",
		"Created a `RequireRole` middleware that chains after `JWTAuth`. Roles are embedded in the JWT claims as a `roles` array. Applied to routes: admin panel requires `admin`, content endpoints require `editor` or `admin`.",
		50},
	{"ses-027", 5, "Add tests for all the auth flows — happy path, expired token, wrong role, revoked token",
		"Added comprehensive test suite with 14 test cases covering: valid token, expired token, malformed token, missing header, wrong signing algorithm, role authorization, revoked token, and concurrent token rotation. Coverage: 94%.",
		60},
	{"ses-027", 6, "The tests look great. Can you add rate limiting specifically for the auth endpoints?",
		"Added per-IP rate limiting for auth endpoints: login (5 req/min to prevent brute force), refresh (10 req/min), and logout (30 req/min). Uses a sliding window counter in Redis.",
		75},
	{"ses-027", 7, "Perfect. Let's also add structured logging for all auth events for the security audit trail",
		"Added structured logging using `slog` for all auth events: login success/failure (with IP, user agent), token refresh, logout, and rate limit hits. Each log entry includes a correlation ID. Sensitive data is never logged.",
		85},

	// ── ses-007: kimoyo bead sync (340min ago, 2 turns) ───────────────
	{"ses-007", 0, "Build a mesh network sync protocol for Kimoyo beads to track vibranium inventory",
		"I'll implement a CRDT-based sync protocol using operation-based counters. Each bead maintains a local replica and syncs via BLE mesh. Conflict resolution uses vector clocks.",
		0},
	{"ses-007", 1, "Add encryption — we can't have Klaue intercepting vibranium shipment data",
		"Added end-to-end encryption using vibranium-lattice-based post-quantum cryptography (VL-PQC). Key exchange happens during the Wakandan handshake protocol.",
		60},

	// ── ses-008: double jump (420min ago, 1 turn) ───────────────────
	{"ses-008", 0, "Implement double jump with coyote time (6 frames) and input buffering (4 frames)",
		"Implemented in `physics/jump.go`. Coyote time lets players jump 6 frames after leaving a platform edge. Input buffer queues jump presses 4 frames before landing.",
		0},

	// ── ses-009: fleet monitoring (600min ago, 2 turns) ───────────────
	{"ses-009", 0, "Set up full observability for the fleet operations dashboard",
		"Deployed Prometheus with custom `fleet_ops_*` metrics: ship_count, warp_factor_avg, alert_count. Grafana dashboard has real-time gauges, historical trends, and captain's log annotations.",
		0},
	{"ses-009", 1, "Add an alert for if any ship goes offline unexpectedly",
		"Added tiered alerts: WARNING at 1 ship offline (Slack), CRITICAL at 3+ ships (PagerDuty + bridge klaxon), EMERGENCY at fleet-wide outage (auto-engages red alert).",
		80},

	// ── ses-028: database migration (780min ago, 2 turns) ─────────────
	{"ses-028", 0, "We need to refactor the database migration scripts for the user management module",
		"I've audited the existing migrations. Found 3 issues: duplicate column additions, a missing index on `users.email`, and an orphaned migration referencing a dropped table. I'll consolidate.",
		0},
	{"ses-028", 1, "Make sure we have rollback scripts for each migration",
		"Added corresponding down migrations for all 12 migration files. Each rollback is tested in CI. Also added a `migrate:status` command showing applied and pending migrations.",
		45},

	// ── ses-029: crew roster API (960min ago, 2 turns) ────────────────
	{"ses-029", 0, "Build CRUD API endpoints for the crew roster with proper role checks",
		"Created RESTful endpoints under `/api/v1/crew`: GET (list with pagination), GET /:id, POST (admin only), PUT /:id (admin/editor), DELETE /:id (admin only). All endpoints validate input with JSON schema.",
		0},
	{"ses-029", 1, "Add filtering and sorting to the list endpoint",
		"Added query params: `rank`, `department`, `species` for filtering, `sort_by` with `order` (asc/desc). Also added full-text search via `q` param. Cursor-based pagination.",
		60},

	// ── ses-010: dotfiles update (1200min ago, 1 turn) ────────────────
	{"ses-010", 0, "Update my shell config with new project aliases and fix the PATH for Go toolchain",
		"Updated `.zshrc` with project aliases, added Go 1.22 to PATH, and created a `dev-setup.sh` script that bootstraps the complete dev environment.",
		0},

	// ── ses-011: replicator menu (1560min ago, 2 turns) ───────────────
	{"ses-011", 0, "We need a food replicator menu system with dietary preference filtering",
		"I'll build a React frontend with faceted search. Filters: species-safe (Vulcan, Klingon, Betazoid), meal type, calorie range, and comfort food mode.",
		0},
	{"ses-011", 1, "Add a 'Tea, Earl Grey, Hot' quick-order button for the captain",
		"Added customizable quick-order toolbar. The captain's profile auto-pins 'Tea, Earl Grey, Hot' with Ctrl+T. Also added Worf's prune juice preset.",
		60},

	// ── ses-012: holodeck memory leak (1920min ago, 2 turns) ────────────
	{"ses-012", 0, "The holodeck is leaking memory badly. We're at 98% RAM utilization",
		"Running pprof... Found it. Moriarty NPC spawns goroutines for each plan and never cleans up. 47,000 leaked goroutines right now.",
		0},
	{"ses-012", 1, "Kill the leaking goroutines and add a safeguard",
		"Added context-based lifecycle management for all NPC goroutines. Each holodeck session has a parent context that cancels on exit. Goroutine budget: 50/NPC.",
		40},

	// ── ses-030: database pooling — LONG CONVERSATION (2400min ago, 10 turns) ─
	{"ses-030", 0, "We need to refactor the database connection pooling. Single pool is hitting limits during peak traffic",
		"I see you're using `database/sql` with a single pool configured for max 25 connections. During peak, you serve 500+ concurrent requests. That's your bottleneck. I'll design a read-replica-aware pool.",
		0},
	{"ses-030", 1, "Yes, we have 2 read replicas. How should we split the traffic?",
		"I'll create a `DBPool` wrapper that routes queries based on operation type. Writes go to the primary, reads are distributed round-robin across replicas using `context.Context` to carry a ReadOnly flag.",
		15},
	{"ses-030", 2, "What about connection health checks? We've had issues where replicas go down",
		"Added health checking with configurable intervals. Each replica gets pinged every 5s. Unhealthy replicas are removed from round-robin and re-added after 3 consecutive successful pings.",
		30},
	{"ses-030", 3, "We also need automatic failover. If primary goes down, a replica should be promoted",
		"Implemented automatic failover with 10s detection window. If primary fails 3 consecutive health checks, the healthiest replica (lowest latency) is promoted to primary. Slack alert fires on failover.",
		45},
	{"ses-030", 4, "How do we handle in-flight transactions during failover?",
		"In-flight transactions on the old primary receive connection errors. Added retry logic at the service layer: failed transactions are retried once on the new primary. Returns `ErrFailoverInProgress` if retry also fails.",
		55},
	{"ses-030", 5, "Add connection pool metrics so we can monitor this in Grafana",
		"Added Prometheus metrics: `db_pool_connections_active`, `db_pool_query_duration_seconds` (histogram), `db_pool_errors_total`, `db_pool_failover_total`. Created Grafana dashboard with connection utilization and latency percentiles.",
		70},
	{"ses-030", 6, "Now add graceful connection draining when a replica is removed",
		"Added graceful drain with 30s window. When replica is marked unhealthy: stop new queries, wait for in-flight to complete (up to 30s), close pool. Added `db_pool_drain_duration_seconds` metric.",
		85},
	{"ses-030", 7, "Write comprehensive tests for the failover scenarios",
		"Added 22 test cases covering: normal routing, round-robin, health check failure/recovery, primary failover with in-flight txns, connection draining, concurrent failover, and metrics. Coverage: 91%.",
		95},
	{"ses-030", 8, "Can you add support for read-your-writes consistency?",
		"Added `ReadYourWrites` context option. Reads within the same request that performed a write are routed to primary instead of replicas. Uses request-scoped flag via context, not global state.",
		105},
	{"ses-030", 9, "Perfect. Let's do a final review of the complete implementation",
		"Summary: Primary + N replicas with round-robin reads, 5s health pings, auto-failover with 10s detection, 30s graceful drain, optional read-your-writes, 6 Prometheus metrics, 22 tests at 91% coverage. Ready for review.",
		115},

	// ── ses-013: mount doom autoscale (2880min ago, 2 turns) ────────────
	{"ses-013", 0, "Set up auto-scaling for the Mount Doom forge based on ring production queue depth",
		"I'll configure a custom HPA metric based on `forge_queue_depth`. Scale range: 3-20 forge workers, cooldown: 5 minutes.",
		0},
	{"ses-013", 1, "Add a PodDisruptionBudget — last time we scaled down mid-forge, a ring came out cursed",
		"Added PDB with minAvailable: 2 and a `forging-in-progress` annotation that prevents scale-down during active ring casting.",
		90},

	// ── ses-031: deploy pipeline — LONG CONVERSATION (3360min ago, 6 turns) ──
	{"ses-031", 0, "Set up a deploy pipeline for our webapp. We want blue-green deployments with automatic rollback",
		"I'll create a GitHub Actions workflow with blue-green deployment to Kubernetes: build → test → deploy to green → health check → switch traffic → monitor → auto-rollback if error rate spikes.",
		0},
	{"ses-031", 1, "Start with the build and test stages",
		"Created `.github/workflows/deploy.yml` with: checkout, Go build with race detector, unit tests with coverage gate (>80%), integration tests, golangci-lint, and gosec security scan.",
		15},
	{"ses-031", 2, "Now add the blue-green deployment stage",
		"Added Kubernetes deployment stage: creates green deployment alongside blue, waits for Ready pods, runs smoke tests against green ClusterIP. If smoke tests pass, updates Service selector to green.",
		30},
	{"ses-031", 3, "Add the automatic rollback logic",
		"Implemented rollback monitor: after traffic switch, watches error rate and p99 via Prometheus for 5 minutes. If error rate > 1% or p99 > 500ms, auto-switches back to blue and sends Slack alert.",
		50},
	{"ses-031", 4, "Add manual approval gate for production deploys",
		"Added environment protection rule: production deployments require approval from 1 team lead. Staging auto-approves. Approval request includes commit range, test results, and diff link.",
		65},
	{"ses-031", 5, "Create a rollback command for manual emergency rollbacks",
		"Added `make rollback` and workflow_dispatch trigger. Lists last 5 deployments with SHA, timestamp, health status. Operator selects version to roll back to. Added `deploy:status` command.",
		80},

	// ── ses-014: portal gun v2 (3840min ago, 2 turns) ─────────────────
	{"ses-014", 0, "Upgrade the portal rendering to handle recursive portals without infinite loops",
		"I'll use a dual-framebuffer approach: render from each portal's perspective into offscreen FBOs with recursion depth limit of 8. Each level halves the resolution.",
		0},
	{"ses-014", 1, "Add unit tests for the edge case where both portals face each other",
		"Added tests for face-to-face portals, 90-degree angles, and portals on moving surfaces. Recursion limit prevents infinite loops, resolution halving bounds GPU memory.",
		70},

	// ── ses-015: owl delivery (4680min ago, 2 turns) ──────────────────
	{"ses-015", 0, "Owl mail delivery keeps retrying forever. Some owls have been flying in circles for weeks",
		"The retry loop has no max attempt limit. I'll add a dead-letter queue for undeliverable mail and cap retries at 5 with exponential backoff.",
		0},
	{"ses-015", 1, "Also log the owl ID so we can track individual delivery failures",
		"Added structured logging with owl_id, recipient, and attempt_number. Failed deliveries after 5 retries get routed to the Howler escalation queue.",
		60},

	// ── ses-016: weapon durability (5760min ago, 2 turns) ───────────────
	{"ses-016", 0, "Implement weapon durability. Weapons show warnings at 25% and auto-swap when broken",
		"Created `DurabilitySystem` with per-weapon hit counters. At 25% a screen-edge flash warns. At 0% the weapon shatters and auto-swaps to highest-DPS weapon.",
		0},
	{"ses-016", 1, "The Master Sword should be unbreakable",
		"Added `Indestructible` trait that bypasses durability checks. Applied to Master Sword and `legendary` weapons. Added repair mechanics at Great Fairy Fountains.",
		50},

	// ── ses-017: red pill blue pill (7200min ago, 2 turns) ──────────────
	{"ses-017", 0, "Build an A/B testing framework themed around the red pill / blue pill choice",
		"Created `PillTest` framework: red-pill cohort sees reality, blue-pill sees control. Metrics: engagement, reality_divergence_score, existential_crisis_rate.",
		0},
	{"ses-017", 1, "Add a chi-squared significance test for the results",
		"Added statistical analysis module with chi-squared test, p-value calculation, and confidence intervals. Results page shows Matrix-style green-text visualization.",
		80},

	// ── ses-032: CSS styling fix (8640min ago, 2 turns) ─────────────────
	{"ses-032", 0, "The test chamber control panel looks broken after Bootstrap 5 migration. CSS styling is all wrong",
		"Found issues: Bootstrap 5 dropped `.btn-default` (now `.btn-secondary`), removed jQuery breaking custom dropdowns, and grid changed from `.col-xs-*` to `.col-*`. I'll fix all class names.",
		0},
	{"ses-032", 1, "Also fix the dark theme — the contrast ratios are failing WCAG checks",
		"Updated dark theme CSS custom properties: increased text contrast to 4.8:1 (WCAG AA), fixed focus ring visibility, updated button hover states. All 12 WCAG AA checks pass.",
		40},

	// ── ses-033: performance optimization (9360min ago, 2 turns) ──────────
	{"ses-033", 0, "The superlaser targeting query is way too slow. P99 is 2 seconds",
		"Profiled with EXPLAIN ANALYZE. Bottleneck: sequential scan on `targets` table (2.3M rows) with missing index on `sector_id`. Adding composite index and rewriting the JOIN should fix it.",
		0},
	{"ses-033", 1, "Did the optimization work? What are the new numbers?",
		"P99 dropped from 2.1s to 187ms (91% improvement). Added composite index on `(sector_id, priority, status)`, rewrote subquery as CTE, added 30s result caching for non-critical targeting data.",
		60},

	// ── ses-018: pipe collision (10800min ago, 2 turns) ─────────────────
	{"ses-018", 0, "Mario clips through pipes at high velocity. I think it's a tunneling issue",
		"Classic CCD problem. At velocities above 12 px/frame, Mario passes through the 16px pipe collision box. I'll add swept AABB collision detection.",
		0},
	{"ses-018", 1, "Make sure this doesn't break existing moving platform collisions",
		"Added regression tests for all 12 platform types. Swept AABB handles moving platforms correctly. All 47 existing physics tests pass.",
		35},

	// ── ses-019: companion cube GC (12960min ago, 2 turns) ──────────────
	{"ses-019", 0, "Companion cubes aren't being garbage collected after test chambers end",
		"The `CompanionCube` struct holds a reference to the chamber's event bus preventing GC. I'll switch to weak references and add a `Dispose()` method.",
		0},
	{"ses-019", 1, "Add a finalizer warning if a cube is GC'd without Dispose being called",
		"Added `runtime.SetFinalizer` that logs a warning with stack trace if `Dispose()` wasn't called. Added cube lifecycle dashboard: active/disposed/leaked counts.",
		60},

	// ── ses-020: nazgul DNS (15120min ago, 1 turn) ───────────────────
	{"ses-020", 0, "The Nazgul keep showing up in the Shire instead of Mordor. DNS issue?",
		"Found it — service mesh was using round-robin DNS and Shire had lowest latency. Switched to locality-aware routing with region affinity for `mordor-east-1`.",
		0},

	// ── ses-034: terminal themes (18720min ago, 1 turn) ─────────────────
	{"ses-034", 0, "Update my Neovim config and terminal theme for better readability",
		"Updated to Catppuccin Mocha theme. Added Treesitter, configured LSP for Go and TypeScript, added telescope.nvim for fuzzy finding. Updated tmux status bar to match.",
		0},

	// ── ses-021: herb dosage (21600min ago, 1 turn) ───────────────────
	{"ses-021", 0, "The heart-shaped herb dosage calculator is giving doses for Hulk-sized patients",
		"Weight was in grams but formula expected kilograms. A 75kg person got 75,000x the intended dose. Fixed unit conversion and added input validation with min/max bounds.",
		0},

	// ── ses-022: korok seed overflow (28800min ago, 1 turn) ─────────────
	{"ses-022", 0, "Collecting korok seed #32768 wipes the entire inventory. Players are furious",
		"Classic integer overflow. Seed counter is int16 (max 32767). Seed 32768 wraps to -32768, triggering 'reset inventory' debug command. Changed to int32 and removed debug backdoor.",
		0},

	// ── ses-023: balrog blocklist (34560min ago, 1 turn) ────────────────
	{"ses-023", 0, "Create a firewall rule system that blocks Balrog-class threats at Khazad-dum gateway",
		"Built `BalrogBlocklist` using IP reputation scoring. Threats classified: balrog (block), orc (rate limit), troll (challenge-response). Rules in YAML with `you-shall-not-pass` severity.",
		0},

	// ── ses-024: bullet time profiler (43200min ago, 2 turns) ─────────────
	{"ses-024", 0, "Build a CPU profiler that visualizes hot paths in slow motion, Matrix bullet-time style",
		"I'll hook into runtime/pprof and create a TUI flamegraph with time-dilation playback. Hot paths render in green, cold in gray. Supports 0.1x to 10x speed.",
		0},
	{"ses-024", 1, "Add export to SVG for sharing",
		"Added SVG export with interactive tooltips: function duration, call count, allocation stats. Matrix green palette by default, with 'Morpheus Mode' dark theme.",
		80},

	// ── ses-025: emotion leak (50400min ago, 2 turns) ─────────────────
	{"ses-025", 0, "The Vulcan logic engine has an emotion leak. Pon Farr edge case bypassing the sentiment filter",
		"The sentiment filter uses a blocklist but Pon Farr emotions have unique spectral signatures not in training data. I'll add a dedicated Pon Farr detector using temporal pattern matching.",
		0},
	{"ses-025", 1, "Add unit tests, Spock would insist on full coverage",
		"Added 42 tests (naturally). Coverage includes all 7 Pon Farr phases, mixed-species edge cases, and meditation cooldown verification. `TestLogic_PonFarr_Phase3_EmotionContainment`.",
		60},

	// ── ses-035: arc reactor WebSocket (60480min ago, 2 turns) ────────────
	{"ses-035", 0, "Build a WebSocket API endpoint for streaming arc reactor telemetry data",
		"Created `/ws/telemetry` endpoint with gorilla/websocket. Streams reactor metrics at configurable intervals (default 1s). Added auth via query param token and 100 client limit.",
		0},
	{"ses-035", 1, "Add backpressure handling — slow clients shouldn't block the telemetry stream",
		"Added per-client write buffer (64 messages). If client falls behind, oldest messages drop and a `gaps` counter is sent. Clients can request full state snapshot via `sync` message type.",
		60},
}

var checkpoints = []checkpoint{
	// ses-001: auth middleware (1 checkpoint)
	{"fa800b7b-3a24-4e3b-9f2d-a414198b27ab", 1, "Auth middleware complete",
		"Added JWT authentication middleware with rate limiting for Death Star API.",
		"User requested authentication middleware with security hardening.",
		"Created middleware, added rate limiting, wrote tests. All validation rules in place.",
		"Go, chi router, JWT RS256, token-bucket rate limiter, 100 req/s/IP",
		"middleware/auth.go, middleware/ratelimit.go, middleware/auth_test.go",
		"Add integration tests, deploy to staging",
		28},

	// ses-002: superlaser refactor (1 checkpoint)
	{"ses-002", 1, "Superlaser pipeline refactored",
		"Extracted 2000-line charging function into 3-stage worker pool pipeline with progress reporting.",
		"Refactored monolithic superlaser charging sequence into maintainable pipeline.",
		"Worker pools for crystal alignment, energy accumulation, beam focusing. SSE progress updates.",
		"Go channels, worker pool pattern, SSE, buffered channels",
		"services/superlaser/pipeline.go, services/superlaser/workers.go, services/superlaser/progress.go",
		"Load test with full power charging sequence, add cancellation support",
		40},

	// ses-005: login fix (1 checkpoint)
	{"ses-005", 1, "Login flow fixed",
		"Fixed nil pointer panic in token validation and added retry with exponential backoff.",
		"Investigated and fixed 500 errors in Sith login flow during token expiry.",
		"Added nil checks, proper error handling, exponential backoff with jitter for token refresh.",
		"Go, JWT, exponential backoff, jitter, error handling",
		"services/auth.go, services/auth_test.go",
		"Monitor error rates post-deploy, add alerting for auth failures",
		55},

	// ses-006: sorting hat ML (2 checkpoints)
	{"ses-006", 1, "Sorting hat ML integration",
		"Replaced rule engine with GPT-4 model and added fallback chain.",
		"Migrated from deterministic rules to ML-based house assignment.",
		"SortingService with GPT-4, ONNX fallback, and rule engine backup.",
		"OpenAI API, ONNX Runtime, Prometheus, gRPC fallback",
		"services/sorting.go, models/sorting_hat.onnx, config/houses.yaml",
		"A/B test with incoming first-years",
		60},
	{"ses-006", 2, "House distribution balancer",
		"Added HouseBalancer to correct Hufflepuff bias with Prometheus monitoring.",
		"Created bias correction system for house assignment distribution.",
		"HouseBalancer adjusts system prompt based on real-time distribution ratios.",
		"Prometheus client_golang, statistical balancing",
		"services/balancer.go, monitoring/house-metrics.yaml",
		"Monitor distribution ratios over full academic year",
		110},

	// ses-009: fleet monitoring (2 checkpoints)
	{"ses-009", 1, "Fleet monitoring deployed",
		"Prometheus + Grafana observability stack for fleet operations.",
		"Deployed monitoring for Starfleet operations dashboard.",
		"Custom fleet_ops metrics, Grafana dashboards, real-time gauges.",
		"Prometheus, Grafana 10.x, SSE for real-time updates",
		"monitoring/fleet-ops.yaml, grafana/dashboards/fleet.json",
		"Add dilithium crystal degradation prediction model",
		40},
	{"ses-009", 2, "Tiered alerting configured",
		"Added WARNING/CRITICAL/EMERGENCY alert tiers with auto-remediation.",
		"Configured multi-tier alert system for fleet operations.",
		"Tiered alerts: Slack (WARNING), PagerDuty (CRITICAL), red alert (EMERGENCY).",
		"PagerDuty webhook, Slack API, custom alert controller",
		"alerts/fleet-tiers.yaml, controllers/red-alert.go",
		"Add predictive alerting based on historical patterns",
		100},

	// ses-011: replicator menu (1 checkpoint)
	{"ses-011", 1, "Replicator menu system",
		"React frontend with faceted search, dietary filters, and quick-order toolbar.",
		"Built food replicator menu with species-safe dietary preference filtering.",
		"Faceted search, species filters, quick-order with keyboard shortcuts.",
		"React, TypeScript, faceted search, keyboard shortcuts",
		"src/components/ReplicatorMenu.tsx, src/components/QuickOrder.tsx",
		"Add nutritional analysis integration, expand species compatibility database",
		70},

	// ses-012: holodeck memory leak (2 checkpoints)
	{"ses-012", 1, "Goroutine leak identified",
		"Found Moriarty NPC spawning unbounded goroutines. 47,000 leaked goroutines.",
		"Debugged memory leak caused by unbounded NPC goroutine spawning.",
		"pprof analysis, identified root cause in NPC replication logic.",
		"Go runtime, pprof, goroutine analysis",
		"holodeck/npc.go, holodeck/session.go",
		"Apply fix and verify memory stabilization",
		20},
	{"ses-012", 2, "Lifecycle management added",
		"Added context-based goroutine lifecycle and per-NPC budget of 50.",
		"Implemented goroutine lifecycle management and budget system.",
		"Parent context per session, SetFinalizer warnings, goroutine budget.",
		"context.Context, runtime.SetFinalizer, goroutine pools",
		"holodeck/npc.go, holodeck/session.go, holodeck/npc_test.go, holodeck/monitor.go",
		"Investigate other holodeck programs for similar leaks",
		50},

	// ses-013: mount doom autoscale (2 checkpoints)
	{"ses-013", 1, "HPA configured",
		"Custom HPA with forge_queue_depth metric. Scale range 3-20 workers.",
		"Set up Kubernetes auto-scaling for ring production.",
		"HPA manifest with custom Prometheus metric adapter.",
		"Kubernetes HPA v2, custom-metrics-apiserver, Prometheus adapter",
		"k8s/hpa.yaml, monitoring/forge-metrics.yaml",
		"Load test with simulated ring production spike",
		60},
	{"ses-013", 2, "PDB and safety guardrails",
		"Added PodDisruptionBudget to prevent mid-forge scale-down.",
		"Added safety guarantees for ring forging operations.",
		"PDB with minAvailable: 2, forging-in-progress annotation check.",
		"PodDisruptionBudget, custom annotations, admission webhook",
		"k8s/pdb.yaml, k8s/forge-webhook.yaml",
		"Verify PDB prevents mid-forge scale-down under load",
		120},

	// ses-015: owl delivery (2 checkpoints)
	{"ses-015", 1, "Retry logic fixed",
		"Capped retries at 5 with exponential backoff. Added dead-letter queue.",
		"Fixed infinite retry loop in owl mail delivery system.",
		"Max 5 retries, exponential backoff, dead-letter queue for failures.",
		"Exponential backoff, dead-letter queue pattern",
		"services/owl_mail.go, services/dead_letter.go",
		"Monitor dead-letter queue volume",
		30},
	{"ses-015", 2, "Structured logging added",
		"Added owl_id, recipient, attempt tracking. Howler escalation queue for failures.",
		"Added observability for individual owl delivery tracking.",
		"Structured logging with correlation IDs, Howler escalation after 5 failures.",
		"slog, structured logging, escalation queues",
		"services/owl_mail.go, services/owl_mail_test.go",
		"Build owl delivery dashboard in Grafana",
		70},

	// ses-017: red pill blue pill (2 checkpoints)
	{"ses-017", 1, "A/B framework built",
		"PillTest cohort assignment with reality-divergence tracking.",
		"Built experimentation platform with Matrix theming.",
		"PillTest cohort system, metric collection, Matrix visualizations.",
		"Go statistics library, SSE for live results, terminal rendering",
		"experiment/pill_test.go, viz/matrix_rain.go",
		"Add multi-armed bandit support",
		40},
	{"ses-017", 2, "Statistical analysis",
		"Chi-squared significance testing with confidence intervals.",
		"Added statistical analysis for experiment results.",
		"Chi-squared test, p-value calculation, confidence intervals.",
		"Statistics library, chi-squared distribution",
		"analysis/chi_squared.go, analysis/chi_squared_test.go",
		"Integrate with existing feature flags",
		100},

	// ses-018: pipe collision (2 checkpoints)
	{"ses-018", 1, "CCD analysis",
		"Identified discrete-step tunneling at >12 px/frame velocities.",
		"Diagnosed collision tunneling issue in pipe physics.",
		"Discrete step checking fails at high velocity. 16px pipe box too narrow.",
		"AABB collision detection, continuous collision detection",
		"physics/collision.go",
		"Implement swept AABB solution",
		15},
	{"ses-018", 2, "Swept AABB implemented",
		"Added swept AABB collision with regression tests for all 12 platform types.",
		"Fixed collision tunneling with continuous collision detection.",
		"Swept AABB resolves in velocity-space. All 47 physics tests pass.",
		"Swept AABB, velocity-space resolution, regression testing",
		"physics/collision.go, physics/collision_test.go",
		"Profile performance impact on low-end hardware",
		45},

	// ses-026: fleet status widget (1 checkpoint)
	{"ses-026", 1, "Widget deployed",
		"Real-time fleet status widget with SSE live updates deployed to dashboard.",
		"Deployed fleet status widget showing ship count, warp factor, alerts.",
		"SSE-powered live widget, staged rollout with smoke tests.",
		"Server-Sent Events, React, staged deployment",
		"src/components/FleetStatusWidget.tsx, api/fleet-status.go",
		"Add drill-down to individual ship details",
		15},

	// ses-027: auth middleware — 4 checkpoints (long session)
	{"ses-027", 1, "JWT validation middleware",
		"Created JWT validation with RS256 signature verification and context injection.",
		"Implemented core authentication middleware for webapp.",
		"Bearer token extraction, RS256 validation, expiry check, claims in context.",
		"Go, chi router, JWT, RS256, context.Context",
		"middleware/auth.go",
		"Add refresh token endpoint",
		15},
	{"ses-027", 2, "Refresh tokens and revocation",
		"HTTP-only cookie refresh tokens with Redis-backed revocation blacklist.",
		"Added token lifecycle management with secure refresh and revocation.",
		"Refresh endpoint, HTTP-only cookies, Redis blacklist with TTL.",
		"Redis, HTTP-only cookies, SameSite=Strict, token rotation",
		"middleware/auth.go, handlers/auth_refresh.go, services/token_store.go",
		"Add role-based access control",
		40},
	{"ses-027", 3, "RBAC and test suite",
		"RequireRole middleware with 14 test cases at 94% coverage.",
		"Added role-based access control and comprehensive test suite.",
		"RequireRole middleware chains after JWTAuth. 14 tests cover all auth flows.",
		"RBAC, admin/editor/viewer roles, table-driven tests",
		"middleware/rbac.go, middleware/auth_test.go",
		"Add rate limiting for auth endpoints",
		65},
	{"ses-027", 4, "Rate limiting and audit logging",
		"Per-IP rate limiting on auth endpoints and structured security audit trail.",
		"Hardened auth endpoints with rate limiting and comprehensive logging.",
		"Sliding window rate limiter in Redis, slog structured logging with correlation IDs.",
		"Redis sliding window, slog, correlation IDs, SIEM integration",
		"middleware/ratelimit.go, middleware/audit_log.go",
		"Security review and penetration testing",
		88},

	// ses-028: database migration (2 checkpoints)
	{"ses-028", 1, "Migration audit complete",
		"Found 3 issues: duplicate columns, missing index, orphaned migration.",
		"Audited existing database migration scripts.",
		"Identified duplicate column additions, missing email index, orphaned reference.",
		"SQL migrations, database schema analysis",
		"migrations/005_add_email.sql, migrations/008_add_email_v2.sql",
		"Consolidate and add rollback scripts",
		20},
	{"ses-028", 2, "Migrations consolidated",
		"Consolidated 15 migrations to 12 with rollback scripts and status command.",
		"Cleaned up migration scripts with proper rollback support.",
		"Down migrations for all files, CI-tested rollbacks, migrate:status command.",
		"golang-migrate, CI rollback testing",
		"migrations/*.sql, cmd/migrate/status.go",
		"Run consolidated migrations against staging database",
		55},

	// ses-029: crew roster API (1 checkpoint)
	{"ses-029", 1, "Crew roster CRUD API",
		"RESTful crew roster endpoints with RBAC, filtering, and cursor-based pagination.",
		"Built complete CRUD API for crew management.",
		"GET/POST/PUT/DELETE with JSON schema validation, full-text search, cursor pagination.",
		"Go, chi router, JSON Schema, cursor-based pagination",
		"handlers/crew.go, handlers/crew_test.go, services/crew_search.go",
		"Add batch import endpoint for crew transfers",
		80},

	// ses-030: database pooling — 5 checkpoints (longest session)
	{"ses-030", 1, "Read-replica routing",
		"DBPool wrapper routes writes to primary, reads round-robin across replicas.",
		"Designed read-replica-aware connection pool architecture.",
		"Context-based ReadOnly flag, atomic round-robin counter for replica selection.",
		"database/sql, sync/atomic, context.Context",
		"db/pool.go",
		"Add health checking for replicas",
		20},
	{"ses-030", 2, "Health checking and failover",
		"5s ping interval, 3-strike removal, auto-promotion of healthiest replica.",
		"Added replica health checking and automatic primary failover.",
		"Health checker goroutine, 3-strike policy, failover with latency-based promotion.",
		"Health checking, leader election, Slack alerting",
		"db/health.go, db/failover.go",
		"Handle in-flight transactions during failover",
		50},
	{"ses-030", 3, "Transaction retry and draining",
		"Retry logic for in-flight txns, 30s graceful drain on replica removal.",
		"Added robustness for failover edge cases.",
		"ErrFailoverInProgress sentinel, request-scoped retry, 30s drain window.",
		"Sentinel errors, graceful shutdown, context cancellation",
		"db/pool.go, db/drain.go",
		"Add monitoring metrics",
		70},
	{"ses-030", 4, "Prometheus metrics and Grafana dashboard",
		"6 pool metrics with Grafana dashboard showing utilization and latency.",
		"Added comprehensive observability for connection pool.",
		"connections_active, query_duration histogram, errors_total, failover_total, drain_duration.",
		"Prometheus client_golang, Grafana dashboard JSON",
		"db/metrics.go, grafana/dashboards/db-pool.json",
		"Add read-your-writes consistency",
		90},
	{"ses-030", 5, "Read-your-writes and test suite",
		"Context-based read-your-writes consistency. 22 tests at 91% coverage.",
		"Added consistency guarantees and comprehensive test suite.",
		"ReadYourWrites via request-scoped context flag. 22 tests including failover scenarios.",
		"Context propagation, sqlmock, deterministic failure injection",
		"db/pool.go, db/pool_test.go",
		"Performance benchmarking under production-like load",
		118},

	// ses-031: deploy pipeline — 3 checkpoints
	{"ses-031", 1, "Build and test pipeline",
		"GitHub Actions workflow with build, test (>80% coverage), lint, and security scan.",
		"Created CI pipeline with quality gates for webapp deployment.",
		"Go build with race detector, unit + integration tests, golangci-lint, gosec.",
		"GitHub Actions, Go race detector, golangci-lint, gosec",
		".github/workflows/deploy.yml",
		"Add blue-green deployment stage",
		20},
	{"ses-031", 2, "Blue-green deployment",
		"Kubernetes blue-green with smoke tests and automatic traffic switching.",
		"Implemented zero-downtime deployment strategy.",
		"Green deployment alongside blue, smoke tests, Service selector update.",
		"Kubernetes deployments, Service selectors, smoke testing",
		".github/workflows/deploy.yml, k8s/deployment-green.yaml",
		"Add automatic rollback monitoring",
		45},
	{"ses-031", 3, "Rollback and approval gates",
		"Auto-rollback on error rate spike, manual approval for production, rollback command.",
		"Added safety nets for production deployments.",
		"5-min Prometheus watch, 1% error rate threshold, team lead approval gate.",
		"Prometheus queries, GitHub environment protection, workflow_dispatch",
		".github/workflows/deploy.yml, .github/workflows/rollback.yml, Makefile",
		"Add deployment dashboard and notification improvements",
		85},

	// ses-032: CSS styling (1 checkpoint)
	{"ses-032", 1, "Bootstrap 5 migration fixed",
		"Fixed CSS class name changes and dark theme contrast for WCAG AA compliance.",
		"Resolved CSS styling regression from Bootstrap 5 migration.",
		"Updated btn-default to btn-secondary, col-xs to col, fixed contrast ratios to 4.8:1.",
		"Bootstrap 5, CSS custom properties, WCAG AA, axe audit",
		"src/styles/dark-theme.css, src/components/ControlPanel.tsx",
		"Run full visual regression test suite",
		45},

	// ses-033: performance optimization (2 checkpoints)
	{"ses-033", 1, "Query analysis",
		"EXPLAIN ANALYZE revealed sequential scan on 2.3M row targets table.",
		"Profiled slow superlaser targeting query.",
		"Missing index on sector_id, inefficient JOIN condition, no covering index.",
		"PostgreSQL EXPLAIN ANALYZE, query planning, index analysis",
		"db/targeting_query.sql",
		"Apply optimizations and measure improvement",
		30},
	{"ses-033", 2, "91% latency reduction",
		"P99 from 2.1s to 187ms. Composite index, CTE rewrite, 30s result caching.",
		"Optimized targeting query with multi-layered improvements.",
		"Composite index (sector_id, priority, status), CTE, query cache with TTL.",
		"PostgreSQL indexes, CTEs, application-level caching, slow query logging",
		"db/targeting_query.sql, services/targeting_cache.go, monitoring/slow-queries.yaml",
		"Monitor cache hit rate and query plan stability",
		80},

	// ses-014: portal gun v2 (3 checkpoints)
	{"ses-014", 1, "Dual-framebuffer architecture",
		"Offscreen FBO rendering with 8-level recursion and resolution halving.",
		"Designed recursive portal rendering without infinite loops.",
		"Render from each portal perspective into FBOs, composite with depth limit.",
		"OpenGL 4.5, FBO, dual-buffer compositing",
		"render/portal.go, render/fbo_pool.go",
		"Add edge case tests",
		40},
	{"ses-014", 2, "Portal edge case tests",
		"Tests for face-to-face, 90-degree, and moving-surface portal configurations.",
		"Verified recursive rendering handles all portal orientations.",
		"Face-to-face portals, angular configurations, moving surfaces all handled.",
		"Table-driven tests, GPU memory bounds verification",
		"render/portal_test.go",
		"Profile GPU performance on various hardware",
		80},
	{"ses-014", 3, "Performance profiling",
		"GPU profiling shows 60fps maintained up to recursion depth 6 on mid-range hardware.",
		"Verified rendering performance across hardware tiers.",
		"Benchmark suite for portal rendering. 60fps at depth 6, 30fps at depth 8.",
		"GPU profiling, benchmark suite, hardware tier testing",
		"render/portal_bench_test.go, docs/portal-performance.md",
		"Add portal particle effects",
		110},

	// ses-016: weapon durability (3 checkpoints)
	{"ses-016", 1, "Durability system core",
		"Per-weapon hit counters with 25% warning flash and 0% auto-swap.",
		"Implemented base weapon durability tracking system.",
		"DurabilitySystem with hit counters, screen-edge warning, particle shatter effect.",
		"ECS pattern, particle system, inventory management",
		"inventory/durability.go, inventory/durability_test.go",
		"Add weapon traits system",
		25},
	{"ses-016", 2, "Weapon traits",
		"Indestructible trait for legendary weapons. Master Sword bypasses durability.",
		"Added trait system for special weapon behaviors.",
		"Trait-based weapon modifiers: Indestructible, legendary rarity tag.",
		"Trait pattern, weapon rarity system",
		"inventory/traits.go, inventory/traits_test.go",
		"Add repair mechanics",
		50},
	{"ses-016", 3, "Repair mechanics",
		"Great Fairy Fountain repair with cost scaling based on weapon rarity.",
		"Implemented weapon repair at designated locations.",
		"Repair costs scale with weapon rarity. Legendary weapons repair for free.",
		"Cost calculation, rarity tiers, location-based interactions",
		"inventory/repair.go, inventory/repair_test.go",
		"Balance repair costs through playtesting",
		80},

	// ses-019: companion cube GC (3 checkpoints)
	{"ses-019", 1, "GC root analysis",
		"CompanionCube holds event bus reference preventing garbage collection.",
		"Identified root cause of companion cube memory leak.",
		"Event bus strong reference keeps cube alive past chamber lifecycle.",
		"Go GC analysis, reference graph, pprof heap",
		"entities/companion_cube.go",
		"Implement weak reference pattern",
		20},
	{"ses-019", 2, "Weak references and Dispose",
		"Switched to weak references with explicit Dispose() for event unsubscription.",
		"Fixed GC leak with proper lifecycle management.",
		"Weak reference pattern, Dispose() unsubscribes from all events.",
		"Weak references, disposable pattern, event bus cleanup",
		"entities/companion_cube.go, entities/event_bus.go",
		"Add finalizer warning",
		50},
	{"ses-019", 3, "Finalizer and dashboard",
		"SetFinalizer warns on undisposed cubes. Lifecycle dashboard tracks counts.",
		"Added leak detection and monitoring for companion cubes.",
		"runtime.SetFinalizer logs warning with stack trace. Dashboard: active/disposed/leaked.",
		"runtime.SetFinalizer, lifecycle monitoring, Grafana dashboard",
		"entities/companion_cube.go, entities/companion_cube_test.go, grafana/cubes.json",
		"Extend pattern to other test chamber entities",
		80},

	// ses-024: bullet time profiler (3 checkpoints)
	{"ses-024", 1, "TUI flamegraph",
		"pprof-integrated TUI flamegraph with time-dilation playback (0.1x-10x).",
		"Created novel CPU profiler visualization.",
		"pprof integration, bubbletea TUI, Matrix green color scheme, playback controls.",
		"runtime/pprof, bubbletea TUI, ANSI color rendering",
		"profiler/flamegraph.go, profiler/tui.go, profiler/playback.go",
		"Add SVG export",
		50},
	{"ses-024", 2, "SVG export",
		"Interactive SVG with tooltips showing duration, call count, allocations.",
		"Added shareable SVG export for profiling results.",
		"SVG generation with interactive tooltips, Matrix green palette, Morpheus Mode.",
		"SVG generation, interactive tooltips, CSS themes",
		"profiler/svg_export.go, profiler/themes.go",
		"Add memory allocation visualization",
		90},
	{"ses-024", 3, "Memory visualization",
		"Added heap allocation overlay to flamegraph with per-function allocation stats.",
		"Extended profiler to visualize memory allocation patterns.",
		"Heap profile overlay, allocation hotspot detection, GC pressure indicators.",
		"runtime/pprof heap, allocation tracking, GC metrics",
		"profiler/memory.go, profiler/flamegraph.go",
		"GPU profiling integration",
		115},
}

var files = []file{
	// ses-001: auth middleware
	{"fa800b7b-3a24-4e3b-9f2d-a414198b27ab", "middleware/auth.go", "create", 0, 5},
	{"fa800b7b-3a24-4e3b-9f2d-a414198b27ab", "middleware/auth_test.go", "create", 1, 12},
	{"fa800b7b-3a24-4e3b-9f2d-a414198b27ab", "middleware/ratelimit.go", "create", 2, 26},

	// ses-026: fleet status widget
	{"ses-026", "src/components/FleetStatusWidget.tsx", "create", 0, 5},
	{"ses-026", "api/fleet-status.go", "create", 0, 8},

	// ses-002: superlaser refactor
	{"ses-002", "services/superlaser/pipeline.go", "create", 0, 10},
	{"ses-002", "services/superlaser/workers.go", "create", 0, 15},
	{"ses-002", "services/superlaser/progress.go", "create", 1, 35},

	// ses-003: warp metrics
	{"ses-003", "grafana/dashboards/warp-metrics.json", "create", 0, 10},
	{"ses-003", "prometheus/scrape-targets.yaml", "create", 0, 5},
	{"ses-003", "alerts/warp-alerts.yaml", "create", 1, 30},

	// ses-004: cake promise API
	{"ses-004", "api/cake_promise.go", "create", 0, 5},
	{"ses-004", "api/cake_promise_test.go", "create", 0, 15},

	// ses-005: login fix
	{"ses-005", "services/auth.go", "edit", 0, 10},
	{"ses-005", "services/auth_test.go", "edit", 1, 50},

	// ses-006: sorting hat ML
	{"ses-006", "services/sorting.go", "create", 0, 10},
	{"ses-006", "services/balancer.go", "create", 2, 105},
	{"ses-006", "config/houses.yaml", "create", 0, 5},
	{"ses-006", "models/sorting_hat.onnx", "create", 1, 65},

	// ses-027: auth middleware (long session)
	{"ses-027", "middleware/auth.go", "create", 1, 12},
	{"ses-027", "handlers/auth_refresh.go", "create", 2, 27},
	{"ses-027", "services/token_store.go", "create", 3, 37},
	{"ses-027", "middleware/rbac.go", "create", 4, 52},
	{"ses-027", "middleware/auth_test.go", "create", 5, 62},
	{"ses-027", "middleware/ratelimit.go", "create", 6, 77},
	{"ses-027", "middleware/audit_log.go", "create", 7, 87},

	// ses-007: kimoyo bead sync
	{"ses-007", "sync/crdt.go", "create", 0, 10},
	{"ses-007", "sync/mesh.go", "create", 0, 15},
	{"ses-007", "crypto/vlpqc.go", "create", 1, 65},

	// ses-008: double jump
	{"ses-008", "physics/jump.go", "create", 0, 10},
	{"ses-008", "physics/jump_test.go", "create", 0, 20},

	// ses-009: fleet monitoring
	{"ses-009", "monitoring/fleet-ops.yaml", "create", 0, 10},
	{"ses-009", "grafana/dashboards/fleet.json", "create", 0, 15},
	{"ses-009", "alerts/fleet-tiers.yaml", "create", 1, 85},
	{"ses-009", "controllers/red-alert.go", "create", 1, 90},

	// ses-028: database migration
	{"ses-028", "migrations/005_add_email.sql", "edit", 0, 10},
	{"ses-028", "migrations/008_add_email_v2.sql", "edit", 0, 15},
	{"ses-028", "cmd/migrate/status.go", "create", 1, 50},

	// ses-029: crew roster API
	{"ses-029", "handlers/crew.go", "create", 0, 10},
	{"ses-029", "handlers/crew_test.go", "create", 0, 20},
	{"ses-029", "services/crew_search.go", "create", 1, 65},

	// ses-010: dotfiles
	{"ses-010", ".zshrc", "edit", 0, 5},
	{"ses-010", "dev-setup.sh", "create", 0, 15},

	// ses-011: replicator menu
	{"ses-011", "src/components/ReplicatorMenu.tsx", "create", 0, 10},
	{"ses-011", "src/components/QuickOrder.tsx", "create", 1, 65},

	// ses-012: holodeck memory leak
	{"ses-012", "holodeck/npc.go", "edit", 0, 10},
	{"ses-012", "holodeck/session.go", "edit", 1, 45},
	{"ses-012", "holodeck/npc_test.go", "create", 1, 48},
	{"ses-012", "holodeck/monitor.go", "create", 1, 50},

	// ses-030: database pooling (long session)
	{"ses-030", "db/pool.go", "create", 1, 17},
	{"ses-030", "db/health.go", "create", 2, 32},
	{"ses-030", "db/failover.go", "create", 3, 47},
	{"ses-030", "db/drain.go", "create", 6, 87},
	{"ses-030", "db/metrics.go", "create", 5, 72},
	{"ses-030", "db/pool_test.go", "create", 7, 97},
	{"ses-030", "grafana/dashboards/db-pool.json", "create", 5, 75},

	// ses-013: mount doom autoscale
	{"ses-013", "k8s/hpa.yaml", "create", 0, 10},
	{"ses-013", "k8s/pdb.yaml", "create", 1, 95},
	{"ses-013", "monitoring/forge-metrics.yaml", "create", 0, 15},

	// ses-031: deploy pipeline (long session)
	{"ses-031", ".github/workflows/deploy.yml", "create", 1, 17},
	{"ses-031", "k8s/deployment-green.yaml", "create", 2, 32},
	{"ses-031", ".github/workflows/rollback.yml", "create", 5, 82},
	{"ses-031", "Makefile", "edit", 5, 82},

	// ses-014: portal gun v2
	{"ses-014", "render/portal.go", "edit", 0, 10},
	{"ses-014", "render/fbo_pool.go", "create", 0, 15},
	{"ses-014", "render/portal_test.go", "create", 1, 75},
	{"ses-014", "render/portal_bench_test.go", "create", 1, 100},

	// ses-015: owl delivery
	{"ses-015", "services/owl_mail.go", "edit", 0, 10},
	{"ses-015", "services/dead_letter.go", "create", 0, 15},
	{"ses-015", "services/owl_mail_test.go", "edit", 1, 65},

	// ses-016: weapon durability
	{"ses-016", "inventory/durability.go", "create", 0, 10},
	{"ses-016", "inventory/durability_test.go", "create", 0, 15},
	{"ses-016", "inventory/traits.go", "create", 1, 55},
	{"ses-016", "inventory/repair.go", "create", 1, 70},

	// ses-017: red pill blue pill
	{"ses-017", "experiment/pill_test.go", "create", 0, 10},
	{"ses-017", "analysis/chi_squared.go", "create", 1, 85},
	{"ses-017", "viz/matrix_rain.go", "create", 1, 90},

	// ses-032: CSS styling
	{"ses-032", "src/styles/dark-theme.css", "edit", 0, 10},
	{"ses-032", "src/components/ControlPanel.tsx", "edit", 1, 45},

	// ses-033: performance optimization
	{"ses-033", "db/targeting_query.sql", "edit", 0, 10},
	{"ses-033", "services/targeting_cache.go", "create", 1, 65},
	{"ses-033", "monitoring/slow-queries.yaml", "create", 1, 70},

	// ses-018: pipe collision
	{"ses-018", "physics/collision.go", "edit", 0, 10},
	{"ses-018", "physics/collision_test.go", "edit", 1, 40},

	// ses-019: companion cube GC
	{"ses-019", "entities/companion_cube.go", "edit", 0, 10},
	{"ses-019", "entities/event_bus.go", "edit", 1, 50},
	{"ses-019", "entities/companion_cube_test.go", "create", 1, 65},

	// ses-020: nazgul DNS
	{"ses-020", "istio/destination-rules.yaml", "edit", 0, 10},

	// ses-034: terminal themes
	{"ses-034", "init.lua", "edit", 0, 10},
	{"ses-034", ".tmux.conf", "edit", 0, 15},

	// ses-021: herb dosage
	{"ses-021", "med/herb_dosage.go", "edit", 0, 10},
	{"ses-021", "med/herb_dosage_test.go", "edit", 0, 15},

	// ses-022: korok seed overflow
	{"ses-022", "inventory/seeds.go", "edit", 0, 10},
	{"ses-022", "inventory/seeds_test.go", "edit", 0, 15},

	// ses-023: balrog blocklist
	{"ses-023", "firewall/balrog_blocklist.go", "create", 0, 10},
	{"ses-023", "firewall/rules.yaml", "create", 0, 15},

	// ses-024: bullet time profiler
	{"ses-024", "profiler/flamegraph.go", "create", 0, 10},
	{"ses-024", "profiler/tui.go", "create", 0, 15},
	{"ses-024", "profiler/svg_export.go", "create", 1, 85},
	{"ses-024", "profiler/playback.go", "create", 0, 20},
	{"ses-024", "profiler/memory.go", "create", 1, 110},

	// ses-025: emotion leak
	{"ses-025", "logic/sentiment_filter.go", "edit", 0, 10},
	{"ses-025", "logic/pon_farr_detector.go", "create", 0, 15},
	{"ses-025", "logic/pon_farr_detector_test.go", "create", 1, 65},

	// ses-035: arc reactor WebSocket
	{"ses-035", "handlers/telemetry_ws.go", "create", 0, 10},
	{"ses-035", "handlers/telemetry_ws_test.go", "create", 0, 20},
	{"ses-035", "services/backpressure.go", "create", 1, 65},
}

var refs = []ref{
	// ses-001: auth middleware (3 refs — PR + commit + issue)
	{"fa800b7b-3a24-4e3b-9f2d-a414198b27ab", "pr", "42", 2, 27},
	{"fa800b7b-3a24-4e3b-9f2d-a414198b27ab", "commit", "a1b2c3d", 2, 27},
	{"fa800b7b-3a24-4e3b-9f2d-a414198b27ab", "issue", "41", 0, 0},

	// ses-026: fleet status widget (1 ref)
	{"ses-026", "pr", "99", 0, 15},

	// ses-002: superlaser refactor (2 refs)
	{"ses-002", "pr", "55", 1, 35},
	{"ses-002", "commit", "b2c3d4e", 1, 35},

	// ses-003: warp metrics (1 ref)
	{"ses-003", "pr", "56", 1, 30},

	// ses-004: cake promise API (1 ref)
	{"ses-004", "issue", "10", 0, 0},

	// ses-005: login fix (3 refs)
	{"ses-005", "issue", "17", 0, 0},
	{"ses-005", "pr", "43", 1, 50},
	{"ses-005", "commit", "e4f5a6b", 1, 50},

	// ses-006: sorting hat ML (3 refs)
	{"ses-006", "pr", "101", 2, 105},
	{"ses-006", "issue", "98", 0, 0},
	{"ses-006", "commit", "b7c8d9e", 2, 105},

	// ses-027: auth middleware (5 refs — rich for scroll demo)
	{"ses-027", "issue", "200", 0, 0},
	{"ses-027", "pr", "201", 5, 62},
	{"ses-027", "commit", "c3d4e5f", 5, 62},
	{"ses-027", "pr", "205", 7, 87},
	{"ses-027", "commit", "f6a7b8c", 7, 87},

	// ses-007: kimoyo bead sync (1 ref)
	{"ses-007", "pr", "3", 1, 70},

	// ses-008: double jump (1 ref)
	{"ses-008", "commit", "d4e5f6a", 0, 10},

	// ses-009: fleet monitoring (2 refs)
	{"ses-009", "pr", "1337", 1, 95},
	{"ses-009", "commit", "cafebabe", 1, 95},

	// ses-028: database migration (2 refs)
	{"ses-028", "issue", "150", 0, 0},
	{"ses-028", "pr", "151", 1, 50},

	// ses-029: crew roster API (2 refs)
	{"ses-029", "pr", "160", 1, 65},
	{"ses-029", "commit", "e5f6a7b", 1, 65},

	// ses-010: dotfiles (1 ref)
	{"ses-010", "commit", "f7a8b9c", 0, 10},

	// ses-011: replicator menu (1 ref)
	{"ses-011", "pr", "88", 1, 65},

	// ses-012: holodeck memory leak (2 refs)
	{"ses-012", "issue", "88", 0, 0},
	{"ses-012", "pr", "89", 1, 45},

	// ses-030: database pooling (5 refs — rich for scroll demo)
	{"ses-030", "issue", "300", 0, 0},
	{"ses-030", "pr", "301", 7, 97},
	{"ses-030", "commit", "a8b9c0d", 7, 97},
	{"ses-030", "issue", "305", 4, 55},
	{"ses-030", "pr", "310", 9, 117},

	// ses-013: mount doom autoscale (2 refs)
	{"ses-013", "pr", "7", 1, 100},
	{"ses-013", "commit", "c1d2e3f", 1, 100},

	// ses-031: deploy pipeline (4 refs)
	{"ses-031", "issue", "400", 0, 0},
	{"ses-031", "pr", "401", 2, 32},
	{"ses-031", "commit", "d2e3f4a", 2, 32},
	{"ses-031", "pr", "405", 5, 82},

	// ses-014: portal gun v2 (2 refs)
	{"ses-014", "pr", "15", 1, 75},
	{"ses-014", "commit", "e3f4a5b", 1, 75},

	// ses-015: owl delivery (2 refs)
	{"ses-015", "issue", "112", 0, 0},
	{"ses-015", "pr", "113", 1, 65},

	// ses-016: weapon durability (1 ref)
	{"ses-016", "pr", "20", 1, 55},

	// ses-017: red pill blue pill (1 ref)
	{"ses-017", "pr", "25", 1, 85},

	// ses-032: CSS styling (2 refs)
	{"ses-032", "issue", "500", 0, 0},
	{"ses-032", "pr", "501", 1, 45},

	// ses-033: performance optimization (3 refs)
	{"ses-033", "issue", "600", 0, 0},
	{"ses-033", "pr", "601", 1, 65},
	{"ses-033", "commit", "f4a5b6c", 1, 65},

	// ses-018: pipe collision (2 refs)
	{"ses-018", "issue", "404", 0, 0},
	{"ses-018", "pr", "405", 1, 40},

	// ses-019: companion cube GC (1 ref)
	{"ses-019", "issue", "256", 0, 0},

	// ses-020: nazgul DNS (1 ref)
	{"ses-020", "issue", "700", 0, 0},

	// ses-034: terminal themes (1 ref)
	{"ses-034", "commit", "a5b6c7d", 0, 10},

	// ses-022: korok seed overflow (3 refs)
	{"ses-022", "issue", "32768", 0, 0},
	{"ses-022", "pr", "900", 0, 15},
	{"ses-022", "commit", "deadbeef", 0, 15},

	// ses-023: balrog blocklist (1 ref)
	{"ses-023", "commit", "b6c7d8e", 0, 10},

	// ses-024: bullet time profiler (2 refs)
	{"ses-024", "pr", "30", 1, 85},
	{"ses-024", "commit", "c7d8e9f", 1, 85},

	// ses-025: emotion leak (2 refs)
	{"ses-025", "issue", "7", 0, 0},
	{"ses-025", "pr", "8", 1, 65},

	// ses-035: arc reactor WebSocket (2 refs)
	{"ses-035", "pr", "50", 1, 65},
	{"ses-035", "commit", "d8e9f0a", 1, 65},

	// No refs: ses-021 (intentionally — realistic for quick fixes)
}

func main() {
	_, src, _, _ := runtime.Caller(0)
	dbPath := filepath.Join(filepath.Dir(src), "fake_sessions.db")

	os.Remove(dbPath) // start fresh
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Create schema.
	if _, err := db.Exec(ddl); err != nil {
		log.Fatalf("schema: %v", err)
	}

	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}

	now := time.Now()
	const tsFmt = "2006-01-02 15:04:05"

	// Build a map of session start times for offset calculations.
	sesStart := make(map[string]time.Time)
	for _, s := range sessions {
		sesStart[s.id] = now.Add(-time.Duration(s.ageMinutes) * time.Minute)
	}

	// Sessions
	stmtSes, _ := tx.Prepare(`INSERT INTO sessions (id,cwd,repository,branch,summary,created_at,updated_at) VALUES (?,?,?,?,?,?,?)`)
	for _, s := range sessions {
		created := sesStart[s.id]
		updated := created.Add(time.Duration(s.durationMinutes) * time.Minute)
		if _, err := stmtSes.Exec(s.id, s.cwd, s.repo, s.branch, s.summary, created.Format(tsFmt), updated.Format(tsFmt)); err != nil {
			log.Fatalf("session %s: %v", s.id, err)
		}
	}

	// Turns
	stmtTurn, _ := tx.Prepare(`INSERT INTO turns (session_id,turn_index,user_message,assistant_response,timestamp) VALUES (?,?,?,?,?)`)
	for _, t := range turns {
		ts := sesStart[t.sessionID].Add(time.Duration(t.offsetMin) * time.Minute)
		if _, err := stmtTurn.Exec(t.sessionID, t.idx, t.user, t.bot, ts.Format(tsFmt)); err != nil {
			log.Fatalf("turn %s/%d: %v", t.sessionID, t.idx, err)
		}
	}

	// Checkpoints
	stmtCP, _ := tx.Prepare(`INSERT INTO checkpoints (session_id,checkpoint_number,title,overview,history,work_done,technical_details,important_files,next_steps,created_at) VALUES (?,?,?,?,?,?,?,?,?,?)`)
	for _, c := range checkpoints {
		ts := sesStart[c.sessionID].Add(time.Duration(c.offsetMin) * time.Minute)
		if _, err := stmtCP.Exec(c.sessionID, c.num, c.title, c.overview, c.history, c.workDone, c.techDetails, c.files, c.nextSteps, ts.Format(tsFmt)); err != nil {
			log.Fatalf("checkpoint %s/%d: %v", c.sessionID, c.num, err)
		}
	}

	// Files
	stmtFile, _ := tx.Prepare(`INSERT INTO session_files (session_id,file_path,tool_name,turn_index,first_seen_at) VALUES (?,?,?,?,?)`)
	for _, f := range files {
		ts := sesStart[f.sessionID].Add(time.Duration(f.offsetMin) * time.Minute)
		if _, err := stmtFile.Exec(f.sessionID, f.path, f.tool, f.turnIdx, ts.Format(tsFmt)); err != nil {
			log.Fatalf("file %s/%s: %v", f.sessionID, f.path, err)
		}
	}

	// Refs
	stmtRef, _ := tx.Prepare(`INSERT INTO session_refs (session_id,ref_type,ref_value,turn_index,created_at) VALUES (?,?,?,?,?)`)
	for _, r := range refs {
		ts := sesStart[r.sessionID].Add(time.Duration(r.offsetMin) * time.Minute)
		if _, err := stmtRef.Exec(r.sessionID, r.refType, r.refValue, r.turnIdx, ts.Format(tsFmt)); err != nil {
			log.Fatalf("ref %s/%s/%s: %v", r.sessionID, r.refType, r.refValue, err)
		}
	}

	// Search index — index turns and checkpoint fields for FTS5.
	stmtIdx, _ := tx.Prepare(`INSERT INTO search_index (content, session_id, source_type, source_id) VALUES (?,?,?,?)`)

	for _, t := range turns {
		content := t.user + "\n" + t.bot
		sourceID := fmt.Sprintf("%s/%d", t.sessionID, t.idx)
		if _, err := stmtIdx.Exec(content, t.sessionID, "turn", sourceID); err != nil {
			log.Fatalf("search turn %s: %v", sourceID, err)
		}
	}

	for _, c := range checkpoints {
		sid := c.sessionID
		cpID := fmt.Sprintf("%s/%d", sid, c.num)

		entries := []struct {
			content, srcType string
		}{
			{c.overview, "checkpoint_overview"},
			{c.history, "checkpoint_history"},
			{c.workDone, "checkpoint_work_done"},
			{c.techDetails, "checkpoint_technical"},
			{c.files, "checkpoint_files"},
			{c.nextSteps, "checkpoint_next_steps"},
		}
		for _, e := range entries {
			if strings.TrimSpace(e.content) == "" {
				continue
			}
			if _, err := stmtIdx.Exec(e.content, sid, e.srcType, cpID); err != nil {
				log.Fatalf("search cp %s %s: %v", cpID, e.srcType, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}

	// Verify counts.
	for _, q := range []struct{ label, sql string }{
		{"sessions", "SELECT COUNT(*) FROM sessions"},
		{"turns", "SELECT COUNT(*) FROM turns"},
		{"checkpoints", "SELECT COUNT(*) FROM checkpoints"},
		{"files", "SELECT COUNT(*) FROM session_files"},
		{"refs", "SELECT COUNT(*) FROM session_refs"},
		{"search_index", "SELECT COUNT(*) FROM search_index"},
	} {
		var n int
		db.QueryRow(q.sql).Scan(&n)
		fmt.Printf("%-15s %d\n", q.label, n)
	}
	fmt.Printf("\n✅ Generated %s\n", dbPath)
}
