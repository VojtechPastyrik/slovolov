<script>
  export let stats = null;
  export let previous = null;
  export let loading = false;
  export let error = '';
  export let mode = 'day';
  export let onClose = () => {};

  function handleBackdrop(e) {
    if (e.target === e.currentTarget) onClose();
  }

  // Bars are scaled against the busiest bucket, not against the total, so a
  // lopsided distribution still shows shape in its smaller columns.
  $: peak = stats ? Math.max(1, ...stats.buckets.map((b) => b.players)) : 1;
  $: previousLabel = mode === 'week' ? 'Minulé týdenní slovo' : 'Včerejší slovo';
</script>

<svelte:window on:keydown={(e) => e.key === 'Escape' && onClose()} />

<div class="backdrop" on:click={handleBackdrop} role="presentation">
  <div class="card" role="dialog" aria-modal="true" aria-labelledby="stats-title">
    <button type="button" class="close" on:click={onClose} aria-label="Zavřít">×</button>
    <h2 id="stats-title" class="title">Statistiky</h2>

    {#if loading}
      <p class="muted">Načítám…</p>
    {:else if error}
      <p class="muted">{error}</p>
    {:else if !stats || stats.players === 0}
      <p class="muted">Tuhle hru zatím nikdo nedohrál. Buď první.</p>
    {:else}
      <div class="figures">
        <div class="figure">
          <span class="value">{stats.players}</span>
          <span class="label">{stats.players === 1 ? 'hráč' : stats.players < 5 ? 'hráči' : 'hráčů'}</span>
        </div>
        <div class="figure">
          <span class="value">{stats.median}</span>
          <span class="label">medián tipů</span>
        </div>
      </div>

      <h3>Rozložení tipů</h3>
      <ul class="hist">
        {#each stats.buckets as bucket}
          <li>
            <span class="range">{bucket.label}</span>
            <span class="bar" style="width: {(bucket.players / peak) * 100}%"
              class:empty={bucket.players === 0}></span>
            <span class="count">{bucket.players}</span>
          </li>
        {/each}
      </ul>

      <p class="note">
        Medián místo průměru schválně: hry se hlásí bez přihlášení, takže pár
        nesmyslných výsledků se vždycky najde. Medián a tvar rozložení jim
        odolají, průměr ani rekordy ne.
      </p>
    {/if}

    {#if previous}
      <h3>{previousLabel}</h3>
      <p class="previous">{previous.word}</p>
    {/if}
  </div>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 25;
    background: rgba(10, 11, 14, 0.72);
    backdrop-filter: blur(6px);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 20px;
    animation: slv-fadein 0.2s ease-out;
  }

  .card {
    position: relative;
    max-width: 420px;
    width: 100%;
    background: #16181e;
    border: 1px solid #2c303a;
    border-radius: 20px;
    padding: 28px 26px 26px;
    box-shadow: 0 24px 70px rgba(0, 0, 0, 0.6);
    color: #e9e9ec;
    animation: slv-pop 0.3s cubic-bezier(0.2, 0.8, 0.2, 1);
    max-height: 90vh;
    overflow-y: auto;
  }

  .close {
    position: absolute;
    top: 10px;
    right: 12px;
    width: 30px;
    height: 30px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: none;
    color: #71757f;
    font-size: 24px;
    line-height: 1;
    border-radius: 8px;
    transition: background 0.15s, color 0.15s;
  }

  .close:hover {
    background: rgba(255, 255, 255, 0.06);
    color: #e9e9ec;
  }

  .title {
    margin: 0 0 16px;
    font-size: 20px;
    font-weight: 700;
    letter-spacing: -0.01em;
  }

  h3 {
    margin: 20px 0 8px;
    font-family: 'Space Mono', monospace;
    font-size: 11px;
    letter-spacing: 0.14em;
    text-transform: uppercase;
    color: #f4b942;
    font-weight: 700;
  }

  .figures {
    display: flex;
    gap: 10px;
  }

  .figure {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 2px;
    background: #1c1f26;
    border: 1px solid #2c2f38;
    border-radius: 14px;
    padding: 12px 14px;
  }

  .value {
    font-family: 'Space Grotesk', sans-serif;
    font-size: 26px;
    font-weight: 700;
    line-height: 1.1;
  }

  .label {
    font-family: 'Space Mono', monospace;
    font-size: 11px;
    color: #8b8e97;
  }

  .hist {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .hist li {
    display: grid;
    grid-template-columns: 52px 1fr auto;
    align-items: center;
    gap: 10px;
  }

  .range,
  .count {
    font-family: 'Space Mono', monospace;
    font-size: 12px;
    color: #8b8e97;
  }

  .count {
    text-align: right;
    min-width: 24px;
  }

  .bar {
    height: 14px;
    min-width: 3px;
    border-radius: 999px;
    background: linear-gradient(90deg, #4ea1ff, #3b82f6);
  }

  .bar.empty {
    background: #262931;
  }

  .note {
    margin: 16px 0 0;
    font-size: 12.5px;
    line-height: 1.5;
    color: #71757f;
  }

  .muted {
    margin: 0;
    font-size: 14.5px;
    line-height: 1.55;
    color: #cfd2d8;
  }

  .previous {
    margin: 0;
    font-family: 'Space Grotesk', sans-serif;
    font-size: 22px;
    font-weight: 700;
    color: #f4b942;
  }
</style>
