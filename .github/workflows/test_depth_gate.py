#!/usr/bin/env python3
"""Enforce Film/TV test depth without repeating the ordinary test pass."""

from __future__ import annotations

import hashlib
import json
import re
import shlex
import subprocess
import sys
import tempfile
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / ".github" / "workflows" / "conformance.yml"
GATE_SCRIPT = ROOT / "script" / "gate.sh"
FLOORS = ROOT / "testing" / "coverage-floor.json"
CONTRACT_TEST = ROOT / ".github" / "workflows" / "test_depth_gate_test.py"
CHECK_COMMAND = "python3 .github/workflows/test_depth_gate.py check"
RACE_COMMAND = "script/gate.sh race"
FUZZ_COMMAND = "script/gate.sh fuzz"
SDK_IF = "${{ inputs.sdk_ref != '' }}"
FUZZ_IF = "${{ github.event_name == 'schedule' || github.event_name == 'workflow_dispatch' }}"
CHECKOUT_ACTION = "actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5"
SETUP_GO_ACTION = "actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff"
LINT_ACTION = "golangci/golangci-lint-action@25e2cdc5eb1d7a04fdc45ff538f1a00e960ae128"
PLUGIN_DIRECTORY = "beamfall-plugin-filmtv"
REQUIRED_FUZZ_PAIRS = {
    (".", "FuzzParseFilename"),
    (".", "FuzzParseTraktCSV"),
    ("./internal/mediaimport", "FuzzParseNFO"),
    ("./internal/mediaimport", "FuzzParseOrderedLists"),
    ("./internal/mediaimport", "FuzzParseWatchStates"),
    ("./internal/foreignimport", "FuzzReadForeignWatchRows"),
}
TOTAL_RE = re.compile(r"^total:\s+\(statements\)\s+([0-9]+(?:\.[0-9]+)?)%$")


class GateError(RuntimeError):
    """A deterministic test-depth failure."""


def run(args: list[str]) -> subprocess.CompletedProcess[str]:
    result = subprocess.run(args, cwd=ROOT, capture_output=True, text=True, check=False)
    if result.returncode != 0:
        raise GateError(
            f"command failed ({result.returncode}): {' '.join(args)}\n"
            f"{result.stdout}{result.stderr}"
        )
    return result


def load_floors(path: Path = FLOORS) -> dict[str, float]:
    try:
        document = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        raise GateError(f"cannot read coverage floors: {error}") from error
    packages = document.get("packages")
    if document.get("schema_version") != 1 or not isinstance(packages, dict) or not packages:
        raise GateError("coverage-floor.json must have schema_version 1 and a non-empty packages object")
    floors: dict[str, float] = {}
    for package, raw_floor in packages.items():
        if (
            not isinstance(package, str)
            or not package
            or isinstance(raw_floor, bool)
            or not isinstance(raw_floor, (int, float))
        ):
            raise GateError("coverage floors must map non-empty package strings to numbers")
        floor = float(raw_floor)
        if floor <= 0 or floor > 100:
            raise GateError(f"coverage floor for {package} must be in (0, 100]")
        floors[package] = floor
    return floors


def discover_packages() -> set[str]:
    packages = {line for line in run(["go", "list", "./..."]).stdout.splitlines() if line}
    if not packages:
        raise GateError("go list ./... discovered no packages")
    return packages


def local_packages() -> list[str]:
    output = run(["go", "list", "-f", "{{.Dir}}", "./..."]).stdout
    packages: list[str] = []
    for line in output.splitlines():
        directory = Path(line).resolve()
        if directory == ROOT:
            packages.append(".")
        else:
            try:
                packages.append("./" + directory.relative_to(ROOT).as_posix())
            except ValueError as error:
                raise GateError(f"package directory escaped repository: {directory}") from error
    return sorted(packages)


def discover_fuzz_pairs() -> set[tuple[str, str]]:
    found: set[tuple[str, str]] = set()
    for package in local_packages():
        output = run(["go", "test", "-list", r"^Fuzz[A-Za-z0-9_]+$", package]).stdout
        for target in re.findall(r"^Fuzz[A-Za-z0-9_]+$", output, flags=re.MULTILINE):
            found.add((package, target))
    return found


def coverage_errors(
    floors: dict[str, float],
    packages: set[str],
    measured: dict[str, float],
) -> list[str]:
    errors = [
        f"discovered package has no coverage floor: {package}"
        for package in sorted(packages - floors.keys())
    ]
    errors.extend(
        f"coverage floor targets missing package: {package}"
        for package in sorted(floors.keys() - packages)
    )
    for package in sorted(packages & floors.keys()):
        if package not in measured:
            errors.append(f"package has no measured coverage: {package}")
        elif measured[package] < floors[package]:
            errors.append(
                f"{package} coverage {measured[package]:.1f}% is below {floors[package]:.1f}%"
            )
    return errors


def parse_total(output: str, package: str) -> float:
    for line in output.splitlines():
        match = TOTAL_RE.match(line.strip())
        if match:
            return float(match.group(1))
    raise GateError(f"cannot parse total statement coverage for {package}\n{output}")


