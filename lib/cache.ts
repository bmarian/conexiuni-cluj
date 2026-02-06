"use server";

import { getDb } from "@/lib/db";

interface CacheEntry<T> {
  timestamp: number;
  data: T;
}

async function readCache<T>(key: string): Promise<CacheEntry<T> | null> {
  const db = await getDb();
  const result = await db.execute({
    sql: "SELECT data, timestamp FROM cache WHERE key = ?",
    args: [key],
  });

  if (result.rows.length === 0) return null;

  const row = result.rows[0];
  return {
    timestamp: row.timestamp as number,
    data: JSON.parse(row.data as string) as T,
  };
}

async function writeCache<T>(key: string, data: T): Promise<void> {
  const db = await getDb();
  await db.execute({
    sql: `INSERT INTO cache (key, data, timestamp) VALUES (?, ?, ?)
          ON CONFLICT(key) DO UPDATE SET data = excluded.data, timestamp = excluded.timestamp`,
    args: [key, JSON.stringify(data), Date.now()],
  });
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
