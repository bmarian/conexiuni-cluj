import { createClient } from "@libsql/client";

const db = createClient({
  url: "file:local.db",
});

let initialized = false;

export async function getDb() {
  if (!initialized) {
    await db.execute(`
      CREATE TABLE IF NOT EXISTS cache (
        key TEXT PRIMARY KEY,
        data TEXT NOT NULL,
        timestamp INTEGER NOT NULL
      )
    `);
    initialized = true;
  }
  return db;
}