def measure_coverage(packages: set[str]) -> dict[str, float]:
    measured: dict[str, float] = {}
    with tempfile.TemporaryDirectory(prefix="filmtv-coverage-") as temp_dir:
        for index, package in enumerate(sorted(packages)):
            profile = Path(temp_dir) / f"package-{index}.out"
            test = run(
                [
                    "go",
                    "test",
                    "-count=1",
                    "-covermode=set",
                    f"-coverprofile={profile}",
                    package,
                ]
            )
            sys.stdout.write(test.stdout)
            summary = run(["go", "tool", "cover", f"-func={profile}"]).stdout
            measured[package] = parse_total(summary, package)
    return measured


YamlRecord = tuple[int, str, int]
YamlBlock = tuple[str, str, tuple[str, ...]]


class WorkflowYAMLError(ValueError):
    """The workflow uses invalid or unsupported YAML structure."""


def yaml_uncomment(value: str) -> str:
    single_quoted = False
    double_quoted = False
    escaped = False
    index = 0
    while index < len(value):
        character = value[index]
        if double_quoted:
            if escaped:
                escaped = False
            elif character == "\\":
                escaped = True
            elif character == '"':
                double_quoted = False
        elif single_quoted:
            if character == "'":
                if index + 1 < len(value) and value[index + 1] == "'":
                    index += 1
                else:
                    single_quoted = False
        elif character == '"':
            double_quoted = True
        elif character == "'":
            single_quoted = True
        elif character == "#" and (index == 0 or value[index - 1].isspace()):
            return value[:index].rstrip()
        index += 1
    if single_quoted or double_quoted:
        raise WorkflowYAMLError("unterminated quoted scalar")
    return value.rstrip()


def yaml_records(document: str) -> list[YamlRecord]:
    records: list[YamlRecord] = []
    for line_number, raw_line in enumerate(document.splitlines(), start=1):
        leading = raw_line[: len(raw_line) - len(raw_line.lstrip(" \t"))]
        if "\t" in leading:
            raise WorkflowYAMLError(f"line {line_number} uses indentation tabs")
        content = yaml_uncomment(raw_line[len(leading) :])
        if not content:
            continue
        if len(leading) % 2:
            raise WorkflowYAMLError(f"line {line_number} has odd indentation")
        records.append((len(leading), content, line_number))
    return records


def yaml_scan(value: str, delimiter: str) -> list[int]:
    positions: list[int] = []
    stack: list[str] = []
    pairs = {"[": "]", "{": "}", "(": ")"}
    single_quoted = False
    double_quoted = False
    escaped = False
    index = 0
    while index < len(value):
        character = value[index]
        if double_quoted:
            if escaped:
                escaped = False
            elif character == "\\":
                escaped = True
            elif character == '"':
                double_quoted = False
        elif single_quoted:
            if character == "'":
                if index + 1 < len(value) and value[index + 1] == "'":
                    index += 1
                else:
                    single_quoted = False
        elif character == '"':
            double_quoted = True
        elif character == "'":
            single_quoted = True
        elif character in pairs:
            stack.append(pairs[character])
        elif character in pairs.values():
            if not stack or stack.pop() != character:
                raise WorkflowYAMLError(f"unbalanced flow delimiter in {value!r}")
        elif character == delimiter and not stack:
            positions.append(index)
        index += 1
    if single_quoted or double_quoted or stack:
        raise WorkflowYAMLError(f"unterminated flow value {value!r}")
    return positions


def yaml_split_flow(value: str) -> list[str]:
    positions = yaml_scan(value, ",")
    if not positions:
        return [value.strip()] if value.strip() else []
    parts: list[str] = []
    start = 0
    for position in positions:
        part = value[start:position].strip()
        if not part:
            raise WorkflowYAMLError(f"empty flow item in {value!r}")
        parts.append(part)
        start = position + 1
    tail = value[start:].strip()
    if not tail:
        raise WorkflowYAMLError(f"trailing flow delimiter in {value!r}")
    parts.append(tail)
    return parts


def yaml_split_pair(value: str) -> tuple[str, str]:
    positions = yaml_scan(value, ":")
    if not positions:
        raise WorkflowYAMLError(f"expected mapping entry, got {value!r}")
    position = positions[0]
    key = value[:position].strip()
    if not key:
        raise WorkflowYAMLError(f"empty mapping key in {value!r}")
    return key, value[position + 1 :].strip()


