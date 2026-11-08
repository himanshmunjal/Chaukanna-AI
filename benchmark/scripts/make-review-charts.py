#!/usr/bin/env python3
"""Draws the fuller set of charts for reading the results, including the unflattering ones.

    python3 benchmark/scripts/make-review-charts.py
    open benchmark/charts/review/index.html

The output folder isn't committed. holdout-gpt-oss-120b is skipped on purpose, and set C
only ever appears as totals, so its individual calls can't be tuned on.
"""
import collections
import csv
import json
import pathlib
import statistics

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt

BENCH = pathlib.Path(__file__).resolve().parent.parent
RES = BENCH / "results"
OUT = BENCH / "charts" / "review"

SURFACE, INK, MUTED, GRID = "#fcfcfb", "#0b0b0b", "#52514e", "#e4e3df"
WARNED, CAUTIONED = "#2a78d6", "#eb6834"
COLORS = {
    "Rules only": "#2a78d6",
    "Rules + Evon": "#eb6834",
    "Evon reviews rules": "#1baf7a",
    "Evon only": "#eda100",
    "Rules + gpt-oss": "#e87ba4",
    "gpt-oss reviews rules": "#6250d6",
}
plt.rcParams.update({
    "font.family": "DejaVu Sans", "font.size": 10.5, "text.color": INK, "axes.labelcolor": MUTED,
    "xtick.color": MUTED, "ytick.color": INK, "axes.edgecolor": GRID, "figure.facecolor": SURFACE,
    "axes.facecolor": SURFACE, "savefig.facecolor": SURFACE,
})

FILES = {
    "Set C (sealed)": {
        "Rules only": "rules/holdout-c-rules-current", "Rules + Evon": "evon/holdout-c-evon-adds",
        "Evon reviews rules": "evon/holdout-c-evon", "Evon only": "evon/holdout-c-evon-only",
    },
    "Staged calls": {
        "Rules only": "rules/staged-rules", "Rules + Evon": "evon/staged-evon-adds",
        "Evon reviews rules": "evon/staged-evon", "Evon only": "evon/staged-evon-only",
        "Rules + gpt-oss": "gpt-oss/staged-gpt-oss-adds", "gpt-oss reviews rules": "gpt-oss/staged-gpt-oss",
    },
    "Simple set": {
        "Rules only": "rules/simple-rules", "Rules + Evon": "evon/simple-evon-adds",
        "Evon reviews rules": "evon/simple-evon", "Evon only": "evon/simple-evon-only",
    },
    "Sets A + B (tuned)": {"Rules only": "rules/dev-rules-current", "Evon reviews rules": "evon/dev-evon"},
    "Cases (tuned)": {"Rules only": "rules/cases-rules-current", "Evon reviews rules": "evon/cases-evon"},
}


def load(name):
    path = RES / f"{name}.json"
    return json.loads(path.read_text()) if path.exists() else None


DATA = {s: {k: load(f) for k, f in m.items()} for s, m in FILES.items()}
DATA = {s: {k: r for k, r in m.items() if r} for s, m in DATA.items()}


def is_scam(r):
    return r["label"] == "scam"


def tally(results, scam):
    rows = [r for r in results if is_scam(r) == scam]
    warned = sum(bool(r["detected"]) for r in rows)
    either = sum(bool(r["detected"] or r.get("cautioned")) for r in rows)
    return warned, either, len(rows)


def tidy(ax, grid="x"):
    ax.set_axisbelow(True)
    if grid:
        ax.grid(True, axis=grid, color=GRID, linewidth=1)
    ax.tick_params(length=0)
    for side in ax.spines.values():
        side.set_visible(False)


def finish(fig, name, title, subtitle, top=None):
    height = fig.get_figheight()
    fig.text(0.02, 1 - 0.14 / height, title, fontsize=14, fontweight="bold", va="top")
    fig.text(0.02, 1 - 0.42 / height, subtitle, fontsize=10, color=MUTED, va="top")
    fig.subplots_adjust(top=top or 1 - 1.0 / height)
    fig.savefig(OUT / f"{name}.png", dpi=110)
    plt.close(fig)
    PAGES.append((name, title, subtitle))


