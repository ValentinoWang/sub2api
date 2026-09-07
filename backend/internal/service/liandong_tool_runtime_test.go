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

	runtimeService, err := NewLiandongToolkitRuntimeWithExpectedSHA256(LiandongToolkitRuntimeConfig{
		DataDir:   dataDir,
		AssetPath: assetPath,
		Version:   "1.2.3",
	}, hex.EncodeToString(digest[:]))
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
	asset := []byte("new asset")
	require.NoError(t, os.WriteFile(assetPath, asset, 0o600))
	programPath := DefaultLiandongToolkitProgramPath(dataDir)
	require.NoError(t, os.MkdirAll(filepath.Dir(programPath), 0o700))
	require.NoError(t, os.WriteFile(programPath, []byte("old installed program"), 0o700))

	runtimeService, err := NewLiandongToolkitRuntimeWithExpectedSHA256(
		LiandongToolkitRuntimeConfig{DataDir: dataDir, AssetPath: assetPath},
		strings.Repeat("0", sha256.Size*2),
	)
	require.NoError(t, err)
	_, err = runtimeService.Install()
	require.Error(t, err)
	require.True(t, infraerrors.IsServiceUnavailable(err))
	require.Equal(t, "LDXP_TOOLKIT_CHECKSUM_MISMATCH", infraerrors.Reason(err))
	installed, readErr := os.ReadFile(programPath)
	require.NoError(t, readErr)
	require.Equal(t, []byte("old installed program"), installed)
}

func TestLiandongToolkitRuntimeRequiresTrustedDigestForStatusAndInstall(t *testing.T) {
	dataDir := t.TempDir()
	assetPath := filepath.Join(t.TempDir(), "ldxp-toolkit")
	asset := []byte("release asset")
	require.NoError(t, os.WriteFile(assetPath, asset, 0o600))
	programPath := DefaultLiandongToolkitProgramPath(dataDir)
	require.NoError(t, os.MkdirAll(filepath.Dir(programPath), 0o700))
	require.NoError(t, os.WriteFile(programPath, asset, 0o700))

	runtimeService, err := NewLiandongToolkitRuntimeWithExpectedSHA256(
		LiandongToolkitRuntimeConfig{DataDir: dataDir, AssetPath: assetPath}, "",
	)
	require.NoError(t, err)
	status := runtimeService.Status()
	require.False(t, status.Ready)
	require.Contains(t, status.Diagnostics, "trusted toolkit SHA-256 is not configured")

	_, err = runtimeService.Install()
	require.Error(t, err)
	require.Equal(t, "LDXP_TOOLKIT_CHECKSUM_REQUIRED", infraerrors.Reason(err))
}

func TestLiandongToolkitRuntimeStatusRejectsStaleInstalledBytesAndAssetChanges(t *testing.T) {
	dataDir := t.TempDir()
	assetPath := filepath.Join(t.TempDir(), "ldxp-toolkit")
	asset := []byte("trusted release asset")
	digest := sha256.Sum256(asset)
	require.NoError(t, os.WriteFile(assetPath, asset, 0o600))

	runtimeService, err := NewLiandongToolkitRuntimeWithExpectedSHA256(
		LiandongToolkitRuntimeConfig{DataDir: dataDir, AssetPath: assetPath},
		hex.EncodeToString(digest[:]),
	)
	require.NoError(t, err)
	_, err = runtimeService.Install()
	require.NoError(t, err)

	programPath := DefaultLiandongToolkitProgramPath(dataDir)
	require.NoError(t, os.WriteFile(programPath, []byte("tampered installed bytes"), 0o700))
	status := runtimeService.Status()
	require.False(t, status.Ready)
	require.Contains(t, status.Diagnostics, "installed toolkit SHA-256 does not match the configured release digest")

	require.NoError(t, os.WriteFile(programPath, asset, 0o700))
	require.NoError(t, os.Remove(assetPath))
	status = runtimeService.Status()
	require.False(t, status.Ready)
	require.Contains(t, status.Diagnostics, "bundled toolkit asset is unavailable")
}

