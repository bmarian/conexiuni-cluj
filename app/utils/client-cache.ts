interface CacheEntry<T> {
  value: T;
  timestamp: number;
}

const DEFAULT_TTL = 60_000; // 60 seconds

export class ClientCache {
  private store = new Map<string, CacheEntry<unknown>>();
  private ttl: number;

  constructor(ttl = DEFAULT_TTL) {
    this.ttl = ttl;
  }

  get<T>(key: string): T | null {
    const entry = this.store.get(key);
    if (!entry) return null;
    if (Date.now() - entry.timestamp > this.ttl) {
      this.store.delete(key);
      return null;
    }
    return entry.value as T;
  }

  set<T>(key: string, value: T): void {
    this.store.set(key, {value, timestamp: Date.now()});
  }

  clear(): void {
    this.store.clear();
  }
}

export const clientCache = new ClientCache();
