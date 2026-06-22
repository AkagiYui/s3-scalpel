#!/usr/bin/env python3
"""
为 GitHub Actions Build workflow 生成 Action Summary。

通过 GitHub API 获取当前工作流运行的 job 结果，收集各平台构建状态，
生成 Markdown 格式的摘要并写入 GITHUB_STEP_SUMMARY 文件。

用法：
  python3 scripts/generate_summary.py

环境变量（GitHub Actions 自动注入，无需手动设置）：
  GITHUB_TOKEN        - GitHub Actions 令牌（用于 API 请求）
  GITHUB_REPOSITORY   - GitHub 仓库（如 owner/repo）
  GITHUB_RUN_ID       - GitHub Actions 运行 ID
  GITHUB_SERVER_URL   - GitHub 服务器 URL（默认 https://github.com）
  GITHUB_STEP_SUMMARY - GitHub Actions 摘要文件路径

手动传入的环境变量：
  COMMITS_JSON - JSON 格式的 commits 数组（可选，用于显示触发构建的 commit）
"""

import json
import os
import sys
import urllib.error
import urllib.request
from datetime import datetime, timezone


# ---------------------------------------------------------------------------
# GitHub API 交互
# ---------------------------------------------------------------------------

def get_workflow_jobs(run_id: str, repo: str, token: str) -> list[dict]:
    """通过 GitHub API 获取当前工作流运行的所有 job 信息。"""
    url = f"https://api.github.com/repos/{repo}/actions/runs/{run_id}/jobs?per_page=100"
    headers = {
        "Authorization": f"Bearer {token}",
        "Accept": "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
    }
    req = urllib.request.Request(url, headers=headers)
    try:
        with urllib.request.urlopen(req) as resp:
            data = json.loads(resp.read().decode("utf-8"))
            return data.get("jobs", [])
    except urllib.error.URLError as e:
        print(f"⚠️ 获取 GitHub Jobs 失败: {e}", file=sys.stderr)
        return []


def parse_build_job(job: dict) -> dict | None:
    """从 GitHub API 返回的 job 数据中提取构建结果。

    构建 job 名称格式为 "构建 <os> (<arch>)"，例如 "构建 macos-latest (arm64)"。
    """
    name = job.get("name", "")
    if not name.startswith("构建 "):
        return None

    # 解析 OS 和架构
    try:
        rest = name[len("构建 "):]
        os_part, arch_part = rest.rsplit(" (", 1)
        arch = arch_part.rstrip(")")
    except ValueError:
        os_part = name
        arch = "未知"

    # 映射 job 结论到构建状态
    conclusion = job.get("conclusion") or "in_progress"
    status_map = {
        "success": "success",
        "failure": "failure",
        "cancelled": "cancelled",
        "timed_out": "failure",
        None: "running",
    }
    status = status_map.get(conclusion, conclusion)

    # 根据操作系统和架构生成产物名称（与 workflow 中 artifact 名称一致）
    artifact_map = {
        "macos-latest": f"s3-scalpel-macos-{arch}",
        "windows-latest": f"s3-scalpel-windows-{arch}",
    }
    artifact = artifact_map.get(os_part, f"s3-scalpel-{os_part}-{arch}")

    return {
        "os": os_part,
        "arch": arch,
        "status": status,
        "artifact": artifact,
    }


# ---------------------------------------------------------------------------
# JSON 辅助
# ---------------------------------------------------------------------------

def load_json_env(name: str) -> list | None:
    """从环境变量加载 JSON 数据，若不存在或解析失败则返回 None。"""
    raw = os.environ.get(name)
    if not raw:
        return None
    try:
        return json.loads(raw)
    except json.JSONDecodeError as e:
        print(f"⚠️ 解析环境变量 {name} 失败: {e}", file=sys.stderr)
        return None


# ---------------------------------------------------------------------------
# Markdown 渲染
# ---------------------------------------------------------------------------

def get_platform_icon(os_name: str) -> str:
    """根据操作系统返回对应图标。"""
    return {"macos": "🍎", "windows": "🪟"}.get(
        os_name.lower().replace("-latest", ""), "📦"
    )


def get_status_badge(status: str) -> str:
    """根据构建状态返回对应徽章。"""
    return {
        "success": "✅ 成功",
        "failure": "❌ 失败",
        "cancelled": "🚫 取消",
        "running": "🔄 运行中",
    }.get(status, f"❓ {status}")


