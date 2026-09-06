package membership

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/redis/go-redis/v9"
)

type Keyring struct {
	Active      string
	Keys        map[string][]byte
	IdentityKey []byte
}

func ParseKeyring(raw, active, identity string) (*Keyring, error) {
	if raw == "" && active == "" && identity == "" {
		return nil, nil
	}
	var encoded map[string]string
	if json.Unmarshal([]byte(raw), &encoded) != nil {
		return nil, ErrInvalid
	}
	k := &Keyring{Active: active, Keys: map[string][]byte{}}
	for id, value := range encoded {
		key, err := base64.StdEncoding.DecodeString(value)
		if err != nil || len(key) != 32 || id == "" || strings.Contains(id, ".") {
			return nil, ErrInvalid
		}
		k.Keys[id] = key
	}
	var err error
	k.IdentityKey, err = base64.StdEncoding.DecodeString(identity)
	if err != nil || len(k.IdentityKey) != 32 || len(k.Keys[active]) != 32 {
		return nil, ErrInvalid
	}
	return k, nil
}

func (k *Keyring) Encrypt(purpose, text string) (string, error) {
	if k == nil {
		return "", ErrUnavailable
	}
	block, err := aes.NewCipher(k.Keys[k.Active])
	if err != nil {
		return "", ErrUnavailable
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", ErrUnavailable
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nonce, nonce, []byte(text), []byte(purpose))
	return k.Active + "." + base64.RawStdEncoding.EncodeToString(sealed), nil
}

func (k *Keyring) Decrypt(purpose, text string) (string, error) {
	if k == nil {
		return "", ErrUnavailable
	}
	parts := strings.SplitN(text, ".", 2)
	if len(parts) != 2 || len(k.Keys[parts[0]]) != 32 {
		return "", ErrCredential
	}
	raw, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", ErrCredential
	}
	block, err := aes.NewCipher(k.Keys[parts[0]])
	if err != nil {
		return "", ErrCredential
	}
	aead, err := cipher.NewGCM(block)
	if err != nil || len(raw) < aead.NonceSize() {
		return "", ErrCredential
	}
	plain, err := aead.Open(nil, raw[:aead.NonceSize()], raw[aead.NonceSize():], []byte(purpose))
	if err != nil {
		return "", ErrCredential
	}
	return string(plain), nil
}

func (k *Keyring) Fingerprint(purpose, text string) string {
	if k == nil {
		return ""
	}
	mac := hmac.New(sha256.New, k.IdentityKey)
	mac.Write([]byte(purpose + "\x00" + text))
	return hex.EncodeToString(mac.Sum(nil))
}

type RedisVault struct {
	Client *redis.Client
	Keys   *Keyring
}

func (v *RedisVault) Ready(ctx context.Context) error {
	if v == nil || v.Client == nil || v.Keys == nil {
		return ErrUnavailable
	}
	config, err := v.Client.ConfigGet(ctx, "save").Result()
	if err != nil || config["save"] != "" {
		return ErrUnavailable
	}
	config, err = v.Client.ConfigGet(ctx, "appendonly").Result()
	if err != nil || config["appendonly"] != "no" {
		return ErrUnavailable
	}
	return nil
}

func (v *RedisVault) Put(ctx context.Context, id string, input Credential) error {
	if err := v.Ready(ctx); err != nil {
		return err
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return ErrInvalid
	}
	encrypted, err := v.Keys.Encrypt("credential:"+id, string(raw))
	if err != nil {
		return err
	}
	return v.Client.Set(ctx, "membership:credential:"+id, encrypted, CredentialTTL).Err()
}

func (v *RedisVault) Get(ctx context.Context, id string) (Credential, error) {
	var out Credential
	if v == nil || v.Client == nil || v.Keys == nil {
		return out, ErrUnavailable
	}
	raw, err := v.Client.Get(ctx, "membership:credential:"+id).Result()
	if err != nil {
		return out, ErrCredential
	}
	plain, err := v.Keys.Decrypt("credential:"+id, raw)
	if err != nil {
		return out, ErrCredential
	}
	if json.Unmarshal([]byte(plain), &out) != nil {
		return out, ErrCredential
	}
	return out, nil
}

func (v *RedisVault) Delete(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}
	if v == nil || v.Client == nil {
		return ErrUnavailable
	}
	return v.Client.Del(ctx, "membership:credential:"+id).Err()
}
