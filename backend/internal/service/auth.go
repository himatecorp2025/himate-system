package service

import (
	"errors"
	"strings"

	"himate.local/backend/internal/model"
	"himate.local/backend/internal/security"
	"himate.local/backend/internal/store"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type AuthService struct {
	users store.UserStore
}

func NewAuthService(users store.UserStore) *AuthService {
	return &AuthService{users: users}
}

func (s *AuthService) Authenticate(email, password string) (model.User, error) {
	user, err := s.users.FindUserByEmail(strings.TrimSpace(email))
	if err != nil || !user.Active || !security.VerifyPassword(user.PasswordHash, password) {
		return model.User{}, ErrInvalidCredentials
	}
	return user, nil
}

func (s *AuthService) FindUser(id string) (model.User, error) {
	return s.users.FindUserByID(id)
}
