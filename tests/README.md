# pytest-bdd E2E prototype

Two-scenario prototype that exercises `taskgate show` end-to-end using
[pytest-bdd](https://pytest-bdd.readthedocs.io/) (Gherkin scenarios + Python
step definitions). Side-by-side with the existing testscript suite under
`taskgate/testdata/show/`.

Goal: evaluate whether to migrate the full suite from testscript to
pytest-bdd, gaining:
- Scenario text that reads as spec (`Scenario: FR-013 collision is a hard error`)
- Structured JSON assertions (Python's `json.loads` + `==` on dicts)
- Precise exit-code assertions
- Background / Examples for fixture reuse

## Run

Prerequisites: `mise install` activates Python 3.14 and uv.

```sh
uv sync                 # install pytest + pytest-bdd into .venv
uv run pytest -v        # run the 2 scenarios
```

Expected output: 3 PASS (2 collision sub-scenarios + 1 inspect-task-ai).

## Layout

```
pyproject.toml          # project meta + dev deps (pytest, pytest-bdd)
tests/
├── conftest.py         # fixtures (workspace, taskgate) + step definitions
├── test_show.py        # pytest-bdd loader: scenarios("features")
├── features/
│   ├── collision.feature
│   └── inspect_task_ai.feature
└── README.md           # this file
```

For a full-suite migration, split step definitions out of `conftest.py`
into `tests/steps/*.py` modules.