def yaml_scalar(value: str) -> object:
    value = value.strip()
    if not value:
        raise WorkflowYAMLError("empty inline scalar")
    if value.startswith("'"):
        if len(value) < 2 or not value.endswith("'"):
            raise WorkflowYAMLError(f"unterminated single-quoted scalar {value!r}")
        return value[1:-1].replace("''", "'")
    if value.startswith('"'):
        if not value.endswith('"'):
            raise WorkflowYAMLError(f"unterminated double-quoted scalar {value!r}")
        try:
            parsed = json.loads(value)
        except json.JSONDecodeError as error:
            raise WorkflowYAMLError(f"invalid double-quoted scalar {value!r}") from error
        if not isinstance(parsed, str):
            raise WorkflowYAMLError(f"quoted scalar is not a string: {value!r}")
        return parsed
    if value.startswith("["):
        if not value.endswith("]"):
            raise WorkflowYAMLError(f"unterminated flow sequence {value!r}")
        return [yaml_scalar(item) for item in yaml_split_flow(value[1:-1])]
    if value.startswith("{"):
        if not value.endswith("}"):
            raise WorkflowYAMLError(f"unterminated flow mapping {value!r}")
        result: dict[str, object] = {}
        for item in yaml_split_flow(value[1:-1]):
            raw_key, raw_value = yaml_split_pair(item)
            key = yaml_scalar(raw_key)
            if not isinstance(key, str) or not key:
                raise WorkflowYAMLError(f"flow mapping key must be a non-empty string: {raw_key!r}")
            if key in result:
                raise WorkflowYAMLError(f"duplicate flow mapping key {key!r}")
            result[key] = yaml_scalar(raw_value)
        return result
    if value in {"null", "Null", "NULL", "~"}:
        return None
    if value in {"true", "True", "TRUE"}:
        return True
    if value in {"false", "False", "FALSE"}:
        return False
    if re.fullmatch(r"-?(?:0|[1-9][0-9]*)", value):
        return int(value)
    return value


def yaml_key(value: str) -> str:
    key = yaml_scalar(value)
    if not isinstance(key, str) or not key:
        raise WorkflowYAMLError(f"mapping key must be a non-empty string: {value!r}")
    return key


class WorkflowYAMLParser:
    """Small YAML 1.2 subset parser for this workflow's mappings and sequences."""

    def __init__(self, records: list[YamlRecord]) -> None:
        self.records = records

    def parse(self) -> dict[str, object]:
        if not self.records:
            raise WorkflowYAMLError("workflow is empty")
        if self.records[0][0] != 0:
            raise WorkflowYAMLError("workflow root must start at column zero")
        parsed, index = self.parse_block(0, 0)
        if index != len(self.records):
            _, _, line_number = self.records[index]
            raise WorkflowYAMLError(f"unparsed workflow content at line {line_number}")
        if not isinstance(parsed, dict):
            raise WorkflowYAMLError("workflow root must be a mapping")
        return parsed

    def parse_block(self, index: int, indent: int) -> tuple[object, int]:
        if index >= len(self.records) or self.records[index][0] != indent:
            raise WorkflowYAMLError(f"missing block at indentation {indent}")
        content = self.records[index][1]
        if content == "-" or content.startswith("- "):
            return self.parse_sequence(index, indent)
        return self.parse_mapping(index, indent)

    def parse_mapping(
        self,
        index: int,
        indent: int,
        first_entry: tuple[str, int] | None = None,
    ) -> tuple[dict[str, object], int]:
        result: dict[str, object] = {}
        if first_entry is not None:
            index = self.add_mapping_entry(
                result,
                first_entry[0],
                first_entry[1],
                index,
                indent,
            )
        while index < len(self.records):
            entry_indent, content, line_number = self.records[index]
            if entry_indent < indent:
                break
            if entry_indent > indent:
                raise WorkflowYAMLError(f"unexpected indentation at line {line_number}")
            if content == "-" or content.startswith("- "):
                raise WorkflowYAMLError(f"sequence item in mapping at line {line_number}")
            index += 1
            index = self.add_mapping_entry(result, content, line_number, index, indent)
        return result, index

    def add_mapping_entry(
        self,
        result: dict[str, object],
        content: str,
        line_number: int,
        index: int,
        indent: int,
    ) -> int:
        raw_key, raw_value = yaml_split_pair(content)
        key = yaml_key(raw_key)
        if key in result:
            raise WorkflowYAMLError(f"duplicate mapping key {key!r} at line {line_number}")
        if raw_value in {"|", "|-", ">", ">-"}:
            base_indent = indent + 2
            lines: list[str] = []
            while index < len(self.records) and self.records[index][0] > indent:
                line_indent, line, _ = self.records[index]
                if line_indent < base_indent:
                    raise WorkflowYAMLError(f"invalid block scalar indentation after line {line_number}")
                lines.append(" " * (line_indent - base_indent) + line)
                index += 1
            if not lines:
                raise WorkflowYAMLError(f"empty block scalar at line {line_number}")
            result[key] = ("block", raw_value, tuple(lines))
            return index
        if raw_value:
            result[key] = yaml_scalar(raw_value)
            return index
        if index < len(self.records) and self.records[index][0] > indent:
            child_indent = self.records[index][0]
            if child_indent != indent + 2:
                raise WorkflowYAMLError(f"invalid child indentation after line {line_number}")
            result[key], index = self.parse_block(index, child_indent)
        else:
            result[key] = None
        return index

    def parse_sequence(self, index: int, indent: int) -> tuple[list[object], int]:
        result: list[object] = []
        while index < len(self.records):
            item_indent, content, line_number = self.records[index]
            if item_indent < indent:
                break
            if item_indent > indent:
                raise WorkflowYAMLError(f"unexpected sequence indentation at line {line_number}")
            if content == "-":
                tail = ""
            elif content.startswith("- "):
                tail = content[2:].strip()
            else:
                break
            index += 1
            if not tail:
                if index >= len(self.records) or self.records[index][0] <= indent:
                    result.append(None)
                    continue
                child_indent = self.records[index][0]
                if child_indent != indent + 2:
                    raise WorkflowYAMLError(f"invalid sequence child after line {line_number}")
                item, index = self.parse_block(index, child_indent)
                result.append(item)
                continue
            try:
                yaml_split_pair(tail)
            except WorkflowYAMLError:
                item = yaml_scalar(tail)
                if index < len(self.records) and self.records[index][0] > indent:
                    raise WorkflowYAMLError(f"scalar sequence item has children at line {line_number}")
            else:
                item, index = self.parse_mapping(
                    index,
                    indent + 2,
                    first_entry=(tail, line_number),
                )
            result.append(item)
        return result, index


