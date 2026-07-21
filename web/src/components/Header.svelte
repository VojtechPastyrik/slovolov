<script>
  import { guessesPlural, lettersPlural } from '../lib/heat.js';
  export let guessCount = 0;
  export let wordLength = 0;
  export let showHint = false;
  export let onNewGame = () => {};
  export let onOpenRules = () => {};
</script>

<header class="hdr">
  <div class="row">
    <div class="brand">
      <span class="wordmark">Slovolov</span>
      <span class="subtitle">hádej tajné slovo</span>
    </div>
    <div class="actions">
      <span class="pill">{guessesPlural(guessCount)}</span>
      <button
        type="button"
        class="icon-btn"
        on:click={onOpenRules}
        aria-label="Pravidla"
        title="Pravidla"
      >?</button>
      <button type="button" class="new" on:click={onNewGame}>Nová hra</button>
    </div>
  </div>
  {#if showHint && wordLength > 0}
    <p class="hint">Tajné slovo má {lettersPlural(wordLength)}.</p>
  {/if}
</header>

<style>
  .hdr {
    position: sticky;
    top: 0;
    z-index: 6;
    padding: 16px 0 12px;
    background: linear-gradient(#0f1013 62%, rgba(15, 16, 19, 0));
    backdrop-filter: blur(2px);
  }

  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .brand {
    display: flex;
    align-items: baseline;
    gap: 9px;
    min-width: 0;
  }

  .wordmark {
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
  }

  .actions {
    display: flex;
    align-items: center;
    gap: 8px;
    flex: none;
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

  .icon-btn {
    width: 30px;
    height: 30px;
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

  .icon-btn:hover {
    background: #262a33;
    border-color: #3a3e49;
    color: #e9e9ec;
  }

  .new {
    background: #1c1f26;
    border: 1px solid #2c2f38;
    border-radius: 999px;
    padding: 7px 13px;
    color: #e9e9ec;
    font-weight: 600;
    font-size: 13px;
    transition: background 0.15s, border-color 0.15s;
  }

  .new:hover {
    background: #262a33;
    border-color: #3a3e49;
  }

  .hint {
    margin: 10px 0 0;
    font-family: 'Space Mono', monospace;
    font-size: 12px;
    color: #7f838d;
  }
</style>
