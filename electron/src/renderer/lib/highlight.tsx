import React from 'react';

/**
 * Splits text into segments and wraps matched portions in <mark> tags.
 * Uses case-insensitive literal matching (not regex) for safety.
 *
 * When splitting on a regex with a capture group, the captured matches
 * appear at odd indices in the resulting array. We use this to determine
 * which segments to highlight without stateful regex testing.
 */
export function highlightMatches(text: string, query: string): React.ReactNode {
  if (!query || !text) return text;

  const trimmed = query.trim();
  if (!trimmed) return text;

  // Escape regex special characters for safe literal matching
  const escaped = trimmed.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const regex = new RegExp(`(${escaped})`, 'gi');
  const parts = text.split(regex);

  if (parts.length === 1) return text;

  return (
    <>
      {parts.map((part, i) =>
        i % 2 === 1 ? (
          <mark
            key={i}
            className="bg-yellow-200/30 text-foreground rounded-sm"
          >
            {part}
          </mark>
        ) : (
          <React.Fragment key={i}>{part}</React.Fragment>
        ),
      )}
    </>
  );
}
