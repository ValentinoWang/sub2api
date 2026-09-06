package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	liandongToolkitDirectoryName = "ldxp"
	liandongToolkitTargetDirName = "toolkit"
	liandongToolkitProgramName   = "ldxp-toolkit"
	liandongToolkitDefaultVer    = "unpackaged"
	liandongToolkitChecksumLimit = 4096
)

var liandongToolkitArchiveSuffixes = []string{
	".7z",
	".bz2",
	".gz",
	".rar",
	".tar",
	".tgz",
	".xz",
	".zip",
}

// LiandongToolkitRuntime manages only the fixed local toolkit executable. It
// deliberately does not embed, download, unpack, or execute an asset.
type LiandongToolkitRuntime struct {
	dataDir     string
	assetPath   string
	programPath string
	version     string
}

// DefaultLiandongToolkitAssetPath derives the package handoff location from
// the application's data directory. The packaged release may populate this
// path later; its absence is a supported not-ready state.
func DefaultLiandongToolkitAssetPath(dataDir string) string {
	return filepath.Join(dataDir, liandongToolkitDirectoryName, "assets", liandongToolkitProgramName)
}

// DefaultLiandongToolkitProgramPath is the only destination used by Install.
func DefaultLiandongToolkitProgramPath(dataDir string) string {
	return filepath.Join(dataDir, liandongToolkitDirectoryName, liandongToolkitTargetDirName, liandongToolkitProgramName)
}

// NewLiandongToolkitRuntime validates the application-provided local paths
// and keeps the install destination independent from request data.
func NewLiandongToolkitRuntime(cfg LiandongToolkitRuntimeConfig) (*LiandongToolkitRuntime, error) {
	dataDir, err := normalizeLiandongToolkitLocalPath(cfg.DataDir, true)
	if err != nil {
		return nil, err
	}

	assetPath := strings.TrimSpace(cfg.AssetPath)
	if assetPath == "" {
		assetPath = DefaultLiandongToolkitAssetPath(dataDir)
	}
	assetPath, err = normalizeLiandongToolkitLocalPath(assetPath, false)
	if err != nil {
		return nil, err
	}
	if isLiandongToolkitArchive(assetPath) {
		return nil, infraerrors.BadRequest("LDXP_TOOLKIT_ASSET_REJECTED", "LDXP toolkit asset must be a local executable file")
	}

	version := strings.TrimSpace(cfg.Version)
	if version == "" {
		version = liandongToolkitDefaultVer
	}

	return &LiandongToolkitRuntime{
		dataDir:     dataDir,
		assetPath:   assetPath,
		programPath: DefaultLiandongToolkitProgramPath(dataDir),
		version:     version,
	}, nil
}

func normalizeLiandongToolkitLocalPath(value string, dataPath bool) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		reason := "LDXP_TOOLKIT_ASSET_PATH_REQUIRED"
		message := "LDXP toolkit asset path is not configured"
		if dataPath {
			reason = "LDXP_TOOLKIT_DATA_DIRECTORY_REQUIRED"
			message = "LDXP toolkit data directory is not configured"
		}
		return "", infraerrors.BadRequest(reason, message)
	}
	if strings.ContainsAny(value, "\x00\r\n;|&`$<>") || strings.Contains(value, "://") {
		return "", infraerrors.BadRequest("LDXP_TOOLKIT_LOCAL_PATH_REQUIRED", "LDXP toolkit paths must be local filesystem paths")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "" {
		return "", infraerrors.BadRequest("LDXP_TOOLKIT_LOCAL_PATH_REQUIRED", "LDXP toolkit paths must be local filesystem paths")
	}
	absolute, err := filepath.Abs(filepath.Clean(value))
	if err != nil {
		return "", infraerrors.BadRequest("LDXP_TOOLKIT_LOCAL_PATH_INVALID", "LDXP toolkit path is invalid")
	}
	return absolute, nil
}