def parse_workflow_yaml(document: str) -> dict[str, object]:
    return WorkflowYAMLParser(yaml_records(document)).parse()


EXPECTED_JOB_STEPS = {
    "fast-conformance": (
        "Checkout plugin",
        "Checkout SDK override",
        "Set up Go",
        "Use SDK override",
        "gofmt",
        "vet",
        "test contracts and per-package coverage",
        "govulncheck",
        "golangci-lint",
    ),
    "race": (
        "Checkout plugin",
        "Checkout SDK override",
        "Set up Go",
        "Use SDK override",
        "race",
    ),
    "conformance": ("Require fast and race lanes",),
    "fuzz": (
        "Checkout plugin",
        "Checkout SDK override",
        "Set up Go",
        "Use SDK override",
        "bounded parser fuzz",
    ),
}
EXPECTED_WORKFLOW_FINGERPRINT = "cce2d90d1dca68677501e743ace346b96f67c1f24855832c5a129f266369fa04"


def workflow_fingerprint(document: dict[str, object]) -> str:
    canonical = json.dumps(
        document,
        ensure_ascii=True,
        separators=(",", ":"),
        sort_keys=True,
    )
    return hashlib.sha256(canonical.encode("utf-8")).hexdigest()


def workflow_structure_errors(workflow: str) -> list[str]:
    try:
        document = parse_workflow_yaml(workflow)
    except WorkflowYAMLError as error:
        return [f"workflow YAML structure is invalid: {error}"]

    errors: list[str] = []
    job_document = document.get("jobs")
    if not isinstance(job_document, dict):
        return ["workflow jobs must be a mapping"]
    if tuple(job_document) != tuple(EXPECTED_JOB_STEPS):
        errors.append("workflow job set or order changed")
    for job_name, expected_steps in EXPECTED_JOB_STEPS.items():
        job = job_document.get(job_name)
        if not isinstance(job, dict):
            errors.append(f"workflow job {job_name} must be a mapping")
            continue
        job_steps = job.get("steps")
        if not isinstance(job_steps, list):
            errors.append(f"workflow job {job_name} steps must be a sequence")
            continue
        names = tuple(
            step.get("name") if isinstance(step, dict) else None
            for step in job_steps
        )
        if names != expected_steps:
            errors.append(f"workflow job {job_name} ordered steps changed")
    if workflow_fingerprint(document) != EXPECTED_WORKFLOW_FINGERPRINT:
        errors.append("workflow trigger, job, or full step semantics changed")
    return errors


def active(text: str) -> str:
    return "\n".join(
        line for line in text.splitlines() if not line.lstrip().startswith("#")
    )


def jobs(workflow: str) -> dict[str, list[str]]:
    text = active(workflow)
    match = re.search(r"^jobs:\s*$", text, flags=re.MULTILINE)
    if not match:
        return {}
    blocks: dict[str, list[str]] = {}
    for found in re.finditer(
        r"^  ([A-Za-z0-9_-]+):\n.*?(?=^  [A-Za-z0-9_-]+:\n|\Z)",
        text[match.end() :],
        flags=re.MULTILINE | re.DOTALL,
    ):
        blocks.setdefault(found.group(1), []).append(found.group(0))
    return blocks


def steps(job: str) -> list[str]:
    return re.findall(
        r"^      - .*?(?=^      - |\Z)",
        active(job),
        flags=re.MULTILINE | re.DOTALL,
    )


def scalar_values(block: str, indent: int, key: str) -> list[str]:
    return re.findall(
        rf"^{' ' * indent}{re.escape(key)}:\s*(.*?)\s*$",
        active(block),
        flags=re.MULTILINE,
    )


def step_name(step: str) -> str:
    names = scalar_values(step, 6, "- name")
    return names[0] if len(names) == 1 else ""


def named_steps(job: str, name: str) -> list[str]:
    return [step for step in steps(job) if step_name(step) == name]


