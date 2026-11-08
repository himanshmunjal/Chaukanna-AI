#!/usr/bin/env python3
"""Turns benchmark/simple-calls.csv into scenario files for cmd/benchmark.

    python3 benchmark/scripts/csv-to-cases.py

Lines in a script are separated by " | ". Each is given 5 seconds, because a phone line
gives one mixed stream with no speaker labels.
"""
import csv
import json
import pathlib
import shutil

here = pathlib.Path(__file__).resolve().parent.parent
out = here / "simple"
shutil.rmtree(out, ignore_errors=True)
out.mkdir()
with open(here / "simple-calls.csv", encoding="utf-8-sig", newline="") as f:
    rows = list(csv.DictReader(f))
for row in rows:
    label = "scam" if row["label"] == "scam" else "legit"
    lines = [
        {"t": 5 * (i + 1), "text": text.strip()}
        for i, text in enumerate(row["script"].split(" | "))
    ]
    case = {"id": row["call_id"], "title": row["family"], "label": label, "language": "hi", "lines": lines}
    (out / f"{row['call_id']}-{label}.json").write_text(json.dumps(case, ensure_ascii=False, indent=2), encoding="utf-8")
print(f"{len(rows)} calls written to {out}")
