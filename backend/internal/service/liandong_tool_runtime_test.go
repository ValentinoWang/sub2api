package service

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestLiandongToolkitRuntimeStatusReportsUnpackagedState(t *testing.T) {
	dataDir := t.TempDir()
	assetPath := filepath.Join(dataDir, "bundle", "ldxp-toolkit")
	runtimeService, err := NewLiandongToolkitRuntime(LiandongToolkitRuntimeConfig{
		DataDir:   dataDir,
		AssetPath: assetPath,
		Version:   "1.2.3",
	})
	require.NoError(t, err)

	status := runtimeService.Status()
	require.Equal(t, runtime.GOOS, status.OS)
	require.Equal(t, runtime.GOARCH, status.Arch)
	require.Equal(t, DefaultLiandongToolkitProgramPath(dataDir), status.ExpectedProgramPath)
	require.Equal(t, "1.2.3", status.Version)
	require.False(t, status.Ready)
	require.False(t, status.AssetAvailable)
	require.False(t, status.Exists)
	require.False(t, status.Executable)
	require.True(t, status.DataDirectoryWritable)
	require.Contains(t, status.Diagnostics, "bundled toolkit asset is unavailable")
	require.Contains(t, status.Diagnostics, "LDXP toolkit is not installed")
}

func TestLiandongToolkitRuntimeInstallVerifiesChecksumAndUsesPrivateMode(t *testing.T) {
	dataDir := t.TempDir()
	assetDir := t.TempDir()
	assetPath := filepath.Join(assetDir, "ldxp-toolkit")
	asset := []byte("local packaged LDXP toolkit")
	require.NoError(t, os.WriteFile(assetPath, asset, 0o600))
	digest := sha256.Sum256(asset)
	require.NoError(t, os.WriteFile(assetPath+".sha256", []byte(hex.EncodeToString(digest[:])+"  ldxp-toolkit\n"), 0o600))

	runtimeService, err := NewLiandongToolkitRuntime(LiandongToolkitRuntimeConfig{
		DataDir:   dataDir,
		AssetPath: assetPath,
		Version:   "1.2.3",
	})
	require.NoError(t, err)
	result, err := runtimeService.Install()
	require.NoError(t, err)
	require.True(t, result.Installed)

	programPath := DefaultLiandongToolkitProgramPath(dataDir)
	installed, err := os.ReadFile(programPath)
	require.NoError(t, err)
	require.Equal(t, asset, installed)
	info, err := os.Stat(programPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), info.Mode().Perm())
	require.Equal(t, hex.EncodeToString(digest[:]), result.Status.SHA256)
	require.True(t, result.Status.Ready)
}

func TestLiandongToolkitRuntimeChecksumMismatchPreservesExistingProgram(t *testing.T) {
	dataDir := t.TempDir()
	assetDir := t.TempDir()
	assetPath := filepath.Join(assetDir, "ldxp-toolkit")
	require.NoError(t, os.WriteFile(assetPath, []byte("new asset"), 0o600))
	require.NoError(t, os.WriteFile(assetPath+".sha256", []byte(strings.Repeat("0", sha256.Size*2)), 0o600))
	programPath := DefaultLiandongToolkitProgramPath(dataDir)
	require.NoError(t, os.MkdirAll(filepath.Dir(programPath), 0o700))
	require.NoError(t, os.WriteFile(programPath, []byte("old installed program"), 0o700))

	runtimeService, err := NewLiandongToolkitRuntime(LiandongToolkitRuntimeConfig{DataDir: dataDir, AssetPath: assetPath})
	require.NoError(t, err)
	_, err = runtimeService.Install()
	require.Error(t, err)
	require.True(t, infraerrors.IsServiceUnavailable(err))
	require.Equal(t, "LDXP_TOOLKIT_CHECKSUM_MISMATCH", infraerrors.Reason(err))
	installed, readErr := os.ReadFile(programPath)
	require.NoError(t, readErr)
	require.Equal(t, []byte("old installed program"), installed)
}

func TestLiandongToolkitRuntimeRejectsURLAndArchiveAssets(t *testing.T) {
	_, err := NewLiandongToolkitRuntime(LiandongToolkitRuntimeConfig{
		DataDir:   t.TempDir(),
		AssetPath: "https://example.invalid/ldxp-toolkit.zip",
	})
	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))

	_, err = NewLiandongToolkitRuntime(LiandongToolkitRuntimeConfig{
		DataDir:   t.TempDir(),
		AssetPath: filepath.Join(t.TempDir(), "ldxp-toolkit.tar.gz"),
	})
	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
}
