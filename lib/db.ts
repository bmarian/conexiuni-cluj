"use server";

import {createClient} from "@libsql/client";

const db = createClient({url: "file:local.db"});

export async function getDb() {
  return db;
}