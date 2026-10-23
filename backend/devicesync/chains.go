package devicesync

import (
	"conexiuni-cluj/web"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

// conflict carries a 409 answer out of a transaction.
type conflict fiber.Map

func (conflict) Error() string { return "conflict" }

var errUnauthorized = errors.New("unauthorized")

func answer(c fiber.Ctx, err error) error {
	var body conflict
	switch {
	case errors.As(err, &body):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map(body))
	case errors.Is(err, errUnauthorized), errors.Is(err, sql.ErrNoRows):
		return c.SendStatus(fiber.StatusUnauthorized)
	default:
		return c.SendStatus(fiber.StatusInternalServerError)
	}
}

func hashOf(raw []byte) []byte {
	sum := sha256.Sum256(raw)
	return sum[:]
}

type caller struct{ chain, device string }

// authorize answers 401 for every kind of bad token alike; status 0 means the caller is a device of the chain in the path.
func (s *Service) authorize(c fiber.Ctx) (caller, int) {
	client := web.ClientHash(c)
	now := time.Now()
	s.mu.Lock()
	spent := s.authMisses.spent(client, now)
	s.mu.Unlock()
	if spent {
		return caller{}, fiber.StatusTooManyRequests
	}
	var who caller
	var seen int64
	token, bearer := strings.CutPrefix(c.Get(fiber.HeaderAuthorization), "Bearer ")
	raw, ok := fromB64(token, keySize)
	if ok = ok && bearer; ok {
		err := s.db.QueryRow(`SELECT chain_id, id, seen FROM devices WHERE token_hash = ?`, hashOf(raw)).Scan(&who.chain, &who.device, &seen)
		ok = err == nil && who.chain == c.Params("chain")
	}
	if !ok {
		s.mu.Lock()
		s.authMisses.spend(client, 1, now)
		s.mu.Unlock()
		return caller{}, fiber.StatusUnauthorized
	}
	if today := day(now); seen < today {
		_, _ = s.db.Exec(`UPDATE devices SET seen = ? WHERE chain_id = ? AND id = ?`, today, who.chain, who.device)
		_, _ = s.db.Exec(`UPDATE chains SET seen = ? WHERE id = ?`, today, who.chain)
	}
	return who, 0
}

type deviceRequest struct {
	Join *string `json:"join"`
	Kind *string `json:"kind"`
}

func (r deviceRequest) valid() ([]byte, bool) {
	if r.Join == nil || r.Kind == nil || !slices.Contains(deviceKinds, *r.Kind) {
		return nil, false
	}
	return fromB64(*r.Join, keySize)
}

func addDevice(tx *sql.Tx, chain, kind string, now time.Time) (string, string, error) {
	device, token := randomID(), randomBytes(keySize)
	_, err := tx.Exec(`INSERT INTO devices (chain_id, id, token_hash, kind, created, seen) VALUES (?, ?, ?, ?, ?, ?)`,
		chain, device, hashOf(token), kind, now.Unix(), day(now))
	return device, b64.EncodeToString(token), err
}

