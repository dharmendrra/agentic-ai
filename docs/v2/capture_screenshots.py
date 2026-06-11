#!/usr/bin/env python3
"""Capture HD showcase screenshots for docs/screenshots/ (Go UI, dark/emerald).

Agent must be running (dev instance on :8092; default port is :8082 — adjust BASE).
  pip install playwright && python -m playwright install chromium
  python docs/capture_screenshots.py

1600x1000 viewport @ device_scale_factor=2 => 3200x2000 PNGs.
Answer shots reopen existing conversations (no slow live LLM needed).
"""
from __future__ import annotations

import asyncio
import os
import pathlib
from playwright.async_api import async_playwright

BASE = os.environ.get("BASE", "http://localhost:8092")
OUT = pathlib.Path(__file__).parent / "screenshots"
OUT.mkdir(parents=True, exist_ok=True)
LIBRARY_CONVO = "throne whe"     # a GoT library (pdf) conversation
WEB_CONVO = "throne? searc"      # a web conversation with persisted citations


async def shot(page, name):
    await page.screenshot(path=str(OUT / f"{name}.png"), full_page=False)
    print(f"  wrote screenshots/{name}.png")


async def reopen(page, text):
    await page.click(f'#convo-list >> text={text}', timeout=8000)


async def main():
    async with async_playwright() as p:
        browser = await p.chromium.launch()
        ctx = await browser.new_context(viewport={"width": 1600, "height": 1000},
                                        device_scale_factor=2)
        page = await ctx.new_page()

        await page.goto(BASE, wait_until="networkidle")
        await page.wait_for_timeout(700)
        await shot(page, "chat-empty")

        try:
            await page.click("#toggle-web"); await page.click("#toggle-library")
            await page.wait_for_timeout(250)
            await shot(page, "toggles-on")
        except Exception as e:
            print(f"  [skip] toggles-on: {e}")

        # answer from My Library (pdf)
        try:
            await reopen(page, LIBRARY_CONVO)
            await page.wait_for_selector("#thread .answer-text", timeout=15000)
            await page.wait_for_timeout(500)
            await shot(page, "answer-library")
            await shot(page, "sidebar")  # sidebar visible alongside
        except Exception as e:
            print(f"  [skip] answer-library: {e}")
            await shot(page, "sidebar")

        # answer with clickable Web citations
        try:
            await reopen(page, WEB_CONVO)
            await page.wait_for_selector('#thread a[target="_blank"]', timeout=15000)
            await page.wait_for_timeout(500)
            await shot(page, "answer-web")
        except Exception as e:
            print(f"  [skip] answer-web: {e}")

        # ingest page
        await page.goto(BASE + "/ingest.html", wait_until="networkidle")
        await page.wait_for_timeout(500)
        await shot(page, "ingest")

        await browser.close()
    print("Done -> docs/screenshots/")


if __name__ == "__main__":
    asyncio.run(main())