def render_commits(commits: list[dict]) -> str:
    """渲染 commit 列表为 Markdown 表格。"""
    if not commits:
        return ""

    lines = [
        "### 📝 触发构建的 Commits",
        "",
        "| # | Commit | 信息 |",
        "|---|--------|------|",
    ]

    for i, commit in enumerate(commits, 1):
        sha = commit.get("id", "")[:7]
        message = commit.get("message", "").split("\n")[0]
        author = commit.get("author", {}).get("name", "未知")
        # 截断过长的 message
        if len(message) > 60:
            message = message[:57] + "..."
        lines.append(f"| {i} | `{sha}` | {message} ({author}) |")

    lines.append("")
    return "\n".join(lines)


def render_build_results(results: list[dict]) -> str:
    """渲染构建结果为 Markdown 表格。"""
    if not results:
        return ""

    lines = [
        "### 🔨 构建结果",
        "",
        "| 平台 | 架构 | 状态 | 产物 |",
        "|------|------|------|------|",
    ]

    for r in results:
        os_name = r.get("os", "未知")
        arch = r.get("arch", "未知")
        status = r.get("status", "未知")
        artifact = r.get("artifact", "—")
        icon = get_platform_icon(os_name)
        badge = get_status_badge(status)
        lines.append(f"| {icon} {os_name} | {arch} | {badge} | `{artifact}` |")

    lines.append("")
    return "\n".join(lines)


def render_summary(build_results: list[dict], commits: list[dict] | None) -> str:
    """生成完整的 Action Summary Markdown 内容。"""
    lines = [
        "## 🚀 S3 Scalpel 构建摘要",
        "",
    ]

    # 构建时间
    now = datetime.now(timezone.utc)
    lines.append(f"**构建时间**: {now.strftime('%Y-%m-%d %H:%M:%S UTC')}")
    lines.append("")

    # 运行链接
    server_url = os.environ.get("GITHUB_SERVER_URL", "https://github.com")
    repository = os.environ.get("GITHUB_REPOSITORY", "")
    run_id = os.environ.get("GITHUB_RUN_ID", "")
    if repository and run_id:
        run_url = f"{server_url}/{repository}/actions/runs/{run_id}"
        lines.append(f"**运行链接**: [查看运行详情]({run_url})")
        lines.append("")

    # Commits 信息
    if commits:
        lines.append(render_commits(commits))

    # 构建结果
    if build_results:
        lines.append(render_build_results(build_results))

        # 总体状态
        all_success = all(r.get("status") == "success" for r in build_results)
        if all_success:
            lines.append("> 🎉 **所有平台构建成功！**")
        else:
            failed = [r for r in build_results if r.get("status") != "success"]
            failed_names = ", ".join(
                f"{r.get('os', '?')}({r.get('arch', '?')})" for r in failed
            )
            lines.append(f"> ⚠️ **部分平台构建失败**: {failed_names}")
        lines.append("")

        # 下载提示
        lines.extend([
            "### 📥 下载产物",
            "",
            "构建产物已上传至 Artifacts，可在工作流运行页面的 **Artifacts** 区域下载。",
            "",
        ])
    else:
        lines.append("> ℹ️ 未找到构建结果。")
        lines.append("")

    return "\n".join(lines)


# ---------------------------------------------------------------------------
# 主函数
# ---------------------------------------------------------------------------

def main() -> int:
    token = os.environ.get("GITHUB_TOKEN", "")
    repo = os.environ.get("GITHUB_REPOSITORY", "")
    run_id = os.environ.get("GITHUB_RUN_ID", "")

    # 通过 GitHub API 获取构建结果
    build_results: list[dict] = []
    if token and repo and run_id:
        jobs = get_workflow_jobs(run_id, repo, token)
        for job in jobs:
            result = parse_build_job(job)
            if result:
                build_results.append(result)
        print(f"📋 获取到 {len(jobs)} 个 job，其中 {len(build_results)} 个构建 job", file=sys.stderr)
    else:
        print("⚠️ 未检测到 GitHub Actions 环境，跳过 API 请求", file=sys.stderr)

    # 读取 commits 信息
    commits = load_json_env("COMMITS_JSON") or []

    # 生成摘要
    summary = render_summary(build_results, commits)

    # 输出到 stdout（方便调试）
    print(summary)

    # 写入 GITHUB_STEP_SUMMARY
    step_summary = os.environ.get("GITHUB_STEP_SUMMARY")
    if step_summary:
        with open(step_summary, "a", encoding="utf-8") as f:
            f.write(summary + "\n")
        print(f"\n✅ 摘要已写入 {step_summary}", file=sys.stderr)

    return 0


if __name__ == "__main__":
    sys.exit(main())