PAGES = []
OUT.mkdir(parents=True, exist_ok=True)


# 1. Every set and setup as a table of numbers
def overview():
    setups = list(COLORS)
    sets = list(DATA)
    fig, axes = plt.subplots(1, 2, figsize=(14, 4.8))
    fig.subplots_adjust(left=0.11, right=0.985, bottom=0.05, wspace=0.13)
    for ax, scam, heading, cmap in ((axes[0], True, "Scams warned (higher is better)", "Blues"), (axes[1], False, "Normal calls wrongly warned (lower is better)", "Oranges")):
        shade = [[0.0] * len(setups) for _ in sets]
        for y, s in enumerate(sets):
            for x, k in enumerate(setups):
                res = DATA[s].get(k)
                if not res:
                    ax.text(x, y, "not run", ha="center", va="center", fontsize=8.5, color=MUTED)
                    continue
                w, _, n = tally(res, scam)
                shade[y][x] = w / n
                ax.text(x, y, f"{w}/{n}\n{round(100 * w / n)}%", ha="center", va="center", fontsize=9.5, color="white" if w / n > 0.62 else INK)
        ax.imshow(shade, cmap=cmap, vmin=0, vmax=1 if scam else 0.5, aspect="auto")
        ax.set_xticks(range(len(setups)), [k.replace(" + ", " +\n").replace(" reviews", "\nreviews") for k in setups], fontsize=9)
        ax.set_yticks(range(len(sets)), sets if scam else [""] * len(sets))
        ax.xaxis.tick_top()
        ax.set_title(heading, fontsize=11, fontweight="bold", pad=38, loc="left")
        ax.tick_params(length=0)
        for side in ax.spines.values():
            side.set_visible(False)
    finish(fig, "01-overview", "Everything at a glance", "Each cell is one test set run with one setup. Tuned sets flatter rules, because the rules were built on them.", top=0.72)


# 2. The trade-off
def tradeoff():
    sets = ["Set C (sealed)", "Staged calls", "Simple set"]
    fig, axes = plt.subplots(1, 3, figsize=(14, 5.2), sharey=True)
    fig.subplots_adjust(left=0.06, right=0.985, bottom=0.2, wspace=0.08)
    for ax, s in zip(axes, sets):
        for k, res in DATA[s].items():
            w, _, n = tally(res, True)
            f, _, m = tally(res, False)
            ax.scatter(100 * f / m, 100 * w / n, s=110, color=COLORS[k], edgecolor=SURFACE, linewidth=2, zorder=3, label=k)
        ax.set_title(s, loc="left", fontsize=11, fontweight="bold")
        ax.set_xlim(-2, 35)
        ax.set_ylim(0, 100)
        ax.set_xlabel("Normal calls wrongly warned (%)")
        tidy(ax, "both")
    axes[0].set_ylabel("Scams warned (%)")
    handles = [plt.Line2D([], [], marker="o", linestyle="", markersize=9, color=c, label=k) for k, c in COLORS.items()]
    fig.legend(handles=handles, loc="lower center", ncol=6, frameon=False)
    finish(fig, "02-tradeoff", "The trade-off: top left is best", "Catching more scams against interrupting more normal calls.")


# 3. All models on the real staged calls
def staged_models():
    res = DATA["Staged calls"]
    fig, axes = plt.subplots(1, 2, figsize=(13, 4.8), sharey=True)
    fig.subplots_adjust(left=0.14, right=0.89, bottom=0.12, wspace=0.42)
    names = [k for k in COLORS if k in res]
    for ax, scam, heading in ((axes[0], True, "22 scam calls"), (axes[1], False, "12 normal calls")):
        for y, k in enumerate(names):
            w, e, n = tally(res[k], scam)
            ax.barh(y, w, color=WARNED, height=0.56, edgecolor=SURFACE, linewidth=2)
            ax.barh(y, e - w, left=w, color=CAUTIONED, height=0.56, edgecolor=SURFACE, linewidth=2)
            ax.text(n + 0.02 * n, y, f"{w} warned, +{e - w}", va="center", fontsize=9)
        ax.set_xlim(0, n)
        ax.set_xticks(range(0, n + 1, 2 if n <= 12 else 4))
        ax.set_title(heading, loc="left", fontsize=11, fontweight="bold")
        tidy(ax)
    axes[0].set_yticks(range(len(names)), names)
    axes[0].invert_yaxis()
    fig.legend([plt.Rectangle((0, 0), 1, 1, color=WARNED), plt.Rectangle((0, 0), 1, 1, color=CAUTIONED)], ["Full warning", "Soft caution only"], loc="upper right", ncol=2, frameon=False, bbox_to_anchor=(0.99, 0.995))
    finish(fig, "03-staged-models", "Your 34 staged calls: Evon against gpt-oss", "Three of the scams were used to fix the rules, so treat this as a comparison, not an unseen score.")


