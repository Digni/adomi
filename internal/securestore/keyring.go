package securestore

import (
	"fmt"

	"github.com/zalando/go-keyring"
)

const serviceName = "adomi.azure-devops"

type Store interface {
	Get(profile string) (string, error)
	Set(profile, pat string) error
	Delete(profile string) error
}

type keyringBackend interface {
	Get(service, user string) (string, error)
	Set(service, user, password string) error
	Delete(service, user string) error
}

type zalandoKeyringBackend struct{}

func (zalandoKeyringBackend) Get(service, user string) (string, error) {
	return keyring.Get(service, user)
}

func (zalandoKeyringBackend) Set(service, user, password string) error {
	return keyring.Set(service, user, password)
}

func (zalandoKeyringBackend) Delete(service, user string) error {
	return keyring.Delete(service, user)
}

type KeyringStore struct {
	backend keyringBackend
}

func NewKeyringStore() KeyringStore {
	return KeyringStore{backend: zalandoKeyringBackend{}}
}

func (s KeyringStore) Get(profile string) (string, error) {
	pat, err := s.keyring().Get(serviceName, profile)
	if err != nil {
		return "", fmt.Errorf("reading Azure DevOps PAT for profile %q: %w", profile, err)
	}
	return pat, nil
}

func (s KeyringStore) Set(profile, pat string) error {
	if err := s.keyring().Set(serviceName, profile, pat); err != nil {
		return fmt.Errorf("storing Azure DevOps PAT for profile %q: %w", profile, err)
	}
	return nil
}

func (s KeyringStore) Delete(profile string) error {
	if err := s.keyring().Delete(serviceName, profile); err != nil {
		return fmt.Errorf("deleting Azure DevOps PAT for profile %q: %w", profile, err)
	}
	return nil
}

func (s KeyringStore) keyring() keyringBackend {
	if s.backend == nil {
		return zalandoKeyringBackend{}
	}
	return s.backend
}
