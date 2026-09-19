package store

import (
	"errors"
	"strings"
	"sync"

	"himate.local/backend/internal/model"
)

var ErrNotFound = errors.New("not found")

type UserStore interface {
	CreateUser(user model.User) error
	FindUserByEmail(email string) (model.User, error)
	FindUserByID(id string) (model.User, error)
}

type MemoryUserStore struct {
	mu      sync.RWMutex
	byID    map[string]model.User
	byEmail map[string]string
}

func NewMemoryUserStore() *MemoryUserStore {
	return &MemoryUserStore{
		byID:    make(map[string]model.User),
		byEmail: make(map[string]string),
	}
}

func (s *MemoryUserStore) CreateUser(user model.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := strings.ToLower(strings.TrimSpace(user.Email))
	if key == "" || user.ID == "" {
		return errors.New("user id and email are required")
	}
	if _, exists := s.byEmail[key]; exists {
		return errors.New("email already exists")
	}
	if _, exists := s.byID[user.ID]; exists {
		return errors.New("user id already exists")
	}
	s.byID[user.ID] = user
	s.byEmail[key] = user.ID
	return nil
}

func (s *MemoryUserStore) FindUserByEmail(email string) (model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, ok := s.byEmail[strings.ToLower(strings.TrimSpace(email))]
	if !ok {
		return model.User{}, ErrNotFound
	}
	user, ok := s.byID[id]
	if !ok {
		return model.User{}, ErrNotFound
	}
	return user, nil
}

func (s *MemoryUserStore) FindUserByID(id string) (model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.byID[id]
	if !ok {
		return model.User{}, ErrNotFound
	}
	return user, nil
}
