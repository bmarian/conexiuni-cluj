import fs from "fs/promises";
import path from "path";
import { createClient } from "@libsql/client";

async function migrate() {
  const db = createClient({ url: "file:local.db" });

  await db.execute(`
    CREATE TABLE IF NOT EXISTS cache (
      key TEXT PRIMARY KEY,
      data TEXT NOT NULL,
      timestamp INTEGER NOT NULL
    )
  `);

  const cacheDir = path.join(process.cwd(), ".cache");

  let files;
  try {
    files = await fs.readdir(cacheDir);
  } catch {
    console.error("No .cache directory found. Nothing to migrate.");
    process.exit(1);
  }

  const jsonFiles = files.filter((f) => f.endsWith(".json"));
  console.log(`Found ${jsonFiles.length} cache files to migrate.`);

  let migrated = 0;
  let skipped = 0;

  for (const file of jsonFiles) {
    const key = file.replace(".json", "");
    try {
      const raw = await fs.readFile(path.join(cacheDir, file), "utf-8");
      const entry = JSON.parse(raw);

      if (!entry.data || !entry.timestamp) {
        console.log(`  Skipped (invalid): ${key}`);
        skipped++;
        continue;
      }

      await db.execute({
        sql: `INSERT OR REPLACE INTO cache (key, data, timestamp) VALUES (?, ?, ?)`,
        args: [key, JSON.stringify(entry.data), entry.timestamp],
      });

      console.log(`  Migrated: ${key}`);
      migrated++;
    } catch (err) {
      console.error(`  Error migrating ${key}:`, err.message);
      skipped++;
    }
  }

  console.log(`\nDone. Migrated: ${migrated}, Skipped: ${skipped}`);
}

migrate().catch(console.error);
