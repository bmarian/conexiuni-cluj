import { readdir, readFile, unlink } from "fs/promises";
import { join } from "path";

const CACHE_DIR = join(process.cwd(), ".cache");

try {
  const files = await readdir(CACHE_DIR);
  let deleted = 0;

  for (const file of files) {
    if (!file.endsWith(".json")) continue;
    const filePath = join(CACHE_DIR, file);
    const raw = await readFile(filePath, "utf-8");
    const entry = JSON.parse(raw);
    if (entry.data === null) {
      await unlink(filePath);
      console.log(`Deleted ${file}`);
      deleted++;
    }
  }

  console.log(`Done. Deleted ${deleted} null cache entries.`);
} catch (e) {
  if (e.code === "ENOENT") {
    console.log("No .cache directory found.");
  } else {
    throw e;
  }
}
