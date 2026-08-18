<script>
  export let value = '';
  export let disabled = false;
  // busy marks an in-flight guess. It must not disable the input: the browser
  // blurs a focused element the moment it turns disabled, so the player would
  // have to click back into the field after every guess (and on mobile the
  // keyboard would close). Only the submit button dims.
  export let busy = false;
  export let onSubmit = () => {};

  let inputEl;

  export function focus() {
    inputEl?.focus();
  }

  function submit(e) {
    e.preventDefault();
    const trimmed = value.trim();
    if (!trimmed || disabled || busy) return;
    onSubmit(trimmed);
    value = '';
    inputEl?.focus();
  }
</script>

<form on:submit={submit}>
  <input
    bind:this={inputEl}
    bind:value
    type="text"
    placeholder="Napiš slovo…"
    autocomplete="off"
    autocapitalize="off"
    spellcheck="false"
    {disabled}
  />
  <button type="submit" disabled={disabled || busy}>Tip</button>
</form>

<style>
  form {
    display: flex;
    gap: 9px;
    margin-top: 6px;
  }

  input {
    flex: 1;
    min-width: 0;
    background: #16181e;
    border: 1px solid #2a2e37;
    color: #f2f2f4;
    font-size: 16px;
    padding: 14px 16px;
    border-radius: 14px;
    outline: none;
    transition: border-color 0.15s, box-shadow 0.15s;
  }

  input::placeholder {
    color: #5f636d;
  }

  input:focus {
    border-color: #4ea1ff;
    box-shadow: 0 0 0 3px rgba(78, 161, 255, 0.15);
  }

  button {
    flex: none;
    background: linear-gradient(150deg, #4ea1ff, #3b82f6);
    color: #fff;
    font-weight: 700;
    font-size: 16px;
    padding: 0 20px;
    border-radius: 14px;
    border: none;
    box-shadow: 0 6px 18px rgba(59, 130, 246, 0.35);
    transition: filter 0.15s, transform 0.05s;
  }

  button:hover:not(:disabled) {
    filter: brightness(1.08);
  }

  button:active:not(:disabled) {
    transform: translateY(1px);
  }

  button:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }
</style>
