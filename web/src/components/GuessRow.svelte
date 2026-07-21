<script>
  import { heat, heatColor, fillWidth } from '../lib/heat.js';
  export let word;
  export let rank;
  export let order;
  export let variant = 'row'; // 'row' | 'last'
  export let hint = false;

  $: pct = heat(rank);
  $: fill = fillWidth(pct);
  $: color = heatColor(pct);
</script>

<div class="wrap {variant}" class:hint>
  <div class="fill" style="width: {fill}; background: {color};" />
  <div class="content">
    <div class="left">
      <span class="badge">{hint ? '💡' : order}</span>
      <span class="word">{word}</span>
    </div>
    <span class="rank">#{rank}</span>
  </div>
</div>

<style>
  .wrap {
    position: relative;
    overflow: hidden;
    border-radius: 13px;
    background: #16181e;
    height: 52px;
    border: 1px solid #22252d;
    display: flex;
    align-items: center;
  }

  .wrap.last {
    height: 62px;
    border-radius: 15px;
    background: #181a20;
    border-color: #2a2e37;
    box-shadow: 0 8px 26px rgba(0, 0, 0, 0.35);
    animation: slv-pop 0.34s cubic-bezier(0.2, 0.8, 0.2, 1);
  }

  .wrap.hint {
    border-color: #4a3a1c;
    border-style: dashed;
  }

  .fill {
    position: absolute;
    left: 0;
    top: 0;
    bottom: 0;
    opacity: 0.88;
    transition: width 0.4s ease-out;
  }

  .wrap.last .fill {
    opacity: 0.92;
  }

  .content {
    position: relative;
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 14px;
    gap: 10px;
  }

  .wrap.last .content {
    padding: 0 16px;
  }

  .left {
    display: flex;
    align-items: center;
    gap: 10px;
    min-width: 0;
  }

  .wrap.last .left {
    gap: 11px;
  }

  .badge {
    font-family: 'Space Mono', monospace;
    font-weight: 700;
    font-size: 11px;
    color: #c9ccd3;
    background: rgba(255, 255, 255, 0.1);
    width: 22px;
    height: 22px;
    border-radius: 6px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex: none;
  }

  .wrap.hint .badge {
    background: rgba(244, 185, 66, 0.18);
    color: #f4b942;
    font-size: 13px;
  }

  .wrap.last .badge {
    background: rgba(255, 255, 255, 0.82);
    color: #0f1013;
    width: 24px;
    height: 24px;
    border-radius: 7px;
    font-size: 12px;
  }

  .wrap.last.hint .badge {
    background: rgba(244, 185, 66, 0.9);
    color: #0f1013;
    font-size: 14px;
  }

  .word {
    font-size: 16px;
    font-weight: 600;
    text-shadow: 0 1px 2px rgba(0, 0, 0, 0.5);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .wrap.last .word {
    font-size: 19px;
    text-shadow: 0 1px 3px rgba(0, 0, 0, 0.5);
  }

  .rank {
    font-family: 'Space Mono', monospace;
    font-weight: 700;
    font-size: 15px;
    text-shadow: 0 1px 2px rgba(0, 0, 0, 0.55);
    color: #f2f2f4;
    flex: none;
  }

  .wrap.last .rank {
    font-size: 17px;
    text-shadow: 0 1px 3px rgba(0, 0, 0, 0.55);
  }
</style>