# 4. Call-by-call on staged calls
def staged_grid():
    res = DATA["Staged calls"]
    names = [k for k in COLORS if k in res]
    ids = sorted({r["id"] for r in res[names[0]]})
    by = {k: {r["id"]: r for r in res[k]} for k in names}
    grid, text = [], []
    for i in ids:
        row, labels = [], []
        for k in names:
            r = by[k].get(i)
            level = 0 if not r else 2 if r["detected"] else 1 if r.get("cautioned") else 0
            row.append(level)
            labels.append({2: "W", 1: "c", 0: ""}[level])
        grid.append(row)
        text.append(labels)
    fig, ax = plt.subplots(figsize=(10.5, 11))
    fig.subplots_adjust(left=0.17, right=0.98, bottom=0.03)
    ax.imshow(grid, cmap=matplotlib.colors.ListedColormap(["#f0efec", "#f5c6ae", "#2a78d6"]), vmin=0, vmax=2, aspect="auto")
    for y, row in enumerate(text):
        for x, t in enumerate(row):
            ax.text(x, y, t, ha="center", va="center", fontsize=9.5, color="white" if t == "W" else INK)
    ax.set_xticks(range(len(names)), [k.replace(" + ", " +\n").replace(" reviews", "\nreviews") for k in names], fontsize=9)
    ax.xaxis.tick_top()
    ax.set_yticks(range(len(ids)), [i.replace("call-", "#").replace("-scam", "  scam").replace("-legit", "  normal") for i in ids], fontsize=9)
    ax.tick_params(length=0)
    for side in ax.spines.values():
        side.set_visible(False)
    finish(fig, "04-staged-calls", "Call by call: who caught what", "W = full warning, c = caution, blank = nothing. Scams want W. Normal calls want blank.", top=0.87)


# 5. Simple set, by scam family
def families():
    with open(BENCH / "simple-calls.csv", encoding="utf-8-sig", newline="") as f:
        meta = {r["call_id"]: r for r in csv.DictReader(f)}
    res = DATA["Simple set"]
    names = [k for k in COLORS if k in res]
    fams = list(dict.fromkeys(m["family"] for m in meta.values()))
    fams = sorted(fams, key=lambda x: next(m["label"] for m in meta.values() if m["family"] == x) != "scam")
    grid = []
    for fam in fams:
        row = []
        for k in names:
            row.append(sum(bool(r["detected"]) for r in res[k] if meta[r["id"]]["family"] == fam))
        grid.append(row)
    fig, ax = plt.subplots(figsize=(10, 9.6))
    fig.subplots_adjust(left=0.3, right=0.98, bottom=0.03)
    ax.imshow(grid, cmap="Blues", vmin=0, vmax=6, aspect="auto")
    for y, row in enumerate(grid):
        for x, v in enumerate(row):
            ax.text(x, y, f"{v}/6", ha="center", va="center", fontsize=9.5, color="white" if v >= 4 else INK)
    ax.set_xticks(range(len(names)), names, fontsize=9.5)
    ax.xaxis.tick_top()
    labels = [f"{f}  ({next(m['label'] for m in meta.values() if m['family'] == f)})" for f in fams]
    ax.set_yticks(range(len(fams)), labels, fontsize=9.5)
    ax.tick_params(length=0)
    for side in ax.spines.values():
        side.set_visible(False)
    finish(fig, "05-families", "Simple set: which kinds of call get a full warning", "Scam rows want dark cells. Normal rows want 0/6. These calls were written by the same author as the rules.", top=0.88)


