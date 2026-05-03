package auth

import (
	"context"
	"fmt"
)

// OAuthProvider defines the contract for a provider that can refresh its own tokens.
type OAuthProvider interface {
	// ID returns a unique identifier for this provider (e.g., "claude_code", "github").
	ID() string

	// RefreshToken exchanges a refresh token for a new access token.
	RefreshToken(ctx context.Context, currentCred *Credential) (*Credential, error)
}

// Manager handles fetching and refreshing credentials.
type Manager struct {
	store     Store
	providers map[string]OAuthProvider
}

// NewManager creates a new auth manager.
func NewManager(store Store) *Manager {
	return &Manager{
		store:     store,
		providers: make(map[string]OAuthProvider),
	}
}

// Register adds an OAuth provider to the manager.
func (m *Manager) Register(p OAuthProvider) {
	m.providers[p.ID()] = p
}

// GetValidCredential returns a valid access token for the given provider ID.
// If the token is expired, it automatically attempts to refresh it.
func (m *Manager) GetValidCredential(ctx context.Context, providerID string) (*Credential, error) {
	cred, err := m.store.Get(providerID)
	if err != nil {
		return nil, fmt.Errorf("failed to read credential: %w", err)
	}
	if cred == nil {
		return nil, fmt.Errorf("no credentials found for provider %s", providerID)
	}

	if !cred.IsExpired() {
		return cred, nil
	}

	// Token is expired, try to refresh
	p, ok := m.providers[providerID]
	if !ok {
		return nil, fmt.Errorf("no oauth handler registered for provider %s", providerID)
	}

	newCred, err := p.RefreshToken(ctx, cred)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	if err := m.store.Set(providerID, newCred); err != nil {
		// Log the error but return the valid token anyway so the request succeeds
		// In a full implementation, we'd pass a logger here.
	}

	return newCred, nil
}