def commands(step: str) -> list[str]:
    lines = active(step).splitlines()
    result: list[str] = []
    for index, line in enumerate(lines):
        match = re.match(r"^        run:\s*(.*?)\s*$", line)
        if not match:
            continue
        value = match.group(1)
        if value not in {"|", "|-", ">", ">-"}:
            return [value] if value else []
        for body_line in lines[index + 1 :]:
            if body_line and len(body_line) - len(body_line.lstrip()) <= 8:
                break
            if body_line.startswith("          ") and body_line.strip():
                result.append(body_line[10:])
        return result
    return result


def mapping(step: str, key: str) -> dict[str, str]:
    lines = active(step).splitlines()
    header = f"        {key}:"
    starts = [index for index, line in enumerate(lines) if line == header]
    if len(starts) != 1:
        return {}
    result: dict[str, str] = {}
    for line in lines[starts[0] + 1 :]:
        if line and len(line) - len(line.lstrip()) <= 8:
            break
        match = re.match(r"^          ([A-Za-z0-9_-]+):\s*(.*?)\s*$", line)
        if match:
            result[match.group(1)] = match.group(2)
    return result


def require_step(
    errors: list[str],
    job_name: str,
    job: str,
    name: str,
    *,
    run_command: list[str] | None = None,
    uses: str | None = None,
    working_directory: str | None = None,
    expected_if: str | None = None,
    env: dict[str, str] | None = None,
    with_values: dict[str, str] | None = None,
) -> None:
    matches = named_steps(job, name)
    if len(matches) != 1:
        errors.append(f"{job_name}/{name} missing or duplicated")
        return
    step = matches[0]
    wanted_if = [] if expected_if is None else [expected_if]
    if scalar_values(step, 8, "if") != wanted_if:
        errors.append(f"{job_name}/{name} condition changed")
    if scalar_values(step, 8, "continue-on-error"):
        errors.append(f"{job_name}/{name} may not continue on error")
    if run_command is not None and commands(step) != run_command:
        errors.append(f"{job_name}/{name} command changed or is inactive")
    if uses is not None and scalar_values(step, 8, "uses") != [uses]:
        errors.append(f"{job_name}/{name} action changed or is inactive")
    wanted_directory = [] if working_directory is None else [working_directory]
    if scalar_values(step, 8, "working-directory") != wanted_directory:
        errors.append(f"{job_name}/{name} working directory changed")
    if env is not None and mapping(step, "env") != env:
        errors.append(f"{job_name}/{name} environment changed")
    if with_values is not None and mapping(step, "with") != with_values:
        errors.append(f"{job_name}/{name} inputs changed")


def sdk_errors(errors: list[str], job_name: str, job: str) -> None:
    require_step(
        errors,
        job_name,
        job,
        "Checkout SDK override",
        uses=CHECKOUT_ACTION,
        expected_if=SDK_IF,
        with_values={
            "repository": "beamfall/plugin-sdk",
            "ref": "${{ inputs.sdk_ref }}",
            "path": "beamfall-plugin-sdk",
        },
    )
    require_step(
        errors,
        job_name,
        job,
        "Use SDK override",
        run_command=["go mod edit -replace github.com/beamfall/plugin-sdk=../beamfall-plugin-sdk"],
        working_directory=PLUGIN_DIRECTORY,
        expected_if=SDK_IF,
    )


def fuzz_matrix(job: str) -> list[tuple[str, str]]:
    return re.findall(
        r"^          - package:\s*(\S+)\s*$\n^            target:\s*(Fuzz[A-Za-z0-9_]+)\s*$",
        active(job),
        flags=re.MULTILINE,
    )


