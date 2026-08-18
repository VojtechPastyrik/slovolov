<script>
  import { guessesPlural, lettersPlural } from '../lib/heat.js';
  export let guessCount = 0;
  export let wordLength = 0;
  export let showHint = false;
  export let puzzleDate = '';
  export let hintsLeft = 0;
  export let hintsMax = 3;
  export let hintDisabled = false;
  export let onHint = () => {};
  export let onOpenRules = () => {};
  export let onOpenStats = () => {};
</script>

<header class="hdr">
  <div class="brand">
    <span class="wordmark">Slovolov</span>
    <span class="subtitle">hádej tajné slovo</span>
  </div>

  <div class="bar">
    <span class="pill">{guessesPlural(guessCount)}</span>

    <button type="button" class="hint-btn" disabled={hintDisabled} on:click={onHint}>
      <span aria-hidden="true">💡</span>
      <span class="hint-label">Nápověda</span>
      <span class="counter">{hintsLeft}/{hintsMax}</span>
    </button>

    <button
      type="button"
      class="icon-btn"
      on:click={onOpenStats}
      aria-label="Statistiky"
      title="Statistiky"
    >
      <svg viewBox="0 0 16 16" aria-hidden="true">
        <rect x="1" y="9" width="3.4" height="6" rx="1" />
        <rect x="6.3" y="5" width="3.4" height="10" rx="1" />
        <rect x="11.6" y="1" width="3.4" height="14" rx="1" />
      </svg>
    </button>

    <button
      type="button"
      class="icon-btn"
      on:click={onOpenRules}
      aria-label="Pravidla"
      title="Pravidla"
    >?</button>

    {#if puzzleDate}<span class="pill date">{puzzleDate}</span>{/if}
  </div>

  {#if showHint && wordLength > 0}
    <p class="letters">Tajné slovo má {lettersPlural(wordLength)}.</p>
  {/if}
</header>

<style>
  .hdr {
    position: sticky;
    top: 0;
    z-index: 6;
    padding: 14px 0 12px;
    background: linear-gradient(#0f1013 72%, rgba(15, 16, 19, 0));
    backdrop-filter: blur(2px);
  }

  /* The brand keeps its own line so the tagline never competes with the
     controls for width — it used to be dropped entirely on narrow screens. */
  .brand {
    display: flex;
    align-items: baseline;
    gap: 9px;
    min-width: 0;
  }

  .wordmark {
    flex: none;
    font-family: 'Space Grotesk', sans-serif;
    font-weight: 700;
    font-size: 22px;
    letter-spacing: -0.02em;
    background: linear-gradient(120deg, #4ea1ff, #f4b942 55%, #ff5a4d);
    -webkit-background-clip: text;
    background-clip: text;
    color: transparent;
    -webkit-text-fill-color: transparent;
  }

  .subtitle {
    font-family: 'Space Mono', monospace;
    font-size: 11px;
    color: #71757f;
    letter-spacing: 0.04em;
    white-space: nowrap;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .bar {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 7px;
    margin-top: 10px;
  }

  .pill {
    font-family: 'Space Mono', monospace;
    font-size: 12.5px;
    color: #8b8e97;
    border: 1px solid #262931;
    border-radius: 999px;
    padding: 6px 10px;
    white-space: nowrap;
  }

  /* The date is the least urgent thing here, so it is the one that gets
     pushed to the far end and dropped first. */
  .date {
    margin-left: auto;
  }

  .icon-btn {
    width: 30px;
    height: 30px;
    flex: none;
    display: flex;
    align-items: center;
    justify-content: center;
    background: #1c1f26;
    border: 1px solid #2c2f38;
    border-radius: 999px;
    color: #a7abb4;
    font-family: 'Space Mono', monospace;
    font-weight: 700;
    font-size: 14px;
    transition: background 0.15s, border-color 0.15s, color 0.15s;
  }

  .icon-btn svg {
    width: 14px;
    height: 14px;
    fill: currentColor;
  }

  .icon-btn:hover {
    background: #262a33;
    border-color: #3a3e49;
    color: #e9e9ec;
  }

  .hint-btn {
    display: flex;
    align-items: center;
    gap: 6px;
    flex: none;
    background: #1c1f26;
    border: 1px solid #2c2f38;
    border-radius: 999px;
    padding: 6px 12px;
    color: #cfd2d8;
    font-size: 13px;
    font-weight: 600;
    white-space: nowrap;
    transition: background 0.15s, border-color 0.15s, color 0.15s;
  }

  .hint-btn:hover:not(:disabled) {
    background: #262a33;
    border-color: #3a3e49;
    color: #fff;
  }

  .hint-btn:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }

  .counter {
    font-family: 'Space Mono', monospace;
    font-size: 11.5px;
    color: #8b8e97;
  }

  .letters {
    margin: 10px 0 0;
    font-family: 'Space Mono', monospace;
    font-size: 12px;
    color: #7f838d;
  }

  /* Below this the five controls stop fitting on one line. The bulb and the
     counter still say what the button is, so only the word gives way. */
  @media (max-width: 430px) {
    .hint-label {
      display: none;
    }

    .hint-btn {
      padding: 6px 10px;
    }
  }
</style>
