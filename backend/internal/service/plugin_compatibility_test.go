package service

import (
	"testing"

	pluginv1 "github.com/Wei-Shaw/sub2api/pkg/pluginapi/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluatePluginCompatibility(t *testing.T) {
	manifest := testPluginManifest(nil)
	host := PluginHostInfo{Version: "0.1.179", BuildType: "release"}

	result := EvaluatePluginCompatibility(manifest, host)
	require.True(t, result.Compatible)
	assert.True(t, result.Tested)
	assert.Equal(t, "compatible", result.Status)

	manifest.Requires.TestedSub2APIVersions = []string{"0.1.178"}
	result = EvaluatePluginCompatibility(manifest, host)
	require.True(t, result.Compatible)
	assert.False(t, result.Tested)
	assert.Equal(t, "untested", result.Status)

	manifest.Requires.Sub2API = ">=0.2.0 <0.3.0"
	result = EvaluatePluginCompatibility(manifest, host)
	assert.False(t, result.Compatible)
	assert.Equal(t, "incompatible", result.Status)
}

func TestEvaluatePluginCompatibilityRejectsProtocolMismatch(t *testing.T) {
	manifest := testPluginManifest(nil)
	manifest.Requires.PluginProtocol = pluginv1.ProtocolVersion + 1

	result := EvaluatePluginCompatibility(manifest, PluginHostInfo{Version: "0.1.179"})

	assert.False(t, result.Compatible)
	assert.Equal(t, "incompatible", result.Status)
}

func TestMatchesSemverRange(t *testing.T) {
	assert.True(t, matchesSemverRange("0.1.179", ">=0.1.170, <0.2.0"))
	assert.True(t, matchesSemverRange("v1.2.3", "=1.2.3"))
	assert.False(t, matchesSemverRange("0.1.169", ">=0.1.170 <0.2.0"))
	assert.False(t, matchesSemverRange("dev", ">=0.1.0"))
	assert.False(t, matchesSemverRange("0.1.179", "^0.1.0"))
}

func TestEvaluatePluginCompatibilityFork(t *testing.T) {
	manifest := testPluginManifest(nil)
	manifest.Requires.Sub2API = ">=0.2.3 <0.2.4"
	host := PluginHostInfo{Version: "0.2.4.1", BuildType: "release"}

	manifest.Requires.TestedSub2APIVersions = []string{"0.2.4"}
	result := EvaluatePluginCompatibility(manifest, host)
	require.True(t, result.Compatible)
	assert.False(t, result.Tested, "an upstream release test cannot attest a fork revision")
	assert.Equal(t, "untested", result.Status)
	assert.Equal(t, "0.2.4.1", result.CurrentSub2API)

	manifest.Requires.TestedSub2APIVersions = []string{"v0.2.4.1"}
	result = EvaluatePluginCompatibility(manifest, host)
	require.True(t, result.Compatible)
	assert.True(t, result.Tested)
	assert.Equal(t, "compatible", result.Status)

	manifest.Requires.TestedSub2APIVersions = []string{"0.2.4.2", "invalid"}
	result = EvaluatePluginCompatibility(manifest, host)
	require.True(t, result.Compatible)
	assert.False(t, result.Tested)

	manifest.Requires.Sub2API = ">=0.2.4"
	result = EvaluatePluginCompatibility(manifest, host)
	assert.False(t, result.Compatible, "a fork revision must not claim the upcoming upstream release")
	assert.Equal(t, "incompatible", result.Status)
}

func TestMatchesSemverRangeFork(t *testing.T) {
	for _, tc := range []struct {
		version    string
		expression string
		matches    bool
	}{
		{"0.2.4.1", ">0.2.3 <0.2.4", true},
		{"0.2.4.1", "=0.2.4.1", true},
		{" v0.2.4.1 ", "=v0.2.4.1", true},
		{"0.2.4.2", ">0.2.4.1 <0.2.4.10", true},
		{"0.2.4.10", ">0.2.4.2", true},
		{"0.2.4.1", ">=0.2.4.2", false},
		{"0.2.4.1", "=0.2.4", false},
		{"0.2.4.1", ">=0.2.4", false},
		{"0.2.4", "=0.2.4.1", false},
		{"0.2.4", ">0.2.4.999", true},
		{"0.2.4.01", ">=0.2.3", false},
		{"0.2.04.1", ">=0.2.3", false},
		{"0.2.4.1.1", ">=0.2.3", false},
		{"0.2.4.x", ">=0.2.3", false},
		{"0.2.4.1", ">=0.2.4.x", false},
	} {
		t.Run(tc.version+" "+tc.expression, func(t *testing.T) {
			assert.Equal(t, tc.matches, matchesSemverRange(tc.version, tc.expression))
		})
	}
}

func TestPluginPackageVersionRejectsForkNotation(t *testing.T) {
	manifest := testPluginManifest(nil)
	manifest.Version = "0.2.4.1"
	assert.ErrorContains(t, manifest.Validate(), "插件版本必须是有效的语义化版本")
	assert.Empty(t, normalizeSemver("0.2.4.1"))
}