def workflow_errors(workflow: str, discovered_fuzz: set[tuple[str, str]]) -> list[str]:
    errors = workflow_structure_errors(workflow)
    parsed = jobs(workflow)
    required_jobs: dict[str, str] = {}
    for name in ("fast-conformance", "race", "conformance", "fuzz"):
        matches = parsed.get(name, [])
        if len(matches) != 1:
            errors.append(f"job {name} missing or duplicated")
        else:
            required_jobs[name] = matches[0]

    if not re.search(r"^  schedule:\s*$", active(workflow), flags=re.MULTILINE):
        errors.append("scheduled fuzz trigger missing")

    for name, job in required_jobs.items():
        if scalar_values(job, 4, "continue-on-error") or any(
            scalar_values(step, 8, "continue-on-error") for step in steps(job)
        ):
            errors.append(f"{name} job and steps must not continue on error")
        wanted_if = {
            "fast-conformance": [],
            "race": [],
            "conformance": ["always()"],
            "fuzz": [FUZZ_IF],
        }[name]
        if scalar_values(job, 4, "if") != wanted_if:
            errors.append(f"{name} job condition changed")

    for name in ("fast-conformance", "race", "fuzz"):
        job = required_jobs.get(name)
        if not job:
            continue
        require_step(
            errors,
            name,
            job,
            "Checkout plugin",
            uses=CHECKOUT_ACTION,
            with_values={"path": PLUGIN_DIRECTORY},
        )
        require_step(
            errors,
            name,
            job,
            "Set up Go",
            uses=SETUP_GO_ACTION,
            with_values={"go-version-file": f"{PLUGIN_DIRECTORY}/go.mod"},
        )
        sdk_errors(errors, name, job)

    fast = required_jobs.get("fast-conformance", "")
    for name, run_command, uses, with_values in (
        ("gofmt", ["gofmt -l . | tee /dev/stderr | (! grep -q .)"], None, None),
        ("vet", ["go vet ./..."], None, None),
        ("test contracts and per-package coverage", [CHECK_COMMAND], None, None),
        ("govulncheck", ["go -C tools tool govulncheck -C .. ./..."], None, None),
        (
            "golangci-lint",
            None,
            LINT_ACTION,
            {"version": "v2.15.0", "working-directory": PLUGIN_DIRECTORY},
        ),
    ):
        require_step(
            errors,
            "fast-conformance",
            fast,
            name,
            run_command=run_command,
            uses=uses,
            working_directory=None if name == "golangci-lint" else PLUGIN_DIRECTORY,
            with_values=with_values,
        )

    race = required_jobs.get("race", "")
    require_step(
        errors,
        "race",
        race,
        "race",
        run_command=[RACE_COMMAND],
        working_directory=PLUGIN_DIRECTORY,
    )

    aggregator = required_jobs.get("conformance", "")
    if scalar_values(aggregator, 4, "name") != ["conformance"]:
        errors.append("preserved conformance check name changed")
    if scalar_values(aggregator, 4, "needs") != ["[fast-conformance, race]"]:
        errors.append("conformance aggregator must need fast-conformance and race")
    require_step(
        errors,
        "conformance",
        aggregator,
        "Require fast and race lanes",
        run_command=['test "$FAST_RESULT" = success', 'test "$RACE_RESULT" = success'],
        env={
            "FAST_RESULT": "${{ needs.fast-conformance.result }}",
            "RACE_RESULT": "${{ needs.race.result }}",
        },
    )

    fuzz = required_jobs.get("fuzz", "")
    if scalar_values(fuzz, 4, "timeout-minutes") != ["12"]:
        errors.append("fuzz timeout changed")
    configured_pairs = fuzz_matrix(fuzz)
    if len(configured_pairs) != len(REQUIRED_FUZZ_PAIRS) or set(configured_pairs) != REQUIRED_FUZZ_PAIRS:
        errors.append("fuzz matrix must cover every required target exactly once")
    require_step(
        errors,
        "fuzz",
        fuzz,
        "bounded parser fuzz",
        run_command=[FUZZ_COMMAND],
        working_directory=PLUGIN_DIRECTORY,
        env={
            "FUZZ_PACKAGE": "${{ matrix.package }}",
            "FUZZ_TARGET": "${{ matrix.target }}",
            "FUZZ_TIME": "20s",
        },
    )

    if discovered_fuzz != REQUIRED_FUZZ_PAIRS:
        for package, target in sorted(REQUIRED_FUZZ_PAIRS - discovered_fuzz):
            errors.append(f"required fuzz target was not discovered: {package} {target}")
        for package, target in sorted(discovered_fuzz - REQUIRED_FUZZ_PAIRS):
            errors.append(f"discovered fuzz target is not scheduled: {package} {target}")

    return errors


ShellCommand = tuple[str, ...]
ShellLine = tuple[int, ShellCommand, int]


def shell_lines(script: str) -> tuple[list[ShellLine], list[str]]:
    """Tokenize active shell records while preserving their structural indentation."""
    records: list[ShellLine] = []
    errors: list[str] = []
    for line_number, raw_line in enumerate(script.splitlines(), start=1):
        leading = raw_line[: len(raw_line) - len(raw_line.lstrip(" \t"))]
        if "\t" in leading:
            errors.append(f"local gate line {line_number} uses structural tabs")
            continue
        lexer = shlex.shlex(
            raw_line,
            posix=True,
            punctuation_chars=";&|()<>",
        )
        lexer.whitespace_split = True
        lexer.commenters = "#"
        try:
            command = tuple(lexer)
        except ValueError as error:
            errors.append(f"local gate line {line_number} cannot be parsed: {error}")
            continue
        if command:
            records.append((len(leading), command, line_number))
    return records, errors


def shell_functions(
    records: list[ShellLine],
) -> tuple[dict[str, list[list[ShellCommand]]], list[str]]:
    """Extract top-level functions as ordered, tokenized command records."""
    functions: dict[str, list[list[ShellCommand]]] = {}
    errors: list[str] = []
    index = 0
    while index < len(records):
        indent, command, line_number = records[index]
        if not (
            indent == 0
            and len(command) == 3
            and re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", command[0])
            and command[1:] == ("()", "{")
        ):
            index += 1
            continue
        name = command[0]
        body: list[ShellCommand] = []
        index += 1
        while index < len(records):
            body_indent, body_command, _ = records[index]
            if body_indent == 0 and body_command == ("}",):
                break
            body.append(body_command)
            index += 1
        if index == len(records):
            errors.append(f"local gate function {name} at line {line_number} is unterminated")
            break
        functions.setdefault(name, []).append(body)
        index += 1
    return functions, errors


