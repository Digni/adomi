package securestore

import (
	"errors"
	"testing"
)

func TestKeyringStoreUsesExpectedServiceAndProfileAsUser(t *testing.T) {
	backend := &fakeKeyringBackend{}
	store := KeyringStore{backend: backend}

	if err := store.Set("company-cloud", "secret-pat"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if backend.setService != serviceName || backend.setUser != "company-cloud" || backend.setPassword != "secret-pat" {
		t.Fatalf("Set used service/user/password = %q/%q/%q", backend.setService, backend.setUser, backend.setPassword)
	}

	backend.getValue = "secret-pat"
	got, err := store.Get("company-cloud")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got != "secret-pat" {
		t.Fatalf("Get = %q, want secret-pat", got)
	}
	if backend.getService != serviceName || backend.getUser != "company-cloud" {
		t.Fatalf("Get used service/user = %q/%q", backend.getService, backend.getUser)
	}

	if err := store.Delete("company-cloud"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if backend.deleteService != serviceName || backend.deleteUser != "company-cloud" {
		t.Fatalf("Delete used service/user = %q/%q", backend.deleteService, backend.deleteUser)
	}
}

func TestKeyringStoreWrapsBackendErrors(t *testing.T) {
	store := KeyringStore{backend: &fakeKeyringBackend{err: errors.New("backend failed")}}

	if _, err := store.Get("company-cloud"); err == nil {
		t.Fatal("Get error = nil, want error")
	}
	if err := store.Set("company-cloud", "secret-pat"); err == nil {
		t.Fatal("Set error = nil, want error")
	}
	if err := store.Delete("company-cloud"); err == nil {
		t.Fatal("Delete error = nil, want error")
	}
}

type fakeKeyringBackend struct {
	err           error
	getValue      string
	getService    string
	getUser       string
	setService    string
	setUser       string
	setPassword   string
	deleteService string
	deleteUser    string
}

func (f *fakeKeyringBackend) Get(service, user string) (string, error) {
	f.getService = service
	f.getUser = user
	if f.err != nil {
		return "", f.err
	}
	return f.getValue, nil
}

func (f *fakeKeyringBackend) Set(service, user, password string) error {
	f.setService = service
	f.setUser = user
	f.setPassword = password
	return f.err
}

func (f *fakeKeyringBackend) Delete(service, user string) error {
	f.deleteService = service
	f.deleteUser = user
	return f.err
}
