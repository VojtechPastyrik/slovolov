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
  import StatsModal from './components/StatsModal.svelte';

  import {
    current as apiCurrent,
    guess as apiGuess,
    hint as apiHint,
    reveal as apiReveal,
    stats as apiStats,
    previous as apiPrevious
  } from './lib/api.js';
  import { normalize } from './lib/heat.js';
  import { launchConfetti } from './lib/confetti.js';
  import { load as loadState, save as saveState, clear as clearState } from './lib/persistence.js';

  // Toggle to show "Tajné slovo má N písmen." hint under the header.
  const SHOW_LETTER_HINT = false;

  let mode = 'day'; // 'day' | 'week'
  let gameId = null;
  let resetsAt = null;
  let countdown = '';
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
  let overlayDismissed = false;
  let formRef;
  // Guess tally as counted by the server. It is the number that feeds the
  // stats, so it is the one worth showing; the local list is only a fallback
  // for a restored game the server has not answered for yet.
  let serverGuesses = 0;
  let stats = null;
  let previousPuzzle = null;
  let showStats = false;
  let statsLoading = false;
  let statsError = '';

  $: won = guesses.some((g) => g.rank === 1);
  $: rows = [...guesses].sort((a, b) => a.rank - b.rank);
  $: lastRow = guesses.length > 0 ? guesses[guesses.length - 1] : null;
  $: playerGuesses = guesses.filter((g) => !g.hint);
  $: bestRank = playerGuesses.length
    ? Math.min(...playerGuesses.map((g) => g.rank))
    : 0;
  $: guessCount = serverGuesses || playerGuesses.length;
  $: hintsLeft = Math.max(0, hintsMax - hintsUsed);
  $: gameOver = won || revealed;
  $: puzzleDate = gameId ? formatDate(gameId) : '';

  // Persist on every mutation so tab close / reload keeps the game.
  $: if (gameId && !loading) {
    saveState(mode, { gameId, wordLength, hintsMax, hintsUsed, guesses, secret, revealed });
  }

  // Countdown to the next word, shown once the day is done.
  $: if (gameOver && resetsAt) {
    startCountdown();
  }

  let countdownTimer = null;

  function startCountdown() {
    if (countdownTimer) return;
    const tickCountdown = () => {
      const left = resetsAt - Date.now();
      if (left <= 0) {
        countdown = '';
        clearInterval(countdownTimer);
        countdownTimer = null;
        loadCurrent();
        return;
      }
      const h = Math.floor(left / 3600000);
      const m = Math.floor((left % 3600000) / 60000);
      const sec = Math.floor((left % 60000) / 1000);
      countdown = `${h}:${String(m).padStart(2, '0')}:${String(sec).padStart(2, '0')}`;
    };
    tickCountdown();
    countdownTimer = setInterval(tickCountdown, 1000);
  }

  function formatDate(iso) {
    const [y, m, d] = iso.split('-').map(Number);
    if (!y || !m || !d) return '';
    return `${d}. ${m}. ${y}`;
  }

  // One word per day: the server hands out today's puzzle id, and saved
  // progress is only restored when it belongs to that same day.
  async function loadCurrent() {
    loading = true;
    error = '';
    message = '';
    try {
      const res = await apiCurrent(mode);
      const saved = loadState(mode);
      gameId = res.gameId;
      wordLength = res.wordLength || 0;
      hintsMax = res.hintsMax ?? 3;
      resetsAt = res.resetsAt ? Date.parse(res.resetsAt) : null;

      if (saved && saved.gameId === res.gameId) {
        hintsUsed = saved.hintsUsed ?? 0;
        guesses = Array.isArray(saved.guesses) ? saved.guesses : [];
        secret = saved.secret || '';
        revealed = !!saved.revealed;
      } else {
        clearState(mode);
        hintsUsed = 0;
        guesses = [];
        secret = '';
        revealed = false;
        overlayDismissed = false;
      }
    } catch (err) {
      // 503 = the day's puzzle is still being built; retry shortly.
      if (err.status === 503) {
        error = '';
        message = 'Slovo se připravuje — zkusím to za chvíli znovu.';
        setTimeout(loadCurrent, 15000);
      } else {
        error = err.message || String(err);
      }
    } finally {
      loading = false;
      await tick();
      formRef?.focus();
    }
  }

  // Stats and the previous word are both public and cheap, so they are fetched
  // on demand rather than kept in sync with the board.
  async function loadStats() {
    if (!gameId) return;
    statsLoading = true;
    statsError = '';
    try {
      stats = await apiStats(gameId);
    } catch (err) {
      statsError = err.message || 'Statistiky se nepodařilo načíst.';
    } finally {
      statsLoading = false;
    }
  }

  async function loadPrevious() {
    try {
      previousPuzzle = await apiPrevious(mode);
    } catch {
      // A brand new install has no finished puzzle yet; that is not an error
      // worth showing.
      previousPuzzle = null;
    }
  }

  function openStats() {
    showStats = true;
    loadStats();
    loadPrevious();
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
      serverGuesses = res.guesses || serverGuesses;
      // The model folds inflections ("psa" → "pes"), so a word already on the
      // board can come back under a spelling the check above could not match.
      // The server recognises it by the canonical form and says so; it does not
      // raise the tally either.
      if (res.repeat && !res.isWin) {
        message = 'Tohle slovo už jsi tipoval.';
        return;
      }
      const order = playerGuesses.length + 1;
      guesses = [...guesses, { word: res.word, rank: res.rank, order }];
      if (res.isWin) {
        secret = res.word;
        setTimeout(launchConfetti, 120);
        loadStats();
      }
    } catch (err) {
      // 404 means the day rolled over (or the cache was wiped) while the tab
      // was open — pick up the current puzzle instead of retrying a dead id.
      if (err.status === 404) {
        message = 'Hra se přepnula na aktuální slovo.';
        await loadCurrent();
      } else if (err.status === 422) {
        message = 'Tohle slovo neznám — zkus jiné.';
      } else {
        message = err.message || 'Chyba při odesílání tipu.';
      }
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
      hintsUsed += 1;
      if (guesses.some((g) => normalize(g.word) === normalize(res.word))) {
        message = 'Nápověda se protla s tvým tipem — zkus další.';
        return;
      }
      guesses = [...guesses, { word: res.word, rank: res.rank, order: 0, hint: true }];
    } catch (err) {
      message = err.message || 'Chyba při načtení nápovědy.';
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

  // Switching modes swaps the whole board; each keeps its own saved progress.
  async function switchMode(next) {
    if (next === mode || loading) return;
    mode = next;
    if (countdownTimer) {
      clearInterval(countdownTimer);
      countdownTimer = null;
    }
    countdown = '';
    overlayDismissed = false;
    guesses = [];
    secret = '';
    revealed = false;
    hintsUsed = 0;
    await loadCurrent();
  }

  loadCurrent();
</script>

<main class="shell">
  <Header
    guessCount={guessCount}
    {wordLength}
    showHint={SHOW_LETTER_HINT}
    {puzzleDate}
    hintsLeft={hintsLeft}
    hintsMax={hintsMax}
    hintDisabled={loading || hintLoading || hintsLeft <= 0 || gameOver}
    onHint={handleHint}
    onOpenRules={() => (showRules = true)}
    onOpenStats={openStats}
  />

  <div class="mode-switch" role="group" aria-label="Režim hry">
    <button
      type="button"
      class:active={mode === 'day'}
      disabled={loading}
      on:click={() => switchMode('day')}
    >Denní</button>
    <button
      type="button"
      class:active={mode === 'week'}
      disabled={loading}
      on:click={() => switchMode('week')}
    >Týdenní <span class="tag">těžší</span></button>
  </div>

  {#if error}
    <p class="error">{error}</p>
  {/if}

  <GuessForm
    bind:this={formRef}
    bind:value={inputValue}
    disabled={loading || gameOver}
    busy={submitting}
    onSubmit={handleGuess}
  />

  <div class="actions-row">
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

  {#if gameOver && countdown}
    <p class="next-word">Další slovo za {countdown}</p>
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

{#if gameOver && !overlayDismissed}
  <WinOverlay
    {secret}
    guessCount={playerGuesses.length}
    variant={won ? 'win' : 'reveal'}
    {countdown}
    onClose={() => (overlayDismissed = true)}
  />
{/if}

{#if showStats}
  <StatsModal
    {stats}
    {mode}
    previous={previousPuzzle}
    loading={statsLoading}
    error={statsError}
    onClose={() => (showStats = false)}
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

  .mode-switch {
    display: flex;
    gap: 6px;
    justify-content: center;
    margin: 0 0 14px;
  }

  .mode-switch button {
    background: #14161b;
    border: 1px solid #262931;
    border-radius: 999px;
    padding: 7px 16px;
    color: #8b8e97;
    font-weight: 600;
    font-size: 13px;
    cursor: pointer;
    transition: background 0.15s, border-color 0.15s, color 0.15s;
  }

  .mode-switch button.active {
    background: #1f232b;
    border-color: #3a3e49;
    color: #e9e9ec;
  }

  .mode-switch button:disabled {
    cursor: default;
    opacity: 0.6;
  }

  .mode-switch .tag {
    font-family: 'Space Mono', monospace;
    font-size: 10.5px;
    color: #8b8e97;
  }

  .next-word {
    margin: 6px 0 0;
    text-align: center;
    font-family: 'Space Mono', monospace;
    font-size: 13px;
    color: #8b8e97;
  }

  .actions-row {
    display: flex;
    gap: 8px;
    margin-top: 10px;
  }

  /* Giving up is the only control left down here: it ends the game, so it
     stays away from the header's navigation and keeps its own full-width row. */
  .giveup-btn {
    flex: 1;
    background: #16181e;
    border: 1px solid #2a2e37;
    color: #a7abb4;
    font-weight: 600;
    font-size: 13px;
    padding: 10px 12px;
    border-radius: 12px;
    transition: background 0.15s, border-color 0.15s, color 0.15s;
  }

  .giveup-btn:hover:not(:disabled) {
    background: #1c1f26;
    border-color: #3a3e49;
    color: #e9e9ec;
  }

  .giveup-btn:disabled {
    opacity: 0.45;
    cursor: not-allowed;
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
