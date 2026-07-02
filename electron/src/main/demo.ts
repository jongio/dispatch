import type { Session, SessionDetail, Turn, Checkpoint, SessionFile, SessionRef } from './store';

export interface DemoData {
  sessions: Session[];
  details: Map<string, SessionDetail>;
  attention: Record<string, string>;
}

// Deterministic pseudo-random for reproducible demo data
function seededRandom(seed: number): () => number {
  let s = seed;
  return () => {
    s = (s * 1664525 + 1013904223) & 0xffffffff;
    return (s >>> 0) / 0xffffffff;
  };
}

function randomId(rand: () => number): string {
  const chars = 'abcdef0123456789';
  let id = '';
  for (let i = 0; i < 8; i++) {
    id += chars[Math.floor(rand() * chars.length)];
  }
  return id;
}

function hoursAgo(hours: number): string {
  return new Date(Date.now() - hours * 3600_000).toISOString();
}

interface SessionTemplate {
  repo: string;
  branch: string;
  summary: string;
  hostType: string;
  hoursAgo: number;
  turnCount: number;
  fileCount: number;
  cwd: string;
}

const TEMPLATES: SessionTemplate[] = [
  // jongio/dispatch sessions (active work)
  { repo: 'jongio/dispatch', branch: 'feature/electron-app', summary: 'Implementing Electron desktop app with session viewer', hostType: 'cli', hoursAgo: 0.5, turnCount: 42, fileCount: 18, cwd: 'C:\\code\\dispatch' },
  { repo: 'jongio/dispatch', branch: 'main', summary: 'Add TUI attention indicators and status refresh', hostType: 'cli', hoursAgo: 1.2, turnCount: 15, fileCount: 6, cwd: 'C:\\code\\dispatch' },
  { repo: 'jongio/dispatch', branch: 'fix/watcher-debounce', summary: 'Fix file watcher triggering too many refreshes', hostType: 'cli', hoursAgo: 3, turnCount: 8, fileCount: 3, cwd: 'C:\\code\\dispatch' },
  { repo: 'jongio/dispatch', branch: 'feature/deep-search', summary: 'Implement FTS5 deep search across session content', hostType: 'cli', hoursAgo: 6, turnCount: 23, fileCount: 5, cwd: 'C:\\code\\dispatch' },

  // microsoft/vscode sessions
  { repo: 'microsoft/vscode', branch: 'feature/copilot-inline', summary: 'Add inline completion provider for Copilot suggestions', hostType: 'cli', hoursAgo: 2, turnCount: 31, fileCount: 12, cwd: 'C:\\code\\vscode' },
  { repo: 'microsoft/vscode', branch: 'fix/extension-host-crash', summary: 'Debug extension host crash on large workspaces', hostType: 'cli', hoursAgo: 8, turnCount: 19, fileCount: 4, cwd: 'C:\\code\\vscode' },
  { repo: 'microsoft/vscode', branch: 'main', summary: 'Refactor settings sync to use new storage API', hostType: 'cloud', hoursAgo: 24, turnCount: 27, fileCount: 9, cwd: 'C:\\code\\vscode' },
  { repo: 'microsoft/vscode', branch: 'release/1.96', summary: 'Cherry-pick terminal rendering fix for stable', hostType: 'cli', hoursAgo: 48, turnCount: 5, fileCount: 2, cwd: 'C:\\code\\vscode' },

  // azure/azure-sdk sessions
  { repo: 'azure/azure-sdk-for-go', branch: 'feature/auth-v2', summary: 'Implementing OAuth2 device code flow for CLI auth', hostType: 'cli', hoursAgo: 1.5, turnCount: 18, fileCount: 7, cwd: 'C:\\code\\azure-sdk-for-go' },
  { repo: 'azure/azure-sdk-for-go', branch: 'main', summary: 'Add retry policy with exponential backoff', hostType: 'actions', hoursAgo: 12, turnCount: 11, fileCount: 4, cwd: 'C:\\code\\azure-sdk-for-go' },
  { repo: 'azure/azure-sdk-for-go', branch: 'fix/token-refresh', summary: 'Fix token refresh race condition in concurrent requests', hostType: 'cli', hoursAgo: 36, turnCount: 14, fileCount: 3, cwd: 'C:\\code\\azure-sdk-for-go' },
  { repo: 'azure/azure-sdk-for-js', branch: 'feature/streaming', summary: 'Implement streaming response support for OpenAI client', hostType: 'cloud', hoursAgo: 4, turnCount: 35, fileCount: 11, cwd: 'C:\\code\\azure-sdk-for-js' },

  // github/copilot-cli sessions
  { repo: 'github/copilot-cli', branch: 'main', summary: 'Add session persistence and workspace recovery', hostType: 'cli', hoursAgo: 0.3, turnCount: 47, fileCount: 14, cwd: 'C:\\code\\copilot-cli' },
  { repo: 'github/copilot-cli', branch: 'feature/multi-model', summary: 'Support model selection via CLI flags', hostType: 'cli', hoursAgo: 5, turnCount: 22, fileCount: 8, cwd: 'C:\\code\\copilot-cli' },
  { repo: 'github/copilot-cli', branch: 'fix/context-window', summary: 'Optimize context window packing for large repos', hostType: 'cli', hoursAgo: 18, turnCount: 16, fileCount: 5, cwd: 'C:\\code\\copilot-cli' },
  { repo: 'github/copilot-cli', branch: 'feature/mcp-tools', summary: 'Integrate MCP tool server with agent loop', hostType: 'cloud', hoursAgo: 72, turnCount: 38, fileCount: 13, cwd: 'C:\\code\\copilot-cli' },

  // contoso/webapp sessions
  { repo: 'contoso/webapp', branch: 'feature/dashboard', summary: 'Build analytics dashboard with real-time charts', hostType: 'cli', hoursAgo: 2.5, turnCount: 29, fileCount: 10, cwd: 'C:\\code\\contoso-webapp' },
  { repo: 'contoso/webapp', branch: 'fix/auth-redirect', summary: 'Fix OAuth redirect loop on expired sessions', hostType: 'cli', hoursAgo: 7, turnCount: 9, fileCount: 3, cwd: 'C:\\code\\contoso-webapp' },
  { repo: 'contoso/webapp', branch: 'main', summary: 'Add Playwright E2E tests for checkout flow', hostType: 'actions', hoursAgo: 16, turnCount: 20, fileCount: 7, cwd: 'C:\\code\\contoso-webapp' },
  { repo: 'contoso/webapp', branch: 'feature/i18n', summary: 'Implement internationalization with react-intl', hostType: 'cli', hoursAgo: 96, turnCount: 33, fileCount: 15, cwd: 'C:\\code\\contoso-webapp' },

  // Additional varied sessions
  { repo: 'jongio/life', branch: 'main', summary: 'Refactor game engine to use ECS architecture', hostType: 'cli', hoursAgo: 28, turnCount: 44, fileCount: 20, cwd: 'C:\\code\\life' },
  { repo: 'jongio/life', branch: 'feature/multiplayer', summary: 'Add WebSocket multiplayer with state sync', hostType: 'cloud', hoursAgo: 120, turnCount: 50, fileCount: 22, cwd: 'C:\\code\\life' },
  { repo: 'microsoft/TypeScript', branch: 'fix/narrowing', summary: 'Fix type narrowing in switch statements with generics', hostType: 'cli', hoursAgo: 40, turnCount: 12, fileCount: 4, cwd: 'C:\\code\\TypeScript' },
  { repo: 'azure/azure-dev', branch: 'feature/hooks', summary: 'Implement pre/post deployment hooks for azd', hostType: 'cli', hoursAgo: 60, turnCount: 25, fileCount: 9, cwd: 'C:\\code\\azure-dev' },
  { repo: 'github/copilot-cli', branch: 'feature/agent-swarm', summary: 'Multi-agent coordination with worktree isolation', hostType: 'cli', hoursAgo: 168, turnCount: 48, fileCount: 16, cwd: 'C:\\code\\copilot-cli' },
  { repo: 'contoso/webapp', branch: 'perf/bundle-size', summary: 'Reduce bundle size by 40% with code splitting', hostType: 'cli', hoursAgo: 200, turnCount: 17, fileCount: 6, cwd: 'C:\\code\\contoso-webapp' },
  { repo: 'azure/azure-sdk-for-go', branch: 'feature/tracing', summary: 'Add OpenTelemetry distributed tracing support', hostType: 'actions', hoursAgo: 240, turnCount: 21, fileCount: 8, cwd: 'C:\\code\\azure-sdk-for-go' },
  { repo: 'jongio/dispatch', branch: 'feature/notifications', summary: 'System tray notifications for attention changes', hostType: 'cli', hoursAgo: 336, turnCount: 13, fileCount: 5, cwd: 'C:\\code\\dispatch' },
];

