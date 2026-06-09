import { readdir, readFile, stat, open } from 'fs/promises';
import { homedir } from 'os';
import { join } from 'path';
import { exec } from 'child_process';
import { promisify } from 'util';

const execAsync = promisify(exec);

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type AttentionStatus =
  | 'idle'
  | 'stale'
  | 'active'
  | 'waiting'
  | 'interrupted'
  | 'working'
  | 'thinking'
  | 'compacting';

interface SessionEvent {
  type: string;
  timestamp: string;
}

interface PidResult {
  pid: number; // >0 if a live process owns the session
  hasStale: boolean; // true if at least one lock file references a dead PID
}

// ---------------------------------------------------------------------------
// Constants — mirror the Go implementation
// ---------------------------------------------------------------------------

const SESSION_STATE_REL = '.copilot/session-state';
const LAST_CHUNK_SIZE = 4096;
const MAX_LOCK_FILE_SIZE = 32;
const MAX_LOCK_FILES_PER_SESSION = 10;
const MAX_PID = 4_194_304;

// Event type prefixes
const EVENT_TURN_END = 'assistant.turn_end';
const EVENT_MESSAGE = 'assistant.message';
const EVENT_TURN_START = 'assistant.turn_start';
const EVENT_TOOL_EXECUTION = 'tool.execution';
const EVENT_HOOK = 'hook.';
const EVENT_SUBAGENT = 'subagent.';
const EVENT_PLAN_CHANGED = 'session.plan_changed';
const EVENT_SKILL_INVOKED = 'skill.invoked';
const EVENT_SHUTDOWN = 'session.shutdown';
const EVENT_ABORT = 'abort';
const EVENT_MODEL_CHANGE = 'session.model_change';
const EVENT_SYSTEM_MESSAGE = 'system.message';
const EVENT_COMPACTION = 'session.compaction_complete';

// Age thresholds
const DEAD_SESSION_MAX_AGE_MS = 24 * 60 * 60 * 1000; // 24 hours
const INTERRUPTED_MAX_AGE_MS = 72 * 60 * 60 * 1000; // 72 hours

// Session ID validation pattern — matches Go's validate.SessionIDPattern
const SESSION_ID_PATTERN = /^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$/;

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

/**
 * Scans all session-state directories and returns a map of session ID to
 * attention status. Mirrors the Go ScanAttention function.
 *
 * @param thresholdMs - Stale threshold in milliseconds (default: 5 minutes)
 * @param workspaceRecovery - Enable interrupted detection for stale locks
 */
export async function scanAttention(
  thresholdMs = 5 * 60 * 1000,
  workspaceRecovery = true,
): Promise<Record<string, AttentionStatus>> {
  const stateDir = getSessionStatePath();
  if (!stateDir) return {};

  let entries: string[];
  try {
    const dirEntries = await readdir(stateDir, { withFileTypes: true });
    entries = dirEntries
      .filter((e) => e.isDirectory() && SESSION_ID_PATTERN.test(e.name))
      .map((e) => e.name);
  } catch {
    // Directory doesn't exist or isn't readable
    return {};
  }

  const result: Record<string, AttentionStatus> = {};

  // Process sessions in parallel with concurrency cap
  const CONCURRENCY = 20;
  for (let i = 0; i < entries.length; i += CONCURRENCY) {
    const batch = entries.slice(i, i + CONCURRENCY);
    const statuses = await Promise.all(
      batch.map((sessionId) => {
        const dir = join(stateDir, sessionId);
        return classifySession(dir, thresholdMs, workspaceRecovery);
      }),
    );
    for (let j = 0; j < batch.length; j++) {
      result[batch[j]] = statuses[j];
    }
  }

  return result;
}

// ---------------------------------------------------------------------------
// Classification — ports classifySession / classifyLiveSession from Go
// ---------------------------------------------------------------------------

async function classifySession(
  dir: string,
  thresholdMs: number,
  workspaceRecovery: boolean,
): Promise<AttentionStatus> {
  const pidResult = await findSessionPID(dir);

  if (pidResult.pid > 0) {
    return classifyLiveSession(dir, thresholdMs);
  }

  // No live process — check last event
  const eventsPath = join(dir, 'events.jsonl');
  const evt = await readLastEvent(eventsPath);
  if (!evt) return 'idle';

  const eventTime = parseEventTime(evt.timestamp);
  if (!eventTime) return 'idle';

  // Stale lock (dead PID) with workspace recovery enabled
  if (pidResult.hasStale && workspaceRecovery) {
    if (Date.now() - eventTime.getTime() > INTERRUPTED_MAX_AGE_MS) {
      return 'idle';
    }
    if (evt.type.startsWith(EVENT_TURN_END) || evt.type.startsWith(EVENT_MESSAGE)) {
      return 'waiting';
    }
    return 'interrupted';
  }

  // No stale lock — original dead-session logic
  if (Date.now() - eventTime.getTime() > DEAD_SESSION_MAX_AGE_MS) {
    return 'idle';
  }
  if (evt.type.startsWith(EVENT_TURN_END) || evt.type.startsWith(EVENT_MESSAGE)) {
    return 'waiting';
  }
  return 'idle';
}

