import re
import math
import hashlib
import random
import asyncio
import logging
from urllib.parse import urljoin, urlparse

import httpx
from bs4 import BeautifulSoup

logger = logging.getLogger(__name__)

CHUNK_SIZE    = 400
CHUNK_OVERLAP = 80
MAX_PAGES     = 300
MAX_PAGE_BYTES = 2 * 1024 * 1024   # 2 MB per page — skip oversized pages
REQUEST_TIMEOUT = 15
CRAWL_DELAY   = 0.3                 # seconds between requests


def get_embedding(text: str) -> list[float]:
    h    = hashlib.sha256(text.encode()).digest()
    seed = int.from_bytes(h[:8], "big")
    rng  = random.Random(seed)
    vec  = [rng.gauss(0, 1) for _ in range(384)]
    norm = math.sqrt(sum(x * x for x in vec)) or 1.0
    return [x / norm for x in vec]


def chunk_text(text: str, chunk_size: int = CHUNK_SIZE, overlap: int = CHUNK_OVERLAP) -> list[str]:
    words  = text.split()
    chunks = []
    i = 0
    while i < len(words):
        chunk = " ".join(words[i: i + chunk_size])
        if chunk.strip():
            chunks.append(chunk)
        i += chunk_size - overlap
    return chunks


def extract_text(html: str) -> tuple[str, str]:
    soup  = BeautifulSoup(html, "html.parser")
    title = (soup.title.string or "").strip()
    for tag in soup(["nav", "footer", "header", "script", "style", "noscript", "aside"]):
        tag.decompose()
    main = soup.find("main") or soup.find("article") or soup.find("body")
    text = main.get_text(separator="\n", strip=True) if main else ""
    text = re.sub(r"\n{3,}", "\n\n", text)
    return title, text


def is_same_domain(base_url: str, url: str) -> bool:
    return urlparse(url).netloc == urlparse(base_url).netloc


def is_safe_url(url: str) -> bool:
    """Reject private/internal addresses that should never be crawled."""
    parsed = urlparse(url)
    if parsed.scheme not in ("http", "https"):
        return False
    host = parsed.netloc.split(":")[0].lower()
    blocked_prefixes = ("localhost", "127.", "10.", "192.168.", "172.16.",
                        "169.254.", "0.0.0.0", "::1", "[::1]")
    return not any(host == b.rstrip(".") or host.startswith(b) for b in blocked_prefixes)


async def crawl_site(start_url: str, collection) -> int:
    visited: set[str] = set()
    queue   = [start_url]
    total   = 0
    base    = start_url.rstrip("/")

    headers = {
        "User-Agent": "vjailbreak-docs-bot/1.0 (internal knowledge-base crawler)",
        "Accept": "text/html",
    }

    async with httpx.AsyncClient(
        timeout=REQUEST_TIMEOUT,
        follow_redirects=True,
        headers=headers,
        max_redirects=5,
    ) as client:
        while queue and len(visited) < MAX_PAGES:
            url = queue.pop(0)
            if url in visited or not is_safe_url(url):
                continue
            visited.add(url)

            try:
                resp = await client.get(url)
            except Exception as exc:
                logger.warning("Skipping %s: %s", url, exc)
                continue

            if resp.status_code != 200:
                continue
            content_type = resp.headers.get("content-type", "")
            if "text/html" not in content_type:
                continue
            if len(resp.content) > MAX_PAGE_BYTES:
                logger.warning("Skipping oversized page: %s (%d bytes)", url, len(resp.content))
                continue

            title, text = extract_text(resp.text)
            if not text.strip():
                continue

            chunks = chunk_text(text)
            ids, docs, metas, embeds = [], [], [], []
            for i, chunk in enumerate(chunks):
                cid = hashlib.sha256(f"{url}:{i}".encode()).hexdigest()[:32]
                ids.append(cid)
                docs.append(chunk)
                metas.append({"url": url, "title": title})
                embeds.append(get_embedding(chunk))

            if ids:
                collection.upsert(ids=ids, documents=docs, metadatas=metas, embeddings=embeds)
                total += len(ids)
                logger.info("Indexed %s — %d chunks (total: %d)", url, len(ids), total)

            soup = BeautifulSoup(resp.text, "html.parser")
            for a in soup.find_all("a", href=True):
                href = a["href"].strip()
                if not href or href.startswith("#") or href.startswith("mailto:"):
                    continue
                full = urljoin(url, href).split("#")[0].split("?")[0]
                if is_same_domain(base, full) and full not in visited and is_safe_url(full):
                    queue.append(full)

            await asyncio.sleep(CRAWL_DELAY)

    logger.info("Crawl complete — %d total chunks from %d pages", total, len(visited))
    return total
