#!/usr/bin/env python3
"""Draws the README's result charts from the benchmark's saved JSON files.

    python3 benchmark/scripts/make-charts.py

A setup with no results file yet is left out. Nothing here is typed in by hand.
"""
import json
import pathlib

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt

BENCH = pathlib.Path(__file__).resolve().parent.parent
OUT = BENCH / "charts"

SURFACE, INK, MUTED, GRID = "#fcfcfb", "#0b0b0b", "#52514e", "#e4e3df"
WARNED, CAUTIONED = "#2a78d6", "#eb6834"

SETS = [
    ("Sealed set C, typed", "holdout-c"),
    ("Staged phone calls, transcripts replayed", "staged"),
    ("Simple typed set", "simple"),
]
# File names: "evon" is the run where Evon reviews rule hits, "evon-adds" is rules + Evon.
SETUPS = [("Rules only", "rules"), ("Rules + Evon", "evon-adds"), ("Evon reviews rules", "evon"), ("Evon only", "evon-only")]
RULES_FILE = {"holdout-c": "holdout-c-rules-current", "staged": "staged-rules", "simple": "simple-rules"}

plt.rcParams.update({"font.family": "DejaVu Sans", "svg.fonttype": "path", "font.size": 11})


def load(set_key, setup_key):
    if setup_key == "rules":
        path = BENCH / "results" / "rules" / f"{RULES_FILE[set_key]}.json"
    else:
        path = BENCH / "results" / "evon" / f"{set_key}-{setup_key}.json"
    return json.loads(path.read_text()) if path.exists() else None


def counts(results, want_scam):
    rows = [r for r in results if (r["label"] == "scam") == want_scam]
    warned = sum(bool(r["detected"]) for r in rows)
    either = sum(bool(r["detected"] or r.get("cautioned")) for r in rows)
    return warned, either - warned, len(rows)


def chart(want_scam, title, subtitle, path):
    fig, axes = plt.subplots(len(SETS), 1, figsize=(9.6, 7.4), facecolor=SURFACE, sharex=True)
    fig.subplots_adjust(left=0.17, right=0.73, top=0.82, bottom=0.08, hspace=0.42)
    fig.text(0.02, 0.955, title, fontsize=15, fontweight="bold", color=INK)
    fig.text(0.02, 0.92, subtitle, fontsize=10.5, color=MUTED)
    for ax, (set_title, set_key) in zip(axes, SETS):
        ax.set_facecolor(SURFACE)
        rows = [(label, load(set_key, key)) for label, key in SETUPS]
        rows = [(label, counts(r, want_scam)) for label, r in rows if r]
        for y, (label, (warned, cautioned, total)) in enumerate(rows):
            w, c = 100 * warned / total, 100 * cautioned / total
            ax.barh(y, w, height=0.56, color=WARNED, edgecolor=SURFACE, linewidth=2)
            ax.barh(y, c, left=w, height=0.56, color=CAUTIONED, edgecolor=SURFACE, linewidth=2)
            note = f"{warned}/{total} warned"
            if cautioned:
                note += f", +{cautioned} cautioned"
            ax.text(102, y, note, va="center", fontsize=9.5, color=INK, clip_on=False)
        ax.set_yticks(range(len(rows)), [label for label, _ in rows], color=INK)
        ax.invert_yaxis()
        ax.set_xlim(0, 100)
        ax.set_title(set_title, loc="left", fontsize=11.5, fontweight="bold", color=INK, pad=6)
        ax.xaxis.grid(True, color=GRID, linewidth=1)
        ax.set_axisbelow(True)
        ax.tick_params(length=0, colors=MUTED)
        for side in ax.spines.values():
            side.set_visible(False)
    axes[-1].set_xticks(range(0, 101, 25), [f"{n}%" for n in range(0, 101, 25)])
    handles = [plt.Rectangle((0, 0), 1, 1, color=WARNED), plt.Rectangle((0, 0), 1, 1, color=CAUTIONED)]
    fig.legend(handles, ["Full warning", "Soft caution only"], loc="upper left", bbox_to_anchor=(0.015, 0.905), ncol=2, frameon=False, fontsize=10)
    fig.savefig(path, facecolor=SURFACE)
    plt.close(fig)


OUT.mkdir(parents=True, exist_ok=True)
chart(True, "Scams caught: longer is better", "Share of scam calls that got a warning or a caution, on each test set", OUT / "scams-caught.svg")
chart(False, "Normal calls interrupted: shorter is better", "Share of normal calls that wrongly got a warning or a caution, on each test set", OUT / "false-alarms.svg")
print("charts written to", OUT)
