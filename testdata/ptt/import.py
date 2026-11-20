"""Import the library-shaped PTT fixtures into cases.json (CORE-2705).

PTT (https://github.com/dreulavelle/PTT, MIT, Copyright (c) 2024 Spoked; see
LICENSE beside this file) ships pytest fixtures: a release name and what PTT
reads from it. This reads them from a PTT checkout without running PTT:

    python3 -I testdata/ptt/import.py <PTT checkout> > testdata/ptt/cases.json

Fields come from test_title.py (title), test_year.py (year, 0 for none),
test_season.py and test_parser.py's season list (seasons), test_episodes.py
(episodes) and test_main.py, which asserts the whole parse, so its absent year
is 0. test_collection.py, test_adult.py and test_trash.py only mark a name
complete, adult or trash. A name in several files merges its fields; two files
that disagree stop the import, as does a checkout with local changes to the
files read.

A case is a library file, and kept, unless its fields say otherwise (RULE).
"""

import ast
import json
import pathlib
import subprocess
import sys

RULE = (
    "A case is kept unless its fixtures mark it complete, adult or trash, "
    "or declare more than one season, more than three episodes, a season with an "
    "empty episode list, or a season with no episode list on a name without a video "
    "file extension: those are packs, not library files."
)

SOURCES = {
    "test_title.py": ("title",),
    "test_year.py": ("year",),
    "test_season.py": ("seasons",),
    "test_episodes.py": ("episodes",),
    "test_main.py": None,
    "test_parser.py": ("seasons",),
    "test_collection.py": ("complete",),
    "test_adult.py": ("adult",),
    "test_trash.py": ("trash",),
}

FLAGS = ("complete", "adult", "trash")

VIDEO = (".mkv", ".mp4", ".avi", ".m4v")


def literal(node):
    """Evaluate a fixture literal; list(range(a, b)) is the one call PTT uses."""
    if isinstance(node, ast.Call) and getattr(node.func, "id", "") == "list" and len(node.args) == 1:
        inner = node.args[0]
        if isinstance(inner, ast.Call) and getattr(inner.func, "id", "") == "range":
            return list(range(*[literal(a) for a in inner.args]))
    if isinstance(node, (ast.List, ast.Tuple)):
        return [literal(e) for e in node.elts]
    if isinstance(node, ast.Dict):
        return {literal(k): literal(v) for k, v in zip(node.keys, node.values)}
    return ast.literal_eval(node)


def fixtures(path):
    """Yield the parametrize rows of every test in a PTT test file, and the
    test_cases list of test_parser.py's test_season_parser."""
    for node in ast.walk(ast.parse(path.read_text(encoding="utf-8"))):
        if not isinstance(node, ast.FunctionDef):
            continue
        for decorator in node.decorator_list:
            if isinstance(decorator, ast.Call) and getattr(decorator.func, "attr", "") == "parametrize":
                yield from literal(decorator.args[1])
        if path.name == "test_parser.py" and node.name == "test_season_parser":
            for stmt in node.body:
                if isinstance(stmt, ast.Assign) and getattr(stmt.targets[0], "id", "") == "test_cases":
                    yield from literal(stmt.value)


def declared(source, row):
    """Return the release name and the fields a fixture row declares."""
    name = row[0]
    if source == "test_main.py":
        want = row[1]
        fields = {k: want[k] for k in ("title", "seasons", "episodes") if k in want}
        fields["year"] = want.get("year") or 0
        fields.update({k: True for k in FLAGS if want.get(k)})
        return name, fields
    key = SOURCES[source][0]
    if key in FLAGS:
        return name, {key: True} if row[1] else {}
    if source == "test_year.py":
        return name, {"year": row[1] if row[2] else 0}
    return name, {key: row[1]}


def is_library_file(name, fields):
    if any(fields.get(k) for k in FLAGS):
        return False
    seasons, episodes = fields.get("seasons"), fields.get("episodes")
    if seasons is not None and len(seasons) > 1:
        return False
    if episodes is not None and (len(episodes) > 3 or seasons and not episodes):
        return False
    return not (seasons and episodes is None and not name.lower().endswith(VIDEO))


def version(root):
    """PTT's release version, from its pyproject.toml."""
    for line in (root / "pyproject.toml").read_text(encoding="utf-8").splitlines():
        if line.startswith("version = "):
            return line.split('"')[1]
    sys.exit("pyproject.toml names no version")


def main(checkout):
    root = pathlib.Path(checkout)
    git = ["git", "-C", str(root)]
    read = ["pyproject.toml", "LICENSE"] + [f"tests/{source}" for source in SOURCES]
    if subprocess.check_output(git + ["status", "--porcelain", "--"] + read, text=True).strip():
        sys.exit("the PTT checkout has local changes to the files read; import from a clean commit")
    merged, order = {}, []
    for source in SOURCES:
        for row in fixtures(root / "tests" / source):
            name, fields = declared(source, row)
            if name not in merged:
                merged[name] = {}
                order.append(name)
            for key, value in fields.items():
                if merged[name].get(key, value) != value:
                    sys.exit(f"{source}: {name!r} {key} = {value!r}, another file says {merged[name][key]!r}")
                merged[name][key] = value
    cases = []
    for name in order:
        fields = merged[name]
        if is_library_file(name, fields):
            cases.append({"name": name, **{k: fields[k] for k in ("title", "year", "seasons", "episodes") if k in fields}})
    out = {
        "source": "https://github.com/dreulavelle/PTT",
        "version": version(root),
        "commit": subprocess.check_output(git + ["rev-parse", "HEAD"], text=True).strip(),
        "license": "MIT, Copyright (c) 2024 Spoked; see LICENSE in this directory",
        "license_text": (root / "LICENSE").read_text(encoding="utf-8"),
        "rule": RULE,
        "imported": len(order),
        "cases": cases,
    }
    json.dump(out, sys.stdout, ensure_ascii=False, indent=1)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main(sys.argv[1])
