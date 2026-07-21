<script>
  import { flip } from 'svelte/animate';
  import { fly } from 'svelte/transition';
  import { cubicOut } from 'svelte/easing';
  import { tick } from 'svelte';

  import Header from './components/Header.svelte';
  import GuessForm from './components/GuessForm.svelte';
  import GuessRow from './components/GuessRow.svelte';
  import WinOverlay from './components/WinOverlay.svelte';
  import RulesModal from './components/RulesModal.svelte';

  import {
    newGame,
    guess as apiGuess,
    hint as apiHint,
    reveal as apiReveal
  } from './lib/api.js';
  import { normalize } from './lib/heat.js';
  import { launchConfetti } from './lib/confetti.js';

  // Toggle to show "Tajné slovo má N písmen." hint under the header.
  const SHOW_LETTER_HINT = false;

  let gameId = null;
  let wordLength = 0;
  let hintsMax = 3;
  let hintsUsed = 0;
  let loading = true;
  let submitting = false;
  let hintLoading = false;
  let error = '';
  let message = '';
  let inputValue = '';
  let guesses = []; // { word, rank, order, hint? }
  let secret = '';
  let revealed = false;
  let showRules = false;
  let formRef;

  $: won = guesses.some((g) => g.rank === 1);
  $: rows = [...guesses].sort((a, b) => a.rank - b.rank);
  $: lastRow = guesses.length > 0 ? guesses[guesses.length - 1] : null;
  $: playerGuesses = guesses.filter((g) => !g.hint);
  $: bestRank = playerGuesses.length
    ? Math.min(...playerGuesses.map((g) => g.rank))
    : 0;
  $: hintsLeft = Math.max(0, hintsMax - hintsUsed);
  $: gameOver = won || revealed;

  async function startNewGame() {
    loading = true;
    error = '';
    message = '';
    guesses = [];
    secret = '';
    revealed = false;
    hintsUsed = 0;
    try {
      const res = await newGame();
      gameId = res.gameId;
      wordLength = res.wordLength || 0;
      hintsMax = res.hintsMax ?? 3;
    } catch (err) {
      error = err.message || String(err);
    } finally {
      loading = false;
      await tick();
      formRef?.focus();
    }
  }

  async function handleGuess(rawWord) {
    if (!gameId || submitting) return;
    const key = normalize(rawWord);
    if (!key) return;

    if (guesses.some((g) => normalize(g.word) === key)) {
      message = 'Tohle slovo už jsi tipoval.';
      return;
    }

    submitting = true;
    message = '';
    try {
      const res = await apiGuess(gameId, rawWord);
      const order = playerGuesses.length + 1;
      guesses = [...guesses, { word: res.word, rank: res.rank, order }];
      if (res.isWin) {
        secret = res.word;
        setTimeout(launchConfetti, 120);
      }
    } catch (err) {
      message = err.message || 'Chyba při odesílání tipu.';
    } finally {
      submitting = false;
    }
  }

  async function handleHint() {
    if (!gameId || hintLoading || hintsLeft <= 0 || gameOver) return;
    hintLoading = true;
    message = '';
    try {
      const exclude = guesses.map((g) => g.word);
      const res = await apiHint(gameId, { bestRank, exclude });
      hintsUsed = res.hintsUsed ?? hintsUsed + 1;
      if (guesses.some((g) => normalize(g.word) === normalize(res.word))) {
        message = 'Nápověda se protla s tvým tipem — zkus další.';
        return;
      }
      guesses = [...guesses, { word: res.word, rank: res.rank, order: 0, hint: true }];
    } catch (err) {
      if (err.status === 409) {
        message = 'Nápovědy už došly.';
      } else {
        message = err.message || 'Chyba při načtení nápovědy.';
      }
    } finally {
      hintLoading = false;
      await tick();
      formRef?.focus();
    }
  }

  async function handleGiveUp() {
    if (!gameId || gameOver) return;
    if (!confirm('Opravdu se chceš vzdát? Zobrazí se tajné slovo.')) return;
    try {
      const res = await apiReveal(gameId);
      secret = res.word;
      revealed = true;
    } catch (err) {
      message = err.message || 'Chyba při zobrazení tajného slova.';
    }
  }

  startNewGame();
</script>

