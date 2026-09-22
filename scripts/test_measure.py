import copy
import json
import os
from pathlib import Path
import subprocess
import sys
import unittest

import measure


class MeasurementTests(unittest.TestCase):
    def test_linear_percentiles(self):
        values = [30, 0, 20, 10]
        self.assertEqual(measure.percentile(values, 0.5), 15)
        self.assertAlmostEqual(measure.percentile(values, 0.95), 28.5)

    def test_valid_export_discards_unknown_fields(self):
        command, exported = fixture()
        result = measure.summarize(exported, command)
        self.assertEqual(result["command"], command)
        self.assertEqual(len(result["times"]), 30)
        self.assertEqual(result["p50"], 14.5)
        self.assertAlmostEqual(result["p95"], 27.55)
        self.assertNotIn("unknown", result)
        self.assertEqual(result["exit_codes"], [0] * 30)

    def test_invalid_export_is_rejected(self):
        command, original = fixture()
        changes = [
            ("command", "/private-example.md"),
            ("times", [1] * 29),
            ("times", [1] * 29 + [float("nan")]),
            ("times", [1] * 29 + [float("inf")]),
            ("times", [1] * 29 + [-1]),
            ("times", [1] * 29 + ["private-example"]),
            ("mean", float("nan")),
            ("user", -1),
            ("exit_codes", [0] * 29 + [1]),
            ("exit_codes", [0] * 29),
        ]
        for key, value in changes:
            with self.subTest(key=key, value=value):
                exported = copy.deepcopy(original)
                exported["results"][0][key] = value
                with self.assertRaises(ValueError):
                    measure.summarize(exported, command)

    def test_execution_failure_never_prints_private_values(self):
        env = os.environ.copy()
        for name in ("BINARY", "ROOT", "TARGET", "HYPERFINE"):
            env["MDLINK_MEASURE_" + name] = "/private-example-" + name
        completed = subprocess.run(
            [sys.executable, str(Path(measure.__file__)), "outgoing"],
            env=env, capture_output=True, text=True,
        )
        self.assertEqual(completed.returncode, 1)
        self.assertEqual(completed.stdout, "")
        self.assertNotIn("private-example", completed.stderr)
        self.assertEqual(json.loads(completed.stderr), {"error": "measurement-failed"})


def fixture():
    command = '"$MDLINK_MEASURE_BINARY" outgoing --root "$MDLINK_MEASURE_ROOT" --format json "$MDLINK_MEASURE_TARGET" >/dev/null 2>/dev/null'
    result = {
        "command": command, "times": list(range(30)), "exit_codes": [0] * 30,
        "mean": 14.5, "stddev": 1, "median": 14.5, "min": 0, "max": 29,
        "user": 1, "system": 1, "unknown": "private-example",
    }
    return command, {"results": [result]}


if __name__ == "__main__":
    unittest.main()
