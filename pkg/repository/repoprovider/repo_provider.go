package repoprovider

import "context"

type RepositoryProvider interface {
	InitRepo(ctx context.Context, bsl string) error

	ConnectToRepo(ctx context.Context, bsl string) error

	PrepareRepo(ctx context.Context, bsl string) error

	PruneRepo(ctx context.Context, bsl string) error

	PruneRepoQuick(ctx context.Context, bsl string) error

	EnsureUnlockRepo(ctx context.Context, bsl string) error

	Forget(ctx context.Context, snapshotID, bsl string) error
}
