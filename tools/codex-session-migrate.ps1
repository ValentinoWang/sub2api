$ErrorActionPreference = 'Stop'
$script = Join-Path $PSScriptRoot 'codex_session_migrate.py'
& python $script @args
exit $LASTEXITCODE