# 6 and 7. How Evon scores calls, on unsealed sets only
def evon_scores():
    rows = (DATA["Staged calls"].get("Evon only") or []) + (DATA["Simple set"].get("Evon only") or [])
    scams = [r["final_risk"] for r in rows if is_scam(r)]
    normal = [r["final_risk"] for r in rows if not is_scam(r)]
    fig, ax = plt.subplots(figsize=(10, 5))
    fig.subplots_adjust(left=0.08, right=0.97, bottom=0.14)
    edges = [i / 20 for i in range(21)]
    ax.hist(scams, bins=edges, weights=[100 / len(scams)] * len(scams), color=WARNED, alpha=0.85, label=f"Scam calls ({len(scams)})", edgecolor=SURFACE)
    ax.hist(normal, bins=edges, weights=[100 / len(normal)] * len(normal), color=CAUTIONED, alpha=0.7, label=f"Normal calls ({len(normal)})", edgecolor=SURFACE)
    ax.axvline(0.8, color=INK, linewidth=1.5)
    ax.text(0.805, ax.get_ylim()[1] * 0.93, "warning at 0.8", fontsize=9.5)
    ax.axvline(0.55, color=MUTED, linewidth=1.5, linestyle="--")
    ax.text(0.555, ax.get_ylim()[1] * 0.85, "caution at 0.55", fontsize=9.5, color=MUTED)
    ax.set_xlabel("Evon's final risk score")
    ax.set_ylabel("Share of calls (%)")
    ax.legend(frameon=False, loc="upper left")
    tidy(ax, "y")
    finish(fig, "06-evon-scores", "Why Evon rarely gives full warnings", "Evon alone, final score on staged and simple calls. Set C is left out so no threshold gets tuned on it.")

    fig, ax = plt.subplots(figsize=(10, 5))
    fig.subplots_adjust(left=0.08, right=0.97, bottom=0.14)
    xs = [i / 100 for i in range(30, 96, 5)]
    ax.plot(xs, [100 * sum(s >= x for s in scams) / len(scams) for x in xs], color=WARNED, linewidth=2, marker="o", markersize=7, label="Scams warned")
    ax.plot(xs, [100 * sum(s >= x for s in normal) / len(normal) for x in xs], color=CAUTIONED, linewidth=2, marker="o", markersize=7, label="Normal calls wrongly warned")
    ax.axvline(0.8, color=INK, linewidth=1.2)
    ax.text(0.805, 92, "today", fontsize=9.5)
    ax.set_xlabel("Warning threshold")
    ax.set_ylabel("Share of calls (%)")
    ax.set_ylim(0, 100)
    ax.legend(frameon=False)
    tidy(ax, "both")
    finish(fig, "07-threshold", "What a different warning threshold would do", "An estimate for Evon alone, from final scores on staged and simple calls. A peak earlier in a call could differ.")


# 8. Speed
def speed():
    fig, axes = plt.subplots(1, 2, figsize=(13, 5))
    fig.subplots_adjust(left=0.2, right=0.97, bottom=0.12, wspace=0.55)
    rows = []
    for s in ("Set C (sealed)", "Staged calls", "Simple set"):
        for k, res in DATA[s].items():
            caught = [r["detected_at_sec"] for r in res if is_scam(r) and r["detected"]]
            ms = [r["evon_avg_ms"] for r in res if r.get("evon_avg_ms")]
            rows.append((f"{s}: {k}", statistics.median(caught) if caught else 0, statistics.median(ms) / 1000 if ms else None, k))
    labels = [r[0] for r in rows]
    axes[0].barh(labels, [r[1] for r in rows], color=[COLORS[r[3]] for r in rows], height=0.6)
    axes[0].invert_yaxis()
    axes[0].set_title("Median seconds into the call before a warning", loc="left", fontsize=10.5, fontweight="bold", x=-0.3)
    axes[0].tick_params(axis="y", labelsize=8)
    tidy(axes[0])
    llm = [r for r in rows if r[2]]
    axes[1].barh([r[0] for r in llm], [r[2] for r in llm], color=[COLORS[r[3]] for r in llm], height=0.6)
    axes[1].invert_yaxis()
    axes[1].set_title("Median model reply, seconds", loc="left", fontsize=10.5, fontweight="bold", x=-0.3)
    axes[1].tick_params(axis="y", labelsize=8)
    tidy(axes[1])
    finish(fig, "08-speed", "Speed", "Evon's reply times aren't for quoting: another project shared the GPU. Call lengths differ between sets.")


