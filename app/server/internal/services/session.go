package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sushazhi/fnos-logmanager/internal/config"
	"github.com/sushazhi/fnos-logmanager/internal/types"
)

var (
	sessions   = make(map[string]*types.Session)
	csrfTokens = make(map[string]*types.CSRFToken)
	mu         sync.RWMutex
	sessionKey []byte

	dirty     bool
	saveTimer *time.Timer
	saveMu    sync.Mutex
)

const (
	sessionFile = ".sessions.json"
	keyFile     = ".sessions.key"
	keyLength   = 32
	// maxSessions caps the in-memory session map to prevent unbounded growth.
	maxSessions = 10000
)

func sessionFilePath() string {
	return filepath.Join(config.Get().DataDir, sessionFile)
}

func keyFilePath() string {
	return filepath.Join(config.Get().DataDir, keyFile)
}

func getOrCreateKey() []byte {
	if sessionKey != nil {
		return sessionKey
	}

	// Try to load existing key
	if data, err := os.ReadFile(keyFilePath()); err == nil {
		key, err := hex.DecodeString(strings.TrimSpace(string(data)))
		if err == nil && len(key) == keyLength {
			sessionKey = key
			return sessionKey
		}
	}

	// Generate new key
	key := make([]byte, keyLength)
	if _, err := rand.Read(key); err != nil {
		slog.Error("failed to generate session key", "error", err)
		return nil
	}

	// Save key
	if err := os.MkdirAll(config.Get().DataDir, 0700); err == nil {
		os.WriteFile(keyFilePath(), []byte(hex.EncodeToString(key)), 0600)
	}
	sessionKey = key
	return key
}

