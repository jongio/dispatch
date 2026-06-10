import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('fs', () => ({ existsSync: vi.fn() }));
vi.mock('electron', () => ({ app: {}, shell: {} }));

// Shared state for the mock constructor
let mockDbInstance: Record<string, any> | null = null;
let shouldThrow = false;

vi.mock('better-sqlite3', () => {
  // Vitest 4 requires class for `new` — use a class factory
  const MockDatabase = class {
    pragma: any;
    prepare: any;
    close: any;
    constructor() {
      if (shouldThrow) throw new Error('SQLITE_CORRUPT');
      if (mockDbInstance) {
        this.pragma = mockDbInstance.pragma;
        this.prepare = mockDbInstance.prepare;
        this.close = mockDbInstance.close;
      }
    }
  };
  return { default: MockDatabase };
});

import { existsSync } from 'fs';
import { SessionStore } from '../src/main/store';

const existsSyncMock = vi.mocked(existsSync);

function createFakeDb() {
  return {
    pragma: vi.fn(),
    close: vi.fn(),
    prepare: vi.fn(() => ({
      get: vi.fn(() => undefined),
      all: vi.fn(() => []),
    })),
  };
}

describe('SessionStore', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockDbInstance = null;
    shouldThrow = false;
  });

  describe('when database file does not exist', () => {
    beforeEach(() => existsSyncMock.mockReturnValue(false));

    it('list returns empty array', () => {
      expect(new SessionStore().list()).toEqual([]);
    });

    it('search returns empty array', () => {
      expect(new SessionStore().search('q')).toEqual([]);
    });

    it('searchDeep returns empty array', () => {
      expect(new SessionStore().searchDeep('q')).toEqual([]);
    });

    it('getDetail returns null', () => {
      expect(new SessionStore().getDetail('id')).toBeNull();
    });

    it('close does not throw', () => {
      expect(() => new SessionStore().close()).not.toThrow();
    });
  });

  describe('when database constructor throws', () => {
    beforeEach(() => {
      existsSyncMock.mockReturnValue(true);
      shouldThrow = true;
    });

    it('degrades gracefully', () => {
      const store = new SessionStore();
      expect(store.list()).toEqual([]);
      expect(store.getDetail('x')).toBeNull();
    });
  });

  describe('when database opens successfully', () => {
    let fakeDb: ReturnType<typeof createFakeDb>;

    beforeEach(() => {
      existsSyncMock.mockReturnValue(true);
      fakeDb = createFakeDb();
      mockDbInstance = fakeDb;
    });

    it('sets busy_timeout and query_only pragmas', () => {
      new SessionStore();

      expect(fakeDb.pragma).toHaveBeenCalledWith('busy_timeout = 3000');
      expect(fakeDb.pragma).toHaveBeenCalledWith('query_only = ON');
    });

    it('detects schema capabilities via prepare', () => {
      new SessionStore();

      const sqls = fakeDb.prepare.mock.calls.map(([sql]: [string]) => sql);
      expect(sqls.some((s) => s.includes('search_index'))).toBe(true);
      expect(sqls.some((s) => s.includes('host_type'))).toBe(true);
    });

    it('list queries sessions and returns results', () => {
      const sessions = [{ id: 's1', summary: 'Test' }];
      fakeDb.prepare.mockReturnValue({
        get: vi.fn(() => undefined),
        all: vi.fn(() => sessions),
      });

      const store = new SessionStore();
      expect(store.list()).toEqual(sessions);
    });

    it('search uses LIKE with wrapped pattern', () => {
      const allMock = vi.fn(() => []);
      fakeDb.prepare.mockReturnValue({ get: vi.fn(), all: allMock });

      const store = new SessionStore();
      store.search('hello');

      expect(allMock).toHaveBeenCalledWith('%hello%', '%hello%', '%hello%', '%hello%');
    });

    it('search with empty query delegates to list', () => {
      const data = [{ id: 's1' }];
      fakeDb.prepare.mockReturnValue({ get: vi.fn(), all: vi.fn(() => data) });

      const store = new SessionStore();
      expect(store.search('')).toEqual(data);
    });

    it('getDetail returns null when session not found', () => {
      fakeDb.prepare.mockReturnValue({ get: vi.fn(() => undefined), all: vi.fn(() => []) });

      const store = new SessionStore();
      expect(store.getDetail('nope')).toBeNull();
    });

    it('getDetail returns assembled detail when found', () => {
      const session = { id: 's1', cwd: '/', summary: 'Hi' };
      const turns = [{ turn_index: 0 }];
      fakeDb.prepare.mockReturnValue({
        get: vi.fn(() => session),
        all: vi.fn(() => turns),
      });

      const store = new SessionStore();
      const detail = store.getDetail('s1');

      expect(detail).not.toBeNull();
      expect(detail!.session).toEqual(session);
      expect(detail!.turns).toEqual(turns);
    });

    it('close releases db and prevents further queries', () => {
      const store = new SessionStore();
      store.close();

      expect(fakeDb.close).toHaveBeenCalledTimes(1);
      expect(store.list()).toEqual([]);
    });
  });
});
