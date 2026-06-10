import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { SessionRefresher } from '../src/main/refresher';

describe('SessionRefresher', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  describe('start', () => {
    it('begins periodic refresh at the configured interval', () => {
      const onRefresh = vi.fn();
      const refresher = new SessionRefresher({ intervalMs: 1000, onRefresh });

      refresher.start();
      vi.advanceTimersByTime(1000);

      expect(onRefresh).toHaveBeenCalledTimes(1);

      vi.advanceTimersByTime(1000);
      expect(onRefresh).toHaveBeenCalledTimes(2);

      refresher.stop();
    });

    it('is idempotent — calling start twice does not create duplicate timers', () => {
      const onRefresh = vi.fn();
      const refresher = new SessionRefresher({ intervalMs: 1000, onRefresh });

      refresher.start();
      refresher.start(); // second call should no-op

      vi.advanceTimersByTime(1000);
      expect(onRefresh).toHaveBeenCalledTimes(1);

      refresher.stop();
    });

    it('defaults to 30s interval when not specified', () => {
      const onRefresh = vi.fn();
      const refresher = new SessionRefresher({ onRefresh });

      refresher.start();
      vi.advanceTimersByTime(29_999);
      expect(onRefresh).not.toHaveBeenCalled();

      vi.advanceTimersByTime(1);
      expect(onRefresh).toHaveBeenCalledTimes(1);

      refresher.stop();
    });
  });

  describe('stop', () => {
    it('clears the interval timer', () => {
      const onRefresh = vi.fn();
      const refresher = new SessionRefresher({ intervalMs: 1000, onRefresh });

      refresher.start();
      refresher.stop();

      vi.advanceTimersByTime(5000);
      expect(onRefresh).not.toHaveBeenCalled();
    });

    it('is safe to call when not started', () => {
      const refresher = new SessionRefresher({ onRefresh: vi.fn() });
      expect(() => refresher.stop()).not.toThrow();
    });
  });

  describe('pause / resume', () => {
    it('suppresses tick callbacks while paused', () => {
      const onRefresh = vi.fn();
      const refresher = new SessionRefresher({ intervalMs: 1000, onRefresh });

      refresher.start();
      refresher.pause();

      vi.advanceTimersByTime(3000);
      expect(onRefresh).not.toHaveBeenCalled();

      refresher.stop();
    });

    it('fires immediately on resume if stale (elapsed >= interval)', () => {
      const onRefresh = vi.fn();
      const refresher = new SessionRefresher({ intervalMs: 1000, onRefresh });

      refresher.start();
      refresher.pause();

      vi.advanceTimersByTime(2000); // stale: 2s > 1s interval
      refresher.resume();

      expect(onRefresh).toHaveBeenCalledTimes(1);

      refresher.stop();
    });

    it('does not fire on resume if not stale', () => {
      const onRefresh = vi.fn();
      const refresher = new SessionRefresher({ intervalMs: 5000, onRefresh });

      refresher.start();
      refresher.pause();

      vi.advanceTimersByTime(1000); // not stale: 1s < 5s interval
      refresher.resume();

      expect(onRefresh).not.toHaveBeenCalled();

      refresher.stop();
    });
  });

  describe('manualRefresh', () => {
    it('invokes onRefresh and returns true', () => {
      const onRefresh = vi.fn();
      const refresher = new SessionRefresher({ intervalMs: 10_000, onRefresh });

      const result = refresher.manualRefresh();

      expect(result).toBe(true);
      expect(onRefresh).toHaveBeenCalledTimes(1);
    });

    it('returns false if already refreshing (re-entrant call)', () => {
      let reentrantResult: boolean | undefined;
      const onRefresh = vi.fn(() => {
        // Attempt re-entrant refresh during callback
        reentrantResult = refresher.manualRefresh();
      });
      const refresher = new SessionRefresher({ intervalMs: 10_000, onRefresh });

      refresher.manualRefresh();

      expect(reentrantResult).toBe(false);
      expect(onRefresh).toHaveBeenCalledTimes(1); // Not called twice
    });

    it('updates lastRefresh timestamp', () => {
      const onRefresh = vi.fn();
      const refresher = new SessionRefresher({ intervalMs: 10_000, onRefresh });

      vi.setSystemTime(new Date('2026-01-01T00:00:00Z'));
      refresher.manualRefresh();

      expect(refresher.lastRefresh).toBe(new Date('2026-01-01T00:00:00Z').getTime());
    });
  });

  describe('isRefreshing', () => {
    it('is true during the onRefresh callback', () => {
      let wasRefreshing = false;
      const onRefresh = vi.fn(() => {
        wasRefreshing = refresher.isRefreshing;
      });
      const refresher = new SessionRefresher({ intervalMs: 10_000, onRefresh });

      refresher.manualRefresh();

      expect(wasRefreshing).toBe(true);
      expect(refresher.isRefreshing).toBe(false); // False after callback returns
    });
  });

  describe('error resilience', () => {
    it('continues working after onRefresh throws', () => {
      const onRefresh = vi.fn()
        .mockImplementationOnce(() => { throw new Error('boom'); })
        .mockImplementation(() => {});
      const refresher = new SessionRefresher({ intervalMs: 1000, onRefresh });

      refresher.start();

      // First tick throws — should not kill the refresher
      expect(() => vi.advanceTimersByTime(1000)).toThrow('boom');

      // Second tick should still fire
      vi.advanceTimersByTime(1000);
      expect(onRefresh).toHaveBeenCalledTimes(2);

      refresher.stop();
    });
  });
});