func isLiandongToolkitArchive(path string) bool {
	lower := strings.ToLower(path)
	for _, suffix := range liandongToolkitArchiveSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

// LiandongToolkitUnavailableInstallationStatus describes a missing injected
// runtime without making the HTTP layer fabricate an installed state.
func LiandongToolkitUnavailableInstallationStatus() LiandongToolkitInstallationStatus {
	return LiandongToolkitInstallationStatus{
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		Version:     "unavailable",
		Diagnostics: []string{"LDXP toolkit runtime is unavailable"},
	}
}

// Status returns local readiness facts and never executes the program.
func (r *LiandongToolkitRuntime) Status() LiandongToolkitInstallationStatus {
	if r == nil {
		return LiandongToolkitUnavailableInstallationStatus()
	}

	status := LiandongToolkitInstallationStatus{
		OS:                  runtime.GOOS,
		Arch:                runtime.GOARCH,
		ExpectedProgramPath: r.programPath,
		Version:             r.version,
		Diagnostics:         make([]string, 0, 4),
	}

	status.DataDirectoryWritable = liandongToolkitDirectoryWritable(r.dataDir)
	if !status.DataDirectoryWritable {
		status.Diagnostics = append(status.Diagnostics, "server data directory is not writable")
	}

	assetInfo, assetErr := os.Lstat(r.assetPath)
	switch {
	case assetErr == nil && assetInfo.Mode().IsRegular():
		status.AssetAvailable = true
	case assetErr == nil:
		status.Diagnostics = append(status.Diagnostics, "bundled toolkit asset is not a regular local file")
	case errors.Is(assetErr, os.ErrNotExist):
		status.Diagnostics = append(status.Diagnostics, "bundled toolkit asset is unavailable")
	default:
		status.Diagnostics = append(status.Diagnostics, "bundled toolkit asset cannot be inspected")
	}

	programInfo, programErr := os.Lstat(r.programPath)
	switch {
	case programErr == nil:
		status.Exists = true
		if programInfo.Mode().IsRegular() {
			status.Executable = programInfo.Mode().Perm()&0o111 != 0
			if !status.Executable {
				status.Diagnostics = append(status.Diagnostics, "installed toolkit is not executable")
			}
			if digest, err := liandongToolkitSHA256(r.programPath); err == nil {
				status.SHA256 = digest
			} else {
				status.Diagnostics = append(status.Diagnostics, "installed toolkit SHA-256 is unavailable")
			}
		} else if programInfo.Mode()&os.ModeSymlink != 0 {
			status.Diagnostics = append(status.Diagnostics, "installed toolkit path is a symlink")
		} else {
			status.Diagnostics = append(status.Diagnostics, "installed toolkit path is not a regular file")
		}
	case errors.Is(programErr, os.ErrNotExist):
		status.Diagnostics = append(status.Diagnostics, "LDXP toolkit is not installed")
	default:
		status.Diagnostics = append(status.Diagnostics, "installed toolkit cannot be inspected")
	}

	status.Ready = status.Exists && status.Executable && status.DataDirectoryWritable
	return status
}

// InstallationStatus is an explicit alias for callers that prefer the
// operation name at the HTTP boundary.
func (r *LiandongToolkitRuntime) InstallationStatus() LiandongToolkitInstallationStatus {
	return r.Status()
}

func liandongToolkitDirectoryWritable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}
	return info.Mode().Perm()&0o222 != 0
}

