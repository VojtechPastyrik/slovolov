<script>
  import { guessesPlural } from '../lib/heat.js';
  export let secret = '';
  export let guessCount = 0;
  export let variant = 'win'; // 'win' | 'reveal'
  export let countdown = '';
  export let onClose = () => {};

  $: eyebrow = variant === 'reveal' ? 'VZDAL SES' : 'UHODL JSI!';
  $: subtitle =
    variant === 'reveal'
      ? guessCount > 0
        ? `Zvládl jsi ${guessesPlural(guessCount)} než ses vzdal.`
        : 'Zkus to znovu — třeba to příště dáš.'
      : `Trefa na ${guessesPlural(guessCount)}.`;
</script>

<div class="overlay">
  <div class="card" class:reveal={variant === 'reveal'}>
    <span class="eyebrow">{eyebrow}</span>
    <h1 class="secret">{secret}</h1>
    <p class="subtitle">{subtitle}</p>
    {#if countdown}
      <p class="next">Další slovo za {countdown}</p>
    {/if}
    <button type="button" class="again" on:click={onClose}>Zavřít</button>
  </div>
</div>

<style>
  .next {
    margin: 0 0 14px;
    font-family: 'Space Mono', monospace;
    font-size: 13px;
    color: #8b8e97;
  }

  .overlay {
    position: fixed;
    inset: 0;
    z-index: 20;
    background: rgba(10, 11, 14, 0.72);
    backdrop-filter: blur(6px);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 20px;
    animation: slv-fadein 0.3s ease-out;
  }

  .card {
    max-width: 360px;
    width: 100%;
    background: #16181e;
    border: 1px solid #2c303a;
    border-radius: 22px;
    padding: 34px 26px;
    box-shadow: 0 24px 70px rgba(0, 0, 0, 0.6);
    text-align: center;
    animation: slv-pop 0.4s cubic-bezier(0.2, 0.8, 0.2, 1);
  }

  .card.reveal .eyebrow {
    color: #a7abb4;
  }

  .eyebrow {
    display: block;
    font-family: 'Space Mono', monospace;
    font-size: 12px;
    letter-spacing: 0.18em;
    color: #f4b942;
  }

  .secret {
    margin: 10px 0 6px;
    font-family: 'Space Grotesk', sans-serif;
    font-size: 40px;
    font-weight: 700;
    letter-spacing: -0.03em;
    background: linear-gradient(120deg, #4ea1ff, #f4b942 55%, #ff5a4d);
    -webkit-background-clip: text;
    background-clip: text;
    color: transparent;
    -webkit-text-fill-color: transparent;
    word-break: break-word;
  }

  .subtitle {
    margin: 0;
    color: #a7abb4;
    font-size: 15px;
    line-height: 1.5;
  }

  .again {
    margin-top: 22px;
    width: 100%;
    background: linear-gradient(150deg, #4ea1ff, #3b82f6);
    color: #fff;
    font-weight: 700;
    font-size: 16px;
    padding: 14px;
    border-radius: 14px;
    border: none;
    box-shadow: 0 8px 22px rgba(59, 130, 246, 0.4);
    transition: filter 0.15s, transform 0.05s;
  }

  .again:hover {
    filter: brightness(1.08);
  }

  .again:active {
    transform: translateY(1px);
  }
</style>