# 9. Noise
def noise():
    fig, ax = plt.subplots(figsize=(10, 4.8))
    fig.subplots_adjust(left=0.08, right=0.97, bottom=0.14)
    levels = ["clean", "10", "5", "0"]
    names = ["Clean", "10 dB", "5 dB", "0 dB"]
    for file, label, color, style in (("noise-results", "Street noise", WARNED, "-"), ("noise-babble", "Babble noise", CAUTIONED, "--")):
        d = json.loads((RES / "noise" / f"{file}.json").read_text())
        recall, fp = [], []
        for lv in levels:
            rows = list(d[lv].values())
            w, _, n = tally(rows, True)
            f, _, m = tally(rows, False)
            recall.append(100 * w / n)
            fp.append(100 * f / m)
        ax.plot(names, recall, color=color, linestyle=style, marker="o", markersize=7, linewidth=2, label=f"{label}: scams warned")
        ax.plot(names, fp, color=color, linestyle=style, marker="s", markersize=7, linewidth=2, label=f"{label}: false alarms")
    ax.set_ylim(-3, 105)
    ax.set_ylabel("Share of calls (%)")
    ax.legend(frameon=False, loc="center left", fontsize=9)
    tidy(ax, "y")
    finish(fig, "09-noise", "Noise: rules only, on the 20 tuned calls", "Synthetic voices over a phone line with added noise. These calls were tuned on, and the rules were older then.")


# 10. Rules over time
def rules_history():
    old = [r for r in load("rules/holdout-rules") if r["id"].startswith("ho-c")]
    versions = [("First rules", old), ("Rules when set C was first scored", load("rules/holdout-c-rules-v2")), ("Rules today", DATA["Set C (sealed)"]["Rules only"])]
    fig, axes = plt.subplots(1, 2, figsize=(12, 3.8), sharey=True)
    fig.subplots_adjust(left=0.24, right=0.97, bottom=0.12, wspace=0.12)
    for ax, scam, heading in ((axes[0], True, "Scams warned"), (axes[1], False, "Normal calls wrongly warned")):
        for y, (_, rows) in enumerate(versions):
            w, _, n = tally(rows, scam)
            ax.barh(y, w, color=WARNED, height=0.56)
            ax.text(w + n * 0.015, y, f"{w}/{n}", va="center", fontsize=10)
        ax.set_xlim(0, n)
        ax.set_title(heading, loc="left", fontsize=11, fontweight="bold")
        tidy(ax)
    axes[0].set_yticks(range(len(versions)), [v[0] for v in versions])
    axes[0].invert_yaxis()
    finish(fig, "10-rules-history", "How much the rules improved, on set C", "The first rules were scored on set C before it was used for anything, then the rules were rebuilt on other calls.")


for draw in (overview, tradeoff, staged_models, staged_grid, families, evon_scores, speed, noise, rules_history):
    draw()

html = "".join(f"<h2>{t}</h2><p>{s}</p><img src='{n}.png' alt='{t}'>" for n, t, s in PAGES)
(OUT / "index.html").write_text(
    "<!doctype html><meta charset='utf-8'><title>Chaukanna results</title><style>body{font:16px system-ui;max-width:1100px;margin:32px auto;padding:0 16px;background:#fcfcfb;color:#0b0b0b}"
    "h2{margin:40px 0 4px}p{margin:0 0 12px;color:#52514e}img{width:100%;height:auto;border:1px solid #e4e3df}</style><h1>Chaukanna: all result charts</h1>" + html,
    encoding="utf-8",
)
print(f"{len(PAGES)} charts written to {OUT}")
