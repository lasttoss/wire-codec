#!/usr/bin/env python3
"""Generate docs/figures/wire-codec-flow.html.

Measured, not positioned by eye: every box is sized from its own text, so editing a line below
cannot overlap anything. Same shape as the generators in this portfolio, for the same reason.

    python3 docs/figures/wire-codec-flow.generator.py
    python3 <diagram-design>/scripts/self_check.py docs/figures/wire-codec-flow.html
    make diagram
"""
import html
import pathlib

PAPER, INK, MUTED, ACCENT, LINE, TINT, BAD = "#f5f5f5", "#2d3142", "#4f5d75", "#b3541e", "#c9ccd1", "#fdf4ef", "#8c2f1f"
FONT = {"sans": "'Geist', system-ui, sans-serif",
        "mono": "'Geist Mono', ui-monospace, monospace",
        "serif": "'Instrument Serif', serif"}
ADVANCE = {"mono": 0.605, "sans": 0.525, "sans-bold": 0.565}
PAD, GAP = 16, 124
MARGIN = 24


def esc(s):
    return html.escape(str(s))


def width(s, size, family="sans", weight="400"):
    key = "mono" if family == "mono" else ("sans-bold" if weight == "600" else "sans")
    return len(s) * ADVANCE[key] * size


def text(x, y, s, size=12.5, weight="400", fill=INK, family="sans", anchor="start"):
    return (f'<text x="{x:.0f}" y="{y:.0f}" font-family="{FONT[family]}" font-size="{size}" '
            f'font-weight="{weight}" fill="{fill}" text-anchor="{anchor}">{esc(s)}</text>')


def rect(x, y, w, h, fill="#ffffff", stroke=LINE, rx=10):
    return f'<rect x="{x:.0f}" y="{y:.0f}" width="{w:.0f}" height="{h:.0f}" rx="{rx}" fill="{fill}" stroke="{stroke}"/>'


def arrow(x1, y1, x2, y2, label="", dashed=False):
    dash = ' stroke-dasharray="5 4"' if dashed else ""
    # A label wider than the gap between the two endpoints is a label that covers a block, and the
    # covering is invisible in the HTML (it looks fine until the export shows the subtitle gone).
    if label:
        room = abs(x2 - x1) - 16
        need = width(label, 11, "mono") + 12
        if need > room:
            raise SystemExit(f"label {label!r} needs {need:.0f}px, the gap gives {room:.0f}px: shorten it")
    out = [f'<line x1="{x1:.0f}" y1="{y1:.0f}" x2="{x2:.0f}" y2="{y2:.0f}" stroke="{MUTED}" stroke-width="1.6" '
           f'marker-end="url(#arrowhead)"{dash}/>']
    if label:
        mx, my = (x1 + x2) / 2, (y1 + y2) / 2
        lw = width(label, 11, "mono") + 12
        out.append(rect(mx - lw / 2, my - 12, lw, 20, fill=PAPER, stroke="none", rx=5))
        out.append(text(mx, my + 2, label, size=11, fill=MUTED, family="mono", anchor="middle"))
    return "".join(out)


class Block:
    def __init__(self, title, subtitle, lines, tint=None, bad=False):
        self.title, self.subtitle, self.lines, self.tint = title, subtitle, lines, tint
        widths = [width(title, 15, weight="600")]
        if subtitle:
            widths.append(width(subtitle, 11.5, "mono"))
        for line, mono in lines:
            widths.append(width(line, 11.5, "mono" if mono else "sans"))
        self.w = max(widths) + PAD * 2
        self.h = PAD * 2 + 22 + (16 if subtitle else 0) + len(lines) * 17

    def render(self, x, y):
        out = [rect(x, y, self.w, self.h, fill=self.tint or "#ffffff")]
        out.append(text(x + PAD, y + PAD + 12, self.title, size=15, weight="600"))
        yy = y + PAD + 34
        if self.subtitle:
            out.append(text(x + PAD, yy + 4, self.subtitle, size=11.5, fill=MUTED, family="mono"))
            yy += 16
        for line, mono in self.lines:
            out.append(text(x + PAD, yy + 9, line, size=11.5, family="mono" if mono else "sans"))
            yy += 17
        return "".join(out)


