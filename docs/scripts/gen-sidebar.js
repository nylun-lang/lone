#!/usr/bin/env node

/**
 * Falone/Lone Documentation Sidebar & Versioning Generator
 * 
 * Recursively scans docs/docs/content/<sem-ver>/ to build:
 * 1. Version-aware VitePress sidebar configurations
 * 2. Navigation items with version switcher
 * 3. Synced content/latest directory matching highest semver
 */

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const DOCS_ROOT = path.resolve(__dirname, '..', 'docs');
const CONTENT_DIR = path.join(DOCS_ROOT, 'content');
const LATEST_DIR = path.join(CONTENT_DIR, 'latest');
const VITEPRESS_DIR = path.join(DOCS_ROOT, '.vitepress');
const OUTPUT_FILE = path.join(VITEPRESS_DIR, 'sidebar.generated.json');

const SECTION_TITLES = {
  guide: 'Getting Started & Guides',
  language: 'Language Reference',
  toolchain: 'Compiler & Toolchain',
  falone: 'Falone Compiler & Adapters',
  spec: 'Language Specification',
  runtime: 'WebAssembly Runtime',
};

const SECTION_ORDER = ['guide', 'language', 'toolchain', 'falone', 'runtime', 'spec'];

function parseSemVer(v) {
  const clean = v.replace(/^v/, '');
  const parts = clean.split('-')[0].split('.').map(n => parseInt(n, 10));
  if (parts.some(isNaN) || parts.length < 2) return null;
  return {
    raw: v,
    clean,
    major: parts[0] || 0,
    minor: parts[1] || 0,
    patch: parts[2] || 0,
  };
}

function compareSemVer(a, b) {
  if (a.major !== b.major) return a.major - b.major;
  if (a.minor !== b.minor) return a.minor - b.minor;
  return a.patch - b.patch;
}

function extractMetadata(filePath) {
  const content = fs.readFileSync(filePath, 'utf-8');
  let title = null;
  let order = 999;

  // Frontmatter check
  if (content.startsWith('---')) {
    const end = content.indexOf('---', 3);
    if (end !== -1) {
      const frontmatter = content.slice(3, end);
      const titleMatch = frontmatter.match(/title:\s*["']?([^"'\n\r]+)["']?/);
      if (titleMatch) title = titleMatch[1].trim();
      const orderMatch = frontmatter.match(/order:\s*(\d+)/);
      if (orderMatch) order = parseInt(orderMatch[1], 10);
    }
  }

  // Heading 1 check
  if (!title) {
    const h1Match = content.match(/^#\s+(.+)$/m);
    if (h1Match) {
      title = h1Match[1].trim();
    }
  }

  // Filename fallback
  if (!title) {
    const base = path.basename(filePath, '.md');
    title = base
      .split(/[-_]/)
      .map(w => w.charAt(0).toUpperCase() + w.slice(1))
      .join(' ');
  }

  return { title, order };
}

function scanDirRecursive(dir, rootDir) {
  let results = [];
  const entries = fs.readdirSync(dir, { withFileTypes: true });

  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      results = results.concat(scanDirRecursive(fullPath, rootDir));
    } else if (entry.isFile() && entry.name.endsWith('.md')) {
      const relPath = path.relative(rootDir, fullPath).replace(/\\/g, '/');
      results.push({
        fullPath,
        relPath,
        ext: path.extname(entry.name),
        baseName: path.basename(entry.name, '.md'),
      });
    }
  }

  return results;
}

function buildVersionSidebar(versionDir, routePrefix) {
  const files = scanDirRecursive(versionDir, versionDir);
  const groups = new Map();

  for (const file of files) {
    const { title, order } = extractMetadata(file.fullPath);
    const linkPath = '/' + routePrefix.replace(/^\/|\/$/g, '') + '/' + file.relPath.replace(/\.md$/, '');

    const dirPart = path.dirname(file.relPath);
    const sectionKey = dirPart === '.' ? 'general' : dirPart.split('/')[0];
    const sectionTitle = SECTION_TITLES[sectionKey] || (sectionKey.charAt(0).toUpperCase() + sectionKey.slice(1));

    if (!groups.has(sectionKey)) {
      groups.set(sectionKey, {
        text: sectionTitle,
        collapsible: true,
        collapsed: false,
        key: sectionKey,
        items: [],
      });
    }

    groups.get(sectionKey).items.push({
      text: title,
      link: linkPath,
      order,
    });
  }

  // Sort groups and items within groups
  const sortedGroups = Array.from(groups.values()).sort((a, b) => {
    const idxA = SECTION_ORDER.indexOf(a.key);
    const idxB = SECTION_ORDER.indexOf(b.key);
    if (idxA !== -1 && idxB !== -1) return idxA - idxB;
    if (idxA !== -1) return -1;
    if (idxB !== -1) return 1;
    return a.text.localeCompare(b.text);
  });

  for (const group of sortedGroups) {
    group.items.sort((a, b) => {
      if (a.order !== b.order) return a.order - b.order;
      return a.text.localeCompare(b.text);
    });
  }

  return sortedGroups;
}