def shell_case_arms(
    records: list[ShellLine],
) -> tuple[dict[str, list[list[ShellCommand]]], list[str]]:
    """Extract the one supported top-level case dispatcher and all of its arms."""
    errors: list[str] = []
    starts = [
        index
        for index, (indent, command, _) in enumerate(records)
        if indent == 0 and command == ("case", "${1:-gate}", "in")
    ]
    if len(starts) != 1:
        return {}, ["local gate case dispatcher missing or duplicated"]
    start = starts[0]
    ends = [
        index
        for index, (indent, command, _) in enumerate(records[start + 1 :], start=start + 1)
        if indent == 0 and command == ("esac",)
    ]
    if len(ends) != 1:
        return {}, ["local gate case dispatcher is unterminated or duplicated"]

    arms: dict[str, list[list[ShellCommand]]] = {}
    index = start + 1
    end = ends[0]
    while index < end:
        indent, command, line_number = records[index]
        if indent != 2 or len(command) != 2 or command[1] != ")":
            errors.append(f"local gate has an invalid case arm at line {line_number}")
            break
        name = command[0]
        body: list[ShellCommand] = []
        index += 1
        while index < end:
            body_indent, body_command, body_line = records[index]
            if body_indent == 4 and body_command == (";;",):
                break
            if body_indent == 2 and len(body_command) == 2 and body_command[1] == ")":
                errors.append(f"local gate case arm {name} before line {body_line} is unterminated")
                return arms, errors
            body.append(body_command)
            index += 1
        if index == end:
            errors.append(f"local gate case arm {name} is unterminated")
            break
        arms.setdefault(name, []).append(body)
        index += 1
    return arms, errors


EXPECTED_FUNCTIONS: dict[str, list[ShellCommand]] = {
    "run_contracts": [
        ("echo", "== test-depth contracts =="),
        ("python3", ".github/workflows/test_depth_gate.py", "contracts"),
    ],
    "run_coverage": [
        ("echo", "== one-pass per-package coverage =="),
        ("python3", ".github/workflows/test_depth_gate.py", "coverage"),
    ],
    "run_govulncheck": [
        ("echo", "== govulncheck (pinned tools module) =="),
        (
            "if",
            "!",
            "go",
            "-C",
            "tools",
            "tool",
            "2",
            ">",
            "/dev/null",
            "|",
            "grep",
            "-q",
            "(golang.org/x/vuln/cmd/govulncheck)$",
            ";",
            "then",
        ),
        (
            "echo",
            "govulncheck: tools module does not declare golang.org/x/vuln/cmd/govulncheck",
            ">&",
            "2",
        ),
        ("exit", "1"),
        ("fi",),
        ("go", "-C", "tools", "tool", "govulncheck", "-C", "..", "./..."),
    ],
    "run_fast_gate": [
        ("echo", "== gofmt =="),
        ("fmt_out=$(gofmt -l .)",),
        ("if", "[", "-n", "$fmt_out", "]", ";", "then"),
        ("echo", "gofmt: files need formatting:"),
        ("echo", "$fmt_out"),
        ("exit", "1"),
        ("fi",),
        ("echo", "== go vet =="),
        ("go", "vet", "./..."),
        ("echo", "== test-depth contracts + one-pass per-package coverage =="),
        ("python3", ".github/workflows/test_depth_gate.py", "check"),
        ("echo", "== golangci-lint =="),
        ("golangci-lint", "run", "./..."),
        ("run_govulncheck",),
        ("echo", "ALL GATES GREEN"),
    ],
    "run_fuzz": [
        ("fuzz_package=${FUZZ_PACKAGE:-}",),
        ("fuzz_target=${FUZZ_TARGET:-}",),
        ("fuzz_time=${FUZZ_TIME:-20s}",),
        (
            "if",
            "[[",
            "!",
            "$fuzz_time",
            "=~",
            "^",
            "(",
            "[1-9][0-9]*",
            ")(",
            "s",
            "|",
            "x",
            ")",
            "$",
            "]]",
            ";",
            "then",
        ),
        (
            "echo",
            "fuzz: FUZZ_TIME must be a positive whole-second or execution budget, got $fuzz_time",
            ">&",
            "2",
        ),
        ("exit", "1"),
        ("fi",),
        (
            "python3",
            ".github/workflows/test_depth_gate.py",
            "fuzz-target",
            "$fuzz_package",
            "$fuzz_target",
        ),
        (
            "go",
            "test",
            "-run",
            "^$",
            "-fuzz",
            "^${fuzz_target}$",
            "-fuzztime=$fuzz_time",
            "$fuzz_package",
        ),
    ],
}

EXPECTED_ARMS: dict[str, list[ShellCommand]] = {
    "gate": [("run_fast_gate",)],
    "govulncheck": [("run_govulncheck",)],
    "contracts": [("run_contracts",)],
    "coverage": [("run_coverage",)],
    "race": [
        ("echo", "== race =="),
        ("go", "test", "-race", "-count=1", "./..."),
    ],
    "fuzz": [
        ("echo", "== bounded parser fuzz =="),
        ("run_fuzz",),
    ],
    "*": [
        (
            "echo",
            "usage: $0 [gate|govulncheck|contracts|coverage|race|fuzz]",
            ">&",
            "2",
        ),
        ("exit", "2"),
    ],
}