# --- column 1: where the truth lives -----------------------------------------
DECL = Block("the one declaration", "schema/ops.yaml - loaded, validated", [
    ("Move 1 · SetColor 2 · Ready 6 · Say 9", True),
    ("Everything 20 - ten logical types,", True),
    ("so a wire-type bug has somewhere to show", True),
    ("refused if it lies: a duplicate opcode, an", False),
    ("unknown type, a repeated field name, a", False),
    ("`raw` that is not last", False),
], tint=TINT)

CANON = Block("the resolution they agree on", "codec.Canonical - the byte wire's", [
    ("1 cm  1/65535 of a turn, exactly", True),
    ("the smallest of the four, so a value", False),
    ("that survives here survives the others", False),
    ("one place to compare instead of four", False),
])

# --- column 2: the four ways to say it ---------------------------------------
BYTE = Block("byte", "big-endian layout, no tags", [
    ("Move   8 B   007dfea200004000", True),
    ("Say   11 B   000978696e206368c3a06f", True),
    ("cost known before it is sent, which is", False),
    ("what a shoot-now protocol needs", False),
])
PROTO = Block("proto", "generated .proto, compiled at run time", [
    ("Move  29 B   Say  11 B", True),
    ("no protoc, no generated Go: a message", False),
    ("added to the declaration is one edit", False),
    ("and a real .proto for C#, TS or Unity", False),
])
FLATBUF = Block("flatbuf", "vtable slots, read with no codegen", [
    ("Move  64 B   Say  36 B", True),
    ("the whole buffer arrives before it is", False),
    ("read and reading allocates nothing -", False),
    ("the case the tick budget cares about", False),
])
JSONC = Block("json", "the reference the others are held to", [
    ("Move  56 B   Say  20 B", True),
    ("not a wire format for players: a way", False),
    ("to write down what the value is, so a", False),
    ("disagreement has something to compare", False),
])

# --- column 3: what keeps them interchangeable ------------------------------
GATE = Block("the interchange gate", "go run ./cmd/wirecodec gate", [
    ("21 checks, 0 failed", True),
    ("4 codecs x 5 messages: JSON in, wire out,", False),
    ("wire back in, JSON out, same value", False),
    ("refused, not guessed: a truncated frame,", False),
    ("trailing bytes, an unknown protobuf field,", False),
    ("an impossible vtable", False),
], tint=TINT)

ARTIFACTS = Block("what a client compiles", "from the same declaration", [
    (".proto  ->  C# · TypeScript · Unity", True),
    (".fbs    ->  flatc, any language", True),
    ("Source(m) returns both; the tests assert", False),
    ("their shape, so the contract has one", False),
    ("author and several readers", False),
])

CLAIMS = [
    "a new message is one edit, not four codecs",
    "interchangeable is a test result, not a promise",
    "a plausible value for a broken frame is the bug",
]

col1 = max(DECL.w, CANON.w)
col2 = max(BYTE.w, PROTO.w, FLATBUF.w, JSONC.w)
col3 = max(GATE.w, ARTIFACTS.w)
total_w = col1 + col2 + col3 + GAP * 2 + MARGIN * 2
x1, x2, x3 = MARGIN, MARGIN + col1 + GAP, MARGIN + col1 + col2 + GAP * 2
y = MARGIN + 8
title_y = y
decl_y = y + 60
canon_y = decl_y + DECL.h + 34
codec_y = y + 60
step = 26
byte_y = codec_y
proto_y = byte_y + BYTE.h + step
flatbuf_y = proto_y + PROTO.h + step
jsonc_y = flatbuf_y + FLATBUF.h + step
gate_y = codec_y
artifacts_y = gate_y + GATE.h + 34
strip_y = max(canon_y + CANON.h, jsonc_y + JSONC.h, artifacts_y + ARTIFACTS.h) + 44
strip_h = 84
H = strip_y + strip_h + MARGIN + 22

