#!/usr/bin/env python3
import os
import time
import random
import string
import urllib.parse
import urllib.request

START = 51
END = 56
STYLE = "avataaars-neutral"
RADIUS = 50

def random_seed(length: int = 12) -> str:
    alphabet = string.ascii_lowercase + string.digits
    return "".join(random.choice(alphabet) for _ in range(length))

def build_url(style: str, seed: str, radius: int) -> str:
    q = urllib.parse.urlencode({"seed": seed, "radius": str(radius)})
    return f"https://api.dicebear.com/9.x/{style}/svg?{q}"

def fetch_svg(url: str, timeout: int = 15) -> bytes:
    req = urllib.request.Request(
        url,
        headers={"User-Agent": "avatar-batch/1.0"},
        method="GET",
    )
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        if resp.status != 200:
            raise RuntimeError(f"HTTP {resp.status} for {url}")
        return resp.read()

def main():
    out_dir = os.path.abspath(os.path.dirname(__file__))

    for i in range(START, END + 1):
        seed = random_seed()
        url = build_url(STYLE, seed, RADIUS)
        filename = os.path.join(out_dir, f"a{i}.svg")

        for attempt in range(3):
            try:
                svg = fetch_svg(url)
                with open(filename, "wb") as f:
                    f.write(svg)
                print(f"[{i}] saved {os.path.basename(filename)}  seed={seed}")
                break
            except Exception as e:
                if attempt == 2:
                    print(f"[{i}] FAILED: {e}")
                else:
                    time.sleep(0.6 * (attempt + 1))

if __name__ == "__main__":
    main()
