"""PreToolUse guard for migrations present in the current HEAD."""

import fnmatch
import json
import os
from pathlib import Path
import re
import shlex
import subprocess
import sys


ROOT = Path(__file__).resolve().parents[2]
READ_COMMANDS = {"cat", "head", "tail", "less", "ls", "stat", "wc", "rg", "grep", "find", "diff", "pwd"}
READ_GIT_COMMANDS = {"status", "diff", "show", "log", "ls-files", "ls-tree", "rev-parse"}


def deny(reason):
    print(json.dumps({"hookSpecificOutput": {
        "hookEventName": "PreToolUse",
        "permissionDecision": "deny",
        "permissionDecisionReason": reason,
    }}, ensure_ascii=False))


def git(*args):
    return subprocess.run(
        ["git", "-C", str(ROOT), *args], capture_output=True, check=True, timeout=5,
    ).stdout


def committed_paths():
    # An unborn branch has no committed files. Other Git failures must block.
    head = subprocess.run(
        ["git", "-C", str(ROOT), "rev-parse", "--verify", "--quiet", "HEAD"],
        capture_output=True, timeout=5,
    )
    if head.returncode == 1:
        return []
    head.check_returncode()
    return [ROOT / os.fsdecode(path) for path in git(
        "ls-tree", "-r", "--name-only", "-z", "HEAD", "--", "backend/migrations/",
    ).split(b"\0") if path]


def overlaps(path, committed, cwd, include_parents=False):
    target = Path(os.path.abspath(cwd / path))
    # Check both the spelling and the resolved path, including symlink aliases.
    for candidate in (target, target.resolve()):
        for protected in committed:
            if candidate == protected or (include_parents and candidate in protected.parents):
                return True
    return False


def edited_paths(event):
    data = event["tool_input"]
    if event["tool_name"] == "apply_patch":
        patch = data if isinstance(data, str) else data.get("command")
        if not isinstance(patch, str):
            raise ValueError("В событии apply_patch отсутствует текст патча.")
        # Add File can overwrite an existing path; Move to protects both ends.
        return re.findall(r"^\*\*\* (?:Add File|Update File|Delete File|Move to): (.+?)\r?$", patch, re.M)
    path = data.get("file_path")
    if not isinstance(path, str):
        raise ValueError("В событии редактирования отсутствует file_path.")
    return [path]


def shell_touches_migrations(data, committed, cwd):
    command = data.get("command", data.get("cmd"))
    if isinstance(command, list):
        command = " ".join(command)
    if not isinstance(command, str):
        raise ValueError("В событии Bash отсутствует команда.")
    lexer = shlex.shlex(command, posix=True, punctuation_chars=";&|()<>")
    lexer.whitespace_split = True
    lexer.commenters = ""
    tokens = list(lexer)

    # Allow ordinary inspection. find -delete/-exec and sed can mutate files.
    commands = re.split(r"\n|[;&|]+", command)
    read_only = True
    for part in commands:
        words = shlex.split(part)
        if not words:
            continue
        name = Path(words[0]).name
        safe = name in READ_COMMANDS
        if name == "git":
            safe = len(words) > 1 and words[1] in READ_GIT_COMMANDS
        if name == "find" and any(w.startswith(("-delete", "-exec", "-ok", "-fprint", "-fls")) for w in words[1:]):
            safe = False
        if name == "rg" and any(w.startswith("--pre") for w in words[1:]):
            safe = False
        read_only = read_only and safe
    if read_only and not any(re.search(r"[<>$`()]", t) for t in tokens):
        return False

    # Shell inspection is deliberately conservative for explicit paths and
    # directory-wide operations. It does not interpret arbitrary program code.
    directories = [cwd]
    cd_arguments = set()
    for i, token in enumerate(tokens[:-1]):
        if token == "cd":
            directories.append((directories[-1] / tokens[i + 1]).resolve())
            cd_arguments.update((i, i + 1))
    for directory in directories:
        for i, token in enumerate(tokens):
            if i in cd_arguments:
                continue
            # Cover assignments, --file=path, globs, and Git pathspecs.
            path = token.rsplit("=", 1)[-1].removeprefix(":(literal)")
            patterns = (os.path.abspath(directory / path), str((directory / path).resolve()))
            if any(fnmatch.fnmatchcase(str(protected), pattern) for protected in committed for pattern in patterns):
                return True
            path = re.split(r"[*?\[]", path, maxsplit=1)[0]
            if path and overlaps(path, committed, directory, include_parents=True):
                return True
    return False


def main():
    event = json.load(sys.stdin)
    committed = committed_paths()
    if not committed:
        return
    data = event["tool_input"]
    cwd = Path(event.get("cwd") or ROOT)
    if isinstance(data, dict):
        cwd = Path(data.get("workdir") or data.get("cwd") or cwd)
    if event["tool_name"] == "Bash":
        blocked = shell_touches_migrations(data, committed, cwd)
    else:
        blocked = any(overlaps(path, committed, cwd) for path in edited_paths(event))
    if blocked:
        deny("Миграции в backend/migrations/, уже присутствующие в HEAD, нельзя изменять, удалять или переименовывать. Создай новую миграцию.")


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        deny(f"Не удалось проверить защиту миграций: {error}")
