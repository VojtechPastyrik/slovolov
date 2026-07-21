const COLORS = ['#4ea1ff', '#f4b942', '#ff5a4d', '#3ddc97', '#c77dff'];

// Port of the design prototype's confetti routine: particles burst upward
// from ~35% viewport height, spread across ~30% of the width.
export function launchConfetti() {
  const canvas = document.createElement('canvas');
  canvas.style.cssText =
    'position:fixed;inset:0;width:100%;height:100%;pointer-events:none;z-index:9999';
  document.body.appendChild(canvas);

  const ctx = canvas.getContext('2d');
  const dpr = Math.min(window.devicePixelRatio || 1, 2);
  const W = (canvas.width = window.innerWidth * dpr);
  const H = (canvas.height = window.innerHeight * dpr);

  const N = 160;
  const particles = [];
  for (let i = 0; i < N; i++) {
    particles.push({
      x: W / 2 + (Math.random() - 0.5) * W * 0.3,
      y: H * 0.35,
      vx: (Math.random() - 0.5) * 16 * dpr,
      vy: (Math.random() * -16 - 4) * dpr,
      g: 0.5 * dpr,
      r: (4 + Math.random() * 5) * dpr,
      c: COLORS[i % COLORS.length],
      a: 1,
      rot: Math.random() * 6,
      vr: (Math.random() - 0.5) * 0.4
    });
  }

  let start = null;
  function step(t) {
    if (!start) start = t;
    const dt = t - start;
    ctx.clearRect(0, 0, W, H);
    for (const p of particles) {
      p.vy += p.g;
      p.x += p.vx;
      p.y += p.vy;
      p.vx *= 0.99;
      p.rot += p.vr;
      if (dt > 1400) p.a -= 0.02;
      ctx.save();
      ctx.globalAlpha = Math.max(0, p.a);
      ctx.translate(p.x, p.y);
      ctx.rotate(p.rot);
      ctx.fillStyle = p.c;
      ctx.fillRect(-p.r, -p.r * 0.6, p.r * 2, p.r * 1.2);
      ctx.restore();
    }
    if (dt < 2600) requestAnimationFrame(step);
    else canvas.remove();
  }
  requestAnimationFrame(step);
}
