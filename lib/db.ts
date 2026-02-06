"use server";

import { Client, createClient} from "@libsql/client";

const db = createClient({url: "file:local.db"});

export async function getDb() {
  return db;
}

export async function getFromDbWithFallback(dbQueryFn, fallback) {
  const db = await getDb();

  try {
    return await dbQueryFn(db);
  } catch (error) {
    console.warn("Could not find data in db, using fallback", error);
    return await fallback(db);
  }
}