// Package auth gère les sessions et les jetons CSRF.
// Les sessions sont en mémoire. Les credentials AWS sont chiffrés en
// AES-GCM avec une clé venant de l'env (SESSION_KEY).
package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Creds représente les identifiants AWS saisis au login.
type Creds struct {
	AccessKeyID     string `json:"a"`
	SecretAccessKey string `json:"s"`
	Region          string `json:"r"`
	SessionToken    string `json:"t,omitempty"`
}

// Session est l'objet que voient les handlers une fois la requête authentifiée.
type Session struct {
	ID         string
	Creds      Creds
	Bucket     string // currently selected bucket
	CSRFToken  string
	CreatedAt  time.Time
	LastActive time.Time
}

type encryptedSession struct {
	id         string
	encCreds   []byte // AES-GCM ciphertext
	bucket     string
	csrf       string
	createdAt  time.Time
	lastActive time.Time
}

type Store struct {
	mu       sync.RWMutex
	sessions map[string]*encryptedSession
	gcm      cipher.AEAD
	idleTTL  time.Duration
	absTTL   time.Duration
}

const (
	CookieName = "sid"
	idLen      = 32
)

func NewStore(key []byte, idleTTL, absTTL time.Duration) (*Store, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("session cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("session GCM: %w", err)
	}
	s := &Store{
		sessions: make(map[string]*encryptedSession),
		gcm:      gcm,
		idleTTL:  idleTTL,
		absTTL:   absTTL,
	}
	go s.gcLoop()
	return s, nil
}

// Create crée une nouvelle session et renvoie l'ID (opaque).
func (s *Store) Create(creds Creds) (*Session, error) {
	enc, err := s.encrypt(creds)
	if err != nil {
		return nil, err
	}
	id, err := randomToken(idLen)
	if err != nil {
		return nil, err
	}
	csrf, err := randomToken(idLen)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	es := &encryptedSession{
		id:         id,
		encCreds:   enc,
		csrf:       csrf,
		createdAt:  now,
		lastActive: now,
	}
	s.mu.Lock()
	s.sessions[id] = es
	s.mu.Unlock()

	return &Session{
		ID:         id,
		Creds:      creds,
		CSRFToken:  csrf,
		CreatedAt:  now,
		LastActive: now,
	}, nil
}

// Get renvoie la session pour un ID (nil si expirée ou inexistante).
func (s *Store) Get(id string) (*Session, bool) {
	if id == "" {
		return nil, false
	}
	s.mu.RLock()
	es, ok := s.sessions[id]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	now := time.Now()
	if now.Sub(es.lastActive) > s.idleTTL || now.Sub(es.createdAt) > s.absTTL {
		s.Destroy(id)
		return nil, false
	}
	creds, err := s.decrypt(es.encCreds)
	if err != nil {
		s.Destroy(id)
		return nil, false
	}
	return &Session{
		ID:         es.id,
		Creds:      creds,
		Bucket:     es.bucket,
		CSRFToken:  es.csrf,
		CreatedAt:  es.createdAt,
		LastActive: es.lastActive,
	}, true
}

// Touch met à jour la date de dernière activité.
func (s *Store) Touch(id string) {
	s.mu.Lock()
	if es, ok := s.sessions[id]; ok {
		es.lastActive = time.Now()
	}
	s.mu.Unlock()
}

// SetBucket mémorise le bucket courant sur la session.
func (s *Store) SetBucket(id, bucket string) {
	s.mu.Lock()
	if es, ok := s.sessions[id]; ok {
		es.bucket = bucket
		es.lastActive = time.Now()
	}
	s.mu.Unlock()
}

// Destroy supprime une session.
func (s *Store) Destroy(id string) {
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
}

// VerifyCSRF compare le token reçu et celui de la session en constant-time.
func (s *Store) VerifyCSRF(id, token string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	es, ok := s.sessions[id]
	if !ok {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(es.csrf), []byte(token)) == 1
}

func (s *Store) encrypt(c Creds) ([]byte, error) {
	plaintext, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return s.gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func (s *Store) decrypt(box []byte) (Creds, error) {
	if len(box) < s.gcm.NonceSize() {
		return Creds{}, errors.New("ciphertext too short")
	}
	nonce, ct := box[:s.gcm.NonceSize()], box[s.gcm.NonceSize():]
	pt, err := s.gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return Creds{}, err
	}
	var c Creds
	if err := json.Unmarshal(pt, &c); err != nil {
		return Creds{}, err
	}
	return c, nil
}

func (s *Store) gcLoop() {
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for range t.C {
		s.gc()
	}
}

func (s *Store) gc() {
	now := time.Now()
	s.mu.Lock()
	for id, es := range s.sessions {
		if now.Sub(es.lastActive) > s.idleTTL || now.Sub(es.createdAt) > s.absTTL {
			delete(s.sessions, id)
		}
	}
	s.mu.Unlock()
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// NewRandomToken expose le helper aléatoire (utilisé pour le CSRF pré-login).
func NewRandomToken(n int) (string, error) { return randomToken(n) }

// SetCookie pose le cookie de session avec les attributs stricts.
func SetCookie(w http.ResponseWriter, id string, secure bool, maxAge time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(maxAge.Seconds()),
	})
}

// ClearCookie efface le cookie côté client.
func ClearCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}