svg = [f'<svg viewBox="0 0 {total_w:.0f} {H:.0f}" role="img" aria-labelledby="fig-title fig-desc" xmlns="http://www.w3.org/2000/svg">']
svg.append('<title id="fig-title">wire-codec: one declaration, four wire formats, and the gate that keeps them interchangeable</title>')
svg.append('<desc id="fig-desc">One YAML declaration is loaded and validated: duplicate opcodes, unknown types, repeated field names and a '
           'raw field that is not last are all refused before anything is encoded. Every codec is driven by that declaration and agrees on '
           'one canonical resolution, the byte wire\u2019s 1 cm and 1/65535 of a turn, because it is the smallest of the formats so a value '
           'that survives it survives the others. The byte codec writes a big-endian layout with no tags: the same Move value is 8 bytes, '
           'and Say is 11. The protobuf codec compiles a descriptor from generated .proto text at run time, so no protoc and no generated Go '
           'are needed: Move 29 bytes, Say 11. The FlatBuffers codec slots values by declaration index and the reader walks the vtable itself '
           'with bounds checks: Move 64 bytes, Say 36, and reading allocates nothing. JSON is the reference the others are compared against: '
           'Move 56 bytes, Say 20. The interchange gate runs every declared message through every codec - JSON in, wire out, wire back in, JSON '
           'out - and refuses the faults instead of guessing at them: a truncated frame, trailing bytes, an unknown protobuf field, an impossible '
           'vtable. That is 21 checks, 0 failed, and it is an exit code rather than a paragraph: the Helm chart runs it as a Job with backoffLimit '
           '0. The same declaration also emits the .proto and .fbs a client team compiles with its own toolchain.</desc>')
svg.append(f'<defs><marker id="arrowhead" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" '
           f'orient="auto-start-reverse"><path d="M 0 0 L 10 5 L 0 10 z" fill="{MUTED}"/></marker></defs>')

svg.append(DECL.render(x1, decl_y))
svg.append(CANON.render(x1, canon_y))
svg.append(BYTE.render(x2, byte_y))
svg.append(PROTO.render(x2, proto_y))
svg.append(FLATBUF.render(x2, flatbuf_y))
svg.append(JSONC.render(x2, jsonc_y))
svg.append(GATE.render(x3, gate_y))
svg.append(ARTIFACTS.render(x3, artifacts_y))

svg.append(arrow(x1 + col1, decl_y + 46, x2, byte_y + 40, "-codec=<name>"))
svg.append(arrow(x2 + col2, byte_y + 40, x3, gate_y + 40, "four frames"))

svg.append(text(MARGIN, H - 14, "make test  ·  make mutate  ·  make gate  ·  make chart  -  every number here comes from one of these",
                size=11, fill=MUTED, family="mono"))
svg.append(rect(MARGIN, strip_y, total_w - MARGIN * 2, strip_h))
svg.append(text(MARGIN + PAD, strip_y + 26, "three rules this repository keeps, each one because breaking it produced a wrong number",
                size=12.5, weight="600", fill=MUTED))
chip_w = (total_w - MARGIN * 2 - PAD * 2 - 24) / 3
# A generator that clips a claim is a generator that ships a figure nobody can read, and the clipping is
# invisible in the HTML: it only shows in the export, which is the artefact people look at. This fails
# instead, naming the claim that does not fit.
for i, claim in enumerate(CLAIMS):
    need = width(f"{i + 1}. {claim}", 11.5) + 20
    if need > chip_w:
        raise SystemExit(f"claim {i + 1} needs {need:.0f}px, the strip gives {chip_w:.0f}px: shorten it")
