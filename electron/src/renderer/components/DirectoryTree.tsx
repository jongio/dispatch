import React, { useMemo, useState, useCallback } from 'react';
import { useSessionStore } from '../stores/sessionStore';

/** A single node in the directory tree. */
interface TreeNode {
  /** The directory segment name (leaf portion). */
  name: string;
  /** Full path from root to this node. */
  path: string;
  /** Number of sessions whose cwd is this node or a descendant. */
  sessionCount: number;
  /** Child directories. */
  children: TreeNode[];
}

/**
 * Counts sessions per tree node by walking the tree and incrementing
 * ancestors for each matching session path.
 */
function countSessions(roots: TreeNode[], cwdPaths: string[]): TreeNode[] {
  // Reset counts
  function resetCounts(nodes: TreeNode[]) {
    for (const node of nodes) {
      node.sessionCount = 0;
      resetCounts(node.children);
    }
  }
  resetCounts(roots);

  // For each session cwd, walk the tree and increment matching nodes
  for (const rawPath of cwdPaths) {
    if (!rawPath) continue;
    const normalized = rawPath.replace(/\\/g, '/').replace(/\/$/, '');

    function incrementMatching(nodes: TreeNode[], path: string) {
      for (const node of nodes) {
        if (path === node.path || path.startsWith(node.path + '/')) {
          node.sessionCount++;
          incrementMatching(node.children, path);
        }
      }
    }
    incrementMatching(roots, normalized);
  }

  return roots;
}

/**
 * Builds and counts a full tree from session cwd paths.
 * Collapses single-child chains into combined nodes for readability.
 */
function buildSessionTree(cwdPaths: string[]): TreeNode[] {
  // Deduplicate paths for tree structure, but keep all for counting
  const uniquePaths = [...new Set(cwdPaths.filter(Boolean).map((p) => p.replace(/\\/g, '/').replace(/\/$/, '')))];

  // Build tree from unique paths
  const tree = buildTreeFromPaths(uniquePaths);

  // Count sessions
  countSessions(tree, cwdPaths);

  // Collapse single-child chains
  return collapseSingleChildren(tree);
}

/**
 * More robust tree builder: inserts each full path as nodes in the tree.
 */
function buildTreeFromPaths(paths: string[]): TreeNode[] {
  interface BuildNode {
    name: string;
    path: string;
    sessionCount: number;
    children: Map<string, BuildNode>;
  }

  const rootMap: Map<string, BuildNode> = new Map();

  for (const path of paths) {
    const segments = path.split('/').filter(Boolean);
    let currentMap = rootMap;
    let currentPath = '';

    for (const segment of segments) {
      currentPath = currentPath ? `${currentPath}/${segment}` : segment;

      if (!currentMap.has(segment)) {
        currentMap.set(segment, {
          name: segment,
          path: currentPath,
          sessionCount: 0,
          children: new Map(),
        });
      }

      currentMap = currentMap.get(segment)!.children;
    }
  }

  function toTreeNodes(map: Map<string, BuildNode>): TreeNode[] {
    return Array.from(map.values()).map((n) => ({
      name: n.name,
      path: n.path,
      sessionCount: n.sessionCount,
      children: toTreeNodes(n.children),
    }));
  }

  return toTreeNodes(rootMap);
}

/**
 * Collapses chains where a node has exactly one child into combined nodes.
 * Example: usr -> local -> bin becomes "usr/local/bin" as a single node.
 */
function collapseSingleChildren(nodes: TreeNode[]): TreeNode[] {
  return nodes.map((node) => {
    // Recursively collapse children first
    let collapsed = { ...node, children: collapseSingleChildren(node.children) };

    // If this node has exactly one child and zero direct sessions beyond the child,
    // merge them into a combined display name
    while (collapsed.children.length === 1) {
      const child = collapsed.children[0];
      // Only collapse if the parent has no sessions that terminate here
      // (i.e., all its sessions are in the single child branch)
      if (collapsed.sessionCount > 0 && collapsed.sessionCount !== child.sessionCount) {
        break;
      }
      collapsed = {
        ...child,
        name: `${collapsed.name}/${child.name}`,
        children: collapseSingleChildren(child.children),
      };
    }

    return collapsed;
  });
}

