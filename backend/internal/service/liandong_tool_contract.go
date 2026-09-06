package service

import (
	"context"
)

// LiandongToolkitService is the narrow boundary consumed by the administrator
// toolkit HTTP layer. The restock service owns the implementation; keeping the
// boundary here lets the HTTP package compile before that implementation and a
// packaged toolkit asset are available.
type LiandongToolkitService interface {
	Status(context.Context) (*LiandongRestockStatus, error)
	UpdateConfiguration(context.Context, LiandongRestockConfigurationUpdate) (*LiandongRestockStatus, error)
	TestConfiguration(context.Context) (*LiandongToolkitConnectivityResult, error)
	ListGoods(context.Context) (*LiandongToolkitGoodsResult, error)
	Preview(context.Context, []int64) (*LiandongRestockPreview, error)
	StartManualJob(context.Context, []int64) (*LiandongRestockJobSummary, error)
	GetJob(context.Context, string) (*LiandongRestockJobSummary, error)
	ResumeJob(context.Context, string) (*LiandongRestockJobSummary, error)
	ExportJob(context.Context, string) (*LiandongRestockJobExport, error)
}

// LiandongToolkitInstallationStatus describes local runtime readiness without
// exposing the configured asset path or any merchant credentials.
type LiandongToolkitInstallationStatus struct {
	OS                    string   `json:"os"`
	Arch                  string   `json:"arch"`
	ExpectedProgramPath   string   `json:"expected_program_path"`
	Version               string   `json:"version"`
	Ready                 bool     `json:"ready"`
	AssetAvailable        bool     `json:"asset_available"`
	Exists                bool     `json:"exists"`
	Executable            bool     `json:"executable"`
	SHA256                string   `json:"sha256,omitempty"`
	DataDirectoryWritable bool     `json:"data_directory_writable"`
	Diagnostics           []string `json:"diagnostics"`
}

// LiandongToolkitInstallationResult is returned after a successful atomic
// install or repair.
type LiandongToolkitInstallationResult struct {
	Installed bool                              `json:"installed"`
	Status    LiandongToolkitInstallationStatus `json:"status"`
}

// LiandongToolkitRuntimeConfig is supplied by the application integration
// layer. AssetPath is local-only and defaults below DataDir when omitted.
type LiandongToolkitRuntimeConfig struct {
	DataDir   string
	AssetPath string
	Version   string
}

// LiandongToolkitConnectivityResult reports a read-only merchant probe.
// It deliberately has no token, URL, or response-body fields.
type LiandongToolkitConnectivityResult struct {
	Configured bool   `json:"configured"`
	Reachable  bool   `json:"reachable"`
	ReadOnly   bool   `json:"read_only"`
	Message    string `json:"message,omitempty"`
}

// LiandongToolkitGood is the safe subset of a remote LDXP card good exposed
// to the administrator UI.
type LiandongToolkitGood struct {
	GoodsID      int64  `json:"goods_id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	CurrentStock int    `json:"current_stock"`
}

type LiandongToolkitGoodsResult struct {
	Goods []LiandongToolkitGood `json:"goods"`
}
