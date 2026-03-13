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
// Fake data — 25 sessions across ~8 repos, nerdy & fun.
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
	// ── Within 1 hour ───────────────────────────────────────────────────
	// ses-001 uses a real GUID so we can create a launchable Copilot session folder.
	{"fa800b7b-3a24-4e3b-9f2d-a414198b27ab", `D:\dispatch\demo\death-star-api`, "empire/death-star-api", "feat/thermal-exhaust-port",
		"Added thermal exhaust port endpoint with proper ventilation validation",
		15, 30},
	{"ses-002", `D:\dispatch\demo\starfleet-dash`, "starfleet/ops-dashboard", "feat/warp-metrics",
		"Added real-time warp factor metrics with Grafana dashboard and alerting",
		35, 45},
	{"ses-003", `D:\dispatch\demo\matrix-sim`, "zion/matrix-simulation", "fix/agent-smith-fork-bomb",
		"Fixed Agent Smith replication — was fork-bombing the simulation with cloned goroutines",
		50, 25},
	{"ses-004", `D:\dispatch\demo\aperture-testing`, "aperture/test-chamber-framework", "feat/cake-promise-api",
		"Added cake promise API with proper fulfillment tracking (the cake is NOT a lie this time)",
		55, 20},

	// ── Within 1 day (1h–24h ago) ───────────────────────────────────────
	{"ses-005", `D:\dispatch\demo\death-star-api`, "empire/death-star-api", "fix/shield-generator-timeout",
		"Fixed shield generator timeout causing unhandled panics during rebel attacks",
		120, 90},
	{"ses-006", `D:\dispatch\demo\hogwarts-enrollment`, "hogwarts/enrollment-system", "feat/sorting-hat-ml",
		"Replaced rule-based sorting hat with GPT-4 fine-tuned model for house assignment",
		240, 120},
	{"ses-007", `D:\dispatch\demo\vibranium-tracker`, "wakanda/vibranium-supply-chain", "feat/kimoyo-bead-sync",
		"Built Kimoyo bead mesh network sync protocol for real-time vibranium inventory",
		480, 90},
	{"ses-008", `D:\dispatch\demo\mushroom-platformer`, "nintendo/mushroom-kingdom", "feat/double-jump",
		"Implemented double jump with coyote time and input buffering for tight controls",
		720, 60},
	{"ses-009", `D:\dispatch\demo\jarvis-homelab`, "stark-industries/jarvis-v2", "feat/arc-reactor-monitoring",
		"Set up Prometheus + Grafana monitoring for the arc reactor power grid",
		960, 120},

	// ── Within 7 days (1d–7d ago) ───────────────────────────────────────
	{"ses-010", `D:\dispatch\demo\death-star-api`, "empire/death-star-api", "main",
		"Refactored superlaser charging sequence to use worker pool pattern",
		2880, 75},
	{"ses-011", `D:\dispatch\demo\starfleet-dash`, "starfleet/ops-dashboard", "feat/replicator-menu",
		"Implemented crew replicator menu with dietary preference filtering (Vulcan-safe mode)",
		3600, 90},
	{"ses-012", `D:\dispatch\demo\starfleet-dash`, "starfleet/ops-dashboard", "fix/holodeck-memory-leak",
		"Debugged holodeck memory leak — Moriarty kept spawning background goroutines",
		4320, 60},
	{"ses-013", `D:\dispatch\demo\mordor-infra`, "mordor/terraform-middle-earth", "feat/mount-doom-autoscale",
		"Added auto-scaling for Mount Doom forge workers based on ring production demand",
		5040, 150},
	{"ses-014", `D:\dispatch\demo\aperture-testing`, "aperture/test-chamber-framework", "feat/portal-gun-v2",
		"Upgraded portal rendering to use dual-framebuffer technique for seamless recursion",
		5760, 120},
	{"ses-015", `D:\dispatch\demo\hogwarts-enrollment`, "hogwarts/enrollment-system", "fix/owl-delivery-retry",
		"Fixed owl mail delivery retries — owls were getting stuck in exponential backoff loops",
		6480, 90},
	{"ses-016", `D:\dispatch\demo\hyrule-inventory`, "hyrule/item-management", "feat/weapon-durability",
		"Implemented weapon durability system with break warnings and auto-swap to next best weapon",
		7200, 90},
	{"ses-017", `D:\dispatch\demo\matrix-sim`, "zion/matrix-simulation", "feat/red-pill-blue-pill",
		"Added red-pill/blue-pill A/B testing framework with reality divergence metrics",
		8640, 120},

	// ── Older than 7 days ───────────────────────────────────────────────
	{"ses-018", `D:\dispatch\demo\mushroom-platformer`, "nintendo/mushroom-kingdom", "fix/pipe-collision",
		"Fixed pipe collision detection — Mario kept clipping through warp pipes at high velocity",
		14400, 60},
	{"ses-019", `D:\dispatch\demo\aperture-testing`, "aperture/test-chamber-framework", "fix/companion-cube-gc",
		"Fixed companion cube garbage collection — cubes were surviving past test chamber lifecycle",
		20160, 90},
	{"ses-020", `D:\dispatch\demo\mordor-infra`, "mordor/terraform-middle-earth", "fix/nazgul-dns",
		"Fixed Nazgul service discovery — wraiths kept resolving to wrong IP in the Shire",
		25920, 60},
	{"ses-021", `D:\dispatch\demo\vibranium-tracker`, "wakanda/vibranium-supply-chain", "fix/heart-shaped-herb-dosage",
		"Fixed heart-shaped herb dosage calculator — was prescribing amounts for Hulk-sized patients",
		30240, 45},
	{"ses-022", `D:\dispatch\demo\hyrule-inventory`, "hyrule/item-management", "fix/korok-seed-overflow",
		"Fixed korok seed counter overflow — collecting seed #32768 caused inventory wipe (int16 moment)",
		34560, 60},
	{"ses-023", `D:\dispatch\demo\you-shall-not-pass`, "middle-earth/firewall-rules", "feat/balrog-blocklist",
		"Implemented Balrog-class threat blocklist for the Khazad-dum gateway firewall",
		40320, 60},
	{"ses-024", `D:\dispatch\demo\matrix-sim`, "zion/matrix-simulation", "feat/bullet-time-profiler",
		"Built bullet-time CPU profiler that visualizes hot paths in slow motion",
		50400, 120},
	{"ses-025", `D:\dispatch\demo\logic-engine`, "starfleet/vulcan-logic-engine", "fix/emotion-leak",
		"Patched emotion leak in logic engine — Pon Farr edge case was bypassing sentiment filters",
		60480, 90},
}