<main class="shell">
  <Header
    guessCount={playerGuesses.length}
    {wordLength}
    showHint={SHOW_LETTER_HINT}
    onNewGame={startNewGame}
    onOpenRules={() => (showRules = true)}
  />

  {#if error}
    <p class="error">{error}</p>
  {/if}

  <GuessForm
    bind:this={formRef}
    bind:value={inputValue}
    disabled={loading || submitting || gameOver}
    onSubmit={handleGuess}
  />

  <div class="actions-row">
    <button
      type="button"
      class="hint-btn"
      disabled={loading || hintLoading || hintsLeft <= 0 || gameOver}
      on:click={handleHint}
    >
      💡 Nápověda <span class="counter">({hintsLeft}/{hintsMax})</span>
    </button>
    <button
      type="button"
      class="giveup-btn"
      disabled={loading || gameOver || playerGuesses.length === 0}
      on:click={handleGiveUp}
    >
      Vzdát se
    </button>
  </div>

  {#if message}
    <p class="msg">{message}</p>
  {/if}

  {#if lastRow && !gameOver}
    <section class="last-tip">
      <div class="label">{lastRow.hint ? 'Poslední nápověda' : 'Poslední tip'}</div>
      {#key `${lastRow.word}-${lastRow.order}-${lastRow.hint ? 'h' : 'g'}`}
        <GuessRow
          word={lastRow.word}
          rank={lastRow.rank}
          order={lastRow.order}
          hint={lastRow.hint}
          variant="last"
        />
      {/key}
    </section>
  {/if}

  {#if rows.length > 0}
    <div class="board-label">Žebříček — od nejteplejšího</div>
    <div class="rows">
      {#each rows as row (row.word)}
        <div
          animate:flip={{ duration: 450, easing: cubicOut }}
          in:fly={{ y: 14, duration: 340, easing: cubicOut }}
        >
          <GuessRow word={row.word} rank={row.rank} order={row.order} hint={row.hint} />
        </div>
      {/each}
    </div>
  {:else if !loading && !error}
    <div class="empty">
      <p>
        Napiš jakékoliv slovo a zjisti, jak blízko jsi tajnému slovu. Čím
        teplejší barva a delší proužek, tím blíž.
      </p>
    </div>
  {/if}
</main>

{#if won}
  <WinOverlay {secret} guessCount={playerGuesses.length} onPlayAgain={startNewGame} />
{:else if revealed}
  <WinOverlay
    {secret}
    guessCount={playerGuesses.length}
    variant="reveal"
    onPlayAgain={startNewGame}
  />
{/if}

{#if showRules}
  <RulesModal onClose={() => (showRules = false)} />
{/if}

<style>
  .shell {
    max-width: 468px;
    margin: 0 auto;
    padding: 0 16px 140px;
    position: relative;
  }

  .error {
    margin: 12px 2px 0;
    font-size: 14px;
    color: #ff8080;
  }

  .msg {
    margin: 10px 2px 0;
    font-size: 13px;
    color: #f0a04b;
    animation: slv-fadein 0.2s ease;
  }

  .actions-row {
    display: flex;
    gap: 8px;
    margin-top: 10px;
  }

  .hint-btn,
  .giveup-btn {
    flex: 1;
    background: #16181e;
    border: 1px solid #2a2e37;
    color: #cfd2d8;
    font-weight: 600;
    font-size: 13px;
    padding: 10px 12px;
    border-radius: 12px;
    transition: background 0.15s, border-color 0.15s, color 0.15s;
  }

  .hint-btn:hover:not(:disabled),
  .giveup-btn:hover:not(:disabled) {
    background: #1c1f26;
    border-color: #3a3e49;
    color: #e9e9ec;
  }

  .hint-btn:disabled,
  .giveup-btn:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }

  .counter {
    font-family: 'Space Mono', monospace;
    font-size: 12px;
    color: #8b8e97;
    margin-left: 4px;
  }

  .giveup-btn {
    color: #a7abb4;
  }

  .last-tip {
    margin-top: 20px;
  }

  .label,
  .board-label {
    font-family: 'Space Mono', monospace;
    font-size: 11px;
    letter-spacing: 0.14em;
    text-transform: uppercase;
    color: #71757f;
  }

  .label {
    margin-bottom: 8px;
  }

  .board-label {
    margin: 24px 0 10px;
  }

  .rows {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .empty {
    margin-top: 40px;
    text-align: center;
    color: #6b6f78;
    animation: slv-fadein 0.3s ease;
  }

  .empty p {
    font-size: 15px;
    line-height: 1.6;
    max-width: 300px;
    margin: 0 auto;
  }
</style>
