package main

import (
	"bytes"
	"crypto/ecdh"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
)

var raw64 = base64.RawURLEncoding

// maskSyncValues replaces the random codes, secrets, ids and tokens a sync answer hands out, wherever they show later.
func (r *recorder) maskSyncValues(body []byte) {
	var answer map[string]any
	if json.Unmarshal(body, &answer) != nil {
		return
	}
	for _, key := range []string{"code", "secret", "chain", "device", "token"} {
		value, ok := answer[key].(string)
		if !ok || value == "" {
			continue
		}
		known := false
		uses := 0
		for _, m := range r.masks {
			known = known || m[0] == value
			if len(m[1]) > len(key)+1 && m[1][1:len(key)+1] == key {
				uses++
			}
		}
		if known {
			continue
		}
		label := "<" + key + ">"
		if uses > 0 {
			label = fmt.Sprintf("<%s-%d>", key, uses+1)
		}
		r.masks = append(r.masks, [2]string{value, label})
	}
}

func parityKey(label string) []byte {
	sum := sha256.Sum256([]byte("parity sync|" + label))
	key, err := ecdh.P256().NewPrivateKey(sum[:])
	if err != nil {
		panic(err)
	}
	spki, err := x509.MarshalPKIXPublicKey(key.PublicKey())
	if err != nil {
		panic(err)
	}
	return spki
}

func (r *recorder) walkSync() {
	jsonBody := []string{"Content-Type", "application/json"}
	shower, enterer := parityKey("d"), parityKey("e")
	commit := sha256.Sum256(shower)

	var slot struct{ Code, Secret string }
	if err := json.Unmarshal(r.do(http.MethodPost, "/api/sync/pair", fmt.Sprintf(`{"commit":%q}`, raw64.EncodeToString(commit[:])), jsonBody...), &slot); err != nil || slot.Code == "" {
		r.t.Fatalf("sync pair: %v %+v", err, slot)
	}
	pair := "/api/sync/pair/" + slot.Code
	r.do(http.MethodPost, "/api/sync/pair", `{"commit":"short"}`, jsonBody...)
	r.do(http.MethodPost, "/api/sync/pair/000000/claim", fmt.Sprintf(`{"pub":%q,"scanned":true}`, raw64.EncodeToString(enterer)), jsonBody...)
	var claimed struct{ Secret string }
	if err := json.Unmarshal(r.do(http.MethodPost, pair+"/claim", fmt.Sprintf(`{"pub":%q,"scanned":true}`, raw64.EncodeToString(enterer)), jsonBody...), &claimed); err != nil || claimed.Secret == "" {
		r.t.Fatalf("sync claim: %v", err)
	}
	r.do(http.MethodPost, pair+"/claim", fmt.Sprintf(`{"pub":%q,"scanned":false}`, raw64.EncodeToString(enterer)), jsonBody...)
	r.get(pair, "X-Pair-Secret", slot.Secret)
	r.do(http.MethodPut, pair+"/pub", fmt.Sprintf(`{"pub":%q}`, raw64.EncodeToString(enterer)), append(jsonBody, "X-Pair-Secret", slot.Secret)...)
	r.do(http.MethodPut, pair+"/pub", fmt.Sprintf(`{"pub":%q}`, raw64.EncodeToString(shower)), append(jsonBody, "X-Pair-Secret", slot.Secret)...)
	message := fmt.Sprintf(`{"message":%q}`, raw64.EncodeToString(make([]byte, 540)))
	r.do(http.MethodPost, pair+"/messages", message, append(jsonBody, "X-Pair-Secret", claimed.Secret)...)
	r.get(pair, "X-Pair-Secret", slot.Secret)
	r.get(pair, "X-Pair-Secret", claimed.Secret)
	r.do(http.MethodDelete, pair, "", "X-Pair-Secret", claimed.Secret)
	r.get(pair, "X-Pair-Secret", slot.Secret)

	join := raw64.EncodeToString(make([]byte, 32))
	var first struct{ Chain, Token string }
	if err := json.Unmarshal(r.do(http.MethodPost, "/api/sync/chains", fmt.Sprintf(`{"join":%q,"kind":"web"}`, join), jsonBody...), &first); err != nil || first.Chain == "" {
		r.t.Fatalf("sync chain: %v", err)
	}
	chain := "/api/sync/chains/" + first.Chain
	bearer := func(token string) []string { return append(jsonBody, "Authorization", "Bearer "+token) }
	r.do(http.MethodPost, chain+"/devices", fmt.Sprintf(`{"join":%q,"kind":"android"}`, raw64.EncodeToString(bytes.Repeat([]byte{1}, 32))), jsonBody...)
	var second struct{ Device, Token string }
	if err := json.Unmarshal(r.do(http.MethodPost, chain+"/devices", fmt.Sprintf(`{"join":%q,"kind":"android"}`, join), jsonBody...), &second); err != nil || second.Token == "" {
		r.t.Fatalf("sync join: %v", err)
	}
	r.get(chain, bearer(first.Token)...)
	r.get(chain)
	name := raw64.EncodeToString(make([]byte, 16))
	record := fmt.Sprintf(`{"epoch":1,"base":0,"body":%q}`, raw64.EncodeToString(make([]byte, 1024+28)))
	r.do(http.MethodPut, chain+"/records/"+name, record, bearer(first.Token)...)
	r.do(http.MethodPut, chain+"/records/"+name, record, bearer(second.Token)...)
	r.do(http.MethodPut, chain+"/records/"+name, fmt.Sprintf(`{"epoch":1,"base":1,"body":%q}`, raw64.EncodeToString(make([]byte, 1000))), bearer(first.Token)...)
	r.get(chain+"/records/"+name, bearer(second.Token)...)
	r.get(chain+"/wrap", bearer(first.Token)...)
	rotation := fmt.Sprintf(`{"epoch":1,"seen":[{"name":%q,"rev":1}],"join":%q,"remove":[%q],"records":[{"name":%q,"body":%q}],"wraps":[]}`,
		name, raw64.EncodeToString(make([]byte, 32)), second.Device, name, raw64.EncodeToString(make([]byte, 4096+28)))
	r.do(http.MethodPost, chain+"/rotate", rotation, bearer(first.Token)...)
	r.get(chain, bearer(second.Token)...)
	r.get(chain, bearer(first.Token)...)
	r.do(http.MethodDelete, chain+"/devices/self", "", bearer(first.Token)...)
	r.get(chain, bearer(first.Token)...)
}