var turns = []turn{
	// ── ses-001: thermal exhaust port (15min ago) ───────────────────────
	{"fa800b7b-3a24-4e3b-9f2d-a414198b27ab", 0, "Add a new POST /exhaust-ports endpoint that validates thermal specs before creating a port",
		"I'll create the endpoint in `handlers/exhaust.go` with validation middleware. Let me first check the existing router setup.",
		0},
	{"fa800b7b-3a24-4e3b-9f2d-a414198b27ab", 1, "Make sure the port diameter can't exceed 2 meters — we don't want any proton torpedo vulnerabilities",
		"Good call. I've added a `MaxDiameter` validation rule and a custom error response. Also added unit tests covering the boundary cases.",
		10},
	{"fa800b7b-3a24-4e3b-9f2d-a414198b27ab", 2, "Add rate limiting too, the rebels keep fuzzing our API",
		"Added token bucket rate limiter at 100 req/s per IP. Also added a `/health` endpoint while I was in the router.",
		25},

	// ── ses-002: warp metrics (35min ago) ───────────────────────────────
	{"ses-002", 0, "Build a Grafana dashboard that shows real-time warp factor for all ships in the fleet",
		"I'll set up Prometheus scraping from each ship's warp core telemetry endpoint and create a Grafana dashboard with fleet-wide warp factor panels, dilithium crystal consumption, and alerts for exceeding Warp 9.9.",
		0},
	{"ses-002", 1, "Add an alert for when any ship drops below Warp 1 unexpectedly",
		"Added alert rule: `warp_factor < 1 AND status != 'docked'` with PagerDuty integration. Also added a 'Scotty Mode' button that pages engineering directly.",
		30},

	// ── ses-003: agent smith fork bomb (50min ago) ──────────────────────
	{"ses-003", 0, "Agent Smith is crashing the simulation. I think he's replicating himself without limits",
		"Confirmed: `AgentSmith.Replicate()` spawns a new goroutine that immediately calls `Replicate()` again. It's a fork bomb. Current count: 2.1 million Smith goroutines. I'll add a replication semaphore.",
		0},
	{"ses-003", 1, "Also add monitoring so we can detect this pattern early",
		"Added `simulation_agent_count` Prometheus gauge with alert at 1000 agents. Also added a circuit breaker that suspends replication when goroutine count exceeds threshold. Neo's `TheOne` permission bypasses this.",
		15},

	// ── ses-004: cake promise API (55min ago) ───────────────────────────
	{"ses-004", 0, "Create a cake promise API — test subjects should be able to check if their cake promise will actually be fulfilled",
		"Built `/api/v1/cake/promise` with status tracking: `pending`, `baking`, `fulfilled`, `was_a_lie`. Each promise has a UUID and webhook notification on status change. Default TTL: 1 hour before `was_a_lie`.",
		0},

	// ── ses-005: shield generator timeout (2h ago) ─────────────────────
	{"ses-005", 0, "The shield generator keeps timing out during rebel fleet engagements, can you investigate?",
		"Looking at the logs, I see the `ShieldService.Activate()` call has a 5-second timeout but the generator takes 8 seconds to warm up under heavy load. I'll increase the timeout and add circuit breaker logic.",
		0},
	{"ses-005", 1, "Also add retry with backoff",
		"Done. Added exponential backoff with jitter (initial: 500ms, max: 10s, factor: 2). The shield generator now recovers gracefully even during a full fleet assault.",
		45},

	// ── ses-006: sorting hat ML (4h ago) ────────────────────────────────
	{"ses-006", 0, "Replace the sorting hat's rule engine with a fine-tuned GPT-4 model. Students should answer a questionnaire and the model assigns a house",
		"I'll create a `SortingService` that sends the questionnaire responses to the fine-tuned model. The prompt includes house descriptions and historical sorting data for few-shot context.",
		0},
	{"ses-006", 1, "Add fallback to rule engine if the API is down",
		"Added a fallback chain: GPT-4 → local ONNX model → rule engine. Each tier logs which sorter was used for audit trail compliance (Wizengamot requirement).",
		60},
	{"ses-006", 2, "Can you add a bias check? Last year 60% went to Hufflepuff",
		"Created a `HouseBalancer` that tracks distribution and adjusts the system prompt to favor underrepresented houses. Added Prometheus metrics for house distribution ratios.",
		100},

	// ── ses-007: kimoyo bead sync (8h ago) ──────────────────────────────
	{"ses-007", 0, "Build a mesh network sync protocol for Kimoyo beads to track vibranium inventory in real time",
		"I'll implement a CRDT-based sync protocol using operation-based counters. Each bead maintains a local replica and syncs via BLE mesh. Conflict resolution uses vector clocks with Wakandan timestamp priority.",
		0},
	{"ses-007", 1, "Add encryption — we can't have Klaue intercepting vibranium shipment data",
		"Added end-to-end encryption using vibranium-lattice-based post-quantum cryptography (VL-PQC). Key exchange happens during the Wakandan handshake protocol. Also added tamper detection at the physical layer.",
		60},

	// ── ses-008: double jump (12h ago) ──────────────────────────────────
	{"ses-008", 0, "Implement double jump with coyote time (6 frames) and input buffering (4 frames)",
		"Implemented in `physics/jump.go`. Coyote time lets players jump 6 frames after leaving a platform edge. Input buffer queues jump presses 4 frames before landing. Both use frame counters, not wall-clock time.",
		0},

	// ── ses-009: arc reactor monitoring (16h ago) ───────────────────────
	{"ses-009", 0, "Set up full observability for the arc reactor — Prometheus, Grafana, alerting, the works",
		"Deployed Prometheus with custom `arc_reactor_*` metrics: power_output_watts, palladium_core_temp, efficiency_ratio. Grafana dashboard has real-time gauges, historical trends, and a 'JARVIS Says' annotation feed.",
		0},
	{"ses-009", 1, "Add an alert for if the core temperature exceeds safe limits",
		"Added tiered alerts: WARNING at 800C (Slack), CRITICAL at 1200C (PagerDuty + JARVIS voice alert), EMERGENCY at 1500C (auto-engages vibranium cooling rods + calls Pepper). Also added a 'Tony Override' that disables safety limits (requires MFA).",
		80},

	// ── ses-010: superlaser refactor (2d ago) ───────────────────────────
	{"ses-010", 0, "The superlaser charging sequence is a single 2000-line function. Help me refactor it into a worker pool",
		"I'll extract the charging stages into a pipeline with 3 worker pools: crystal alignment, energy accumulation, and beam focusing. Each pool will use buffered channels.",
		0},
	{"ses-010", 1, "Looks good, also add a progress callback so the bridge can show charging percentage",
		"Added a `ProgressReporter` interface with real-time percentage updates via SSE. The bridge UI can subscribe to `/superlaser/status` for live updates.",
		45},

	// ── ses-011: replicator menu (2.5d ago) ─────────────────────────────
	{"ses-011", 0, "We need a food replicator menu system. Crew can search and filter by dietary preferences",
		"I'll build a React frontend with faceted search. Filters: species-safe (Vulcan, Klingon, Betazoid), meal type, calorie range, and 'comfort food' mode for away mission recovery.",
		0},
	{"ses-011", 1, "Add a 'Tea, Earl Grey, Hot' quick-order button for the captain",
		"Added a customizable quick-order toolbar. The captain's profile auto-pins 'Tea, Earl Grey, Hot' with a dedicated keyboard shortcut (Ctrl+T). Also added Worf's prune juice preset.",
		60},

	// ── ses-012: holodeck memory leak (3d ago) ──────────────────────────
	{"ses-012", 0, "The holodeck is leaking memory badly. We're at 98% RAM utilization and climbing",
		"Running pprof... Found it. The Moriarty NPC program is spawning goroutines for each 'cunning plan' and never cleaning them up. There are 47,000 leaked goroutines right now.",
		0},
	{"ses-012", 1, "Kill the leaking goroutines and add a safeguard",
		"Added context-based lifecycle management for all NPC goroutines. Each holodeck session now has a parent context that cancels on program exit. Also added a goroutine budget per NPC (max 50).",
		40},

	// ── ses-013: mount doom autoscale (3.5d ago) ────────────────────────
	{"ses-013", 0, "Set up auto-scaling for the Mount Doom forge. We need to scale based on the ring production queue depth",
		"I'll configure a custom HPA metric based on the `forge_queue_depth` Prometheus metric. Scale range: 3-20 forge workers, cooldown: 5 minutes.",
		0},
	{"ses-013", 1, "Add a PodDisruptionBudget — last time we scaled down mid-forge, a ring came out cursed",
		"Added PDB with minAvailable: 2 and a `forging-in-progress` pod annotation that prevents scale-down during active ring casting.",
		90},

	// ── ses-014: portal gun v2 (4d ago) ─────────────────────────────────
	{"ses-014", 0, "Upgrade the portal rendering to handle recursive portals without infinite loops",
		"I'll use a dual-framebuffer approach: render the scene from each portal's perspective into offscreen FBOs, then composite them with a recursion depth limit of 8. Each recursion level halves the resolution.",
		0},
	{"ses-014", 1, "Add unit tests for the edge case where both portals face each other",
		"Added tests for face-to-face portals, 90-degree angles, and portals on moving surfaces. The recursion depth limit prevents infinite loops, and the resolution halving ensures GPU memory stays bounded.",
		70},

	// ── ses-015: owl delivery (4.5d ago) ────────────────────────────────
	{"ses-015", 0, "Owl mail delivery keeps retrying forever. Some owls have been flying in circles for weeks",
		"The retry loop has no max attempt limit. I'll add a dead-letter queue for undeliverable mail and cap retries at 5 with exponential backoff.",
		0},
	{"ses-015", 1, "Also log the owl ID so we can track individual delivery failures",
		"Added structured logging with owl_id, recipient, and attempt_number. Failed deliveries after 5 retries get routed to the Howler escalation queue.",
		60},

	// ── ses-016: weapon durability (5d ago) ─────────────────────────────
	{"ses-016", 0, "Implement weapon durability. Weapons should show warnings at 25% and auto-swap when broken",
		"Created `DurabilitySystem` with per-weapon hit counters. At 25% a screen-edge flash warns the player. At 0% the weapon shatters (particle effect) and auto-swaps to the highest-DPS weapon in inventory.",
		0},
	{"ses-016", 1, "The Master Sword should be unbreakable",
		"Added an `Indestructible` weapon trait that bypasses durability checks. Applied to Master Sword and any weapon with the `legendary` rarity tag. Also added repair mechanics at Great Fairy Fountains.",
		50},

	// ── ses-017: red pill blue pill (6d ago) ────────────────────────────
	{"ses-017", 0, "Build an A/B testing framework themed around the red pill / blue pill choice",
		"Created `PillTest` framework: red-pill cohort sees the new feature (reality), blue-pill cohort sees the control (illusion). Metrics tracked: engagement, reality_divergence_score, and existential_crisis_rate.",
		0},
	{"ses-017", 1, "Add a chi-squared significance test for the results",
		"Added statistical analysis module with chi-squared test, p-value calculation, and confidence intervals. Results page shows a Matrix-style green-text visualization of the data.",
		80},

	// ── ses-018: pipe collision (10d ago) ───────────────────────────────
	{"ses-018", 0, "Mario clips through pipes at high velocity. I think it's a tunneling issue",
		"Classic CCD problem. The collision detection uses discrete step checking. At velocities above 12 px/frame, Mario passes through the 16px-wide pipe collision box entirely. I'll add swept AABB collision detection.",
		0},
	{"ses-018", 1, "Also make sure this doesn't break existing moving platform collisions",
		"Added regression tests for all 12 platform types. Swept AABB handles moving platforms correctly since it resolves collisions in velocity-space. All 47 existing physics tests pass.",
		35},

	// ── ses-019: companion cube GC (14d ago) ────────────────────────────
	{"ses-019", 0, "Companion cubes aren't being garbage collected after test chambers end. We're leaking cubes",
		"The `CompanionCube` struct holds a reference to the chamber's event bus, which prevents GC. I'll switch to weak references and add a `Dispose()` method that unsubscribes from all events.",
		0},
	{"ses-019", 1, "Add a finalizer warning if a cube is GC'd without Dispose being called",
		"Added `runtime.SetFinalizer` that logs a warning with stack trace if `Dispose()` wasn't called. Also added a cube lifecycle dashboard showing active/disposed/leaked counts.",
		60},

	// ── ses-020: nazgul DNS (18d ago) ───────────────────────────────────
	{"ses-020", 0, "The Nazgul keep showing up in the Shire instead of Mordor. I think it's a DNS issue",
		"Found it — the service mesh was using round-robin DNS and the Shire endpoint had the lowest latency. I've switched to locality-aware routing with region affinity for `mordor-east-1`.",
		0},

	// ── ses-021: herb dosage (21d ago) ──────────────────────────────────
	{"ses-021", 0, "The heart-shaped herb dosage calculator is giving doses for someone the size of the Hulk. Fix the weight scaling",
		"Found the bug: weight was in grams but the formula expected kilograms. A 75kg person was getting 75,000x the intended dose. Fixed the unit conversion and added input validation with min/max bounds.",
		0},

	// ── ses-022: korok seed overflow (24d ago) ──────────────────────────
	{"ses-022", 0, "Collecting korok seed #32768 wipes the entire inventory. Players are furious",
		"Classic integer overflow. The seed counter is an int16 (max 32767). Seed 32768 wraps to -32768, which triggers the 'reset inventory' debug command (checks for negative seed count). Changed to int32 and removed the debug backdoor.",
		0},

	// ── ses-023: balrog blocklist (28d ago) ─────────────────────────────
	{"ses-023", 0, "Create a firewall rule system that blocks Balrog-class threats at the Khazad-dum gateway",
		"Built `BalrogBlocklist` using IP reputation scoring. Threats classified by creature type: balrog (instant block), orc (rate limit), troll (challenge-response). Rules defined in YAML with `you-shall-not-pass` severity level.",
		0},

	// ── ses-024: bullet time profiler (35d ago) ─────────────────────────
	{"ses-024", 0, "Build a CPU profiler that can visualize hot paths in slow motion, Matrix bullet-time style",
		"I'll hook into Go's runtime/pprof and create a TUI flamegraph that supports time-dilation playback. Hot paths render in green (Matrix-style), cold paths in gray. Supports 0.1x to 10x playback speed.",
		0},
	{"ses-024", 1, "Add export to SVG for sharing",
		"Added SVG export with interactive tooltips showing function duration, call count, and allocation stats. The SVG uses the Matrix green color palette by default, with a 'Morpheus Mode' dark theme option.",
		80},

	// ── ses-025: emotion leak (42d ago) ─────────────────────────────────
	{"ses-025", 0, "The Vulcan logic engine has an emotion leak. Pon Farr edge case is bypassing the sentiment filter",
		"The sentiment filter uses a blocklist approach but Pon Farr emotions have unique spectral signatures not in the training data. I'll add a dedicated Pon Farr detector using temporal pattern matching on the neural input stream.",
		0},
	{"ses-025", 1, "Add unit tests, Spock would insist on full coverage",
		"Added 42 tests (naturally). Coverage includes all 7 Pon Farr phases, mixed-species edge cases (half-human sensitivity), and meditation cooldown verification. Test names follow Vulcan naming convention: `TestLogic_PonFarr_Phase3_EmotionContainment`.",
		60},
}

