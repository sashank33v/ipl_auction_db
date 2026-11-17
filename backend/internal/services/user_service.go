package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
)

type UserService struct {
	userRepo *repository.UserRepository
}

func NewUserService(userRepo *repository.UserRepository) *UserService {
	return &UserService{
		userRepo: userRepo,
	}
}

func (s *UserService) CreateUser(
	ctx context.Context,
	user *repository.User,
) error {
	if user == nil {
		return errors.New("user is required")
	}

	user.Username = strings.TrimSpace(user.Username)
	user.Email = strings.TrimSpace(user.Email)
	user.PasswordHash = strings.TrimSpace(user.PasswordHash)

	if user.Username == "" {
		return errors.New("username is required")
	}

	if user.Email == "" {
		return errors.New("email is required")
	}

	if user.PasswordHash == "" {
		return errors.New("password hash is required")
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	return nil
}

func (s *UserService) GetUserByID(
	ctx context.Context,
	id string,
) (*repository.User, error) {
	id = strings.TrimSpace(id)

	if id == "" {
		return nil, errors.New("user ID is required")
	}

	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	return user, nil
}

func (s *UserService) GetUserByEmail(
	ctx context.Context,
	email string,
) (*repository.User, error) {
	email = strings.TrimSpace(email)

	if email == "" {
		return nil, errors.New("email is required")
	}

	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	return user, nil
}

func (s *UserService) GetAllUsers(
	ctx context.Context,
) ([]repository.User, error) {
	users, err := s.userRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("get users: %w", err)
	}

	return users, nil
}

func (s *UserService) DeleteUser(
	ctx context.Context,
	id string,
) error {
	id = strings.TrimSpace(id)

	if id == "" {
		return errors.New("user ID is required")
	}

	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get user before deletion: %w", err)
	}
	if user == nil {
		return errors.New("user not found")
	}

	if err := s.userRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	return nil
}
