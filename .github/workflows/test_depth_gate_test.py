#!/usr/bin/env python3
"""Mutation proofs for the Film/TV test-depth gate."""

from __future__ import annotations

import tempfile
import unittest
from pathlib import Path
from unittest import mock

import test_depth_gate as gate


class TestDepthGate(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.workflow = gate.WORKFLOW.read_text(encoding="utf-8")
        cls.gate_script = gate.GATE_SCRIPT.read_text(encoding="utf-8")

    def workflow_errors(
        self,
        workflow: str | None = None,
        discovered: set[tuple[str, str]] | None = None,
    ) -> list[str]:
        return gate.workflow_errors(
            workflow or self.workflow,
            gate.REQUIRED_FUZZ_PAIRS if discovered is None else discovered,
        )

    def assert_workflow_rejected(
        self,
        workflow: str,
        fragment: str | None = None,
    ) -> None:
        errors = self.workflow_errors(workflow)
        self.assertTrue(errors)
        if fragment is not None:
            self.assertTrue(any(fragment in error for error in errors), errors)

    def job(self, name: str) -> str:
        matches = gate.jobs(self.workflow).get(name, [])
        self.assertEqual(len(matches), 1)
        return matches[0]

    def mutate_job(self, name: str, old: str, new: str) -> str:
        job = self.job(name)
        self.assertIn(old, job)
        return self.workflow.replace(job, job.replace(old, new, 1), 1)

    def mutate_step(self, job_name: str, step_name: str, mode: str) -> str:
        job = self.job(job_name)
        matches = gate.named_steps(job, step_name)
        self.assertEqual(len(matches), 1)
        step = matches[0]
        header = f"      - name: {step_name}\n"
        self.assertTrue(step.startswith(header))
        if mode == "skip":
            mutated = step.replace(header, header + "        if: ${{ false }}\n", 1)
        elif mode == "continue":
            mutated = step.replace(header, header + "        continue-on-error: true\n", 1)
        elif mode == "comment":
            mutated = step.replace(header, f"      # - name: {step_name}\n", 1)
        elif mode == "delete":
            mutated = ""
        elif mode == "duplicate":
            mutated = step + step
        else:
            raise AssertionError(f"unknown mutation mode {mode}")
        return self.workflow.replace(job, job.replace(step, mutated, 1), 1)

    def mutate_gate(self, old: str, new: str) -> str:
        self.assertEqual(self.gate_script.count(old), 1)
        return self.gate_script.replace(old, new, 1)

    def assert_gate_rejected(self, script: str, fragment: str) -> None:
        errors = gate.gate_errors(script)
        self.assertTrue(any(fragment in error for error in errors), errors)

    def test_live_contract(self) -> None:
        self.assertEqual(self.workflow_errors(), [])
        self.assertEqual(gate.gate_errors(self.gate_script), [])

    def test_aggregate_coverage_cannot_mask_a_weak_package(self) -> None:
        floors = {"example/high": 80.0, "example/low": 80.0}
        measured = {"example/high": 100.0, "example/low": 70.0}
        self.assertGreater(sum(measured.values()) / len(measured), 80.0)
        self.assertIn(
            "example/low coverage 70.0% is below 80.0%",
            gate.coverage_errors(floors, set(floors), measured),
        )

    def test_package_and_fuzz_discovery_fail_closed(self) -> None:
        self.assertIn(
            "discovered package has no coverage floor: example/new",
            gate.coverage_errors(
                {"example/configured": 80.0},
                {"example/configured", "example/new"},
                {"example/configured": 100.0, "example/new": 100.0},
            ),
        )
        missing = set(gate.REQUIRED_FUZZ_PAIRS)
        missing.remove((".", "FuzzParseFilename"))
        self.assertTrue(
            any("required fuzz target was not discovered" in error for error in self.workflow_errors(discovered=missing))
        )
        extra = set(gate.REQUIRED_FUZZ_PAIRS)
        extra.add(("./internal/foreignimport", "FuzzUnexpected"))
        self.assertTrue(
            any("discovered fuzz target is not scheduled" in error for error in self.workflow_errors(discovered=extra))
        )

    def test_critical_stages_reject_skip_comment_continue_delete_and_duplicate(self) -> None:
        stages = (
            ("fast-conformance", "gofmt"),
            ("fast-conformance", "vet"),
            ("fast-conformance", "test contracts and per-package coverage"),
            ("fast-conformance", "govulncheck"),
            ("fast-conformance", "golangci-lint"),
            ("race", "race"),
            ("conformance", "Require fast and race lanes"),
            ("fuzz", "bounded parser fuzz"),
        )
        for job, step in stages:
            for mode in ("skip", "comment", "continue", "delete", "duplicate"):
                with self.subTest(job=job, step=step, mode=mode):
                    self.assert_workflow_rejected(self.mutate_step(job, step, mode))

    def test_govulncheck_rejects_mutable_selector(self) -> None:
        pinned = "go -C tools tool govulncheck -C .. ./..."
        mutable = "go run golang.org/x/vuln/cmd/govulncheck@latest ./..."
        self.assertEqual(self.workflow.count(pinned), 1)
        self.assert_workflow_rejected(self.workflow.replace(pinned, mutable, 1))

    def test_jobs_cannot_be_conditional_nonblocking_commented_or_duplicated(self) -> None:
        for job in ("fast-conformance", "race", "conformance", "fuzz"):
            with self.subTest(job=job, mode="continue"):
                self.assert_workflow_rejected(
                    self.mutate_job(job, f"  {job}:\n", f"  {job}:\n    continue-on-error: true\n")
                )
            with self.subTest(job=job, mode="comment"):
                self.assert_workflow_rejected(
                    self.workflow.replace(f"  {job}:\n", f"  # {job}:\n", 1),
                    f"job {job} missing or duplicated",
                )
            with self.subTest(job=job, mode="duplicate"):
                block = self.job(job)
                self.assert_workflow_rejected(
                    self.workflow.replace(block, block + "\n" + block, 1),
                )
        self.assert_workflow_rejected(
            self.mutate_job("fast-conformance", "    runs-on:", "    if: ${{ false }}\n    runs-on:")
        )
        self.assert_workflow_rejected(
            self.mutate_job("race", "    runs-on:", "    if: ${{ false }}\n    runs-on:")
        )
        self.assert_workflow_rejected(
            self.mutate_job("conformance", "    if: always()", "    if: false")
        )
        self.assert_workflow_rejected(
            self.mutate_job("fuzz", f"    if: {gate.FUZZ_IF}", "    if: false")
        )

    def test_aggregator_is_attached_and_fails_closed(self) -> None:
        self.assert_workflow_rejected(
            self.mutate_job("conformance", "    needs: [fast-conformance, race]", "    needs: [fast-conformance]"),
            "must need fast-conformance and race",
        )
        self.assert_workflow_rejected(
            self.mutate_job(
                "conformance",
                '          test "$RACE_RESULT" = success',
                '          test -n "$RACE_RESULT"',
            ),
            "command changed or is inactive",
        )
        self.assert_workflow_rejected(
            self.mutate_job("conformance", "    name: conformance", "    name: optional"),
            "preserved conformance check name changed",
        )

    def test_sdk_override_parity_is_required(self) -> None:
        for job in ("fast-conformance", "race", "fuzz"):
            for step in ("Checkout SDK override", "Use SDK override"):
                with self.subTest(job=job, step=step):
                    self.assert_workflow_rejected(self.mutate_step(job, step, "delete"))

    def test_direct_and_wrapper_duplicate_test_stages_are_rejected(self) -> None:
        anchor = "      - name: golangci-lint\n"
        for command in (
            "go test ./...",
            "script/gate.sh",
            "script/gate.sh gate",
            gate.CHECK_COMMAND,
        ):
            with self.subTest(command=command):
                extra = (
                    "      - name: duplicate test stage\n"
                    f"        working-directory: {gate.PLUGIN_DIRECTORY}\n"
                    f"        run: {command}\n"
                )
                self.assert_workflow_rejected(
                    self.mutate_job("fast-conformance", anchor, extra + anchor)
                )

    def test_structural_workflow_rejects_quoted_wrapped_flow_and_extra_steps(self) -> None:
        anchor = "      - name: golangci-lint\n"
        steps = {
            "quoted-run": (
                "      - name: duplicate quoted test\n"
                f"        working-directory: {gate.PLUGIN_DIRECTORY}\n"
                '        run: "go test ./..."\n'
            ),
            "wrapped-run": (
                "      - name: duplicate wrapped test\n"
                '        run: bash -c "go test ./..."\n'
            ),
            "quoted-keys": (
                '      - "name": duplicate quoted keys\n'
                '        "run": "go test ./..."\n'
            ),
            "flow-step": (
                '      - {"name": duplicate flow test, "run": "go test ./..."}\n'
            ),
            "conditional": (
                "      - name: conditional duplicate test\n"
                "        if: false\n"
                "        run: go test ./...\n"
            ),
            "nonblocking": (
                "      - name: nonblocking duplicate test\n"
                "        continue-on-error: true\n"
                "        run: go test ./...\n"
            ),
        }
        for name, step in steps.items():
            with self.subTest(name=name):
                self.assert_workflow_rejected(
                    self.mutate_job("fast-conformance", anchor, step + anchor),
                    "ordered steps changed",
                )

    def test_structural_workflow_rejects_extra_job_key_forms(self) -> None:
        jobs = {
            "block": (
                "\n  duplicate-tests:\n"
                "    runs-on: ubuntu-latest\n"
                "    steps:\n"
                "      - name: duplicate\n"
                "        run: go test ./...\n"
            ),
            "quoted-flow": (
                '\n  "duplicate-tests": {"runs-on": ubuntu-latest, "steps": '
                '[{"name": duplicate, "run": "go test ./..."}]}\n'
            ),
            "conditional": (
                "\n  duplicate-tests:\n"
                "    if: false\n"
                "    runs-on: ubuntu-latest\n"
                "    steps:\n"
                "      - name: duplicate\n"
                "        run: go test ./...\n"
            ),
            "nonblocking": (
                "\n  duplicate-tests:\n"
                "    continue-on-error: true\n"
                "    runs-on: ubuntu-latest\n"
                "    steps:\n"
                "      - name: duplicate\n"
                "        run: go test ./...\n"
            ),
        }
        for name, job in jobs.items():
            with self.subTest(name=name):
                self.assert_workflow_rejected(
                    self.workflow + job,
                    "workflow job set or order changed",
                )

    def test_structural_workflow_rejects_duplicate_job_step_and_run_keys(self) -> None:
        duplicate_job = (
            "\n  race: {runs-on: ubuntu-latest, steps: "
            '[{"name": duplicate, "run": "go test ./..."}]}\n'
        )
        self.assert_workflow_rejected(
            self.workflow + duplicate_job,
            "duplicate mapping key 'race'",
        )
        self.assert_workflow_rejected(
            self.mutate_job(
                "fast-conformance",
                f"        run: {gate.CHECK_COMMAND}\n",
                (
                    f"        run: {gate.CHECK_COMMAND}\n"
                    '        "run": "go test ./..."\n'
                ),
            ),
            "duplicate mapping key 'run'",
        )
        self.assert_workflow_rejected(
            self.mutate_step("race", "race", "duplicate"),
            "ordered steps changed",
        )

    def test_structural_workflow_parser_has_no_pyyaml_dependency(self) -> None:
        original_import = __import__

        def import_without_yaml(name: str, *args: object, **kwargs: object) -> object:
            if name == "yaml" or name.startswith("yaml."):
                raise ModuleNotFoundError(name)
            return original_import(name, *args, **kwargs)

        with mock.patch("builtins.__import__", side_effect=import_without_yaml):
            self.assertEqual(gate.workflow_structure_errors(self.workflow), [])

    def test_fuzz_schedule_budget_matrix_and_local_dispatch_are_pinned(self) -> None:
        self.assert_workflow_rejected(
            self.mutate_job("fuzz", "    timeout-minutes: 12", "    timeout-minutes: 0")
        )
        self.assert_workflow_rejected(
            self.mutate_job("fuzz", "          FUZZ_TIME: 20s", "          FUZZ_TIME: 0s")
        )
        self.assert_workflow_rejected(
            self.mutate_job(
                "fuzz",
                "          - package: .\n            target: FuzzParseFilename\n",
                "",
            )
        )
        self.assertTrue(
            gate.gate_errors(
                self.gate_script.replace('fuzz_time="${FUZZ_TIME:-20s}"', 'fuzz_time="${FUZZ_TIME:-0s}"')
            )
        )
        self.assertTrue(
            gate.gate_errors(
                self.gate_script.replace("go test -race -count=1 ./...", "go test ./...")
            )
        )

    def test_local_race_rejects_nonblocking_conditional_and_duplicate_mutants(self) -> None:
        command = "    go test -race -count=1 ./..."
        mutants = {
            "or-true": f"{command} || true",
            "if-false": f"    if false; then\n{command}\n    fi",
            "duplicate-command": f"{command}\n{command}",
            "duplicate-arm": (
                f"{command}\n    ;;\n"
                "  race)\n"
                "    echo \"== race ==\"\n"
                f"{command}"
            ),
        }
        for name, replacement in mutants.items():
            with self.subTest(name=name):
                self.assert_gate_rejected(
                    self.mutate_gate(command, replacement),
                    "local race dispatch",
                )

    def test_local_fuzz_and_fast_check_reject_nonblocking_mutants(self) -> None:
        mutants = (
            (
                "fuzz-arm-or-true",
                "    run_fuzz",
                "    run_fuzz || true",
                "local fuzz dispatch",
            ),
            (
                "fuzz-command-or-true",
                (
                    "  go test -run '^$' -fuzz \"^${fuzz_target}$\" "
                    "-fuzztime=\"$fuzz_time\" \"$fuzz_package\""
                ),
                (
                    "  go test -run '^$' -fuzz \"^${fuzz_target}$\" "
                    "-fuzztime=\"$fuzz_time\" \"$fuzz_package\" || true"
                ),
                "function run_fuzz command structure",
            ),
            (
                "fast-check-or-true",
                f"  {gate.CHECK_COMMAND}",
                f"  {gate.CHECK_COMMAND} || true",
                "function run_fast_gate command structure",
            ),
            (
                "coverage-arm-or-true",
                "    run_coverage",
                "    run_coverage || true",
                "local coverage dispatch",
            ),
        )
        for name, old, new, fragment in mutants:
            with self.subTest(name=name):
                self.assert_gate_rejected(self.mutate_gate(old, new), fragment)

    def test_local_sensitive_dispatches_reject_operator_and_control_flow_variants(self) -> None:
        cases = (
            (
                "race-pipeline",
                "    go test -race -count=1 ./...",
                "    go test -race -count=1 ./... | cat",
                "local race dispatch",
            ),
            (
                "race-early-success",
                "    go test -race -count=1 ./...",
                "    go test -race -count=1 ./... || exit 0",
                "local race dispatch",
            ),
            (
                "fuzz-pipeline",
                "    run_fuzz",
                "    run_fuzz | cat",
                "local fuzz dispatch",
            ),
            (
                "fast-check-early-success",
                f"  {gate.CHECK_COMMAND}",
                f"  {gate.CHECK_COMMAND} || return 0",
                "function run_fast_gate command structure",
            ),
        )
        for name, old, new, fragment in cases:
            with self.subTest(name=name):
                self.assert_gate_rejected(self.mutate_gate(old, new), fragment)

    def test_local_gate_rejects_race_outside_the_dispatcher(self) -> None:
        self.assert_gate_rejected(
            self.mutate_gate(
                "esac",
                "esac\n  go test -race -count=1 ./...",
            ),
            "local gate executable command structure changed",
        )

    def test_local_gate_requires_unique_exact_first_line_shebang(self) -> None:
        shebang = "#!/usr/bin/env bash"
        body = self.gate_script.split("\n", 1)[1]
        mutants = {
            "wrong": self.gate_script.replace(shebang, "#!/bin/true", 1),
            "missing": body,
            "duplicate": f"{shebang}\n{self.gate_script}",
            "leading-command": f"true\n{self.gate_script}",
        }
        for name, script in mutants.items():
            with self.subTest(name=name):
                self.assert_gate_rejected(script, "local gate executable shebang")

    def test_local_sensitive_dispatches_reject_comment_remove_wrapper_and_quoted_variants(self) -> None:
        race = "    go test -race -count=1 ./..."
        fuzz = "    run_fuzz"
        check = f"  {gate.CHECK_COMMAND}"
        cases = (
            ("race-comment", race, f"    # {race.strip()}", "local race dispatch"),
            ("race-remove", race, "", "local race dispatch"),
            (
                "race-wrapper",
                race,
                "    command go test -race -count=1 ./...",
                "local race dispatch",
            ),
            (
                "race-quoted-eval",
                race,
                "    eval 'go test -race -count=1 ./...'",
                "local race dispatch",
            ),
            ("fuzz-comment", fuzz, f"    # {fuzz.strip()}", "local fuzz dispatch"),
            ("fuzz-remove", fuzz, "", "local fuzz dispatch"),
            ("fuzz-wrapper", fuzz, "    command run_fuzz", "local fuzz dispatch"),
            ("fuzz-quoted-eval", fuzz, "    eval 'run_fuzz'", "local fuzz dispatch"),
            ("check-comment", check, f"  # {gate.CHECK_COMMAND}", "function run_fast_gate"),
            ("check-remove", check, "", "function run_fast_gate"),
            (
                "check-wrapper",
                check,
                f"  command {gate.CHECK_COMMAND}",
                "function run_fast_gate",
            ),
            (
                "check-quoted-eval",
                check,
                f"  eval '{gate.CHECK_COMMAND}'",
                "function run_fast_gate",
            ),
        )
        for name, old, new, fragment in cases:
            with self.subTest(name=name):
                self.assert_gate_rejected(self.mutate_gate(old, new), fragment)

    def test_boolean_and_zero_coverage_floors_are_rejected(self) -> None:
        for value in ("true", "0"):
            with self.subTest(value=value), tempfile.TemporaryDirectory() as directory:
                path = Path(directory) / "coverage-floor.json"
                path.write_text(
                    f'{{"schema_version": 1, "packages": {{"example/package": {value}}}}}',
                    encoding="utf-8",
                )
                with self.assertRaises(gate.GateError):
                    gate.load_floors(path)


if __name__ == "__main__":
    unittest.main(verbosity=2)
