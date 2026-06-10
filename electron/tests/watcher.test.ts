import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { FileWatcher } from '../src/main/watcher';
import type { WatcherCallbacks } from '../src/main/watcher';

// Mock chokidar with event emitter behavior
const mockWatcher = {
  on: vi.fn().mockReturnThis(),
  close: vi.fn(),
};

vi.mock('chokidar', () => ({
  watch: vi.fn(() => mockWatcher),
}));

describe('FileWatcher', () => {
  let callbacks: WatcherCallbacks;
  let watcher: FileWatcher;

  beforeEach(() => {
    vi.useFakeTimers();
    vi.clearAllMocks();
    mockWatcher.on.mockReturnThis();

    callbacks = {
      onSessionsChanged: vi.fn(),
      onAttentionUpdate: vi.fn(),
    };
    watcher = new FileWatcher(callbacks);
  });

  afterEach(() => {
    watcher.stop();
    vi.useRealTimers();
  });

  describe('start', () => {
    it('creates two chokidar watchers (db + state)', async () => {
      const { watch } = await import('chokidar');

      watcher.start();

      expect(watch).toHaveBeenCalledTimes(2);
    });

    it('registers change/add handlers on db watcher', () => {
      watcher.start();

      const events = mockWatcher.on.mock.calls.map(([event]) => event);
      expect(events).toContain('change');
      expect(events).toContain('add');
    });

    it('registers change/add/unlink handlers on state watcher', () => {
      watcher.start();

      const events = mockWatcher.on.mock.calls.map(([event]) => event);
      expect(events).toContain('unlink');
    });
  });

  describe('debounced notifications', () => {
    it('fires onSessionsChanged after debounce period on db change', () => {
      watcher.start();

      // Find the 'change' handler for db watcher (first watcher registered)
      const changeHandler = mockWatcher.on.mock.calls.find(
        ([event]) => event === 'change',
      )?.[1];

      changeHandler?.();
      expect(callbacks.onSessionsChanged).not.toHaveBeenCalled();

      vi.advanceTimersByTime(500);
      expect(callbacks.onSessionsChanged).toHaveBeenCalledTimes(1);
    });

    it('coalesces multiple rapid changes into a single callback', () => {
      watcher.start();

      const changeHandler = mockWatcher.on.mock.calls.find(
        ([event]) => event === 'change',
      )?.[1];

      // Fire 5 rapid changes
      changeHandler?.();
      changeHandler?.();
      changeHandler?.();
      changeHandler?.();
      changeHandler?.();

      vi.advanceTimersByTime(500);
      expect(callbacks.onSessionsChanged).toHaveBeenCalledTimes(1);
    });
  });

  describe('pause / resume', () => {
    it('defers callbacks while paused', () => {
      watcher.start();

      const changeHandler = mockWatcher.on.mock.calls.find(
        ([event]) => event === 'change',
      )?.[1];

      watcher.pause();
      changeHandler?.();
      vi.advanceTimersByTime(500);

      expect(callbacks.onSessionsChanged).not.toHaveBeenCalled();
    });

    it('fires pending callbacks on resume', () => {
      watcher.start();

      const changeHandler = mockWatcher.on.mock.calls.find(
        ([event]) => event === 'change',
      )?.[1];

      watcher.pause();
      changeHandler?.();
      vi.advanceTimersByTime(500);

      watcher.resume();
      expect(callbacks.onSessionsChanged).toHaveBeenCalledTimes(1);
    });

    it('does not fire on resume if no pending changes', () => {
      watcher.start();

      watcher.pause();
      watcher.resume();

      expect(callbacks.onSessionsChanged).not.toHaveBeenCalled();
      expect(callbacks.onAttentionUpdate).not.toHaveBeenCalled();
    });
  });

  describe('stop', () => {
    it('closes both watchers and clears timers', () => {
      watcher.start();
      watcher.stop();

      expect(mockWatcher.close).toHaveBeenCalledTimes(2);
    });

    it('is safe to call when not started', () => {
      expect(() => watcher.stop()).not.toThrow();
    });
  });
});
