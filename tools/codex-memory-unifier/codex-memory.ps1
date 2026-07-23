$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
python "$ScriptDir\codex_memory_unifier.py" @args
exit $LASTEXITCODE