const ATTENTION_STATUSES = ['working', 'thinking', 'waiting', 'interrupted', 'active', 'idle', 'stale'] as const;

const CONVERSATIONS: Array<{ user: string; assistant: string }[]> = [
  [
    { user: 'Can you help me implement a file watcher that debounces rapid changes?', assistant: 'I\'ll create a file watcher using chokidar with a debounce mechanism. The key is to batch filesystem events that fire within a short window (typically 100-300ms) into a single callback invocation.' },
    { user: 'The debounce works but it fires twice on rename operations. Can you fix that?', assistant: 'Rename operations emit both an `unlink` and an `add` event. I\'ll track pending events by path and coalesce rename pairs within the debounce window. Here\'s the updated implementation.' },
  ],
  [
    { user: 'I need to add OAuth2 device code flow. The user should see a URL and code to enter.', assistant: 'I\'ll implement the device code flow following RFC 8628. The flow is: request device code from the authorization server, display the verification URI and user code, then poll the token endpoint until the user completes authorization or the code expires.' },
    { user: 'How should I handle the polling interval and slow_down responses?', assistant: 'The token endpoint returns an `interval` field (default 5s). If we get a `slow_down` error, we must increase our polling interval by 5 seconds. I\'ll implement this with an adaptive delay.' },
    { user: 'Perfect. Now add token caching so users dont have to re-auth every time.', assistant: 'I\'ll store tokens in the OS credential store (Windows Credential Manager / macOS Keychain / Linux Secret Service) using the keytar library. On startup, we check for a cached token, validate its expiry, and refresh if needed.' },
  ],
  [
    { user: 'The extension host is crashing when opening large workspaces (>10k files). Can you investigate?', assistant: 'Looking at the crash dump, the issue is in the file indexer. It\'s loading all file metadata into memory synchronously during activation. For 10k+ files, this exceeds the extension host memory limit. I\'ll switch to a streaming approach with batched processing.' },
    { user: 'That fixed the crash but now indexing is slow. Any way to parallelize?', assistant: 'I\'ll use a worker thread pool (4 workers) to process file batches in parallel. Each worker handles metadata extraction for a batch of 500 files. The main thread coordinates batches and aggregates results.' },
  ],
  [
    { user: 'Build a real-time analytics dashboard showing user activity over the last 24 hours.', assistant: 'I\'ll create a dashboard component using recharts for visualization. The layout will have: a line chart for active users over time, a bar chart for actions by category, and summary cards for key metrics. Data updates every 30 seconds via WebSocket.' },
    { user: 'Can you add a date range picker so users can look at historical data too?', assistant: 'I\'ll add a date range picker using react-day-picker. When the range changes, we switch from the WebSocket real-time feed to a REST API call for historical data. The charts will animate the transition between datasets.' },
  ],
  [
    { user: 'I want to reduce our bundle size. Its currently 2.4MB gzipped which is way too large.', assistant: 'Let me analyze the bundle. Running `npx webpack-bundle-analyzer` shows three main culprits: moment.js (300KB), lodash full import (70KB), and the charting library loaded synchronously. I\'ll address each one.' },
    { user: 'Good start. What about the charting library?', assistant: 'I\'ll lazy-load the charting library using React.lazy() with a Suspense boundary. The dashboard route that uses charts will load them on demand. For the initial page load, this removes 450KB from the critical path.' },
  ],
];