func encrypt(plaintext string) (string, error) {
	key := getOrCreateKey()
	if key == nil {
		// Never fall back to plaintext: sessions must not be persisted unencrypted.
		return "", fmt.Errorf("session key unavailable")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

func decrypt(encoded string) (string, error) {
	key := getOrCreateKey()
	if key == nil {
		return "", fmt.Errorf("session key unavailable")
	}

	ciphertext, err := hex.DecodeString(encoded)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", nil
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func scheduleSave() {
	saveMu.Lock()
	defer saveMu.Unlock()
	dirty = true
	if saveTimer == nil {
		saveTimer = time.AfterFunc(5*time.Second, saveSessions)
	}
}

func saveSessions() {
	saveMu.Lock()
	defer saveMu.Unlock()

	// FIX(bug 1): time.AfterFunc returns a *time.Timer that stays non-nil after
	// firing, and it was never reset, so saveTimer never became nil again and
	// scheduleSave() only ever scheduled the very first flush. Reset it on every
	// exit path (still under saveMu, before saveMu.Unlock runs) so subsequent
	// scheduleSave() calls can arm a fresh timer.
	defer func() {
		saveTimer = nil
	}()

	flushSessionsLocked()
}

func loadSessions() {
	mu.Lock()
	defer mu.Unlock()

	data, err := os.ReadFile(sessionFilePath())
	if err != nil {
		return
	}

	encoded := strings.TrimSpace(string(data))
	if encoded == "" {
		return
	}

	plaintext, err := decrypt(encoded)
	if err != nil {
		slog.Warn("failed to decrypt session file, trying as plaintext")
		plaintext = encoded
	}

	now := time.Now().UnixMilli()
	expiry := config.Get().SessionExpiry
	csrfExpiry := config.Get().CSRF.Expiry

	// Try new format (maps) first
	var newFormat struct {
		Sessions   map[string]*types.Session   `json:"sessions"`
		CSRFTokens map[string]*types.CSRFToken `json:"csrfTokens"`
	}
	if err := json.Unmarshal([]byte(plaintext), &newFormat); err == nil && newFormat.Sessions != nil {
		for token, session := range newFormat.Sessions {
			if now-session.LastAccess <= expiry {
				sessions[token] = session
			}
		}
		for sessionToken, csrfData := range newFormat.CSRFTokens {
			if _, exists := sessions[sessionToken]; exists && now-csrfData.CreatedAt <= csrfExpiry {
				csrfTokens[sessionToken] = csrfData
			}
		}
		slog.Info("sessions loaded (new format)", "count", len(sessions))
		return
	}

	// Try old format (arrays of tuples: sessions=[[token, session], ...])
	var oldFormat struct {
		Sessions   [][2]json.RawMessage `json:"sessions"`
		CSRFTokens [][2]json.RawMessage `json:"csrfTokens"`
	}
	if err := json.Unmarshal([]byte(plaintext), &oldFormat); err != nil {
		slog.Warn("failed to parse session file", "error", err)
		return
	}

	// Convert old format tuples to maps
	for _, pair := range oldFormat.Sessions {
		if len(pair) < 2 {
			continue
		}
		var token string
		var session types.Session
		if err := json.Unmarshal(pair[0], &token); err != nil {
			continue
		}
		if err := json.Unmarshal(pair[1], &session); err != nil {
			continue
		}
		if now-session.LastAccess <= expiry {
			sessions[token] = &session
		}
	}

	for _, pair := range oldFormat.CSRFTokens {
		if len(pair) < 2 {
			continue
		}
		var sessionToken string
		var csrfData types.CSRFToken
		if err := json.Unmarshal(pair[0], &sessionToken); err != nil {
			continue
		}
		if err := json.Unmarshal(pair[1], &csrfData); err != nil {
			continue
		}
		if _, exists := sessions[sessionToken]; exists && now-csrfData.CreatedAt <= csrfExpiry {
			csrfTokens[sessionToken] = &csrfData
		}
	}

	slog.Info("sessions loaded (old format, migrated)", "count", len(sessions))
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// CreateSession creates a new session.
func CreateSession(username string) string {
	token := generateToken()
	now := time.Now().UnixMilli()
	mu.Lock()
	if len(sessions) >= maxSessions {
		evictOldestSessionsLocked(now)
	}
	sessions[token] = &types.Session{
		Username:   username,
		CreatedAt:  now,
		LastAccess: now,
	}
	mu.Unlock()

	scheduleSave()
	return token
}

// GetOrCreateUserSession returns an existing valid session for the given
// username, creating one only when none exists. Gateway mode calls this on
// every request, so reusing sessions prevents unbounded map growth and
// constant re-encrypted disk writes.
func GetOrCreateUserSession(username string) string {
	now := time.Now().UnixMilli()
	expiry := config.Get().SessionExpiry

	mu.RLock()
	for token, s := range sessions {
		if s.Username == username && now-s.LastAccess <= expiry {
			mu.RUnlock()
			mu.Lock()
			if sess, ok := sessions[token]; ok {
				sess.LastAccess = now
			}
			mu.Unlock()
			// FIX(bug 4): LastAccess was only refreshed in memory. Without
			// scheduleSave() the persisted LastAccess stays stale, so after a
			// restart this still-active session would be judged expired. Flag
			// the change so the next flush persists it.
			scheduleSave()
			return token
		}
	}
	mu.RUnlock()

	return CreateSession(username)
}

// evictOldestSessionsLocked removes expired sessions first, then the oldest
// 10% by LastAccess when the map is still full. Caller must hold mu.
func evictOldestSessionsLocked(now int64) {
	expiry := config.Get().SessionExpiry
	for token, s := range sessions {
		if now-s.LastAccess > expiry {
			delete(sessions, token)
			delete(csrfTokens, token)
		}
	}
	if len(sessions) < maxSessions {
		return
	}
	type kv struct {
		token      string
		lastAccess int64
	}
	all := make([]kv, 0, len(sessions))
	for token, s := range sessions {
		all = append(all, kv{token, s.LastAccess})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].lastAccess < all[j].lastAccess })
	evict := len(all)/10 + 1
	for i := 0; i < evict && i < len(all); i++ {
		delete(sessions, all[i].token)
		delete(csrfTokens, all[i].token)
	}
	slog.Warn("session map full, evicted oldest sessions", "evicted", evict)
}

// ValidateSession validates and refreshes a session token.
func ValidateSession(token string) bool {
	if token == "" {
		return false
	}

	mu.RLock()
	session, exists := sessions[token]
	mu.RUnlock()

	if !exists {
		return false
	}

	now := time.Now().UnixMilli()
	if now-session.LastAccess > config.Get().SessionExpiry {
		mu.Lock()
		delete(sessions, token)
		delete(csrfTokens, token)
		mu.Unlock()
		scheduleSave()
		return false
	}

	mu.Lock()
	session.LastAccess = now
	mu.Unlock()
	// FIX(bug 4): persist the refreshed LastAccess so restarts do not treat
	// still-active sessions as expired.
	scheduleSave()
	return true
}

// DeleteSession deletes a session and its CSRF tokens.
func DeleteSession(token string) {
	mu.Lock()
	delete(sessions, token)
	delete(csrfTokens, token)
	mu.Unlock()
	scheduleSave()
}

// GetCSRFToken returns a CSRF token for the given session token.
func GetCSRFToken(sessionToken string) string {
	if sessionToken == "" {
		return ""
	}

	mu.RLock()
	_, sessionExists := sessions[sessionToken]
	mu.RUnlock()

	if !sessionExists {
		return ""
	}

	mu.RLock()
	stored, exists := csrfTokens[sessionToken]
	mu.RUnlock()

	now := time.Now().UnixMilli()
	csrfExpiry := config.Get().CSRF.Expiry

	if exists && now-stored.CreatedAt < csrfExpiry {
		return stored.Token
	}

	// Generate new CSRF token
	token := generateToken()
	mu.Lock()
	csrfTokens[sessionToken] = &types.CSRFToken{
		Token:     token,
		CreatedAt: now,
	}
	mu.Unlock()
	scheduleSave()
	return token
}

// ValidateCSRFToken validates a CSRF token with timing-safe comparison.
func ValidateCSRFToken(sessionToken, csrfToken string) bool {
	if sessionToken == "" || csrfToken == "" {
		return false
	}

	mu.RLock()
	stored, exists := csrfTokens[sessionToken]
	mu.RUnlock()

	if !exists {
		return false
	}

	now := time.Now().UnixMilli()
	if now-stored.CreatedAt > config.Get().CSRF.Expiry {
		mu.Lock()
		delete(csrfTokens, sessionToken)
		mu.Unlock()
		scheduleSave()
		return false
	}

	// Timing-safe comparison
	if len(stored.Token) != len(csrfToken) {
		return false
	}
	var result byte
	for i := 0; i < len(stored.Token); i++ {
		result |= stored.Token[i] ^ csrfToken[i]
	}
	return result == 0
}

// cleanExpiredSessions removes expired sessions and CSRF tokens.
func cleanExpiredSessions() {
	mu.Lock()
	defer mu.Unlock()

	now := time.Now().UnixMilli()
	expiry := config.Get().SessionExpiry
	csrfExpiry := config.Get().CSRF.Expiry
	changed := false

	for token, session := range sessions {
		if now-session.LastAccess > expiry {
			delete(sessions, token)
			delete(csrfTokens, token)
			changed = true
		}
	}

	for sessionToken, csrfData := range csrfTokens {
		if now-csrfData.CreatedAt > csrfExpiry {
			delete(csrfTokens, sessionToken)
			changed = true
		}
	}

	if changed {
		scheduleSave()
	}
}

// FlushSessions synchronously persists any in-memory session/CSRF changes to
// disk. It is a final safety net invoked from main() during graceful shutdown:
// scheduleSave() flushes at most 5s after the last change, but a process that
// is killed before the timer fires would otherwise lose every unsaved session
// (users logged in again on every restart).
func FlushSessions() {
	saveMu.Lock()
	defer saveMu.Unlock()

	if !dirty {
		return
	}
	// Reuse the same serialization+write path as saveSessions by temporarily
	// forcing a fresh timer is overkill — just call saveSessions' core under the
	// same lock is impossible (it re-locks). Instead, replicate via a helper.
	flushSessionsLocked()
}

// flushSessionsLocked performs the actual persistence. Caller must hold saveMu.
func flushSessionsLocked() {
	if !dirty {
		return
	}

	mu.RLock()
	sessionCopy := make(map[string]*types.Session, len(sessions))
	for k, v := range sessions {
		sessionCopy[k] = v
	}
	csrfCopy := make(map[string]*types.CSRFToken, len(csrfTokens))
	for k, v := range csrfTokens {
		csrfCopy[k] = v
	}
	mu.RUnlock()

	data := struct {
		Sessions   map[string]*types.Session   `json:"sessions"`
		CSRFTokens map[string]*types.CSRFToken `json:"csrfTokens"`
	}{
		Sessions:   sessionCopy,
		CSRFTokens: csrfCopy,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		slog.Error("failed to marshal sessions", "error", err)
		return
	}
	encrypted, err := encrypt(string(jsonData))
	if err != nil {
		slog.Error("failed to encrypt sessions", "error", err)
		return
	}
	if err := os.MkdirAll(config.Get().DataDir, 0700); err != nil {
		slog.Error("failed to create data dir", "error", err)
		return
	}
	tmpFile := sessionFilePath() + ".tmp"
	if err := os.WriteFile(tmpFile, []byte(encrypted), 0600); err != nil {
		slog.Error("failed to write session file", "error", err)
		return
	}
	if err := os.Rename(tmpFile, sessionFilePath()); err != nil {
		slog.Error("failed to rename session file", "error", err)
		return
	}
	// FIX(bug 3): dirty is only cleared after every persistence step succeeded.
	// If marshal/encrypt/WriteFile/Rename failed above, dirty stays true so the
	// next flush (timer-armed or FlushSessions on exit) retries instead of
	// silently dropping the in-memory changes.
	dirty = false
}

func init() {
	loadSessions()

	// Periodic cleanup
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		for range ticker.C {
			cleanExpiredSessions()
		}
	}()

	// Save on exit is invoked explicitly by main() via FlushSessions().
}