func (s *Service) createChain(c fiber.Ctx) error {
	var req deviceRequest
	if !decode(c, 256, &req) {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	join, ok := req.valid()
	if !ok {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	client := web.ClientHash(c)
	now := time.Now()
	s.mu.Lock()
	spent := s.chainsMade.spent(client, now)
	if !spent {
		s.chainsMade.spend(client, 1, now)
	}
	s.mu.Unlock()
	if spent {
		return c.SendStatus(fiber.StatusTooManyRequests)
	}
	if size, err := s.size(); err != nil || size > s.maxBytes {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	chain := randomID()
	var device, token string
	err := s.inTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO chains (id, epoch, join_hash, created, seen) VALUES (?, 1, ?, ?, ?)`,
			chain, hashOf(join), now.Unix(), day(now)); err != nil {
			return err
		}
		var err error
		device, token, err = addDevice(tx, chain, *req.Kind, now)
		return err
	})
	if err != nil {
		return answer(c, err)
	}
	return c.JSON(fiber.Map{"chain": chain, "epoch": 1, "device": device, "token": token})
}

func (s *Service) join(c fiber.Ctx) error {
	var req deviceRequest
	if !decode(c, 256, &req) {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	join, ok := req.valid()
	if !ok {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	client := web.ClientHash(c)
	now := time.Now()
	s.mu.Lock()
	spent := s.authMisses.spent(client, now)
	s.mu.Unlock()
	if spent {
		return c.SendStatus(fiber.StatusTooManyRequests)
	}
	chain := c.Params("chain")
	var epoch int64
	var device, token string
	err := s.inTx(func(tx *sql.Tx) error {
		var stored []byte
		err := tx.QueryRow(`SELECT epoch, join_hash FROM chains WHERE id = ?`, chain).Scan(&epoch, &stored)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && subtle.ConstantTimeCompare(hashOf(join), stored) != 1) {
			return errUnauthorized
		}
		if err != nil {
			return err
		}
		var count int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM devices WHERE chain_id = ?`, chain).Scan(&count); err != nil {
			return err
		}
		if count >= maxDevices {
			return conflict{"error": "full"}
		}
		device, token, err = addDevice(tx, chain, *req.Kind, now)
		return err
	})
	if errors.Is(err, errUnauthorized) {
		s.mu.Lock()
		s.authMisses.spend(client, 1, now)
		s.mu.Unlock()
	}
	if err != nil {
		return answer(c, err)
	}
	return c.JSON(fiber.Map{"epoch": epoch, "device": device, "token": token})
}

type deviceState struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Created int64  `json:"created"`
	Seen    int64  `json:"seen"`
}

type recordState struct {
	Name string `json:"name"`
	Rev  int64  `json:"rev"`
}

func (s *Service) state(c fiber.Ctx) error {
	who, status := s.authorize(c)
	if status != 0 {
		return c.SendStatus(status)
	}
	var epoch int64
	if err := s.db.QueryRow(`SELECT epoch FROM chains WHERE id = ?`, who.chain).Scan(&epoch); err != nil {
		return answer(c, err)
	}
	devices, err := queryAll(s.db, `SELECT id, kind, created, seen FROM devices WHERE chain_id = ? ORDER BY created, rowid`, who.chain,
		func(rows *sql.Rows, d *deviceState) error { return rows.Scan(&d.ID, &d.Kind, &d.Created, &d.Seen) })
	if err != nil {
		return answer(c, err)
	}
	records, err := queryAll(s.db, `SELECT name, rev FROM records WHERE chain_id = ? ORDER BY name`, who.chain,
		func(rows *sql.Rows, r *recordState) error { return rows.Scan(&r.Name, &r.Rev) })
	if err != nil {
		return answer(c, err)
	}
	return c.JSON(fiber.Map{"epoch": epoch, "device": who.device, "devices": devices, "records": records})
}

