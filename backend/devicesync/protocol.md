# Device sync protocol, version 1

What the website, the Android app and the Wear OS app must do to sync with each other through `/api/sync`. The server side is this package. `testdata/vectors.json` holds worked values for every derivation below and `testdata/cases.json` the accepted and refused records and the merge cases; every client runs both files in its tests. `vectors_test.go` is the reference that produced and checks them.

## 1. Encodings

- Binary values travel as base64url without padding (RFC 4648 §5). Readers refuse padding, other alphabets and non-zero trailing bits.
- Text is UTF-8. JSON bodies are objects; the server refuses unknown keys, missing keys and trailing data.
- HKDF is HKDF-SHA256 (RFC 5869). "HKDF(ikm, salt, info, n)" with an empty salt means 32 zero bytes (what RFC 5869 does with no salt; Java's `SecretKeySpec` refuses a zero-length key, so pass the zeros).
- AES-GCM is AES-256-GCM with a 12-byte random nonce and a 16-byte tag. A sealed value is `nonce ‖ ciphertext ‖ tag`.
- ECDH is on P-256. Public keys travel as SubjectPublicKeyInfo DER (91 bytes): WebCrypto `exportKey('spki')`, Java `PublicKey.getEncoded()`. The shared secret is the 32-byte x-coordinate: WebCrypto `deriveBits(…, 256)`, Java `KeyAgreement("ECDH").generateSecret()`.
- Padding: a plaintext `p` padded to a bucket `n` is `uint32_be(len(p)) ‖ p ‖ zeros`, `n` bytes in all. Readers refuse a length that does not fit and non-zero bytes after `p`.
- Ids (chain, device, record name) are 16 bytes, so 22 characters. Tokens and keys are 32 bytes, so 43 characters.

## 2. Pairing

One device **shows** (D), the other **enters** (E). Slots live 5 minutes on the server and are used once.

1. D makes a P-256 key pair (`d`, `Pd`) and 16 random bytes `K`.
   `POST /api/sync/pair {"commit": SHA-256(Pd)}` → `{"code", "secret", "expiresIn"}`.
   D shows the QR `https://ceautobuziau.ro/settings?pair=<code>#k=<K>` and the code as `NNN NNN`.
2. E makes (`e`, `Pe`). It has `K` only if it scanned the QR.
   `POST /api/sync/pair/<code>/claim {"pub": Pe, "scanned": bool}` → `{"commit", "secret"}`.
3. D reads `GET /api/sync/pair/<code>` (header `X-Pair-Secret`) until `pub` is E's key, then
   `PUT /api/sync/pair/<code>/pub {"pub": Pd}`. The server refuses a key whose SHA-256 is not the commit.
4. E reads until `pub` is D's key and checks `SHA-256(Pd) == commit` itself.
5. Both compute
   - `Z = ECDH(own private, other public)`
   - `T = HKDF(Z, salt = K if scanned else empty, info = "cai-sync/v1/pair|<code>|" ‖ Pe ‖ Pd, 32)`
   - check number `= uint32_be(HKDF(T, empty, "cai-sync/v1/check", 4)) mod 1 000 000`, shown as six digits `NNN NNN` on both screens.
6. Messages: `POST /api/sync/pair/<code>/messages {"message": sealed}`. The sealed value is the JSON message in canonical form (§4.2) padded to 512 bytes and sealed with `T`, associated data `"cai-sync/v1/pair-message|<code>|<D or E>"` naming the sender, so 540 bytes. Each side sends at most two; `GET` returns `messages` for the reader, oldest first, every time.
   - `{"t": "hello", "kind": K, "label": L, "syncs": bool, "devices": n}`: both, first. `kind` and `label` follow §4.3; `devices` is 0 when `syncs` is false, else 1–10.
   - `{"t": "ready"}`: the receiving side, after its user confirms.
   - `{"t": "secret", "chain": id, "epoch": n, "s": S}`: the giving side, after its own user confirms and the `ready` arrived. `n` ≥ 1, `S` 32 bytes.
7. Who gives: the side that syncs. Neither: E creates the chain (§3) and gives. Both: neither confirms; both show that one of them has to stop syncing first, and close the slot.
8. The receiver joins (§3), merges (§6), and closes the slot with `DELETE /api/sync/pair/<code>`. The giver knows it worked when the new device appears in `GET /api/sync/chains/<chain>`.

A message that fails to open, a wrong commit, an unexpected message or a slot that disappears ends pairing on that device with "Couldn't connect, try again".

## 3. Chain keys and devices

`S` is 32 random bytes; each epoch has its own. From it, with salt `"cai-sync/v1"`:

| Key | info | Use |
|---|---|---|
| join | `cai-sync/v1/join` | proves knowledge of `S`; the server keeps SHA-256(join) |
| data | `cai-sync/v1/data` | AES-GCM key for records |
| names | `cai-sync/v1/names` | record name = first 16 bytes of HMAC-SHA256(names, logical name) |

Logical names: `settings`, `devices`, `city:<slug>`.

- Create: `POST /api/sync/chains {"join", "kind"}` → `{"chain", "epoch": 1, "device", "token"}`.
- Join: `POST /api/sync/chains/<chain>/devices {"join", "kind"}` → `{"epoch", "device", "token"}`.
- Every other call sends `Authorization: Bearer <token>`.

Each device also makes a long-term P-256 key pair when it joins and adds itself to the `devices` record (§4.4) with its public key. The private key never leaves the device.

## 4. Records

### 4.1 Sealing

`body = AES-GCM(data, pad(json, bucket), aad)`, with buckets 1024, 4096, 16384 and 65536 bytes, smallest that fits. Bodies are therefore 1052, 4124, 16412 or 65564 bytes; the server refuses any other length.

`aad = "cai-sync/v1/record|<chain>|<epoch>|<name>|<rev>"`, where `rev` is the revision the record has once written (§5).

### 4.2 Rules every record follows

A record is refused **whole** if anything in it breaks a rule: a missing or unknown key, a wrong type, a value out of range, a duplicate, a list over its cap. A refused record is never applied and never written back; the device keeps its own data and says sync has a problem. A record whose `v` is higher than the device knows is left alone (read-only) and the device asks for an update.

- **Canonical form:** the JSON text must be exactly what a canonical writer makes of its value; a reader parses it, checks the rules, writes the value again and refuses the record if the two texts differ. That rules out duplicate keys, whitespace, other key orders, `391.0`, `46.770`, `A` and every other second spelling, so all readers agree on every record. The canonical writer:
  - writes no whitespace and every key, in the order this document lists them;
  - writes text as its characters between quotes, escaping only `"` as `\"` and `\` as `\\` (anything else that would need escaping is refused by the text rule);
  - writes integers as decimal digits without sign or leading zeros;
  - writes coordinates as the shortest decimal that reads back as the same number, without exponent (`46.77`, `47`, `23.600001`): JavaScript's `String(x)`, Go's `strconv.FormatFloat(x, 'f', -1, 64)`, Kotlin `BigDecimal(x).setScale(6, HALF_UP).stripTrailingZeros().toPlainString()`;
  - writes booleans as `true` and `false`.
- **Integer:** a whole number in the stated range.
- **Text:** a JSON string of the stated length in code points, where
  - no code point is in U+0000–U+001F, U+007F–U+009F, U+061C, U+200B, U+200E, U+200F, U+2028–U+202E, U+2060, U+2066–U+2069, U+FEFF, U+FFFD, U+FFFE, U+FFFF, or a lone surrogate;
  - the first and last code point are not U+0020, U+00A0, U+1680, U+2000–U+200A, U+202F, U+205F or U+3000.
  Writers normalise text to NFC; readers don't check it.
- **Coordinate:** a finite number with `round(x × 10⁶) / 10⁶ == x` (round half up), latitude 43.6–48.3, longitude 20.2–29.8 (Romania).

### 4.3 `city:<slug>`

```json
{"v": 1, "stops": [391], "lines": [[24, "0"]], "places": [["Home", "", 46.77, 23.6]], "followed": ["24"]}
```

| Key | Item | Cap |
|---|---|---|
| `stops` | integer 1 … 2³¹−1 | 50, no duplicates |
| `lines` | `[route id 1 … 2³¹−1, "0" or "1"]` | 50, no duplicate pair |
| `places` | `[name, label, lat, lon]` or `[name, label, lat, lon, origin name, origin lat, origin lon]`: names text 1–100, label text 0–40 (`""` = none), coordinates | 50, no two with the same lat, lon, origin lat, origin lon |
| `followed` | line short name, text 1–16 | 50, no duplicates |

A record for a city the device does not know is left alone.

### 4.4 `devices`

```json
{"v": 1, "devices": [{"id": "…", "kind": "web", "label": "Chrome on Windows", "pub": "…"}]}
```

`id` a device id; `kind` `web`, `android` or `wear`; `label` text 1–40, made by the device (browser and system, `Build.MODEL`), never typed; `pub` its long-term SPKI. At most 10, no duplicate ids.

### 4.5 `settings`

```json
{"v": 1, "theme": "system", "locale": "ro", "autoCenterOnMe": true, "autoFitMap": true, "showVehicleExtras": true, "regions": {"cluj": {"showGreenFriday": true}}}
```

`theme` `light`, `dark` or `system`; `locale` `ro` or `en`; the three flags booleans; `regions` exactly the region settings of this version: `cluj.showGreenFriday`, boolean. A new setting is a new version.

A device with "Also sync settings" off neither reads nor writes this record.

## 5. Reading and writing

- `GET /api/sync/chains/<chain>` → `{"epoch", "device", "devices": [{"id", "kind", "created", "seen"}], "records": [{"name", "rev"}]}`. Times are Unix seconds; `seen` is the start of the UTC day the device last called.
- `GET /api/sync/chains/<chain>/records/<name>` → `{"rev", "body"}`, 404 if there is none.
- `PUT /api/sync/chains/<chain>/records/<name> {"epoch", "base", "body"}`: `base` is the revision the writer started from, 0 for a new record. Stored as `base + 1` → `{"rev"}`. 409 `{"error": "stale", "epoch", "rev"}` when the epoch or base is not the current one; 409 `{"error": "full"}` when the chain has all the records it may have.
- Readers refuse a record whose revision is lower than the highest they have seen for that name in this epoch, and a body that fails to open.

## 6. Merging

Each device keeps, per record, the last version it and the server agreed on (`base`), empty for a record it never had.

- **Lists**, by identity (a stop's id, a line's pair, a place's four coordinates, a followed name, a device's id): an item is kept if both sides have it, or if one side has it and `base` doesn't. An item in `base` that either side dropped is gone.
- **Order:** the local order wins if, over the items both it and `base` have, it differs from `base` while the remote order over the items both remote and `base` have does not; otherwise the remote order wins. The result is the winning list's kept items in its order, then the kept items only the other list has, in its order.
- **Fields** (place name and label; device kind, label and key; each setting): the local value if it differs from `base` and the remote value doesn't; otherwise the remote value. An item new on one side takes that side's values; new on both, the remote's.
- **Caps:** if a list ends up over its cap, items only the local side added are dropped from the end until it fits, and the device says what stayed local.
- **Push:** if the merge differs from the remote record, write it with `base` = the remote revision; on 409 read again and repeat.

## 7. Removing a device

1. Pull and merge every record.
2. Make `S'`; epoch `e + 1`; derive its keys; re-seal every record under the new names with revision 1.
3. For each remaining device R other than this one: an ephemeral key pair (`x`, `Px`),
   `W = HKDF(ECDH(x, R.pub), salt = Px, info = "cai-sync/v1/wrap|<chain>|<e+1>|<R.id>", 32)`,
   wrap `= Px ‖ AES-GCM(W, S', aad = the same info string)`, 151 bytes.
4. `POST /api/sync/chains/<chain>/rotate {"epoch": e, "join": join', "remove": [ids], "records": [{"name", "body"}], "wraps": [{"device", "body"}]}` → `{"epoch": e + 1}`. The server takes it only if `e` is current, `remove` holds other devices of the chain, and `wraps` names exactly the devices that remain apart from the caller.
5. Others see the new epoch, read `GET /api/sync/chains/<chain>/wrap` → `{"epoch", "body"}`, open it with their long-term key, and continue with `S'`. A removed device gets 401 everywhere.

Leaving: write the `devices` record without this device, then `DELETE /api/sync/chains/<chain>/devices/self`; the last device to leave deletes the chain. Deleting everything: `DELETE /api/sync/chains/<chain>`.

## 8. Server answers and limits

| Answer | Meaning |
|---|---|
| 400 | body, id or size not as specified |
| 401 | unknown chain, wrong join key, unknown or removed token, all alike |
| 403 | asked from another website |
| 404 | pairing slot unknown, taken, expired or wrong secret; record or wrap missing |
| 409 | state changed (`stale`, `full`, pairing step out of order) |
| 429 | a budget below is spent |
| 503 | the server stores no more chains or slots |

| Budget | Limit |
|---|---|
| open pairing slots | 1000 |
| slots made | 20 per client per 10 minutes |
| wrong codes | 20 per client per 10 minutes, 300 per minute for everyone |
| chains made | 5 per client per day |
| 401 answers | 20 per client per 10 minutes |
| record writes | 120 per device per hour (a rotation counts each record) |
| devices | 10 per chain |
| records | cities + 2 per chain |
| storage | no new chains above `SYNC_DB_MAX_MB` |
| unused | a chain no device has called for 365 days is deleted |