// Install copies the fixed local asset into the fixed target directory. The
// temporary file is created beside the target so rename is atomic on the
// server filesystem.
func (r *LiandongToolkitRuntime) Install() (*LiandongToolkitInstallationResult, error) {
	if r == nil {
		return nil, infraerrors.ServiceUnavailable("LDXP_TOOLKIT_UNAVAILABLE", "LDXP toolkit runtime is unavailable")
	}
	if filepath.Clean(r.assetPath) == filepath.Clean(r.programPath) {
		return nil, infraerrors.BadRequest("LDXP_TOOLKIT_ASSET_REJECTED", "LDXP toolkit asset and destination must be different")
	}

	assetInfo, err := os.Lstat(r.assetPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, infraerrors.ServiceUnavailable("LDXP_TOOLKIT_NOT_READY", "LDXP toolkit asset is unavailable; installation is not ready")
	}
	if err != nil {
		return nil, infraerrors.ServiceUnavailable("LDXP_TOOLKIT_ASSET_UNREADABLE", "LDXP toolkit asset cannot be inspected")
	}
	if assetInfo.Mode()&os.ModeSymlink != 0 || !assetInfo.Mode().IsRegular() {
		return nil, infraerrors.BadRequest("LDXP_TOOLKIT_ASSET_REJECTED", "LDXP toolkit asset must be a regular local file")
	}

	expectedChecksum, err := liandongToolkitAdjacentChecksum(r.assetPath)
	if err != nil {
		return nil, err
	}
	actualChecksum, err := liandongToolkitSHA256(r.assetPath)
	if err != nil {
		return nil, infraerrors.ServiceUnavailable("LDXP_TOOLKIT_ASSET_UNREADABLE", "LDXP toolkit asset cannot be hashed")
	}
	if expectedChecksum != "" && !strings.EqualFold(expectedChecksum, actualChecksum) {
		return nil, infraerrors.ServiceUnavailable("LDXP_TOOLKIT_CHECKSUM_MISMATCH", "LDXP toolkit asset checksum verification failed")
	}

	targetDir := filepath.Dir(r.programPath)
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		return nil, infraerrors.ServiceUnavailable("LDXP_TOOLKIT_DATA_DIRECTORY_UNAVAILABLE", "LDXP toolkit server data directory is unavailable")
	}
	if err := os.Chmod(targetDir, 0o700); err != nil {
		return nil, infraerrors.ServiceUnavailable("LDXP_TOOLKIT_DATA_DIRECTORY_UNAVAILABLE", "LDXP toolkit server data directory is unavailable")
	}

	targetInfo, err := os.Lstat(r.programPath)
	if err == nil && targetInfo.Mode()&os.ModeSymlink != 0 {
		return nil, infraerrors.BadRequest("LDXP_TOOLKIT_DESTINATION_REJECTED", "existing LDXP toolkit destination is a symlink")
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, infraerrors.ServiceUnavailable("LDXP_TOOLKIT_DESTINATION_UNAVAILABLE", "LDXP toolkit destination cannot be inspected")
	}

	temporary, err := os.CreateTemp(targetDir, ".ldxp-toolkit-*.tmp")
	if err != nil {
		return nil, infraerrors.ServiceUnavailable("LDXP_TOOLKIT_DESTINATION_UNAVAILABLE", "LDXP toolkit destination is not writable")
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()

	asset, err := os.Open(r.assetPath)
	if err != nil {
		_ = temporary.Close()
		return nil, infraerrors.ServiceUnavailable("LDXP_TOOLKIT_ASSET_UNREADABLE", "LDXP toolkit asset cannot be opened")
	}
	_, copyErr := io.Copy(temporary, asset)
	closeAssetErr := asset.Close()
	if copyErr != nil || closeAssetErr != nil {
		_ = temporary.Close()
		return nil, infraerrors.ServiceUnavailable("LDXP_TOOLKIT_INSTALL_FAILED", "LDXP toolkit asset could not be copied")
	}
	if err := temporary.Chmod(0o700); err != nil {
		_ = temporary.Close()
		return nil, infraerrors.ServiceUnavailable("LDXP_TOOLKIT_INSTALL_FAILED", "LDXP toolkit permissions could not be set")
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return nil, infraerrors.ServiceUnavailable("LDXP_TOOLKIT_INSTALL_FAILED", "LDXP toolkit asset could not be persisted")
	}
	if err := temporary.Close(); err != nil {
		return nil, infraerrors.ServiceUnavailable("LDXP_TOOLKIT_INSTALL_FAILED", "LDXP toolkit asset could not be closed")
	}
	if err := os.Rename(temporaryPath, r.programPath); err != nil {
		return nil, infraerrors.ServiceUnavailable("LDXP_TOOLKIT_INSTALL_FAILED", "LDXP toolkit could not be atomically installed")
	}
	committed = true
	if err := os.Chmod(r.programPath, 0o700); err != nil {
		return nil, infraerrors.ServiceUnavailable("LDXP_TOOLKIT_INSTALL_FAILED", "LDXP toolkit permissions could not be finalized")
	}
	if err := syncLiandongToolkitDirectory(targetDir); err != nil {
		return nil, infraerrors.ServiceUnavailable("LDXP_TOOLKIT_INSTALL_FAILED", "LDXP toolkit installation could not be finalized")
	}

	status := r.Status()
	if !status.Exists || !status.Executable {
		return nil, infraerrors.ServiceUnavailable("LDXP_TOOLKIT_INSTALL_FAILED", "LDXP toolkit installation did not produce an executable file")
	}
	return &LiandongToolkitInstallationResult{Installed: true, Status: status}, nil
}

func liandongToolkitAdjacentChecksum(assetPath string) (string, error) {
	checksumPath := assetPath + ".sha256"
	info, err := os.Lstat(checksumPath)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", infraerrors.ServiceUnavailable("LDXP_TOOLKIT_CHECKSUM_INVALID", "adjacent LDXP toolkit checksum is unavailable")
	}
	raw, err := os.ReadFile(checksumPath)
	if err != nil || len(raw) > liandongToolkitChecksumLimit {
		return "", infraerrors.ServiceUnavailable("LDXP_TOOLKIT_CHECKSUM_INVALID", "adjacent LDXP toolkit checksum is invalid")
	}
	fields := strings.Fields(string(raw))
	if len(fields) == 0 || len(fields[0]) != sha256.Size*2 {
		return "", infraerrors.ServiceUnavailable("LDXP_TOOLKIT_CHECKSUM_INVALID", "adjacent LDXP toolkit checksum is invalid")
	}
	if _, err := hex.DecodeString(fields[0]); err != nil {
		return "", infraerrors.ServiceUnavailable("LDXP_TOOLKIT_CHECKSUM_INVALID", "adjacent LDXP toolkit checksum is invalid")
	}
	return strings.ToLower(fields[0]), nil
}

func liandongToolkitSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func syncLiandongToolkitDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = directory.Close() }()
	return directory.Sync()
}
