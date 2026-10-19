#!/usr/bin/env python3
"""Diff-scoped review checks; Python stdlib only. Never checks out or fixes code."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile


def git(root, *args):
    result = subprocess.run(
        ["git", "-C", str(root), *args], capture_output=True, check=False
    )
    if result.returncode:
        # Do not echo user-supplied ref values or potentially sensitive stderr.
        raise ValueError("Git command failed; check repository, refs and permissions")
    return result.stdout


def resolve(root, ref):
    return git(root, "rev-parse", "--verify", "--end-of-options", ref + "^{commit}").decode().strip()


def paths(data):
    return [os.fsdecode(value) for value in data.split(b"\0") if value]


def fingerprint(root):
    """Detect content edits even if a file was already marked modified by Git."""
    digest = hashlib.sha256()
    digest.update(git(root, "rev-parse", "HEAD"))
    digest.update(git(root, "diff", "--binary", "--no-ext-diff", "--no-textconv", "HEAD", "--"))
    digest.update(git(root, "diff", "--cached", "--binary", "--no-ext-diff", "--no-textconv", "HEAD", "--"))
    for name in sorted(paths(git(root, "ls-files", "--others", "--exclude-standard", "-z"))):
        path = root / name
        digest.update(os.fsencode(name) + b"\0")
        if path.is_symlink():
            digest.update(os.fsencode(os.readlink(path)))
        else:
            with path.open("rb") as source:
                for chunk in iter(lambda: source.read(65536), b""):
                    digest.update(chunk)
        digest.update(b"\0")
    return digest.hexdigest()


def scope(root, args):
    head = resolve(root, "HEAD")
    mode, base, target = "worktree", head, head
    untracked = paths(git(root, "ls-files", "--others", "--exclude-standard", "-z"))
    dirty = bool(git(root, "status", "--porcelain=v1", "-z"))
    if args.base or args.commit:
        target = resolve(root, args.commit or args.head)
        if target != head or dirty:
            raise ValueError("Checks require a clean checkout of the target commit")
        if args.commit:
            mode = "commit"
            parents = git(root, "rev-list", "--parents", "-n", "1", target).decode().split()[1:]
            base = parents[0] if parents else None
        else:
            mode = "branch"
            base = git(root, "merge-base", resolve(root, args.base), target).decode().strip()
    elif args.staged:
        mode = "staged"
        if git(root, "diff", "--name-only", "-z") or untracked:
            raise ValueError("Staged checks require no unstaged or untracked files")

    diff_args = ["--no-ext-diff", "--no-renames"]
    if mode == "commit" and base is None:
        changed = paths(git(root, "diff-tree", "--root", "--no-commit-id", "-r",
                            "--name-only", "-z", *diff_args, target, "--"))
        whitespace = ["git", "diff-tree", "--root", "--no-commit-id", "-r",
                      "--check", *diff_args, target, "--"]
    else:
        revisions = [base, target] if mode in ("branch", "commit") else (
            ["--cached", head] if mode == "staged" else [head]
        )
        changed = paths(git(root, "diff", "--name-only", "-z", *diff_args, *revisions, "--"))
        whitespace = ["git", "diff", "--check", *diff_args, *revisions, "--"]
    if mode == "worktree":
        changed.extend(untracked)
    changed = sorted(set(changed))
    return {"mode": mode, "base": base, "target": target, "files": changed}, whitespace


def execute(name, command, cwd, env, output_dir, timeout, require_empty=False):
    log = output_dir / (name + ".log")
    entry = {"name": name, "command": [str(x) for x in command],
             "cwd": str(cwd), "log": str(log)}
    with log.open("wb") as stream:
        try:
            child = subprocess.Popen(command, cwd=cwd, env=env, stdout=stream,
                                     stderr=subprocess.STDOUT, start_new_session=True)
            try:
                code = child.wait(timeout=timeout)
            except subprocess.TimeoutExpired:
                os.killpg(child.pid, signal.SIGTERM)
                try:
                    child.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    os.killpg(child.pid, signal.SIGKILL)
                    child.wait()
                entry.update(status="blocked", reason="timeout")
                return entry
        except OSError:
            entry.update(status="blocked", reason="tool unavailable or cannot start")
            return entry
    entry.update(returncode=code, status="passed" if code == 0 else "failed")
    if require_empty and code == 0 and log.stat().st_size:
        entry.update(status="failed", reason="gofmt reported unformatted files")
    return entry


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    modes = parser.add_mutually_exclusive_group()
    modes.add_argument("--base", help="review merge-base(BASE, HEAD)..HEAD")
    modes.add_argument("--commit", help="review a commit against its first parent")
    modes.add_argument("--staged", action="store_true")
    modes.add_argument("--worktree", action="store_true", help="default: HEAD + local and new files")
    parser.add_argument("--head", default="HEAD", help="target ref, only with --base")
    parser.add_argument("--repo", type=Path, default=Path.cwd())
    parser.add_argument("--tests", action="store_true")
    parser.add_argument("--build", action="store_true", help="include Next.js production build")
    parser.add_argument("--integration", action="store_true",
                        help="include Go tests with explicitly provided TEST_DATABASE_URL")
    parser.add_argument("--timeout", type=int, default=300, help="seconds per command")
    args = parser.parse_args()
    if args.timeout < 1 or (args.head != "HEAD" and not args.base):
        parser.error("timeout must be positive; --head requires --base")
    output_dir = Path(tempfile.mkdtemp(prefix="uptime-review-"))
    report = {"checks": [], "output_dir": str(output_dir)}

    def record(entry):
        report["checks"].append(entry)
        print(entry["status"] + ": " + entry["name"], flush=True)

    try:
        root = Path(os.fsdecode(git(args.repo, "rev-parse", "--show-toplevel")).strip())
        snapshot, whitespace = scope(root, args)
        report["scope"] = snapshot
        before = fingerprint(root)
        report["snapshot_fingerprint"] = before
        backend = any(p.startswith("backend/") for p in snapshot["files"])
        frontend = any(p.startswith("frontend/") for p in snapshot["files"])
        report["projects"] = {"backend": backend, "frontend": frontend}
        env = os.environ.copy()
        # Use the local toolchain and cached modules; never change go.mod/go.sum.
        env.update(GOTOOLCHAIN="local", GOPROXY="off")
        env["GOFLAGS"] = (env.get("GOFLAGS", "") + " -mod=readonly").strip()
        if not args.integration:
            env.pop("TEST_DATABASE_URL", None)

        def run(name, command, cwd=root, require_empty=False):
            entry = execute(name, command, cwd, env, output_dir, args.timeout, require_empty)
            record(entry)
            return entry

        # All static checks run before tests/build, and failures do not hide later checks.
        run("diff-whitespace", whitespace)
        if backend:
            go_files = sorted("./" + str(p.relative_to(root / "backend"))
                              for p in (root / "backend").rglob("*.go")
                              if "vendor" not in p.relative_to(root / "backend").parts)
            if go_files:
                run("go-format", ["gofmt", "-l", *go_files], root / "backend", True)
            else:
                record({"name": "go-format", "status": "blocked", "reason": "no Go source files"})
            run("go-vet", ["go", "vet", "./..."], root / "backend")
        else:
            record({"name": "backend", "status": "not_applicable"})
        if frontend:
            bin_dir = root / "frontend/node_modules/.bin"
            typegen_ready = False
            for name, binary, command in (
                ("eslint", "eslint", ["npm", "run", "lint"]),
                ("next-typegen", "next", [str(bin_dir / "next"), "typegen"]),
                ("typescript", "tsc", [str(bin_dir / "tsc"), "--noEmit", "--incremental", "false"]),
            ):
                if name == "typescript" and not typegen_ready:
                    record({"name": name, "status": "blocked", "reason": "Next.js route type generation did not pass"})
                elif not (bin_dir / binary).is_file():
                    record({"name": name, "status": "blocked", "reason": "local dependency missing"})
                else:
                    entry = run(name, command, root / "frontend")
                    if name == "next-typegen":
                        typegen_ready = entry["status"] == "passed"
        else:
            record({"name": "frontend", "status": "not_applicable"})

        if backend:
            if args.integration and not env.get("TEST_DATABASE_URL"):
                record({"name": "postgres-integration", "status": "blocked",
                        "reason": "--integration requires TEST_DATABASE_URL for an approved test DB"})
            elif args.tests or args.integration:
                run("go-tests", ["go", "test", "-count=1", "-timeout=120s", "-cover", "./..."], root / "backend")
            else:
                record({"name": "go-tests", "status": "skipped", "reason": "use --tests"})
            if not args.integration:
                record({"name": "postgres-integration", "status": "skipped",
                        "reason": "use --integration with an approved test DB"})
        if frontend:
            for name, enabled, binary, command in (
                ("frontend-tests", args.tests, "vitest", ["npm", "test"]),
                ("next-build", args.build, "next", ["npm", "run", "build"]),
            ):
                if not enabled:
                    record({"name": name, "status": "skipped", "reason": "flag not requested"})
                elif not (root / "frontend/node_modules/.bin" / binary).is_file():
                    record({"name": name, "status": "blocked", "reason": "local dependency missing"})
                else:
                    run(name, command, root / "frontend")
        if fingerprint(root) != before:
            record({"name": "checkout-stability", "status": "blocked",
                    "reason": "Checkout content or index changed during checks; inspect without discarding user work"})
    except (OSError, ValueError) as exc:
        record({"name": "scope", "status": "blocked", "reason": str(exc)})

    statuses = {check["status"] for check in report["checks"]}
    code = 2 if "blocked" in statuses else (1 if "failed" in statuses else 0)
    report["exit_code"] = code
    report_path = output_dir / "report.json"
    report_path.write_text(json.dumps(report, ensure_ascii=True, indent=2) + "\n")
    print("Report: " + str(report_path))
    return code


if __name__ == "__main__":
    sys.exit(main())