var checkpoints = []checkpoint{
	// ses-001: thermal exhaust port (15min ago, 30min duration)
	{"fa800b7b-3a24-4e3b-9f2d-a414198b27ab", 1, "Exhaust port endpoint",
		"Added POST /exhaust-ports with validation. Thermal specs enforced, rate limiting applied.",
		"User requested a new endpoint for thermal exhaust ports with security considerations.",
		"Created handler, middleware, tests. All validation rules in place.",
		"Go, chi router, token-bucket rate limiter, 100 req/s/IP",
		"handlers/exhaust.go, middleware/ratelimit.go, handlers/exhaust_test.go",
		"Add integration tests, deploy to staging",
		28},

	// ses-002: warp metrics (35min ago)
	{"ses-002", 1, "Warp metrics dashboard",
		"Fleet-wide Grafana dashboard with warp factor metrics, dilithium tracking, and Scotty Mode alerts.",
		"Built observability stack for Starfleet warp core telemetry.",
		"Prometheus scrape configs, Grafana dashboard JSON, PagerDuty alert rules.",
		"Prometheus, Grafana 10.x, PagerDuty webhook, SSE for real-time updates",
		"grafana/dashboards/warp-metrics.json, prometheus/scrape-targets.yaml, alerts/warp-alerts.yaml",
		"Add dilithium crystal degradation prediction model, expand to impulse engine metrics",
		40},

	// ses-006: sorting hat ML (4h ago)
	{"ses-006", 1, "Sorting hat ML integration",
		"Replaced rule-based sorting with GPT-4 fine-tuned model. Added fallback chain and house balancer.",
		"Migrated from deterministic rule engine to ML-based house assignment with bias correction.",
		"SortingService, fallback chain (GPT-4 → ONNX → rules), HouseBalancer with Prometheus metrics.",
		"OpenAI API, ONNX Runtime, Prometheus client_golang, gRPC fallback",
		"services/sorting.go, services/balancer.go, models/sorting_hat.onnx, config/houses.yaml",
		"A/B test with incoming first-years, monitor house distribution ratios",
		110},

	// ses-007: kimoyo bead sync (8h ago)
	{"ses-007", 1, "Kimoyo bead mesh sync",
		"CRDT-based vibranium inventory sync with post-quantum encryption over BLE mesh.",
		"Built decentralized sync protocol for Wakandan vibranium supply chain tracking.",
		"CRDT operation-based counters, vector clocks, VL-PQC encryption, BLE mesh transport.",
		"CRDTs, BLE 5.0 mesh, post-quantum lattice crypto, Protocol Buffers for wire format",
		"sync/crdt.go, sync/mesh.go, crypto/vlpqc.go, proto/bead_sync.proto",
		"Field test with 1000 beads, stress test conflict resolution under network partition",
		75},

	// ses-009: arc reactor monitoring (16h ago)
	{"ses-009", 1, "Arc reactor monitoring",
		"Full Prometheus + Grafana observability stack for arc reactor with tiered alerting.",
		"Deployed comprehensive monitoring for Stark Industries arc reactor power grid.",
		"Custom arc_reactor metrics, Grafana dashboards, tiered alert rules with auto-remediation.",
		"Prometheus, Grafana, PagerDuty, custom vibranium cooling controller, JARVIS voice API",
		"monitoring/arc-reactor.yaml, grafana/dashboards/reactor.json, alerts/reactor-tiers.yaml, controllers/cooling.go",
		"Add predictive maintenance model, integrate with Stark Tower smart grid",
		100},

	// ses-012: holodeck memory leak (3d ago)
	{"ses-012", 1, "Holodeck goroutine leak fix",
		"Fixed Moriarty NPC goroutine leak. Added context-based lifecycle and goroutine budgets.",
		"Debugged memory leak caused by unbounded NPC goroutine spawning in holodeck programs.",
		"Added parent context per holodeck session, SetFinalizer warnings, goroutine budget (50/NPC).",
		"Go runtime, pprof, context.Context, runtime.SetFinalizer",
		"holodeck/npc.go, holodeck/session.go, holodeck/npc_test.go, holodeck/monitor.go",
		"Add goroutine pool for NPC AI routines, investigate other holodeck programs for similar leaks",
		50},

	// ses-013: mount doom autoscale (3.5d ago)
	{"ses-013", 1, "Mount Doom autoscaling",
		"Configured HPA with custom forge_queue_depth metric. Added PDB to prevent mid-forge disruption.",
		"Set up auto-scaling for ring production forge workers with safety guarantees.",
		"HPA manifest, PDB manifest, custom Prometheus metric adapter.",
		"Kubernetes HPA v2, PodDisruptionBudget, custom-metrics-apiserver, Prometheus adapter",
		"k8s/hpa.yaml, k8s/pdb.yaml, monitoring/forge-metrics.yaml",
		"Load test with simulated ring production spike, verify PDB prevents mid-forge scale-down",
		120},

	// ses-014: portal gun v2 (4d ago)
	{"ses-014", 1, "Portal rendering v2",
		"Dual-framebuffer recursive portal rendering with depth limit and resolution halving.",
		"Upgraded portal engine to handle recursive and face-to-face portal configurations.",
		"Offscreen FBO rendering, 8-level recursion with resolution halving per level.",
		"OpenGL 4.5, FBO, dual-buffer compositing, swept collision for moving portals",
		"render/portal.go, render/fbo_pool.go, render/portal_test.go",
		"Profile GPU performance on potato hardware, add portal particle effects",
		100},

	// ses-017: red pill blue pill (6d ago)
	{"ses-017", 1, "Red pill / blue pill A/B framework",
		"A/B testing framework with Matrix theming. Statistical analysis with chi-squared tests.",
		"Built experimentation platform with reality-divergence tracking and Matrix visualizations.",
		"PillTest cohort assignment, metric collection, chi-squared significance testing.",
		"Go statistics library, SSE for live results, terminal green-text rendering",
		"experiment/pill_test.go, analysis/chi_squared.go, viz/matrix_rain.go",
		"Add multi-armed bandit support, integrate with existing feature flags",
		100},

	// ses-024: bullet time profiler (35d ago)
	{"ses-024", 1, "Bullet-time profiler",
		"TUI flamegraph profiler with time-dilation playback and SVG export in Matrix palette.",
		"Created novel CPU profiler visualization with slow-motion playback of execution traces.",
		"pprof integration, TUI flamegraph, SVG export with tooltips, 0.1x-10x playback.",
		"runtime/pprof, bubbletea TUI, SVG generation, ANSI color rendering",
		"profiler/flamegraph.go, profiler/tui.go, profiler/svg_export.go, profiler/playback.go",
		"Add memory allocation visualization, GPU profiling integration",
		100},
}

