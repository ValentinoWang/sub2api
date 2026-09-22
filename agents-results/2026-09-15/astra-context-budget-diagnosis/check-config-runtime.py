"""Run one ephemeral, read-only model check using the installed user config."""
import json
from pathlib import Path
import selectors
import subprocess
import sys
import time
import tomllib


binary = "/Applications/ChatGPT.app/Contents/Resources/codex"
config_path = Path("/Users/vsiyo/.codex/config.toml")
config = tomllib.loads(config_path.read_text())
assert config["model_catalog_json"] == "/Users/vsiyo/.codex/model-catalogs/sub2api.json"
assert config["model"] == "gpt-6-astra"
assert "model_context_window" not in config
assert "model_auto_compact_token_limit" not in config

# The probe needs no connectors. These overrides affect this process only;
# model, provider, catalog and context budgets all come from the saved config.
overrides = {f"mcp_servers.{name}.enabled": False for name in config.get("mcp_servers", {})}
overrides.update({"features.apps": False, "features.memories": False})
process = subprocess.Popen(
    [binary, "app-server"], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
    stderr=subprocess.DEVNULL, bufsize=0,
)
selector = selectors.DefaultSelector()
selector.register(process.stdout, selectors.EVENT_READ)
result = {"checked_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
          "binary": binary, "config": str(config_path), "ephemeral": True,
          "model_or_catalog_overrides": False, "long_input_test": False}


def send(method, params, request_id=None):
    message = {"method": method, "params": params}
    if request_id is not None:
        message["id"] = request_id
    process.stdin.write((json.dumps(message) + "\n").encode())
    process.stdin.flush()


try:
    send("initialize", {"clientInfo": {"name": "catalog_config_check", "version": "1.0.0"}}, 1)
    deadline = time.monotonic() + 90
    completed = False
    while time.monotonic() < deadline:
        if not selector.select(timeout=1):
            if process.poll() is not None:
                raise RuntimeError("App-server exited before completion")
            continue
        line = process.stdout.readline()
        if not line:
            raise RuntimeError("App-server closed its output")
        message = json.loads(line)
        if message.get("error"):
            raise RuntimeError(f"RPC error code {message['error'].get('code')}")
        if message.get("id") == 1:
            send("initialized", {})
            send("thread/start", {"cwd": "/tmp/codex-catalog-smoke.LpQRkA", "ephemeral": True,
                                  "approvalPolicy": "never", "sandbox": "read-only", "config": overrides}, 2)
        elif message.get("id") == 2:
            response = message["result"]
            result.update({"model": response.get("model"), "provider": response.get("modelProvider"),
                           "thread_id": response["thread"]["id"]})
            assert response["thread"]["ephemeral"] is True
            assert response.get("model") == "gpt-6-astra", "Wrong effective model"
            send("turn/start", {"threadId": result["thread_id"], "input": [{"type": "text",
                 "text": "这是一次已授权的模型连通性检查，不需要读取文件、记忆或调用任何工具。请只回复 CATALOG_OK。"}]}, 3)
        elif message.get("method") == "thread/tokenUsage/updated":
            result["token_usage"] = message["params"]["tokenUsage"]
        elif message.get("method") == "item/completed":
            item = message["params"]["item"]
            if item.get("type") == "agentMessage":
                result["reply"] = item.get("text")
        elif message.get("method") == "item/started":
            if message["params"]["item"].get("type") in {"commandExecution", "fileChange", "mcpToolCall", "dynamicToolCall"}:
                raise RuntimeError("Unexpected tool use in model-only probe")
        elif message.get("method") == "turn/completed":
            result["status"] = message["params"]["turn"]["status"]
            completed = True
            break
        elif message.get("method") and message.get("id") is not None:
            raise RuntimeError("Unexpected server request in model-only probe")
    assert completed, "Model check timed out"
    assert result["status"] == "completed", "Model request did not complete"
    assert result.get("reply", "").strip() == "CATALOG_OK", "Unexpected model reply"
    assert result.get("token_usage", {}).get("modelContextWindow") == 997500, "Unexpected effective context window"
    result["passed"] = True
except Exception as error:
    result.update({"passed": False, "failure": str(error)})
finally:
    selector.close()
    process.terminate()
    try:
        process.wait(timeout=5)
    except subprocess.TimeoutExpired:
        process.kill()
        process.wait()
    output = Path(__file__).with_name("config-runtime-result-" + time.strftime("%Y%m%dT%H%M%SZ", time.gmtime()) + ".json")
    output.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps(result, ensure_ascii=False, indent=2))
sys.exit(0 if result.get("passed") else 1)