/** Filters tree nodes by search query (matches node name case-insensitively). */
function filterTree(nodes: TreeNode[], query: string): TreeNode[] {
  if (!query) return nodes;
  const lower = query.toLowerCase();

  return nodes.reduce<TreeNode[]>((acc, node) => {
    const nameMatch = node.name.toLowerCase().includes(lower);
    const filteredChildren = filterTree(node.children, query);

    if (nameMatch || filteredChildren.length > 0) {
      acc.push({
        ...node,
        children: nameMatch ? node.children : filteredChildren,
      });
    }
    return acc;
  }, []);
}

interface TreeNodeRowProps {
  node: TreeNode;
  depth: number;
  isExcluded: boolean;
  expandedPaths: Set<string>;
  onToggleExpand: (path: string) => void;
  onToggleExclude: (path: string, include: boolean) => void;
}

function TreeNodeRow({ node, depth, isExcluded, expandedPaths, onToggleExpand, onToggleExclude }: TreeNodeRowProps) {
  const isExpanded = expandedPaths.has(node.path);
  const hasChildren = node.children.length > 0;

  return (
    <>
      <div
        className={`
          flex items-center gap-1 py-0.5 px-1 rounded cursor-pointer text-xs
          hover:bg-[var(--hover-bg)] transition-colors duration-75
          ${isExcluded ? 'opacity-50' : ''}
        `}
        style={{ paddingLeft: `${depth * 12 + 4}px` }}
      >
        {/* Expand/collapse arrow */}
        <button
          onClick={() => hasChildren && onToggleExpand(node.path)}
          className={`
            w-4 h-4 flex items-center justify-center flex-shrink-0 text-[var(--fg-muted)]
            ${hasChildren ? 'hover:text-[var(--fg-primary)]' : 'invisible'}
          `}
        >
          <svg
            width="10"
            height="10"
            viewBox="0 0 10 10"
            className={`transition-transform duration-100 ${isExpanded ? 'rotate-90' : ''}`}
          >
            <path d="M3 2 L7 5 L3 8" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
          </svg>
        </button>

        {/* Checkbox */}
        <input
          type="checkbox"
          checked={!isExcluded}
          onChange={(e) => onToggleExclude(node.path, e.target.checked)}
          className="w-3 h-3 rounded border-[var(--border-primary)] accent-[var(--accent-primary)] cursor-pointer flex-shrink-0"
          onClick={(e) => e.stopPropagation()}
        />

        {/* Directory name */}
        <span
          className="truncate text-[var(--fg-secondary)] hover:text-[var(--fg-primary)] flex-1"
          onClick={() => hasChildren && onToggleExpand(node.path)}
          title={node.path}
        >
          {node.name}
        </span>

        {/* Session count badge */}
        {node.sessionCount > 0 && (
          <span className="text-[10px] text-[var(--fg-muted)] bg-[var(--bg-tertiary)] px-1 rounded flex-shrink-0">
            {node.sessionCount}
          </span>
        )}
      </div>

      {/* Children (lazy: only render when expanded) */}
      {isExpanded && hasChildren && (
        <TreeChildren
          nodes={node.children}
          depth={depth + 1}
          expandedPaths={expandedPaths}
          onToggleExpand={onToggleExpand}
          onToggleExclude={onToggleExclude}
        />
      )}
    </>
  );
}

interface TreeChildrenProps {
  nodes: TreeNode[];
  depth: number;
  expandedPaths: Set<string>;
  onToggleExpand: (path: string) => void;
  onToggleExclude: (path: string, include: boolean) => void;
}