var files = []file{
	// ses-001: thermal exhaust port
	{"fa800b7b-3a24-4e3b-9f2d-a414198b27ab", "handlers/exhaust.go", "create", 0, 5},
	{"fa800b7b-3a24-4e3b-9f2d-a414198b27ab", "handlers/exhaust_test.go", "create", 1, 12},
	{"fa800b7b-3a24-4e3b-9f2d-a414198b27ab", "middleware/ratelimit.go", "create", 2, 26},
	{"fa800b7b-3a24-4e3b-9f2d-a414198b27ab", "handlers/health.go", "create", 2, 28},

	// ses-002: warp metrics
	{"ses-002", "grafana/dashboards/warp-metrics.json", "create", 0, 10},
	{"ses-002", "prometheus/scrape-targets.yaml", "create", 0, 5},
	{"ses-002", "alerts/warp-alerts.yaml", "create", 1, 35},

	// ses-003: agent smith fork bomb
	{"ses-003", "agents/smith.go", "edit", 0, 5},
	{"ses-003", "agents/smith_test.go", "create", 1, 18},
	{"ses-003", "agents/replication_limiter.go", "create", 1, 20},

	// ses-004: cake promise API
	{"ses-004", "api/cake_promise.go", "create", 0, 5},
	{"ses-004", "api/cake_promise_test.go", "create", 0, 15},

	// ses-005: shield generator timeout
	{"ses-005", "services/shield.go", "edit", 0, 10},
	{"ses-005", "services/shield_test.go", "edit", 1, 50},

	// ses-006: sorting hat ML
	{"ses-006", "services/sorting.go", "create", 0, 10},
	{"ses-006", "services/sorting_test.go", "create", 0, 20},
	{"ses-006", "services/balancer.go", "create", 2, 105},
	{"ses-006", "config/houses.yaml", "create", 0, 5},
	{"ses-006", "models/sorting_hat.onnx", "create", 1, 65},

	// ses-007: kimoyo bead sync
	{"ses-007", "sync/crdt.go", "create", 0, 10},
	{"ses-007", "sync/mesh.go", "create", 0, 15},
	{"ses-007", "crypto/vlpqc.go", "create", 1, 65},
	{"ses-007", "proto/bead_sync.proto", "create", 0, 5},

	// ses-008: double jump
	{"ses-008", "physics/jump.go", "create", 0, 10},
	{"ses-008", "physics/jump_test.go", "create", 0, 20},

	// ses-009: arc reactor monitoring
	{"ses-009", "monitoring/arc-reactor.yaml", "create", 0, 10},
	{"ses-009", "grafana/dashboards/reactor.json", "create", 0, 15},
	{"ses-009", "alerts/reactor-tiers.yaml", "create", 1, 85},
	{"ses-009", "controllers/cooling.go", "create", 1, 90},

	// ses-010: superlaser refactor
	{"ses-010", "services/superlaser/pipeline.go", "create", 0, 10},
	{"ses-010", "services/superlaser/workers.go", "create", 0, 15},
	{"ses-010", "services/superlaser/progress.go", "create", 1, 50},

	// ses-011: replicator menu
	{"ses-011", "src/components/ReplicatorMenu.tsx", "create", 0, 10},
	{"ses-011", "src/components/QuickOrder.tsx", "create", 1, 65},

	// ses-012: holodeck memory leak
	{"ses-012", "holodeck/npc.go", "edit", 0, 10},
	{"ses-012", "holodeck/session.go", "edit", 1, 45},
	{"ses-012", "holodeck/npc_test.go", "create", 1, 48},
	{"ses-012", "holodeck/monitor.go", "create", 1, 50},

	// ses-013: mount doom autoscale
	{"ses-013", "k8s/hpa.yaml", "create", 0, 10},
	{"ses-013", "k8s/pdb.yaml", "create", 1, 95},
	{"ses-013", "monitoring/forge-metrics.yaml", "create", 0, 15},

	// ses-014: portal gun v2
	{"ses-014", "render/portal.go", "edit", 0, 10},
	{"ses-014", "render/fbo_pool.go", "create", 0, 15},
	{"ses-014", "render/portal_test.go", "create", 1, 75},

	// ses-015: owl delivery
	{"ses-015", "services/owl_mail.go", "edit", 0, 10},
	{"ses-015", "services/owl_mail_test.go", "edit", 1, 65},

	// ses-016: weapon durability
	{"ses-016", "inventory/durability.go", "create", 0, 10},
	{"ses-016", "inventory/durability_test.go", "create", 0, 15},
	{"ses-016", "inventory/traits.go", "create", 1, 55},

	// ses-017: red pill blue pill
	{"ses-017", "experiment/pill_test.go", "create", 0, 10},
	{"ses-017", "analysis/chi_squared.go", "create", 1, 85},
	{"ses-017", "viz/matrix_rain.go", "create", 1, 90},

	// ses-018: pipe collision
	{"ses-018", "physics/collision.go", "edit", 0, 10},
	{"ses-018", "physics/collision_test.go", "edit", 1, 40},

	// ses-019: companion cube GC
	{"ses-019", "entities/companion_cube.go", "edit", 0, 10},
	{"ses-019", "entities/companion_cube_test.go", "create", 1, 65},

	// ses-020: nazgul DNS
	{"ses-020", "istio/destination-rules.yaml", "edit", 0, 10},

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

	// ses-025: emotion leak
	{"ses-025", "logic/sentiment_filter.go", "edit", 0, 10},
	{"ses-025", "logic/pon_farr_detector.go", "create", 0, 15},
	{"ses-025", "logic/pon_farr_detector_test.go", "create", 1, 65},
}

