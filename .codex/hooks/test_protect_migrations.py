import json
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


SOURCE = Path(__file__).with_name("protect-migrations.py")


class MigrationGuardTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()
        self.hook = self.root / ".codex/hooks/protect-migrations.py"
        self.hook.parent.mkdir(parents=True)
        shutil.copyfile(SOURCE, self.hook)
        self.old = "backend/migrations/000001_old.up.sql"
        self.new = "backend/migrations/000002_new.up.sql"
        (self.root / self.old).parent.mkdir(parents=True)
        (self.root / self.old).write_text("SELECT 1;\n")
        self.git("init", "-q")
        self.git("add", self.old)
        self.git("-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "initial")
        (self.root / self.new).write_text("SELECT 2;\n")

    def git(self, *args):
        return subprocess.run(["git", "-C", str(self.root), *args], check=True, capture_output=True)

    def blocked(self, tool, data, cwd=None):
        result = subprocess.run(
            ["python3", str(self.hook)],
            input=json.dumps({"tool_name": tool, "tool_input": data, "cwd": str(cwd or self.root)}),
            text=True, capture_output=True, check=True,
        )
        if not result.stdout:
            return False
        output = json.loads(result.stdout)["hookSpecificOutput"]
        self.assertEqual(output["hookEventName"], "PreToolUse")
        self.assertEqual(output["permissionDecision"], "deny")
        self.assertTrue(output["permissionDecisionReason"])
        return True

    def test_committed_patch_operations_are_blocked(self):
        for operation in ("Update File", "Delete File", "Add File", "Move to"):
            with self.subTest(operation=operation):
                patch = f"*** Begin Patch\n*** {operation}: {self.old}\n*** End Patch"
                self.assertTrue(self.blocked("apply_patch", {"command": patch}))

    def test_move_source_and_destination_are_protected(self):
        for source, target in ((self.old, self.new), (self.new, self.old)):
            patch = f"*** Begin Patch\n*** Update File: {source}\n*** Move to: {target}\n*** End Patch"
            self.assertTrue(self.blocked("apply_patch", patch))

    def test_new_staged_and_unrelated_files_are_allowed(self):
        self.git("add", self.new)
        for path in (self.new, "backend/main.go", "frontend/app/page.tsx"):
            for tool in ("Edit", "Write"):
                with self.subTest(path=path, tool=tool):
                    self.assertFalse(self.blocked(tool, {"file_path": path}))
        patch = f"*** Begin Patch\n*** Add File: {self.new}\n+SELECT 2;\n*** End Patch"
        self.assertFalse(self.blocked("apply_patch", {"command": patch}))

    def test_absolute_relative_and_symlink_paths_are_protected(self):
        (self.root / "alias").symlink_to(self.root / "backend/migrations", target_is_directory=True)
        for path in (str(self.root / self.old), "alias/000001_old.up.sql", "backend/../" + self.old):
            self.assertTrue(self.blocked("Write", {"file_path": path}))
        self.assertTrue(self.blocked("Edit", {"file_path": "migrations/000001_old.up.sql"}, self.root / "backend"))

    def test_staged_deletion_remains_protected(self):
        self.git("rm", self.old)
        self.assertTrue(self.blocked("Write", {"file_path": self.old}))

    def test_new_file_is_protected_after_commit(self):
        self.git("add", self.new)
        self.git("-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "next")
        self.assertTrue(self.blocked("Edit", {"file_path": self.new}))

    def test_shell_mutations_are_blocked(self):
        commands = (
            f"rm '{self.old}'", f"sed -i '' 's/1/2/' {self.old}",
            f"echo changed > {self.old}", f"mv {self.old} /tmp/old.sql",
            "rm -rf backend/migrations", "rm -rf backend", "rm backend/migrations/000001*",
            "find backend/migrations -type f -delete", "cd backend && rm migrations/*.sql",
            f"cat $(echo {self.old}) > {self.old}",
        )
        for command in commands:
            with self.subTest(command=command):
                self.assertTrue(self.blocked("Bash", {"command": command}))
        self.assertTrue(self.blocked("Bash", {"cmd": "rm *.sql", "workdir": str(self.root / "backend/migrations")}))

    def test_shell_inspection_and_new_files_are_allowed(self):
        for command in (f"cat {self.old}", "ls backend/migrations", "git diff -- backend/migrations", "rg SELECT backend/migrations", f"rm {self.new}", f"echo new > {self.new}", "npm test"):
            with self.subTest(command=command):
                self.assertFalse(self.blocked("Bash", {"command": command}))
        self.assertFalse(self.blocked("Bash", {"cmd": "rm 000002_new.up.sql", "workdir": str(self.root / "backend/migrations")}))
        self.assertFalse(self.blocked("Bash", {"command": "cd backend/migrations && rm 000002_new.up.sql"}))

    def test_invalid_input_and_git_failures_deny(self):
        self.assertTrue(self.blocked("apply_patch", {}))
        shutil.rmtree(self.root / ".git")
        self.assertTrue(self.blocked("Edit", {"file_path": self.old}))

    def test_unborn_repository_allows_new_migrations(self):
        shutil.rmtree(self.root / ".git")
        self.git("init", "-q")
        self.assertFalse(self.blocked("Edit", {"file_path": self.old}))


if __name__ == "__main__":
    unittest.main()
