<script>
  export let onClose = () => {};

  function handleBackdrop(e) {
    if (e.target === e.currentTarget) onClose();
  }
</script>

<svelte:window on:keydown={(e) => e.key === 'Escape' && onClose()} />

<div class="backdrop" on:click={handleBackdrop} role="presentation">
  <div class="card" role="dialog" aria-modal="true" aria-labelledby="rules-title">
    <button type="button" class="close" on:click={onClose} aria-label="Zavřít">×</button>
    <h2 id="rules-title" class="title">Jak Slovolov funguje</h2>
    <p class="lead">
      Hádáš tajné české slovo. Po každém tipu vidíš, jak je významově blízko —
      čím teplejší barva a delší proužek, tím blíž.
    </p>
    <h3>Jaká slova zkoušet</h3>
    <p>
      Slovník obsahuje pouze <strong>podstatná jména v 1. pádě jednotného čísla</strong> —
      například <em>pes, stůl, láska, moře</em>.
    </p>
    <h3>Co nefunguje</h3>
    <ul>
      <li>slovesa (běhat, spát)</li>
      <li>přídavná jména (červený, velký)</li>
      <li>jména osob a míst (Praha, Karel)</li>
      <li>čísla a zájmena (pět, ona)</li>
      <li>množné číslo a jiné pády (psi, psovi)</li>
    </ul>
    <p class="foot">Diakritika není povinná — <em>lod</em> i <em>loď</em> se počítá stejně.</p>
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
    margin: 0 0 12px;
    font-size: 20px;
    font-weight: 700;
    letter-spacing: -0.01em;
  }

  h3 {
    margin: 18px 0 6px;
    font-family: 'Space Mono', monospace;
    font-size: 11px;
    letter-spacing: 0.14em;
    text-transform: uppercase;
    color: #f4b942;
    font-weight: 700;
  }

  .lead,
  p {
    margin: 0;
    font-size: 14.5px;
    line-height: 1.55;
    color: #cfd2d8;
  }

  ul {
    margin: 4px 0 0;
    padding-left: 22px;
    color: #cfd2d8;
    font-size: 14.5px;
    line-height: 1.55;
  }

  li {
    margin: 2px 0;
  }

  em {
    color: #f2f2f4;
    font-style: normal;
    font-family: 'Space Mono', monospace;
  }

  strong {
    color: #ffffff;
    font-weight: 600;
  }

  .foot {
    margin-top: 18px;
    font-size: 13px;
    color: #8b8e97;
    border-top: 1px solid #22252d;
    padding-top: 14px;
  }
</style>