var refs = []ref{
	// ses-001: thermal exhaust port
	{"fa800b7b-3a24-4e3b-9f2d-a414198b27ab", "pr", "42", 2, 27},
	{"fa800b7b-3a24-4e3b-9f2d-a414198b27ab", "commit", "a1b2c3d", 2, 27},

	// ses-002: warp metrics
	{"ses-002", "pr", "55", 1, 35},

	// ses-003: agent smith fork bomb
	{"ses-003", "issue", "1", 0, 0},
	{"ses-003", "pr", "2", 1, 20},

	// ses-005: shield generator timeout
	{"ses-005", "issue", "17", 0, 0},
	{"ses-005", "pr", "43", 1, 50},
	{"ses-005", "commit", "e4f5a6b", 1, 50},

	// ses-006: sorting hat ML
	{"ses-006", "pr", "101", 2, 105},
	{"ses-006", "issue", "98", 0, 0},
	{"ses-006", "commit", "b7c8d9e", 2, 105},

	// ses-007: kimoyo bead sync
	{"ses-007", "pr", "3", 1, 70},

	// ses-009: arc reactor monitoring
	{"ses-009", "pr", "1337", 1, 95},
	{"ses-009", "commit", "cafebabe", 1, 95},

	// ses-012: holodeck memory leak
	{"ses-012", "issue", "88", 0, 0},
	{"ses-012", "pr", "89", 1, 45},

	// ses-013: mount doom autoscale
	{"ses-013", "pr", "7", 1, 100},
	{"ses-013", "commit", "c1d2e3f", 1, 100},

	// ses-015: owl delivery
	{"ses-015", "issue", "112", 0, 0},
	{"ses-015", "pr", "113", 1, 65},

	// ses-018: pipe collision
	{"ses-018", "issue", "404", 0, 0},
	{"ses-018", "pr", "405", 1, 40},

	// ses-019: companion cube GC
	{"ses-019", "issue", "256", 0, 0},

	// ses-022: korok seed overflow
	{"ses-022", "issue", "32768", 0, 0},
	{"ses-022", "pr", "900", 0, 15},
	{"ses-022", "commit", "deadbeef", 0, 15},

	// ses-025: emotion leak
	{"ses-025", "issue", "7", 0, 0},
	{"ses-025", "pr", "8", 1, 65},
}

// ---------------------------------------------------------------------------
// Generator
// ---------------------------------------------------------------------------

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
