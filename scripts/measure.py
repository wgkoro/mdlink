"""Read-only macOS benchmark runner; private command values stay in the environment."""


import json
import math
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile


def percentile(values, fraction):
    values = sorted(values)
    position = (len(values) - 1) * fraction
    lower = int(position)
    upper = min(lower + 1, len(values) - 1)
    return values[lower] + (values[upper] - values[lower]) * (position - lower)


def summarize(exported, command):
    if not isinstance(exported, dict) or len(exported.get("results", [])) != 1:
        raise ValueError("invalid export")
    result = exported["results"][0]
    if result.get("command") != command or result.get("exit_codes") != [0] * 30:
        raise ValueError("invalid command or exits")
    times = result.get("times", [])
    if len(times) != 30:
        raise ValueError("invalid sample count")
    fields = ("mean", "stddev", "median", "user", "system", "min", "max")
    values = list(times) + [result.get(key) for key in fields]
    if any(type(value) not in (int, float) or not math.isfinite(value) or value < 0 for value in values):
        raise ValueError("invalid time")
    return {
        "command": command, "times": times, "exit_codes": [0] * 30,
        **{key: result[key] for key in fields},
        "p50": percentile(times, 0.5), "p95": percentile(times, 0.95),
    }


def measure(command):
    if command not in ("outgoing", "backlinks") or sys.platform != "darwin":
        raise ValueError("unsupported command or platform")
    env = os.environ.copy()
    env.pop("MDLINK_ROOT", None)
    env.pop("MDLINK_FOLLOW_SYMLINKS", None)
    for key in ("BINARY", "ROOT", "TARGET", "HYPERFINE"):
        if not env.get("MDLINK_MEASURE_" + key):
            raise ValueError("missing environment")
    literal = (
        '"$MDLINK_MEASURE_BINARY" ' + command
        + ' --root "$MDLINK_MEASURE_ROOT" --format json "$MDLINK_MEASURE_TARGET" >/dev/null 2>/dev/null'
    )
    completed = subprocess.run(
        [env["MDLINK_MEASURE_HYPERFINE"], "--warmup", "5", "--runs", "30",
         "--shell", "/bin/sh", "--style", "none", "--export-json", "-", literal],
        env=env, capture_output=True, text=True, check=True,
    )
    wall = summarize(json.loads(completed.stdout), literal)
    rss = []
    with tempfile.TemporaryDirectory(prefix="mdlink-rss-") as temporary:
        stats = Path(temporary) / "time.txt"
        for _ in range(30):
            subprocess.run(
                ["/usr/bin/time", "-l", "-o", str(stats), env["MDLINK_MEASURE_BINARY"],
                 command, "--root", env["MDLINK_MEASURE_ROOT"], "--format", "json", env["MDLINK_MEASURE_TARGET"]],
                env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=True,
            )
            match = re.search(r"^\s*(\d+)\s+maximum resident set size\s*$", stats.read_text(), re.MULTILINE)
            if not match:
                raise ValueError("missing RSS")
            rss.append(int(match[1]))
    return {
        "wall": wall,
        "rss": {"samples_bytes": rss, "max_bytes": max(rss), "max_mib": max(rss) / (1024 * 1024)},
    }


def main():
    try:
        if len(sys.argv) != 2:
            raise ValueError("expected one command")
        print(json.dumps(measure(sys.argv[1]), allow_nan=False))
        return 0
    except subprocess.CalledProcessError as error:
        print(json.dumps({"error": "subprocess-failed", "exit_code": error.returncode}), file=sys.stderr)
    except Exception:
        # Never serialize exception arguments: OS errors can contain private paths.
        print(json.dumps({"error": "measurement-failed"}), file=sys.stderr)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
