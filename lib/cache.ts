"use server";

import fs from 'fs/promises';
import path from 'path';

const CACHE_DIR = path.join(process.cwd(), '.cache');

interface CacheEntry<T> {
  timestamp: number;
  data: T;
}

async function ensureCacheDir() {
  await fs.mkdir(CACHE_DIR, { recursive: true });
}

function cacheFilePath(key: string): string {
  return path.join(CACHE_DIR, `${key}.json`);
}

async function readCache<T>(key: string): Promise<CacheEntry<T> | null> {
  try {
    const raw = await fs.readFile(cacheFilePath(key), 'utf-8');
    return JSON.parse(raw) as CacheEntry<T>;
  } catch {
    return null;
  }
}

async function writeCache<T>(key: string, data: T): Promise<void> {
  await ensureCacheDir();
  const entry: CacheEntry<T> = { timestamp: Date.now(), data };
  await fs.writeFile(cacheFilePath(key), JSON.stringify(entry));
}

export async function fetchWithFallback<T>(key: string, fetcher: () => Promise<T>, revalidate: number): Promise<T> {
  const cached = await readCache<T>(key);

  if (cached && Date.now() - cached.timestamp < revalidate * 1000) {
    return cached.data;
  }

  try {
    const data = await fetcher();
    await writeCache(key, data);
    return data;
  } catch (error) {
    if (cached) {
      return cached.data;
    }
    throw error;
  }
}