function syncDirectory(src, dest) {
  if (fs.existsSync(dest)) {
    fs.rmSync(dest, { recursive: true, force: true });
  }
  fs.mkdirSync(dest, { recursive: true });
  fs.cpSync(src, dest, { recursive: true });
}

export function generateSidebar() {
  if (!fs.existsSync(CONTENT_DIR)) {
    fs.mkdirSync(CONTENT_DIR, { recursive: true });
  }

  const entries = fs.readdirSync(CONTENT_DIR, { withFileTypes: true });
  const versions = [];

  for (const entry of entries) {
    if (entry.isDirectory() && entry.name !== 'latest') {
      const parsed = parseSemVer(entry.name);
      if (parsed) {
        versions.push(parsed);
      }
    }
  }

  if (versions.length === 0) {
    console.log('[gen-sidebar] No sem-ver directories found in content/.');
    return { sidebar: {}, nav: [], latestVersion: null, versions: [] };
  }

  // Sort versions descending (highest version first)
  versions.sort((a, b) => compareSemVer(b, a));
  const latest = versions[0];
  const sortedVersionNames = versions.map(v => v.raw);

  console.log(`[gen-sidebar] Found versions: ${sortedVersionNames.join(', ')}`);
  console.log(`[gen-sidebar] Highest version (latest): ${latest.raw}`);

  // Sync highest version to content/latest
  const highestVerDir = path.join(CONTENT_DIR, latest.raw);
  syncDirectory(highestVerDir, LATEST_DIR);
  console.log(`[gen-sidebar] Synced content/${latest.raw} -> content/latest`);

  const sidebarConfig = {};

  // Build sidebar for latest
  sidebarConfig['/content/latest/'] = buildVersionSidebar(LATEST_DIR, 'content/latest');

  // Build sidebar for each specific version
  for (const ver of versions) {
    const verDir = path.join(CONTENT_DIR, ver.raw);
    sidebarConfig[`/content/${ver.raw}/`] = buildVersionSidebar(verDir, `content/${ver.raw}`);
  }

  // Find entry link for latest
  let latestFirstLink = '/content/latest/';
  const latestGroups = sidebarConfig['/content/latest/'];
  if (latestGroups && latestGroups.length > 0 && latestGroups[0].items.length > 0) {
    latestFirstLink = latestGroups[0].items[0].link;
  }

  // Build nav with version selector
  const navConfig = [
    { text: 'Guide', link: latestFirstLink },
    {
      text: `v${latest.raw} (latest)`,
      items: versions.map(v => {
        const isLatest = v.raw === latest.raw;
        const vGroups = sidebarConfig[`/content/${v.raw}/`];
        const vLink = vGroups && vGroups.length > 0 && vGroups[0].items.length > 0
          ? vGroups[0].items[0].link
          : `/content/${v.raw}/`;
        return {
          text: isLatest ? `${v.raw} (latest)` : v.raw,
          link: vLink,
        };
      }),
    },
  ];

  const result = {
    sidebar: sidebarConfig,
    nav: navConfig,
    latestVersion: latest.raw,
    versions: sortedVersionNames,
  };

  fs.mkdirSync(VITEPRESS_DIR, { recursive: true });
  fs.writeFileSync(OUTPUT_FILE, JSON.stringify(result, null, 2), 'utf-8');
  console.log(`[gen-sidebar] Wrote generated sidebar configuration to ${OUTPUT_FILE}`);

  return result;
}

// Direct execution
if (process.argv[1] && path.resolve(process.argv[1]) === path.resolve(__filename)) {
  generateSidebar();
}
