import Database from 'better-sqlite3';
import { homedir } from 'os';
import { join } from 'path';
import { existsSync } from 'fs';

export interface Session {
  id: string;
  cwd: string;
  repository: string;
  branch: string;
  summary: string;
  created_at: string;
  updated_at: string;
  host_type: string;
  last_active_at: string;
  turn_count: number;
  file_count: number;
}

export interface SessionDetail {
  session: Session;
  turns: Turn[];
  checkpoints: Checkpoint[];
  files: SessionFile[];
  refs: SessionRef[];
}

export interface Turn {
  session_id: string;
  turn_index: number;
  user_message: string;
  assistant_response: string;
  timestamp: string;
}

export interface Checkpoint {
  session_id: string;
  checkpoint_number: number;
  title: string;
  overview: string;
}

export interface SessionFile {
  session_id: string;
  file_path: string;
  tool_name: string;
  turn_index: number;
}

export interface SessionRef {
  session_id: string;
  ref_type: string;
  ref_value: string;
  turn_index: number;
  created_at: string;
}

export interface SearchResult {
  content: string;
  session_id: string;
  source_type: string;
  rank: number;
}

export interface ListOptions {
  limit?: number;
  sort?: string;
  sortOrder?: 'asc' | 'desc';
  timeRange?: string;
  pivot?: string;
  search?: string;
}

/**
 * SessionStore provides read-only access to the Copilot CLI session store.
 * Mirrors the Go data.Store package functionality.
 */
export class SessionStore {
  private db: Database.Database | null = null;
  private hasFTS5 = false;
  private hasHostType = false;

  constructor() {
    this.open();
  }

  private getStorePath(): string {
    const home = homedir();
    // Platform-specific session store paths
    if (process.platform === 'win32') {
      return join(home, '.copilot', 'session-store.db');
    }
    return join(home, '.copilot', 'session-store.db');
  }

  private open(): void {
    const dbPath = this.getStorePath();
    if (!existsSync(dbPath)) {
      console.warn('Session store not found:', dbPath);
      return;
    }

    this.db = new Database(dbPath, { readonly: true, fileMustExist: true });
    this.db.pragma('journal_mode = WAL');
    this.db.pragma('query_only = ON');

    // Detect schema capabilities
    this.detectCapabilities();
  }

  private detectCapabilities(): void {
    if (!this.db) return;

    // Check for FTS5 search_index
    try {
      this.db.prepare("SELECT 1 FROM search_index LIMIT 1").get();
      this.hasFTS5 = true;
    } catch {
      this.hasFTS5 = false;
    }

    // Check for host_type column
    try {
      this.db.prepare("SELECT host_type FROM sessions LIMIT 1").get();
      this.hasHostType = true;
    } catch {
      this.hasHostType = false;
    }
  }

  list(opts: ListOptions = {}): Session[] {
    if (!this.db) return [];

    const limit = opts.limit ?? 100;
    const sort = opts.sort ?? 'updated';
    const order = opts.sortOrder ?? 'desc';

    const orderClause = this.buildOrderClause(sort, order);
    const hostTypeCol = this.hasHostType ? ", s.host_type" : ", '' as host_type";

    const sql = `
      SELECT s.id, COALESCE(s.cwd, '') as cwd, COALESCE(s.repository, '') as repository,
             COALESCE(s.branch, '') as branch, COALESCE(s.summary, '') as summary,
             s.created_at, s.updated_at${hostTypeCol},
             MAX(COALESCE(t_max.max_ts, s.updated_at, s.created_at)) as last_active_at,
             COUNT(DISTINCT t.turn_index) as turn_count,
             COUNT(DISTINCT f.file_path) as file_count
      FROM sessions s
      LEFT JOIN (SELECT session_id, MAX(timestamp) as max_ts FROM turns GROUP BY session_id) t_max
        ON t_max.session_id = s.id
      LEFT JOIN turns t ON t.session_id = s.id
      LEFT JOIN session_files f ON f.session_id = s.id
      GROUP BY s.id
      ${orderClause}
      LIMIT ?
    `;

    return this.db.prepare(sql).all(limit) as Session[];
  }

  search(query: string): Session[] {
    if (!this.db || !query.trim()) return this.list();

    const pattern = `%${query}%`;
    const hostTypeCol = this.hasHostType ? ", s.host_type" : ", '' as host_type";

    const sql = `
      SELECT s.id, COALESCE(s.cwd, '') as cwd, COALESCE(s.repository, '') as repository,
             COALESCE(s.branch, '') as branch, COALESCE(s.summary, '') as summary,
             s.created_at, s.updated_at${hostTypeCol},
             s.updated_at as last_active_at,
             0 as turn_count, 0 as file_count
      FROM sessions s
      WHERE s.summary LIKE ? OR s.repository LIKE ? OR s.branch LIKE ? OR s.cwd LIKE ?
      ORDER BY s.updated_at DESC
      LIMIT 100
    `;

    return this.db.prepare(sql).all(pattern, pattern, pattern, pattern) as Session[];
  }

  searchDeep(query: string): SearchResult[] {
    if (!this.db || !this.hasFTS5 || !query.trim()) return [];

    const sql = `
      SELECT content, session_id, source_type, rank
      FROM search_index
      WHERE search_index MATCH ?
      ORDER BY rank
      LIMIT 50
    `;

    try {
      return this.db.prepare(sql).all(query) as SearchResult[];
    } catch {
      return [];
    }
  }

  getDetail(id: string): SessionDetail | null {
    if (!this.db) return null;

    const hostTypeCol = this.hasHostType ? ", host_type" : ", '' as host_type";
    const session = this.db.prepare(
      `SELECT id, COALESCE(cwd,'') as cwd, COALESCE(repository,'') as repository,
              COALESCE(branch,'') as branch, COALESCE(summary,'') as summary,
              created_at, updated_at${hostTypeCol}, updated_at as last_active_at,
              0 as turn_count, 0 as file_count
       FROM sessions WHERE id = ?`
    ).get(id) as Session | undefined;

    if (!session) return null;

    const turns = this.db.prepare(
      'SELECT * FROM turns WHERE session_id = ? ORDER BY turn_index'
    ).all(id) as Turn[];

    const checkpoints = this.db.prepare(
      'SELECT session_id, checkpoint_number, title, overview FROM checkpoints WHERE session_id = ? ORDER BY checkpoint_number'
    ).all(id) as Checkpoint[];

    const files = this.db.prepare(
      'SELECT session_id, file_path, tool_name, turn_index FROM session_files WHERE session_id = ? LIMIT 50'
    ).all(id) as SessionFile[];

    const refs = this.db.prepare(
      'SELECT * FROM session_refs WHERE session_id = ? ORDER BY created_at DESC'
    ).all(id) as SessionRef[];

    return { session, turns, checkpoints, files, refs };
  }

  close(): void {
    this.db?.close();
    this.db = null;
  }

  private buildOrderClause(sort: string, order: string): string {
    const dir = order === 'asc' ? 'ASC' : 'DESC';
    switch (sort) {
      case 'created': return `ORDER BY s.created_at ${dir}`;
      case 'turns': return `ORDER BY turn_count ${dir}`;
      case 'name': return `ORDER BY s.summary ${dir}`;
      case 'folder': return `ORDER BY s.cwd ${dir}`;
      default: return `ORDER BY s.updated_at ${dir}`;
    }
  }
}