func queryAll[T any](db *sql.DB, query, chain string, scan func(*sql.Rows, *T) error) ([]T, error) {
	rows, err := db.Query(query, chain)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		var item T
		if err := scan(rows, &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Service) readRecord(c fiber.Ctx) error {
	who, status := s.authorize(c)
	if status != 0 {
		return c.SendStatus(status)
	}
	var rev int64
	var body []byte
	err := s.db.QueryRow(`SELECT rev, body FROM records WHERE chain_id = ? AND name = ?`, who.chain, c.Params("name")).Scan(&rev, &body)
	if errors.Is(err, sql.ErrNoRows) {
		return c.SendStatus(fiber.StatusNotFound)
	}
	if err != nil {
		return answer(c, err)
	}
	return c.JSON(fiber.Map{"rev": rev, "body": b64.EncodeToString(body)})
}

// spendWrites answers false once the device has written its hourly share.
func (s *Service) spendWrites(device string, n int) bool {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.writes.spent(device, now) {
		return false
	}
	s.writes.spend(device, n, now)
	return true
}

func currentEpoch(tx *sql.Tx, chain string) (int64, error) {
	var epoch int64
	err := tx.QueryRow(`SELECT epoch FROM chains WHERE id = ?`, chain).Scan(&epoch)
	return epoch, err
}

func (s *Service) writeRecord(c fiber.Ctx) error {
	who, status := s.authorize(c)
	if status != 0 {
		return c.SendStatus(status)
	}
	var req struct {
		Epoch *int64  `json:"epoch"`
		Base  *int64  `json:"base"`
		Body  *string `json:"body"`
	}
	name := c.Params("name")
	if !validID(name) || !decode(c, b64.EncodedLen(maxRecordBody())+128, &req) || req.Epoch == nil || req.Base == nil || req.Body == nil || *req.Base < 0 {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	body, ok := recordFromB64(*req.Body)
	if !ok {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	if !s.spendWrites(who.device, 1) {
		return c.SendStatus(fiber.StatusTooManyRequests)
	}
	err := s.inTx(func(tx *sql.Tx) error {
		epoch, err := currentEpoch(tx, who.chain)
		if err != nil {
			return err
		}
		var rev int64
		err = tx.QueryRow(`SELECT rev FROM records WHERE chain_id = ? AND name = ?`, who.chain, name).Scan(&rev)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if epoch != *req.Epoch || rev != *req.Base {
			return conflict{"error": "stale", "epoch": epoch, "rev": rev}
		}
		if rev == 0 {
			var count int
			if err := tx.QueryRow(`SELECT COUNT(*) FROM records WHERE chain_id = ?`, who.chain).Scan(&count); err != nil {
				return err
			}
			if count >= s.maxRecords {
				return conflict{"error": "full"}
			}
		}
		_, err = tx.Exec(`INSERT INTO records (chain_id, name, rev, body) VALUES (?, ?, ?, ?)
			ON CONFLICT (chain_id, name) DO UPDATE SET rev = excluded.rev, body = excluded.body`, who.chain, name, rev+1, body)
		return err
	})
	if err != nil {
		return answer(c, err)
	}
	return c.JSON(fiber.Map{"rev": *req.Base + 1})
}

func maxRecordBody() int { return recordBuckets[len(recordBuckets)-1] + sealing }

func (s *Service) readWrap(c fiber.Ctx) error {
	who, status := s.authorize(c)
	if status != 0 {
		return c.SendStatus(status)
	}
	var epoch int64
	var body []byte
	err := s.db.QueryRow(`SELECT epoch, body FROM wraps WHERE chain_id = ? AND device_id = ?`, who.chain, who.device).Scan(&epoch, &body)
	if errors.Is(err, sql.ErrNoRows) {
		return c.SendStatus(fiber.StatusNotFound)
	}
	if err != nil {
		return answer(c, err)
	}
	return c.JSON(fiber.Map{"epoch": epoch, "body": b64.EncodeToString(body)})
}

type sealedRecord struct {
	Name string `json:"name"`
	Body string `json:"body"`
}

type sealedWrap struct {
	Device string `json:"device"`
	Body   string `json:"body"`
}

type rotation struct {
	Epoch   *int64          `json:"epoch"`
	Join    *string         `json:"join"`
	Remove  *[]string       `json:"remove"`
	Records *[]sealedRecord `json:"records"`
	Wraps   *[]sealedWrap   `json:"wraps"`
}

func (s *Service) rotationLimit() int {
	return s.maxRecords*(b64.EncodedLen(maxRecordBody())+64) + maxDevices*(b64.EncodedLen(wrapSize)+64) + 256
}

func distinctIDs(ids []string) bool {
	seen := map[string]bool{}
	for _, id := range ids {
		if !validID(id) || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

// check decodes what a rotation carries, refusing anything not exactly as protocol.md §7 says.
func (r rotation) check(maxRecords int) (join []byte, records map[string][]byte, wraps map[string][]byte, ok bool) {
	if r.Epoch == nil || r.Join == nil || r.Remove == nil || r.Records == nil || r.Wraps == nil || len(*r.Records) > maxRecords {
		return nil, nil, nil, false
	}
	if join, ok = fromB64(*r.Join, keySize); !ok || !distinctIDs(*r.Remove) {
		return nil, nil, nil, false
	}
	records, wraps = map[string][]byte{}, map[string][]byte{}
	for _, rec := range *r.Records {
		body, valid := recordFromB64(rec.Body)
		if !valid || !validID(rec.Name) || records[rec.Name] != nil {
			return nil, nil, nil, false
		}
		records[rec.Name] = body
	}
	for _, w := range *r.Wraps {
		body, valid := fromB64(w.Body, wrapSize)
		if !valid || !validID(w.Device) || wraps[w.Device] != nil {
			return nil, nil, nil, false
		}
		if _, valid := p256(b64.EncodeToString(body[:spkiSize])); !valid {
			return nil, nil, nil, false
		}
		wraps[w.Device] = body
	}
	return join, records, wraps, true
}

func (s *Service) rotate(c fiber.Ctx) error {
	who, status := s.authorize(c)
	if status != 0 {
		return c.SendStatus(status)
	}
	var req rotation
	if !decode(c, s.rotationLimit(), &req) {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	join, records, wraps, ok := req.check(s.maxRecords)
	if !ok {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	if !s.spendWrites(who.device, max(len(records), 1)) {
		return c.SendStatus(fiber.StatusTooManyRequests)
	}
	next := *req.Epoch + 1
	err := s.inTx(func(tx *sql.Tx) error {
		epoch, err := currentEpoch(tx, who.chain)
		if err != nil {
			return err
		}
		if epoch != *req.Epoch {
			return conflict{"error": "stale", "epoch": epoch}
		}
		var devices []string
		rows, err := tx.Query(`SELECT id FROM devices WHERE chain_id = ?`, who.chain)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			devices = append(devices, id)
		}
		rows.Close()
		remaining := map[string]bool{}
		for _, id := range devices {
			remaining[id] = id != who.device
		}
		for _, id := range *req.Remove {
			if !remaining[id] {
				return conflict{"error": "devices"}
			}
			remaining[id] = false
		}
		for _, id := range devices {
			if remaining[id] != (wraps[id] != nil) {
				return conflict{"error": "devices"}
			}
		}
		if len(wraps) != countTrue(remaining) {
			return conflict{"error": "devices"}
		}
		for _, id := range *req.Remove {
			if _, err := tx.Exec(`DELETE FROM devices WHERE chain_id = ? AND id = ?`, who.chain, id); err != nil {
				return err
			}
		}
		for _, stmt := range []string{`DELETE FROM records WHERE chain_id = ?`, `DELETE FROM wraps WHERE chain_id = ?`} {
			if _, err := tx.Exec(stmt, who.chain); err != nil {
				return err
			}
		}
		for name, body := range records {
			if _, err := tx.Exec(`INSERT INTO records (chain_id, name, rev, body) VALUES (?, ?, 1, ?)`, who.chain, name, body); err != nil {
				return err
			}
		}
		for device, body := range wraps {
			if _, err := tx.Exec(`INSERT INTO wraps (chain_id, device_id, epoch, body) VALUES (?, ?, ?, ?)`, who.chain, device, next, body); err != nil {
				return err
			}
		}
		_, err = tx.Exec(`UPDATE chains SET epoch = ?, join_hash = ? WHERE id = ?`, next, hashOf(join), who.chain)
		return err
	})
	if err != nil {
		return answer(c, err)
	}
	return c.JSON(fiber.Map{"epoch": next})
}

func countTrue(m map[string]bool) int {
	n := 0
	for _, v := range m {
		if v {
			n++
		}
	}
	return n
}

func (s *Service) leave(c fiber.Ctx) error {
	who, status := s.authorize(c)
	if status != 0 {
		return c.SendStatus(status)
	}
	err := s.inTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM devices WHERE chain_id = ? AND id = ?`, who.chain, who.device); err != nil {
			return err
		}
		_, err := tx.Exec(`DELETE FROM chains WHERE id = ? AND NOT EXISTS (SELECT 1 FROM devices WHERE chain_id = ?)`, who.chain, who.chain)
		return err
	})
	if err != nil {
		return answer(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Service) deleteChain(c fiber.Ctx) error {
	who, status := s.authorize(c)
	if status != 0 {
		return c.SendStatus(status)
	}
	if _, err := s.db.Exec(`DELETE FROM chains WHERE id = ?`, who.chain); err != nil {
		return answer(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