for i, claim in enumerate(CLAIMS):
    cx = MARGIN + PAD + i * (chip_w + 12)
    svg.append(rect(cx, strip_y + 38, chip_w, strip_h - 50, fill="#ffffff", rx=8))
    svg.append(text(cx + 10, strip_y + 58, f"{i + 1}. {claim}", size=11.5, fill=INK))
    svg.append(text(cx + 10, strip_y + 74, ["schema/ops.yaml", "codec/gate_test.go", "codec/codec.go"][i],
                    size=10.5, fill=MUTED, family="mono"))

meta = ('<!-- diagram-meta {"topic":"wire-codec-flow","level":"runtime","type":"data-flow",'
        '"question":"One declaration, four wire formats: what makes them interchangeable, and what refuses a broken frame?",'
        '"sources":["schema/ops.yaml","codec/codec.go","codec/canonical.go","codec/bytecodec/byte.go","codec/jsoncodec/json.go",'
        '"codec/protocodec/proto.go","codec/fbufcodec/flatbuf.go","codec/gate_test.go","cmd/wirecodec/main.go","Makefile"],'
        '"updated":"2026-10-13"} -->')

page = f"""{meta}
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>wire-codec</title>
<link href="https://fonts.googleapis.com/css2?family=Instrument+Serif:ital@0;1&family=Geist:wght@400;500;600&family=Geist+Mono:wght@400;500;600&display=swap" rel="stylesheet">
<style>
  *, *::before, *::after {{ box-sizing: border-box; margin: 0; padding: 0; }}
  :root {{
    --color-paper: {PAPER}; --color-ink: {INK}; --color-muted: {MUTED}; --color-accent: {ACCENT};
    --font-sans: 'Geist', system-ui, sans-serif; --font-serif: 'Instrument Serif', serif;
    --font-mono: 'Geist Mono', ui-monospace, monospace;
  }}
  body {{ font-family: var(--font-sans); background: var(--color-paper); color: var(--color-ink);
         min-height: 100vh; display: flex; align-items: center; justify-content: center; padding: 3rem 2rem; }}
  .frame {{ max-width: {total_w + 40:.0f}px; width: 100%; }}
  .eyebrow {{ font-family: var(--font-mono); font-size: 0.66rem; font-weight: 500; letter-spacing: 0.18em;
              text-transform: uppercase; color: var(--color-muted); margin-bottom: 0.5rem; }}
  h1 {{ font-family: var(--font-serif); font-size: clamp(1.5rem, 2.4vw + 0.75rem, 2rem); font-weight: 400;
        letter-spacing: -0.02em; line-height: 1.15; color: var(--color-ink); margin-bottom: 1.5rem; }}
  .diagram-container {{ width: 100%; overflow-x: auto; }}
  svg {{ width: 100%; max-width: {total_w:.0f}px; display: block; }}
  .caption {{ font-family: var(--font-mono); font-size: 0.7rem; color: var(--color-muted); margin-top: 1rem; }}
</style>
</head>
<body>
<div class="frame">
  <p class="eyebrow">wire-codec · one declaration, three wire formats</p>
  <h1>Four ways to say the same value, and the gate that keeps them from lying</h1>
  <div class="diagram-container">
{"".join(svg)}
  </div>
  <p class="caption">The wire format is usually the decision a game cannot revisit: the client shipped, the protocol is in
  the field, and changing it is a coordinated release. Here it is configuration - the four codecs are held to one canonical
  resolution, and the sizes above are the same Move value in each of them. The gate is the part that makes it a claim
  rather than a claim about a claim: 21 checks across every declared message and every codec, and the four faults it
  refuses instead of decoding into something plausible.</p>
</div>
</body>
</html>
"""
out = pathlib.Path(__file__).with_name("wire-codec-flow.html")
out.write_text(page, encoding="utf-8")
print(f"wrote {out.name}: {total_w:.0f}x{H:.0f} viewBox, {len(page)} bytes")
