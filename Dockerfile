# ClawBench runtime image — runs the pre-built binary
#
# Prerequisites: run ./build.sh first (and ./build.sh --with-pi for setup wizard)
#
# Build & run (one step):
#   ./scripts/docker-build.sh
#
# Or manually:
#   docker build -t clawbench .
#   docker run -p 20300:20300 -v clawbench-data:/data clawbench

FROM ubuntu:24.04

# Install runtime dependencies:
# - ca-certificates: HTTPS (LLM provider APIs)
# - python3 + pip: edge-tts (free TTS engine)
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates python3 python3-pip && \
    pip3 install --break-system-packages edge-tts && \
    rm -rf /var/lib/apt/lists/* /root/.cache/pip

WORKDIR /app

# Copy binary and frontend
COPY clawbench .
COPY public/ ./public/

# Copy Pi binary for setup wizard
# Uses a staging directory created by scripts/docker-build.sh
# If the staging dir is empty, COPY still succeeds (copies empty layer)
COPY docker-staging/ .clawbench/

# Data directory (mounted as volume for persistence)
RUN mkdir -p /data/.clawbench

EXPOSE 20300

ENTRYPOINT ["./clawbench", "--port", "20300", "--data-dir", "/data/.clawbench"]
