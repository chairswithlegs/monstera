package store

import (
	"context"

	"github.com/chairswithlegs/monstera-fed/internal/domain"
)

// Store is the persistence abstraction. All methods use domain types so that
// the service layer and callers depend only on store and domain, not on any
// specific SQL implementation (e.g. postgres).
type Store interface {
	CreateAccount(ctx context.Context, in CreateAccountInput) (*domain.Account, error)
	GetAccountByID(ctx context.Context, id string) (*domain.Account, error)
	GetLocalAccountByUsername(ctx context.Context, username string) (*domain.Account, error)
	GetRemoteAccountByUsername(ctx context.Context, username string, domain *string) (*domain.Account, error)
	WithTx(ctx context.Context, fn func(Store) error) error

	CreateUser(ctx context.Context, in CreateUserInput) (*domain.User, error)

	CreateStatus(ctx context.Context, in CreateStatusInput) (*domain.Status, error)
	GetStatusByID(ctx context.Context, id string) (*domain.Status, error)
	DeleteStatus(ctx context.Context, id string) error
	IncrementStatusesCount(ctx context.Context, accountID string) error
	DecrementStatusesCount(ctx context.Context, accountID string) error

	GetHomeTimeline(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, error)
	GetPublicTimeline(ctx context.Context, localOnly bool, maxID *string, limit int) ([]domain.Status, error)
}