const CHECKPOINT_DATA: Array<{ title: string; overview: string }> = [
  { title: 'Initial scaffolding', overview: 'Set up project structure, dependencies, and basic configuration files.' },
  { title: 'Core implementation', overview: 'Implemented the main business logic and data models.' },
  { title: 'Tests passing', overview: 'All unit and integration tests passing with >80% coverage.' },
  { title: 'Review feedback addressed', overview: 'Fixed issues from code review: error handling, naming, edge cases.' },
  { title: 'Ready for merge', overview: 'All CI checks green, documentation updated, changelog entry added.' },
];

const FILE_PATHS: string[] = [
  'src/main/index.ts', 'src/renderer/App.tsx', 'src/services/auth.ts',
  'src/utils/helpers.ts', 'src/components/Dashboard.tsx', 'src/models/user.ts',
  'tests/auth.test.ts', 'package.json', 'src/config/settings.ts',
  'src/hooks/useWebSocket.ts', 'src/api/client.ts', 'src/types/index.ts',
  'src/middleware/cors.ts', 'src/routes/api.ts', 'docs/README.md',
];

const TOOL_NAMES = ['edit', 'create', 'view', 'grep', 'powershell'];

export function generateDemoData(): DemoData {
  const rand = seededRandom(42);
  const sessions: Session[] = [];
  const details = new Map<string, SessionDetail>();
  const attention: Record<string, string> = {};

  for (let i = 0; i < TEMPLATES.length; i++) {
    const t = TEMPLATES[i];
    const id = `demo-${randomId(rand)}-${randomId(rand)}`;
    const createdAt = hoursAgo(t.hoursAgo + rand() * 2);
    const updatedAt = hoursAgo(t.hoursAgo);

    const session: Session = {
      id,
      cwd: t.cwd,
      repository: t.repo,
      branch: t.branch,
      summary: t.summary,
      created_at: createdAt,
      updated_at: updatedAt,
      host_type: t.hostType,
      last_active_at: updatedAt,
      turn_count: t.turnCount,
      file_count: t.fileCount,
    };
    sessions.push(session);

    // Assign attention status to recent sessions
    if (t.hoursAgo < 10) {
      const statusIdx = i % ATTENTION_STATUSES.length;
      attention[id] = ATTENTION_STATUSES[statusIdx];
    }

    // Build detail with conversation turns
    const convIdx = i % CONVERSATIONS.length;
    const conversation = CONVERSATIONS[convIdx];
    const turns: Turn[] = conversation.map((turn, turnIdx) => ({
      session_id: id,
      turn_index: turnIdx,
      user_message: turn.user,
      assistant_response: turn.assistant,
      timestamp: hoursAgo(t.hoursAgo - turnIdx * 0.1),
    }));

    // Some sessions get checkpoints
    const checkpoints: Checkpoint[] = [];
    if (t.turnCount > 15 && rand() > 0.4) {
      const numCheckpoints = Math.min(3, Math.floor(rand() * 4) + 1);
      for (let c = 0; c < numCheckpoints; c++) {
        const cpData = CHECKPOINT_DATA[c % CHECKPOINT_DATA.length];
        checkpoints.push({
          session_id: id,
          checkpoint_number: c + 1,
          title: cpData.title,
          overview: cpData.overview,
        });
      }
    }

    // Generate file references
    const files: SessionFile[] = [];
    const numFiles = Math.min(t.fileCount, 5);
    for (let f = 0; f < numFiles; f++) {
      files.push({
        session_id: id,
        file_path: FILE_PATHS[Math.floor(rand() * FILE_PATHS.length)],
        tool_name: TOOL_NAMES[Math.floor(rand() * TOOL_NAMES.length)],
        turn_index: Math.floor(rand() * turns.length),
      });
    }

    // Generate refs for some sessions
    const refs: SessionRef[] = [];
    if (rand() > 0.5) {
      const refTypes = ['pr', 'issue', 'commit'];
      const numRefs = Math.floor(rand() * 3) + 1;
      for (let r = 0; r < numRefs; r++) {
        const refType = refTypes[Math.floor(rand() * refTypes.length)];
        let refValue: string;
        if (refType === 'commit') {
          refValue = randomId(rand) + randomId(rand).slice(0, 4);
        } else {
          refValue = String(Math.floor(rand() * 500) + 1);
        }
        refs.push({
          session_id: id,
          ref_type: refType,
          ref_value: refValue,
          turn_index: Math.floor(rand() * turns.length),
          created_at: hoursAgo(t.hoursAgo - rand()),
        });
      }
    }

    details.set(id, { session, turns, checkpoints, files, refs });
  }

  return { sessions, details, attention };
}
