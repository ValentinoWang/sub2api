//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type guardedDeleteProxyRepoStub struct {
	*proxyRepoStub
	errorsByID map[int64]error
	calls      []int64
}

func (s *guardedDeleteProxyRepoStub) DeleteIfUnused(_ context.Context, id int64) error {
	s.calls = append(s.calls, id)
	return s.errorsByID[id]
}

func TestAdminProxyDeleteRejectsBackupReference(t *testing.T) {
	repo := &guardedDeleteProxyRepoStub{
		proxyRepoStub: &proxyRepoStub{},
		errorsByID:    map[int64]error{9: ErrProxyBackupInUse},
	}
	svc := &adminServiceImpl{proxyRepo: repo}

	err := svc.DeleteProxy(context.Background(), 9)

	require.ErrorIs(t, err, ErrProxyBackupInUse)
	require.Equal(t, []int64{9}, repo.calls)
	require.Empty(t, repo.deletedIDs)
}

func TestAdminProxyBatchDeleteSkipsBackupReferences(t *testing.T) {
	repo := &guardedDeleteProxyRepoStub{
		proxyRepoStub: &proxyRepoStub{},
		errorsByID:    map[int64]error{9: ErrProxyBackupInUse},
	}
	svc := &adminServiceImpl{proxyRepo: repo}

	result, err := svc.BatchDeleteProxies(context.Background(), []int64{8, 9, 10})

	require.NoError(t, err)
	require.Equal(t, []int64{8, 10}, result.DeletedIDs)
	require.Len(t, result.Skipped, 1)
	require.Equal(t, int64(9), result.Skipped[0].ID)
	require.Contains(t, result.Skipped[0].Reason, "PROXY_BACKUP_IN_USE")
	require.Equal(t, []int64{8, 9, 10}, repo.calls)
}
