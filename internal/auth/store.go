package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Credential represents stored OAuth or auth tokens for a provider.
type Credential struct {
	AccessToken  string         `json:"access_token"`
	RefreshToken string         `json:"refresh_token"`
	ExpiresAt    time.Time      `json:"expires_at"`
	Extra        map[string]any `json:"extra,omitempty"` // For things like tenant ID, etc.
}

// IsExpired checks if the access token has expired (with a 5 minute safety margin).
func (c *Credential) IsExpired() bool {
	return time.Now().Add(5 * time.Minute).After(c.ExpiresAt)
}

// Store defines the interface for persisting provider credentials.
type Store interface {
	Get(providerID string) (*Credential, error)
	Set(providerID string, cred *Credential) error
	Delete(providerID string) error
}

// FileStore implements Store using a local JSON file.
type FileStore struct {
	path string
	mu   sync.RWMutex
}

func NewFileStore() *FileStore {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".heimsense")
	os.MkdirAll(dir, 0755)
	return &FileStore{
		path: filepath.Join(dir, "credentials.json"),
	}
}

func (s *FileStore) Get(providerID string) (*Credential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Not found, not an error
		}
		return nil, err
	}

	var all map[string]*Credential
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, fmt.Errorf("corrupted credentials file: %w", err)
	}

	return all[providerID], nil
}

func (s *FileStore) Set(providerID string, cred *Credential) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var all map[string]*Credential
	data, err := os.ReadFile(s.path)
	if err == nil {
		_ = json.Unmarshal(data, &all)
	}
	if all == nil {
		all = make(map[string]*Credential)
	}

	all[providerID] = cred

	out, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, out, 0600) // Ensure strict permissions
}

func (s *FileStore) Delete(providerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var all map[string]*Credential
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil
	}
	if err := json.Unmarshal(data, &all); err != nil {
		return nil
	}

	delete(all, providerID)

	out, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, out, 0600)
}
