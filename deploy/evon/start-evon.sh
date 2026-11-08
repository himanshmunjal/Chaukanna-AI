#!/usr/bin/env bash
# Starts Gnani Evon behind an OpenAI-compatible API (vLLM) on a Linux GPU box.
#
# Needs: NVIDIA GPU with >= 80 GB VRAM (A100 80GB / H100), and about 150 GB on /workspace.
# Current vLLM needs Python 3.11+, so this makes its own Python 3.12 environment.
#
# Usage (on the GPU pod):
#   export EVON_API_KEY=$(openssl rand -hex 16)    # any secret; put the same value in backend/.env
#   bash start-evon.sh
#
# Optional:
#   EVON_REPO=...        model repo (default below; the model card also uses gnani/gnani-evon-v3.3)
set -euo pipefail

: "${EVON_API_KEY:?Set EVON_API_KEY first, e.g.  export EVON_API_KEY=\$(openssl rand -hex 16)}"
MODEL="${EVON_REPO:-gnani/gnani-evon-v3.3-30B-A3B}"

# Keep the ~60 GB of weights on the persistent network volume (RunPod mounts it at /workspace).
export HF_HOME="${HF_HOME:-/workspace/hf}"
mkdir -p "$HF_HOME"

echo "== GPU =="
nvidia-smi --query-gpu=name,memory.total --format=csv

echo "== Installing vLLM (>= 0.12.0, required by Evon) + fast downloader =="
pip install -q uv
[ -d /workspace/evon-venv ] || uv venv --python 3.12 /workspace/evon-venv
# shellcheck disable=SC1091
. /workspace/evon-venv/bin/activate
uv pip install -q "vllm>=0.12.0" hf_transfer
# hf_transfer downloads the ~60 GB of weights several times faster (= less GPU time paid).
export HF_HUB_ENABLE_HF_TRANSFER=1 HF_HUB_DISABLE_XET=1

echo "== Starting Evon ($MODEL) on port 8000 =="
echo "   First start downloads ~60 GB; wait for 'Application startup complete'."
exec vllm serve "$MODEL" \
  --served-model-name evon \
  --trust-remote-code \
  --max-model-len 16384 \
  --gpu-memory-utilization 0.92 \
  --host 0.0.0.0 --port 8000 \
  --api-key "$EVON_API_KEY"
