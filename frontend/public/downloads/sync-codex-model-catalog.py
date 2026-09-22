#!/usr/bin/env python3
"""Fetch and validate a personal Codex catalog. Python 3.11+, no dependencies."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import time
import tomllib
import urllib.error
import urllib.parse
import urllib.request


class CatalogError(Exception):
    pass


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise CatalogError("入口发生重定向；请先核对配置地址，未向新地址发送凭证。")


def catalog_url(base_url, version):
    parts = urllib.parse.urlsplit(base_url)
    if parts.username or parts.password or parts.query or parts.fragment or not parts.hostname:
        raise CatalogError("Base URL 不应包含凭证、查询参数或片段。")
    if parts.scheme != "https" and not (
        parts.scheme == "http" and parts.hostname in {"localhost", "127.0.0.1", "::1"}
    ):
        raise CatalogError("远端入口必须使用 HTTPS；HTTP 仅允许本机回环地址。")
    return urllib.parse.urlunsplit((
        parts.scheme, parts.netloc, parts.path.rstrip("/") + "/models",
        urllib.parse.urlencode({"client_version": version}), "",
    ))


def validate_catalog(body, target):
    try:
        catalog = json.loads(body)
    except (ValueError, UnicodeError) as exc:
        raise CatalogError("入口未返回有效 JSON，旧目录保持不变。") from exc
    models = catalog.get("models") if isinstance(catalog, dict) else None
    if not isinstance(models, list) or not models:
        raise CatalogError("需要完整 Codex models 目录，不能使用只有 data/id 的模型列表。")
    slugs = [model.get("slug") if isinstance(model, dict) else None for model in models]
    if any(not isinstance(slug, str) or not slug.strip() for slug in slugs):
        raise CatalogError("目录包含缺失的模型 slug。")
    if len(set(slugs)) != len(slugs):
        raise CatalogError("目录包含重复的模型 slug。")
    if target not in slugs:
        raise CatalogError("目录中没有目标模型，请核对当前入口、模型 ID 和访问权限。")
    model = models[slugs.index(target)]
    window, maximum = model.get("context_window"), model.get("max_context_window")
    if type(window) is not int or window <= 0:
        raise CatalogError("目标模型没有明确的正整数 context_window，不推测窗口。")
    if maximum is not None and (type(maximum) is not int or maximum < window):
        raise CatalogError("目标模型的最大窗口小于默认窗口或格式不正确。")
    percent = model.get("effective_context_window_percent", 95)
    if type(percent) is not int or not 0 < percent <= 100:
        raise CatalogError("目标模型的可用窗口比例无效。")
    summary = {key: model.get(key) for key in (
        "slug", "context_window", "max_context_window", "effective_context_window_percent",
        "auto_compact_token_limit",
    )}
    summary["effective_context_window_percent"] = percent
    return catalog, summary


def load_picker_rules(path, provider, endpoint):
    try:
        rules = json.loads(path.read_bytes())
    except (OSError, ValueError, UnicodeError) as exc:
        raise CatalogError("显示规则文件无法读取或不是有效 JSON，旧目录保持不变。") from exc
    allowed = {"version", "provider", "endpoint", "aliases", "visibility"}
    if not isinstance(rules, dict) or set(rules) - allowed or type(rules.get("version")) is not int or rules["version"] != 1:
        raise CatalogError("显示规则版本或字段无效。")
    if rules.get("provider") != provider or rules.get("endpoint") != endpoint:
        raise CatalogError("显示规则与当前 provider 或模型目录入口不匹配，禁止跨入口套用。")
    for field in ("aliases", "visibility"):
        values = rules.get(field, {})
        if not isinstance(values, dict) or any(
            not isinstance(key, str) or not key.strip() or not isinstance(value, str) or not value.strip()
            for key, value in values.items()
        ):
            raise CatalogError("显示规则需要非空模型 ID 到字符串的映射。")
    if any(value not in {"list", "hide"} for value in rules.get("visibility", {}).values()):
        raise CatalogError("visibility 规则仅支持 list 或 hide。")
    aliases = rules.get("aliases", {})
    for slug in aliases:
        seen = set()
        while slug in aliases:
            if slug in seen:
                raise CatalogError("别名规则存在循环或自引用。")
            seen.add(slug)
            slug = aliases[slug]
    return rules


def apply_picker_visibility(catalog, rules):
    models = {model["slug"]: model for model in catalog["models"]}
    aliases = rules.get("aliases", {})
    visibility = rules.get("visibility", {})
    decisions = []
    for slug, value in visibility.items():
        if slug in models:
            models[slug]["visibility"] = value
        decisions.append({"model": slug, "reason": "explicit_visibility" if slug in models else "model_absent", "visibility": value})
    # Resolve against one snapshot so alias chains do not depend on iteration order.
    for alias in aliases:
        canonical = aliases[alias]
        while canonical in aliases:
            canonical = aliases[canonical]
        if alias not in models:
            reason = "alias_absent"
        elif alias in visibility:
            reason = "explicit_visibility_wins"
        elif canonical not in models:
            reason = "canonical_absent"
        elif models[canonical].get("visibility") != "list":
            reason = "canonical_not_visible"
        else:
            models[alias]["visibility"] = "hide"
            reason = "alias_hidden"
        decisions.append({"model": alias, "canonical": canonical, "reason": reason})
    return decisions


def install_catalog(body, output, codex_bin, target, rules=None):
    catalog, summary = validate_catalog(body, target)
    output = output.expanduser().absolute()
    output.parent.mkdir(parents=True, exist_ok=True)
    if output.is_symlink():
        raise CatalogError("输出路径是符号链接，请使用独立的普通 JSON 文件。")
    decisions = apply_picker_visibility(catalog, rules or {})
    hidden_models = sorted(model["slug"] for model in catalog["models"] if model.get("visibility") == "hide")
    # Validate the candidate with the real client before touching the last good file.
    with tempfile.TemporaryDirectory(prefix=".catalog-check-", dir=output.parent) as folder:
        candidate = Path(folder) / "models.json"
        candidate.write_text(json.dumps(catalog, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        candidate.chmod(0o600)
        check = subprocess.run(
            [codex_bin, "-c", "model_catalog_json=" + json.dumps(str(candidate)), "debug", "models"],
            capture_output=True, text=True, timeout=30, check=False,
        )
        if check.returncode:
            raise CatalogError("当前 Codex 拒绝加载候选目录；请核对客户端版本，旧目录未替换。")
        loaded_catalog, loaded = validate_catalog(check.stdout, target)
        if loaded != summary:
            raise CatalogError("客户端加载值与下载值不一致，旧目录未替换。")
        expected_visibility = {model["slug"]: model.get("visibility") for model in catalog["models"]}
        actual_visibility = {model["slug"]: model.get("visibility") for model in loaded_catalog["models"]}
        if expected_visibility != actual_visibility:
            raise CatalogError("客户端模型列表或显示状态与候选目录不一致，旧目录未替换。")
        backup = None
        if output.exists():
            backup = output.with_name(output.name + ".backup-" + str(time.time_ns()))
            with backup.open("xb") as handle:
                os.chmod(backup, 0o600)
                handle.write(output.read_bytes())
        os.replace(candidate, output)
    return {"output": str(output), "backup": str(backup) if backup else None,
            "sha256": hashlib.sha256(output.read_bytes()).hexdigest(),
            "model_count": len(catalog["models"]), "hidden_models": hidden_models,
            "picker_decisions": decisions, "target": summary}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", type=Path, default=Path.home() / ".codex/config.toml")
    parser.add_argument("--provider", help="实际使用的 provider；有 profile 覆盖时必须显式指定")
    parser.add_argument("--auth-file", type=Path, help="默认使用配置目录下 auth.json")
    parser.add_argument("--codex-bin", default="codex", help="桌面端应传实际内置引擎路径")
    parser.add_argument("--model", help="必须包含的目标模型，默认读取所选配置文件的 model")
    parser.add_argument("--output", required=True, type=Path, help="独立目录文件，禁止指向 config.toml/auth.json")
    parser.add_argument("--rules", type=Path, help="显示规则 JSON；默认自动读取输出路径旁的 <文件名>.rules.json")
    parser.add_argument("--ignore-rules", action="store_true", help="本次保留上游显示状态，不应用本地规则")
    args = parser.parse_args()
    config_path = args.config.expanduser().resolve()
    config = tomllib.loads(config_path.read_text(encoding="utf-8"))
    provider_id = args.provider or config.get("model_provider")
    provider = config.get("model_providers", {}).get(provider_id, {})
    if not provider.get("base_url"):
        raise CatalogError("所选配置中没有该 provider 的 Base URL；先核对实际生效配置。")
    if config.get("profile") and not args.provider:
        raise CatalogError("配置选择了 profile；请检查覆盖关系并显式指定 --provider。")
    if provider.get("auth") or provider.get("http_headers") or provider.get("env_http_headers"):
        raise CatalogError("检测到命令或自定义请求头认证；本脚本只支持普通 API Key，请单独核对。")
    auth_path = (args.auth_file or config_path.parent / "auth.json").expanduser().resolve()
    output = args.output.expanduser().absolute()
    if output.suffix != ".json" or output.resolve() in {config_path, auth_path}:
        raise CatalogError("输出必须为独立的 .json 目录文件，不能覆盖配置或认证。")
    if provider.get("experimental_bearer_token"):
        raise CatalogError("本脚本不处理内嵌 bearer token；请单独核对实际认证方式。")
    env_key = provider.get("env_key")
    if env_key:
        key = os.environ.get(env_key)
    elif provider.get("requires_openai_auth"):
        key = json.loads(auth_path.read_text(encoding="utf-8")).get("OPENAI_API_KEY")
    else:
        raise CatalogError("未识别当前 provider 的 API Key 来源，不尝试使用其他入口的凭证。")
    if not isinstance(key, str) or not key.strip():
        raise CatalogError("没有找到当前 provider 的 API Key；不要把密钥写进命令行。")
    target = args.model or config.get("model")
    if not target:
        raise CatalogError("请用 --model 指定目标模型。")
    binary = shutil.which(args.codex_bin)
    if not binary:
        raise CatalogError("找不到指定 Codex 程序。")
    version_result = subprocess.run([binary, "--version"], capture_output=True, text=True, timeout=10)
    version_match = re.search(r"\b\d+\.\d+\.\d+\b", version_result.stdout)
    if version_result.returncode or not version_match:
        raise CatalogError("无法识别当前 Codex 版本。")
    version = version_match.group()
    url = catalog_url(provider["base_url"], version)
    if args.rules and args.ignore_rules:
        raise CatalogError("--rules 与 --ignore-rules 不能同时使用。")
    rules_path = (args.rules or output.with_suffix(".rules.json")).expanduser().absolute()
    if rules_path.resolve() in {output.resolve(), config_path, auth_path}:
        raise CatalogError("规则文件必须独立于输出、配置和认证文件。")
    endpoint = url.split("?", 1)[0]
    rules = None
    if not args.ignore_rules and (args.rules or rules_path.exists()):
        rules = load_picker_rules(rules_path, provider_id, endpoint)
    request = urllib.request.Request(url, headers={"Authorization": "Bearer " + key, "Accept": "application/json"})
    try:
        with urllib.request.build_opener(NoRedirect).open(request, timeout=30) as response:
            if response.status != 200:
                raise CatalogError("模型目录没有返回 HTTP 200。")
            body = response.read()
            etag = response.headers.get("ETag")
    except urllib.error.HTTPError as exc:
        raise CatalogError(f"模型目录请求失败：HTTP {exc.code}，旧目录保持不变。") from exc
    result = install_catalog(body, output, binary, target, rules)
    result["rules_path"] = str(rules_path) if rules is not None else None
    result["upstream_sha256"] = hashlib.sha256(body).hexdigest()
    result.update({"source": url, "client_version": version, "fetched_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), "etag": etag})
    print(json.dumps(result, ensure_ascii=False, indent=2))
    print("配置未修改。核对预算后，将下面一行添加到 config.toml 顶层（首个 [section] 之前）：")
    print("model_catalog_json = " + json.dumps(str(output), ensure_ascii=False))


if __name__ == "__main__":
    try:
        main()
    except CatalogError as exc:
        raise SystemExit(str(exc))
    except (OSError, ValueError, subprocess.SubprocessError):
        raise SystemExit("读取、网络或客户端验证失败；请核对路径、配置语法与连接。未输出凭证或响应正文。")
