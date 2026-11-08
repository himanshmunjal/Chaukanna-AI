# Running Evon

This is optional. Chaukanna runs on its rules without a reasoning model. Evon isn't on Gnani's API yet, so using it today means running it yourself as described here. Report results under the model that actually produced them.

Chaukanna talks to Evon through any **OpenAI-compatible** endpoint, so you can
pick whichever option fits your budget. Nothing in the code changes; only
`EVON_BASE_URL`, `EVON_MODEL` and `EVON_API_KEY` in `backend/.env`.

| Option | GPU needed | Cost | Quality |
|---|---|---|---|
| **A. 4-bit GGUF + llama.cpp** (recommended on a budget) | 24–32 GB (RTX 3090 / A5000 / 4090 / PRO 4500) | **$0.27–0.72 / hour** | Very close to full precision |
| B. Full precision + vLLM (`start-evon.sh`) | 80 GB (A100 / H100) | ~$1.79 / hour | Reference |
| C. Hosted by Gnani (ask on Discord) | none | free? | Reference |

A laptop cannot run Evon: even the smallest quantization is ~17 GB.

---

## Option A: GGUF on a cheap GPU (≈ 7–18 GPU-hours for $5)

Community GGUF build: [`luminenio/gnani-evon-v3.3-30B-A3B-GGUF`](https://huggingface.co/luminenio/gnani-evon-v3.3-30B-A3B-GGUF)
(architecture `nemotron_h_moe`; Q4_K_M = 23 GB).

### 0. Access
On 4 November 2026 this repo and `gnani/gnani-evon-v3.3-30B-A3B` were not gated. If a download returns `401` or `403`, request access on the model page and set `HF_TOKEN` to a read token from https://huggingface.co/settings/tokens.

### 1. Rent the GPU
RunPod → **Pods → Deploy** → pick any available card with **≥ 24 GB VRAM**:

| Card | VRAM | Quant to use | Context |
|---|---|---|---|
| RTX A5000 / 3090 / 4090 | 24 GB | `Q4_K_S` (20.6 GB) | 4096 |
| RTX PRO 4500 / 5090 | 32 GB | `Q4_K_M` (23 GB) | 8192 |

(Vast.ai is often cheaper for the same cards.) No network volume needed: the download is ~20 GB and takes a few minutes.

### 2. Use the official llama.cpp server image (no install, no build)
In **Edit Template**:

- **Container image:** `ghcr.io/ggml-org/llama.cpp:server-cuda`
- **Container disk:** 40 GB
- **Expose HTTP ports:** `8000`
- **Environment variables:** `HF_TOKEN = hf_...` (your read token)
- **Container start command** (one line; change the quant/context to match your card, and pick your own secret):

```
-hf luminenio/gnani-evon-v3.3-30B-A3B-GGUF:Q4_K_M --host 0.0.0.0 --port 8000 -ngl 99 -c 8192 --jinja --alias evon --api-key CHOOSE-A-SECRET
```

Watch the pod **Logs** until you see `server is listening`.

### 3. Connect Chaukanna
`backend/.env`:

```env
EVON_BASE_URL=https://<POD_ID>-8000.proxy.runpod.net/v1
EVON_MODEL=evon
EVON_API_KEY=CHOOSE-A-SECRET
```

Check it:

```bash
cd backend
go run ./cmd/gnanicheck          # Timbre -> Prisma -> Evon, prints Evon's verdict + latency
go run ./cmd/benchmark           # all scenarios through Evon
```

### 4. Stop the pod as soon as you are done
Billing is per minute. The model downloads again next time (~5 min), which is
cheaper than keeping a volume.

### Troubleshooting
| Symptom | Fix |
|---|---|
| `401` / `403` while downloading | Access to the GGUF repo not approved yet, or `HF_TOKEN` missing |
| `unknown model architecture nemotron_h_moe` | Image too old: use the `server-cuda` tag (latest), not a pinned old version |
| `out of memory` | Smaller quant (`Q4_K_S`, `IQ4_XS`) or smaller `-c` |
| Evon replies are slow (> 5 s) | Make sure `-ngl 99` is set (all layers on GPU) |

---

## Option B: full precision with vLLM

See [`start-evon.sh`](start-evon.sh): needs an 80 GB GPU (A100 / H100), about 40 GB of container disk and 150 GB on `/workspace`; a run on a smaller volume failed with `Disk quota exceeded`. The weights are 63.5 GB. Then use [`evon-hour.sh`](evon-hour.sh) from your own computer to measure what Evon adds.