def expected_gate_commands() -> list[ShellCommand]:
    commands: list[ShellCommand] = [
        ("set", "-euo", "pipefail"),
        ("cd", "$(dirname $0)/.."),
    ]
    for name in (
        "run_contracts",
        "run_coverage",
        "run_govulncheck",
        "run_fast_gate",
        "run_fuzz",
    ):
        commands.append((name, "()", "{"))
        commands.extend(EXPECTED_FUNCTIONS[name])
        commands.append(("}",))
    commands.append(("case", "${1:-gate}", "in"))
    for name in ("gate", "govulncheck", "contracts", "coverage", "race", "fuzz", "*"):
        commands.append((name, ")"))
        commands.extend(EXPECTED_ARMS[name])
        commands.append((";;",))
    commands.append(("esac",))
    return commands


EXPECTED_GATE_COMMANDS = expected_gate_commands()


def gate_errors(script: str) -> list[str]:
    errors: list[str] = []
    raw_lines = script.splitlines()
    if (
        not raw_lines
        or raw_lines[0] != "#!/usr/bin/env bash"
        or sum(line.lstrip().startswith("#!") for line in raw_lines) != 1
    ):
        errors.append(
            "local gate executable shebang must be exactly #!/usr/bin/env bash on line 1 and unique"
        )
    records, parse_errors = shell_lines(script)
    errors.extend(parse_errors)
    functions, function_errors = shell_functions(records)
    errors.extend(function_errors)
    arms, case_errors = shell_case_arms(records)
    errors.extend(case_errors)

    if [command for _, command, _ in records] != EXPECTED_GATE_COMMANDS:
        errors.append("local gate executable command structure changed")

    function_starts = [
        index
        for index, (indent, command, _) in enumerate(records)
        if indent == 0 and len(command) == 3 and command[1:] == ("()", "{")
    ]
    preamble = [command for _, command, _ in records[: min(function_starts, default=0)]]
    if preamble != [("set", "-euo", "pipefail"), ("cd", "$(dirname $0)/..")]:
        errors.append("local gate fail-closed preamble changed")

    for name, expected in EXPECTED_FUNCTIONS.items():
        matches = functions.get(name, [])
        if len(matches) != 1:
            errors.append(f"local gate function {name} missing or duplicated")
        elif matches[0] != expected:
            errors.append(f"local gate function {name} command structure changed")

    if set(arms) != set(EXPECTED_ARMS):
        errors.append("local gate case arm set changed")
    for name, expected in EXPECTED_ARMS.items():
        matches = arms.get(name, [])
        if len(matches) != 1:
            errors.append(f"local {name} dispatch missing or duplicated")
        elif matches[0] != expected:
            errors.append(f"local {name} dispatch command structure changed")
    return errors


def run_contracts() -> None:
    test = run([sys.executable, str(CONTRACT_TEST)])
    sys.stdout.write(test.stdout)
    sys.stderr.write(test.stderr)
    floors = load_floors()
    packages = discover_packages()
    target_errors = coverage_errors(floors, packages, {package: floors[package] for package in packages if package in floors})
    errors = (
        target_errors
        + workflow_errors(WORKFLOW.read_text(encoding="utf-8"), discover_fuzz_pairs())
        + gate_errors(GATE_SCRIPT.read_text(encoding="utf-8"))
    )
    if errors:
        raise GateError("; ".join(errors))


def run_coverage() -> None:
    floors = load_floors()
    packages = discover_packages()
    measured = measure_coverage(packages)
    errors = coverage_errors(floors, packages, measured)
    if errors:
        raise GateError("; ".join(errors))
    for package in sorted(packages):
        print(f"coverage: {package} {measured[package]:.1f}% >= {floors[package]:.1f}%")


def verify_fuzz_target(package: str, target: str) -> None:
    if (package, target) not in REQUIRED_FUZZ_PAIRS:
        raise GateError(f"unconfigured fuzz target: {package or '<empty>'} {target or '<empty>'}")
    output = run(["go", "test", "-list", f"^{target}$", package]).stdout
    found = re.findall(r"^Fuzz[A-Za-z0-9_]+$", output, flags=re.MULTILINE)
    if found != [target]:
        raise GateError(f"expected exactly {package} {target}, discovered {found or 'none'}")


def main(argv: list[str]) -> int:
    try:
        command = argv[1] if len(argv) > 1 else "check"
        if command == "check":
            run_contracts()
            run_coverage()
        elif command == "contracts":
            run_contracts()
        elif command == "coverage":
            run_coverage()
        elif command == "fuzz-target" and len(argv) == 4:
            verify_fuzz_target(argv[2], argv[3])
        else:
            raise GateError("usage: test_depth_gate.py [check|contracts|coverage|fuzz-target PACKAGE TARGET]")
    except (GateError, OSError) as error:
        print(f"test-depth gate: FAIL: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