function TreeChildren({ nodes, depth, expandedPaths, onToggleExpand, onToggleExclude }: TreeChildrenProps) {
  const { excludedDirs } = useSessionStore();

  return (
    <>
      {nodes.map((child) => (
        <TreeNodeRow
          key={child.path}
          node={child}
          depth={depth}
          isExcluded={excludedDirs.some((d) => child.path === d || child.path.startsWith(d + '/'))}
          expandedPaths={expandedPaths}
          onToggleExpand={onToggleExpand}
          onToggleExclude={onToggleExclude}
        />
      ))}
    </>
  );
}

export function DirectoryTree() {
  const { sessions, excludedDirs, setExcludedDirs } = useSessionStore();
  const [expandedPaths, setExpandedPaths] = useState<Set<string>>(new Set());
  const [searchFilter, setSearchFilter] = useState('');

  // Build the tree from session cwd paths
  const tree = useMemo(() => {
    const cwdPaths = sessions.map((s) => s.cwd).filter(Boolean);
    return buildSessionTree(cwdPaths);
  }, [sessions]);

  // Filter tree by search
  const filteredTree = useMemo(() => {
    return filterTree(tree, searchFilter);
  }, [tree, searchFilter]);

  const handleToggleExpand = useCallback((path: string) => {
    setExpandedPaths((prev) => {
      const next = new Set(prev);
      if (next.has(path)) {
        next.delete(path);
      } else {
        next.add(path);
      }
      return next;
    });
  }, []);

  const handleToggleExclude = useCallback((path: string, include: boolean) => {
    const normalizedPath = path.replace(/\\/g, '/');

    if (include) {
      // Remove this path and any children from excluded
      const updated = excludedDirs.filter(
        (d) => d !== normalizedPath && !d.startsWith(normalizedPath + '/')
      );
      setExcludedDirs(updated);
    } else {
      // Add this path to excluded (also remove redundant child exclusions)
      const filtered = excludedDirs.filter((d) => !d.startsWith(normalizedPath + '/'));
      setExcludedDirs([...filtered, normalizedPath]);
    }
  }, [excludedDirs, setExcludedDirs]);

  // Expand all when searching
  const effectiveExpanded = useMemo(() => {
    if (searchFilter) {
      // Auto-expand all nodes when filtering
      const allPaths = new Set<string>();
      function collectPaths(nodes: TreeNode[]) {
        for (const node of nodes) {
          allPaths.add(node.path);
          collectPaths(node.children);
        }
      }
      collectPaths(filteredTree);
      return allPaths;
    }
    return expandedPaths;
  }, [searchFilter, filteredTree, expandedPaths]);

  if (sessions.length === 0) {
    return (
      <div className="text-xs text-[var(--fg-muted)] px-2 py-1">
        No sessions loaded
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-1">
      {/* Search within tree */}
      <input
        type="text"
        value={searchFilter}
        onChange={(e) => setSearchFilter(e.target.value)}
        placeholder="Filter dirs..."
        className="w-full px-2 py-1 text-xs rounded bg-[var(--bg-tertiary)] border border-[var(--border-subtle)] text-[var(--fg-primary)] placeholder-[var(--fg-muted)] outline-none focus:border-[var(--accent-primary)]"
      />

      {/* Tree nodes */}
      <div className="max-h-[300px] overflow-y-auto">
        {filteredTree.length === 0 ? (
          <div className="text-xs text-[var(--fg-muted)] px-2 py-1">
            {searchFilter ? 'No matching directories' : 'No directories'}
          </div>
        ) : (
          filteredTree.map((node) => (
            <TreeNodeRow
              key={node.path}
              node={node}
              depth={0}
              isExcluded={excludedDirs.some((d) => node.path === d || node.path.startsWith(d + '/'))}
              expandedPaths={effectiveExpanded}
              onToggleExpand={handleToggleExpand}
              onToggleExclude={handleToggleExclude}
            />
          ))
        )}
      </div>

      {/* Quick actions */}
      {excludedDirs.length > 0 && (
        <button
          onClick={() => setExcludedDirs([])}
          className="text-[10px] text-[var(--accent-primary)] hover:underline px-2 text-left"
        >
          Clear all filters ({excludedDirs.length})
        </button>
      )}
    </div>
  );
}
