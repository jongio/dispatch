import { generateDemoData } from './demo';
import type { DemoData } from './demo';
import type { Session, SessionDetail, SearchResult, ListOptions } from './store';

/**
 * DemoSessionStore provides synthetic session data for screenshots and demos.
 * It implements the same interface as SessionStore but returns pre-generated
 * data instead of reading from SQLite.
 */
export class DemoSessionStore {
  private data: DemoData;

  constructor() {
    this.data = generateDemoData();
  }

  list(opts: ListOptions = {}): Session[] {
    let sessions = [...this.data.sessions];

    // Apply time range filter
    if (opts.timeRange && opts.timeRange !== 'all') {
      const now = Date.now();
      let cutoffMs: number;
      switch (opts.timeRange) {
        case '1h':
          cutoffMs = 60 * 60 * 1000;
          break;
        case '1d':
          cutoffMs = 24 * 60 * 60 * 1000;
          break;
        case '7d':
          cutoffMs = 7 * 24 * 60 * 60 * 1000;
          break;
        default:
          cutoffMs = Infinity;
      }
      const cutoff = new Date(now - cutoffMs).toISOString();
      sessions = sessions.filter((s) => s.updated_at >= cutoff);
    }

    // Apply sort
    const sort = opts.sort ?? 'updated';
    const order = opts.sortOrder ?? 'desc';
    const dir = order === 'desc' ? -1 : 1;

    sessions.sort((a, b) => {
      switch (sort) {
        case 'created':
          return dir * a.created_at.localeCompare(b.created_at);
        case 'turns':
          return dir * (a.turn_count - b.turn_count);
        case 'name':
          return dir * a.summary.localeCompare(b.summary);
        case 'folder':
          return dir * a.cwd.localeCompare(b.cwd);
        default:
          return dir * a.updated_at.localeCompare(b.updated_at);
      }
    });

    // Apply limit
    const limit = opts.limit ?? 100;
    return sessions.slice(0, limit);
  }

  search(query: string): Session[] {
    if (!query.trim()) return this.list();

    const lower = query.toLowerCase();
    return this.data.sessions.filter(
      (s) =>
        s.summary.toLowerCase().includes(lower) ||
        s.repository.toLowerCase().includes(lower) ||
        s.branch.toLowerCase().includes(lower) ||
        s.cwd.toLowerCase().includes(lower),
    );
  }

  searchDeep(_query: string): SearchResult[] {
    // No FTS in demo mode
    return [];
  }

  getDetail(id: string): SessionDetail | null {
    return this.data.details.get(id) ?? null;
  }

  getAttention(): Record<string, string> {
    return this.data.attention;
  }

  close(): void {
    // No resources to release
  }
}
