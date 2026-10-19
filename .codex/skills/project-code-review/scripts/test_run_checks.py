"""Behavior tests in disposable Git repos, with fake language tools and no network."""

import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest


RUNNER = Path(__file__).with_name("run_checks.py")
GIT = shutil.which("git")


class ChecksTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="review-runner-test-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name) / "repo"
        self.root.mkdir()
        self.git("init", "-q")
        self.git("config", "user.email", "review@example.invalid")
        self.git("config", "user.name", "Review fixture")
        self.write(".gitignore", "frontend/node_modules/\n")
        self.write("backend/main.go", "package main\nfunc main() {}\n")
        self.write("frontend/src/page.tsx", "export default function Page() { return null; }\n")
        self.write("docs/guide.md", "Guide\n")
        self.commit()
        self.bin = Path(self.temp.name) / "bin"
        self.bin.mkdir()
        self.events = Path(self.temp.name) / "events.jsonl"
        self.env = dict(os.environ, PATH=str(self.bin), EVENTS=str(self.events))
        (self.bin / "git").symlink_to(GIT)
        tool = ("#!" + sys.executable + "\n"
                "import json, os, pathlib, sys, time\n"
                "name = pathlib.Path(sys.argv[0]).name\n"
                "with open(os.environ['EVENTS'], 'a') as f:\n"
                " f.write(json.dumps({'tool': name, 'args': sys.argv[1:], "
                "'database': 'TEST_DATABASE_URL' in os.environ}) + '\\n')\n"
                "if os.environ.get('HANG_TOOL') == name: time.sleep(30)\n"
                "if name == 'npm' and os.environ.get('MUTATE_SOURCE'):\n"
                " pathlib.Path('src/page.tsx').write_text('// rewritten by a project command\\n')\n"
                "if name == 'gofmt' and os.environ.get('UNFORMATTED'): print('main.go')\n"
                "sys.exit(1 if os.environ.get('FAIL_TOOL') == name else 0)\n")
        for name in ("gofmt", "go", "npm"):
            self.install(self.bin / name, tool)
        for name in ("eslint", "tsc", "vitest", "next"):
            self.install(self.root / "frontend/node_modules/.bin" / name, tool)

    def install(self, path, source):
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(source)
        path.chmod(0o755)

    def write(self, path, content):
        target = self.root / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(content)

    def git(self, *args):
        return subprocess.check_output([GIT, "-C", str(self.root), *args], stderr=subprocess.STDOUT)

    def commit(self):
        self.git("add", ".")
        self.git("commit", "-qm", "fixture")
        return self.git("rev-parse", "HEAD").decode().strip()

    def run_checks(self, *args):
        result = subprocess.run([sys.executable, "-B", str(RUNNER), "--repo", str(self.root), *args],
                                env=self.env, capture_output=True, text=True, timeout=15)
        report_path = Path(result.stdout.split("Report: ")[-1].strip())
        report = json.loads(report_path.read_text())
        self.addCleanup(shutil.rmtree, report_path.parent)
        self.assertEqual(result.returncode, report["exit_code"], result.stderr)
        return result.returncode, report

    def calls(self):
        return [json.loads(line) for line in self.events.read_text().splitlines()] if self.events.exists() else []

    def test_docs_do_not_run_apps(self):
        self.write("docs/guide.md", "New guide\n")
        code, report = self.run_checks("--tests", "--build")
        self.assertEqual(code, 0)
        self.assertEqual(self.calls(), [])
        self.assertEqual(report["projects"], {"backend": False, "frontend": False})

    def test_backend_untracked_and_failure_aggregation(self):
        self.write("backend/new name.go", "package main\n")
        self.env["UNFORMATTED"] = "yes"
        code, report = self.run_checks("--tests")
        self.assertEqual(code, 1)
        self.assertIn("backend/new name.go", report["scope"]["files"])
        self.assertEqual([c["tool"] for c in self.calls()], ["gofmt", "go", "go"])
        self.assertIn("test", self.calls()[-1]["args"])
        self.assertEqual(next(c for c in report["checks"] if c["name"] == "go-format")["status"], "failed")

    def test_frontend_only_and_static_before_tests_build(self):
        self.write("frontend/src/page.tsx", "export default function Page() { return 1; }\n")
        code, _ = self.run_checks("--tests", "--build")
        self.assertEqual(code, 0)
        self.assertEqual([(c["tool"], c["args"][0]) for c in self.calls()],
                         [("npm", "run"), ("next", "typegen"), ("tsc", "--noEmit"), ("npm", "test"), ("npm", "run")])

    def test_rename_across_projects_routes_both(self):
        self.git("mv", "backend/main.go", "frontend/old-main.go")
        code, report = self.run_checks()
        self.assertEqual(code, 2)  # No Go files remain: formatting cannot run.
        self.assertEqual(report["projects"], {"backend": True, "frontend": True})
        self.assertIn("backend/main.go", report["scope"]["files"])
        self.assertIn("frontend/old-main.go", report["scope"]["files"])

    def test_missing_dependency_is_blocked_but_others_run(self):
        self.write("frontend/src/page.tsx", "// changed\n")
        (self.root / "frontend/node_modules/.bin/eslint").unlink()
        code, report = self.run_checks()
        self.assertEqual(code, 2)
        self.assertEqual([c["tool"] for c in self.calls()], ["next", "tsc"])
        self.assertEqual(next(c for c in report["checks"] if c["name"] == "eslint")["status"], "blocked")

    def test_root_commit_and_historical_checkout_guard(self):
        root_sha = self.git("rev-parse", "HEAD").decode().strip()
        code, report = self.run_checks("--commit", root_sha)
        self.assertEqual(code, 0)
        self.assertIsNone(report["scope"]["base"])
        self.assertIn("backend/main.go", report["scope"]["files"])
        self.write("docs/guide.md", "Second\n")
        self.commit()
        calls_before = len(self.calls())
        code, _ = self.run_checks("--commit", root_sha)
        self.assertEqual(code, 2)
        self.assertEqual(len(self.calls()), calls_before)

    def test_merge_base_excludes_changes_only_on_base(self):
        initial = self.git("rev-parse", "HEAD").decode().strip()
        self.git("checkout", "-qb", "base")
        self.write("backend/main.go", "package main\n// only on base\n")
        self.commit()
        self.git("checkout", "-qb", "topic", initial)
        self.write("frontend/src/page.tsx", "// topic\n")
        self.commit()
        code, report = self.run_checks("--base", "base")
        self.assertEqual(code, 0)
        self.assertEqual(report["scope"]["base"], initial)
        self.assertEqual(report["projects"], {"backend": False, "frontend": True})

    def test_staged_rejects_other_local_content(self):
        self.write("backend/main.go", "package main\n// staged\n")
        self.git("add", "backend/main.go")
        self.write("frontend/new.ts", "// untracked\n")
        code, _ = self.run_checks("--staged")
        self.assertEqual(code, 2)
        self.assertEqual(self.calls(), [])
        (self.root / "frontend/new.ts").unlink()
        code, report = self.run_checks("--staged")
        self.assertEqual(code, 0)
        self.assertEqual(report["scope"]["files"], ["backend/main.go"])

    def test_database_requires_explicit_integration(self):
        self.write("backend/main.go", "package main\n// changed\n")
        self.env["TEST_DATABASE_URL"] = "not-a-real-database"
        self.run_checks("--tests")
        self.assertTrue(all(not call["database"] for call in self.calls()))
        self.events.unlink()
        code, _ = self.run_checks("--integration")
        self.assertEqual(code, 0)
        self.assertTrue(all(call["database"] for call in self.calls()))
        self.assertIn("test", self.calls()[-1]["args"])

    def test_integration_without_database_is_blocked(self):
        self.env.pop("TEST_DATABASE_URL", None)
        self.write("backend/main.go", "package main\n// changed\n")
        code, report = self.run_checks("--integration")
        self.assertEqual(code, 2)
        self.assertFalse(any("test" in c["args"] for c in self.calls()))
        self.assertEqual(next(c for c in report["checks"] if c["name"] == "postgres-integration")["status"], "blocked")

    def test_timeout_does_not_hide_later_checks(self):
        self.write("backend/main.go", "package main\n// changed\n")
        self.env["HANG_TOOL"] = "gofmt"
        code, report = self.run_checks("--timeout", "1")
        self.assertEqual(code, 2)
        self.assertEqual([c["tool"] for c in self.calls()], ["gofmt", "go"])
        self.assertEqual(next(c for c in report["checks"] if c["name"] == "go-format")["reason"], "timeout")

    def test_dirty_target_and_invalid_ref_are_blocked(self):
        self.write("docs/guide.md", "Dirty\n")
        code, _ = self.run_checks("--commit", "HEAD")
        self.assertEqual(code, 2)
        code, _ = self.run_checks("--base", "missing-ref")
        self.assertEqual(code, 2)
        self.assertEqual(self.calls(), [])

    def test_detects_changes_to_already_modified_source(self):
        self.write("frontend/src/page.tsx", "// already modified\n")
        self.env["MUTATE_SOURCE"] = "yes"
        code, report = self.run_checks()
        self.assertEqual(code, 2)
        self.assertEqual(next(c for c in report["checks"] if c["name"] == "checkout-stability")["status"], "blocked")

    def test_missing_executable_does_not_hide_vet(self):
        self.write("backend/main.go", "package main\n// changed\n")
        (self.bin / "gofmt").unlink()
        code, report = self.run_checks()
        self.assertEqual(code, 2)
        self.assertEqual([c["tool"] for c in self.calls()], ["go"])
        self.assertEqual(next(c for c in report["checks"] if c["name"] == "go-format")["status"], "blocked")

    def test_route_type_failure_blocks_tsc_but_not_tests(self):
        self.write("frontend/src/page.tsx", "// changed\n")
        self.env["FAIL_TOOL"] = "next"
        code, report = self.run_checks("--tests")
        self.assertEqual(code, 2)
        self.assertNotIn("tsc", [c["tool"] for c in self.calls()])
        self.assertEqual(self.calls()[-1]["args"], ["test"])
        self.assertEqual(next(c for c in report["checks"] if c["name"] == "next-typegen")["status"], "failed")

    def test_staged_plus_unstaged_and_deletion_are_in_local_scope(self):
        self.write("docs/guide.md", "// staged\n")
        self.git("add", "docs/guide.md")
        self.write("backend/main.go", "package main\n// unstaged\n")
        (self.root / "frontend/src/page.tsx").unlink()
        code, report = self.run_checks()
        self.assertEqual(code, 0)
        self.assertEqual(report["scope"]["files"], ["backend/main.go", "docs/guide.md", "frontend/src/page.tsx"])


if __name__ == "__main__":
    unittest.main()