func TestLiandongToolkitRuntimeRejectsMalformedTrustedDigest(t *testing.T) {
	runtimeService, err := NewLiandongToolkitRuntimeWithExpectedSHA256(
		LiandongToolkitRuntimeConfig{DataDir: t.TempDir(), AssetPath: filepath.Join(t.TempDir(), "asset")},
		"not-a-sha256",
	)
	require.NoError(t, err)
	require.False(t, runtimeService.Status().Ready)
	require.Contains(t, runtimeService.Status().Diagnostics, "configured toolkit SHA-256 is invalid")
	_, err = runtimeService.Install()
	require.Error(t, err)
	require.Equal(t, "LDXP_TOOLKIT_CHECKSUM_INVALID", infraerrors.Reason(err))
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

func TestLiandongToolkitRuntimeUsesExplicitReleaseManifest(t *testing.T) {
	dataDir := t.TempDir()
	assetPath := filepath.Join(t.TempDir(), "ldxp-toolkit")
	asset := []byte("linux-amd64 release asset")
	require.NoError(t, os.WriteFile(assetPath, asset, 0o555))
	digest := sha256.Sum256(asset)
	manifestPath := writeLiandongToolkitReleaseManifest(t, filepath.Join(t.TempDir(), "ldxp-toolkit-release.json"), "2.4.6", hex.EncodeToString(digest[:]))

	runtimeService, err := NewLiandongToolkitRuntime(LiandongToolkitRuntimeConfig{
		DataDir:           dataDir,
		AssetPath:         assetPath,
		AssetManifestPath: manifestPath,
	})
	require.NoError(t, err)
	status := runtimeService.Status()
	require.Equal(t, "2.4.6", status.Version)
	require.True(t, status.AssetAvailable)
	if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
		result, installErr := runtimeService.Install()
		require.NoError(t, installErr)
		require.True(t, result.Status.Ready)
	} else {
		require.False(t, status.Ready)
		require.Contains(t, status.Diagnostics, "LDXP toolkit release asset is available only for linux/amd64")
		_, installErr := runtimeService.Install()
		require.Equal(t, "LDXP_TOOLKIT_PLATFORM_UNSUPPORTED", infraerrors.Reason(installErr))
	}
}

func TestLiandongToolkitRuntimeRejectsManifestDigestOrVersionMismatch(t *testing.T) {
	assetPath := filepath.Join(t.TempDir(), "ldxp-toolkit")
	asset := []byte("release asset")
	require.NoError(t, os.WriteFile(assetPath, asset, 0o555))
	digest := sha256.Sum256(asset)
	manifestPath := writeLiandongToolkitReleaseManifest(t, filepath.Join(t.TempDir(), "ldxp-toolkit-release.json"), "2.4.6", hex.EncodeToString(digest[:]))

	runtimeService, err := NewLiandongToolkitRuntime(LiandongToolkitRuntimeConfig{
		DataDir:           t.TempDir(),
		AssetPath:         assetPath,
		AssetManifestPath: manifestPath,
		AssetSHA256:       strings.Repeat("0", sha256.Size*2),
		Version:           "2.4.6",
	})
	require.NoError(t, err)
	status := runtimeService.Status()
	require.False(t, status.Ready)
	require.Contains(t, status.Diagnostics, "configured toolkit release manifest is invalid or unavailable")
	_, err = runtimeService.Install()
	require.Equal(t, "LDXP_TOOLKIT_RELEASE_MANIFEST_INVALID", infraerrors.Reason(err))

	runtimeService, err = NewLiandongToolkitRuntime(LiandongToolkitRuntimeConfig{
		DataDir:           t.TempDir(),
		AssetPath:         assetPath,
		AssetManifestPath: manifestPath,
		Version:           "2.4.7",
	})
	require.NoError(t, err)
	require.False(t, runtimeService.Status().Ready)
	_, err = runtimeService.Install()
	require.Equal(t, "LDXP_TOOLKIT_RELEASE_MANIFEST_INVALID", infraerrors.Reason(err))
}

func writeLiandongToolkitReleaseManifest(t *testing.T, path, version, digest string) string {
	t.Helper()
	content := []byte(`{"schema_version":1,"program":"ldxp-toolkit","version":"` + version + `","os":"linux","arch":"amd64","sha256":"` + digest + `"}`)
	require.NoError(t, os.WriteFile(path, content, 0o444))
	return path
}
