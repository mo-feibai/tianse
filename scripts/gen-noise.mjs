// Generates the grain tile the cards are overlaid with.
//
// The grain is not decoration: a perfectly flat dark surface reads as a rendered
// rectangle, while a few levels of noise make it read as a material. It is
// generated here rather than drawn by hand so it is reproducible and stays out of
// the diff.
import { deflateSync } from "node:zlib";
import { writeFileSync, mkdirSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const SIZE = 128;
const LOW = 118;
const HIGH = 138;

const here = dirname(fileURLToPath(import.meta.url));
const out = resolve(here, "../frontend/public/textures/noise-128.png");

function crc32(buf) {
  let c = ~0;
  for (let i = 0; i < buf.length; i++) {
    c ^= buf[i];
    for (let k = 0; k < 8; k++) c = (c >>> 1) ^ (0xedb88320 & -(c & 1));
  }
  return ~c >>> 0;
}

function chunk(type, data) {
  const len = Buffer.alloc(4);
  len.writeUInt32BE(data.length);
  const body = Buffer.concat([Buffer.from(type, "ascii"), data]);
  const crc = Buffer.alloc(4);
  crc.writeUInt32BE(crc32(body));
  return Buffer.concat([len, body, crc]);
}

// A cheap value-noise field: smooth enough to survive tiling without visible
// seams, since every octave wraps at the tile edge.
function value(x, y, period) {
  const xi = Math.floor(x);
  const yi = Math.floor(y);
  const xf = x - xi;
  const yf = y - yi;
  const wrap = (v) => ((v % period) + period) % period;
  const h = (a, b) => {
    let n = wrap(a) * 374761393 + wrap(b) * 668265263;
    n = (n ^ (n >> 13)) * 1274126177;
    return ((n ^ (n >> 16)) >>> 0) / 4294967295;
  };
  const s = (t) => t * t * (3 - 2 * t);
  const u = s(xf);
  const v = s(yf);
  const a = h(xi, yi);
  const b = h(xi + 1, yi);
  const c = h(xi, yi + 1);
  const d = h(xi + 1, yi + 1);
  return a * (1 - u) * (1 - v) + b * u * (1 - v) + c * (1 - u) * v + d * u * v;
}

const raw = Buffer.alloc(SIZE * (SIZE + 1));
let o = 0;
for (let y = 0; y < SIZE; y++) {
  raw[o++] = 0;
  for (let x = 0; x < SIZE; x++) {
    let n = 0;
    let amp = 0.5;
    let period = 4;
    let total = 0;
    for (let oct = 0; oct < 4; oct++) {
      const p = period;
      n += value((x / SIZE) * p, (y / SIZE) * p, p) * amp;
      total += amp;
      amp *= 0.5;
      period *= 2;
    }
    n /= total;
    raw[o++] = Math.round(LOW + n * (HIGH - LOW));
  }
}

const ihdr = Buffer.alloc(13);
ihdr.writeUInt32BE(SIZE, 0);
ihdr.writeUInt32BE(SIZE, 4);
ihdr[8] = 8; // bit depth
ihdr[9] = 0; // greyscale
ihdr[10] = 0;
ihdr[11] = 0;
ihdr[12] = 0;

const png = Buffer.concat([
  Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
  chunk("IHDR", ihdr),
  chunk("IDAT", deflateSync(raw, { level: 9 })),
  chunk("IEND", Buffer.alloc(0)),
]);

mkdirSync(dirname(out), { recursive: true });
writeFileSync(out, png);
console.log("wrote", out, png.length, "bytes");