async function classifyLiveSession(
  dir: string,
  thresholdMs: number,
): Promise<AttentionStatus> {
  const eventsPath = join(dir, 'events.jsonl');
  const evt = await readLastEvent(eventsPath);
  if (!evt) return 'stale';

  const eventTime = parseEventTime(evt.timestamp);
  if (eventTime && Date.now() - eventTime.getTime() > thresholdMs) {
    return 'stale';
  }

  // Classify by event type
  if (evt.type.startsWith(EVENT_TURN_END) || evt.type.startsWith(EVENT_MESSAGE)) {
    return 'waiting';
  }
  if (evt.type.startsWith(EVENT_TOOL_EXECUTION)) {
    return 'working';
  }
  if (evt.type.startsWith(EVENT_TURN_START)) {
    return 'thinking';
  }
  if (evt.type === EVENT_COMPACTION) {
    return 'compacting';
  }
  if (
    evt.type.startsWith(EVENT_HOOK) ||
    evt.type.startsWith(EVENT_SUBAGENT) ||
    evt.type.startsWith(EVENT_PLAN_CHANGED) ||
    evt.type.startsWith(EVENT_SKILL_INVOKED)
  ) {
    return 'active';
  }
  if (evt.type === EVENT_SHUTDOWN || evt.type === EVENT_ABORT) {
    return 'idle';
  }
  if (evt.type === EVENT_MODEL_CHANGE || evt.type === EVENT_SYSTEM_MESSAGE) {
    return 'waiting';
  }

  // user.message, session.start, or unknown — AI hasn't started responding
  return 'active';
}

// ---------------------------------------------------------------------------
// PID / Lock file handling
// ---------------------------------------------------------------------------

async function findSessionPID(dir: string): Promise<PidResult> {
  let matches: string[];
  try {
    const entries = await readdir(dir);
    matches = entries
      .filter((name) => name.startsWith('inuse.') && name.endsWith('.lock'))
      .slice(0, MAX_LOCK_FILES_PER_SESSION)
      .map((name) => join(dir, name));
  } catch {
    return { pid: 0, hasStale: false };
  }

  if (matches.length === 0) {
    return { pid: 0, hasStale: false };
  }

  let hasStale = false;

  for (const lockPath of matches) {
    try {
      const info = await stat(lockPath);
      if (!info.isFile()) continue;
      if (info.size >= MAX_LOCK_FILE_SIZE) continue;

      const raw = await readFile(lockPath, 'utf-8');
      const pidStr = raw.trim();

      if (pidStr.length === 0) continue;
      if (!/^\d+$/.test(pidStr)) continue;

      const pid = parseInt(pidStr, 10);
      if (pid <= 0 || pid > MAX_PID) continue;

      if (await isProcessAlive(pid)) {
        return { pid, hasStale };
      }
      hasStale = true;
    } catch {
      continue;
    }
  }

  return { pid: 0, hasStale };
}

/**
 * Check if a process with the given PID is alive.
 * On Windows: uses tasklist to check for the PID.
 * On other platforms: sends signal 0 via process.kill.
 */
async function isProcessAlive(pid: number): Promise<boolean> {
  if (process.platform === 'win32') {
    try {
      // Use tasklist filtered by PID — exits with code 0 and produces output if found
      const { stdout } = await execAsync(
        `tasklist /FI "PID eq ${pid}" /NH /FO CSV`,
        { timeout: 2000 },
      );
      // tasklist outputs "INFO: No tasks are running..." when PID not found
      return stdout.includes(`"${pid}"`);
    } catch {
      return false;
    }
  }

  // Unix: signal 0 checks existence without killing
  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
}

// ---------------------------------------------------------------------------
// Event file reading — O(1) seek-from-end strategy
// ---------------------------------------------------------------------------

async function readLastEvent(path: string): Promise<SessionEvent | null> {
  let fh;
  try {
    fh = await open(path, 'r');
    const info = await fh.stat();

    if (info.size === 0) return null;

    const chunkSize = Math.min(LAST_CHUNK_SIZE, info.size);
    const offset = info.size - chunkSize;

    const buffer = Buffer.alloc(chunkSize);
    const { bytesRead } = await fh.read(buffer, 0, chunkSize, offset);
    const content = buffer.subarray(0, bytesRead).toString('utf-8');

    // Find the last complete line (skip trailing newlines)
    const trimmed = content.replace(/[\r\n]+$/, '');
    const lastNL = trimmed.lastIndexOf('\n');
    const lastLine = lastNL >= 0 ? trimmed.substring(lastNL + 1) : trimmed;

    if (!lastLine) return null;

    const evt: SessionEvent = JSON.parse(lastLine);
    return evt;
  } catch {
    return null;
  } finally {
    await fh?.close();
  }
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function parseEventTime(ts: string): Date | null {
  if (!ts) return null;
  const date = new Date(ts);
  if (isNaN(date.getTime())) return null;
  return date;
}

function getSessionStatePath(): string {
  const override = process.env.DISPATCH_SESSION_STATE;
  if (override) {
    // Reject UNC paths on Windows
    if (process.platform === 'win32' && override.startsWith('\\\\')) {
      return '';
    }
    return override;
  }
  return join(homedir(), SESSION_STATE_REL);
}
