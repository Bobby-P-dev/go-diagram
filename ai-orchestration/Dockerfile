FROM python:3.12-slim

WORKDIR /app

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt && \
    pip uninstall -y \
    pyarrow \
    lancedb \
    kubernetes \
    onnxruntime \
    pdfminer.six \
    pdfplumber \
    pypdfium2 \
    tokenizers \
    hf_xet \
    huggingface_hub \
    openpyxl \
    et_xmlfile \
    textual \
    pytest \
    || true && \
    rm -f /usr/local/bin/uv /usr/local/bin/uvx && \
    find /usr/local/lib/python3.12/site-packages -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true

COPY . .

EXPOSE 8000

CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]

