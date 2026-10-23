// What this device keeps to stay in its chain, in IndexedDB: the secret never goes to localStorage, and the device key stays non-extractable.
import type {Bytes} from '@/utils/sync/crypto.ts'

export type Membership = {
  chain: string
  epoch: number
  device: string
  token: string
  s: Bytes
  deviceKey: CryptoKey
  devicePub: string
}

// The last version of a record both sides agreed on, by logical name.
export type Base = {text: string; rev: number; epoch: number}

const DB = 'cai-sync'
const STORE = 'kv'

let opening: Promise<IDBDatabase> | null = null

function db(): Promise<IDBDatabase> {
  opening ??= new Promise((resolve, reject) => {
    const request = indexedDB.open(DB, 1)
    request.onupgradeneeded = () => request.result.createObjectStore(STORE)
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => {
      opening = null
      reject(request.error)
    }
  })
  return opening
}

async function run<T>(mode: IDBTransactionMode, fn: (store: IDBObjectStore) => IDBRequest | void): Promise<T> {
  const tx = (await db()).transaction(STORE, mode)
  const request = fn(tx.objectStore(STORE))
  return new Promise((resolve, reject) => {
    tx.oncomplete = () => resolve(request ? request.result as T : undefined as T)
    tx.onerror = tx.onabort = () => reject(tx.error)
  })
}

export const getMembership = () => run<Membership | undefined>('readonly', (s) => s.get('membership')).catch(() => undefined)
export const setMembership = (m: Membership) => run<void>('readwrite', (s) => void s.put(m, 'membership'))

export const getBases = async () => (await run<Record<string, Base> | undefined>('readonly', (s) => s.get('bases'))) ?? {}
export const setBases = (bases: Record<string, Base>) => run<void>('readwrite', (s) => void s.put(bases, 'bases'))

export const forget = () => run<void>('readwrite', (s) => {
  s.delete('membership')
  s.delete('bases')
})
