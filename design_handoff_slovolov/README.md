# Handoff: Slovolov — hra na hádání tajného slova

## Overview
Slovolov je jednostránková mobile-first hra ve stylu **contexto.me**. Hráč hádá tajné slovo; každý tip dostane **rank** (pořadí podle sémantické blízkosti k tajnému slovu, `#1` = tajné slovo). Každý tip se v žebříčku zobrazí s barevným „teploměrem" — proužek, jehož **šířka i barva** odpovídají blízkosti (modrá = daleko → žlutá → červená = blízko). Tipy jsou seřazené od nejteplejšího. Rank `#1` znamená výhru → konfety + obrazovka „Hrát znovu".

Cílový stack: **Svelte** komponenty napojené na **Go API** (výpočet ranku / sémantické podobnosti běží na backendu).

## About the Design Files
Soubor `Slovolov.dc.html` v tomto balíčku je **designová reference vytvořená v HTML** — prototyp ukazující zamýšlený vzhled a chování, **není to produkční kód k přímému zkopírování**. Úkolem je **znovu vytvořit tento design v Svelte** s vlastní strukturou komponent a napojit na Go API. HTML používá vlastní mini-framework (`<x-dc>`, `renderVals`) — ignorujte ho, důležité jsou vizuál, rozměry, barvy, animace a stavová logika popsané níže.

Mock data (3 tajná slova + slovníky ranků + hash pro neznámá slova) v prototypu **nahraďte voláním Go API**.

## Fidelity
**High-fidelity (hifi).** Finální barvy, typografie, spacing i animace. UI recreujte pixel-perfect podle hodnot v sekcích Design Tokens a Screens.

## Screens / Views

### 1. Hlavní obrazovka (herní)
- **Purpose**: Hráč píše tipy, sleduje žebříček a blízkost k tajnému slovu.
- **Layout**: Vertikální sloupec, vycentrovaný, `max-width: 468px`, `padding: 0 16px 140px`. Pozadí: `radial-gradient(120% 80% at 50% -10%, #191b21 0%, #0f1013 60%)`. Celá stránka je scroller.
- **Komponenty** (shora dolů):

  **A) Header (sticky)** — `position: sticky; top: 0; z-index: 6`, `padding: 16px 0 12px`, pozadí `linear-gradient(#0f1013 62%, rgba(15,16,19,0))`, `backdrop-filter: blur(2px)`. Řádek `space-between`:
  - Vlevo: wordmark **„Slovolov"** — `font-family: 'Space Grotesk'`, `font-weight: 700`, `font-size: 22px`, `letter-spacing: -.02em`, výplň gradientem `linear-gradient(120deg,#4ea1ff,#f4b942 55%,#ff5a4d)` clipnutým do textu. Vedle podtitulek **„hádej tajné slovo"** — `Space Mono`, `11px`, `#71757f`.
  - Vpravo: počítadlo tipů — pill, `Space Mono 12.5px`, `#8b8e97`, `border: 1px solid #262931`, `border-radius: 999px`, `padding: 6px 10px`. Text s českým plurálem: „1 tip / 2–4 tipy / 5+ tipů". Vedle tlačítko **„Nová hra"** — `#1c1f26`, `border: 1px solid #2c2f38`, `border-radius: 999px`, `padding: 7px 13px`, `font-weight: 600 13px`. Hover: `background:#262a33; border-color:#3a3e49`.
  - (Volitelně, tweak „nápověda") řádek pod headerem: `Space Mono 12px #7f838d`, text „Tajné slovo má N písmen." (český plurál písmeno/písmena/písmen).

  **B) Input form** — `display:flex; gap:9px; margin-top:6px`. Odeslání přes submit formuláře (funguje Enter i klik).
  - Input: `flex:1`, `background:#16181e`, `border:1px solid #2a2e37`, `color:#f2f2f4`, `font-size:16px` (důležité kvůli iOS zoomu), `padding:14px 16px`, `border-radius:14px`, placeholder „Napiš slovo…". Focus: `border-color:#4ea1ff; box-shadow:0 0 0 3px rgba(78,161,255,.15)`. Atributy `autocomplete=off autocapitalize=off spellcheck=false`.
  - Tlačítko „Tip": `linear-gradient(150deg,#4ea1ff,#3b82f6)`, `color:#fff`, `font-weight:700 16px`, `padding:0 20px`, `border-radius:14px`, `box-shadow:0 6px 18px rgba(59,130,246,.35)`. Hover `filter:brightness(1.08)`, active `translateY(1px)`.
  - Chybová/info hláška pod formulářem (např. „Tohle slovo už jsi tipoval."): `13px`, `#f0a04b`, fade-in `.2s`.

  **C) Poslední tip (highlight karta)** — jen když existuje aspoň 1 tip. `margin-top:20px`. Nad kartou label „POSLEDNÍ TIP" (`Space Mono 11px`, `letter-spacing:.14em`, `text-transform:uppercase`, `#71757f`). Karta = zvětšený řádek žebříčku (viz D), `height:62px`, `border-radius:15px`, `background:#181a20`, `border:1px solid #2a2e37`, `box-shadow:0 8px 26px rgba(0,0,0,.35)`. Objeví se animací `slv-pop .34s`. Slovo `19px`, rank `17px`, pořadové číslo badge s bílým pozadím `rgba(255,255,255,.82)` a tmavým textem.

  **D) Žebříček** — když existují tipy: label „ŽEBŘÍČEK — OD NEJTEPLEJŠÍHO" (stejný styl jako výše), `margin:24px 0 10px`. Pak seznam řádků `display:flex; flex-direction:column; gap:8px`, **seřazený podle ranku vzestupně** (nejteplejší nahoře). Každý řádek:
    - Kontejner: `position:relative; overflow:hidden; border-radius:13px; background:#16181e; height:52px; border:1px solid #22252d`.
    - **Fill (teploměr)**: absolutní div zleva, `width = (4 + pct*96)%`, `background = heat color`, `opacity:.88`.
    - Obsah nad fillem (`position:relative; space-between; padding:0 14px`):
      - Vlevo: badge pořadí tipu (`Space Mono 11px 700`, `#c9ccd3`, `background:rgba(255,255,255,.1)`, `22×22`, `border-radius:6px`) + slovo (`16px 600`, `text-shadow:0 1px 2px rgba(0,0,0,.5)`, ellipsis).
      - Vpravo: rank `#N` (`Space Mono 15px 700`, `text-shadow:0 1px 2px rgba(0,0,0,.55)`).

  **E) Prázdný stav** — když 0 tipů: vycentrovaný text `#6b6f78 15px`, max-width 300px: „Napiš jakékoliv slovo a zjisti, jak blízko jsi tajnému slovu. Čím teplejší barva a delší proužek, tím blíž."

### 2. Výherní overlay
- Zobrazí se když padne rank `#1`. `position:fixed; inset:0; z-index:20`, `background:rgba(10,11,14,.72)`, `backdrop-filter:blur(6px)`, fade-in `.3s`.
- Karta vycentrovaná, `max-width:360px`, `background:#16181e`, `border:1px solid #2c303a`, `border-radius:22px`, `padding:34px 26px`, `box-shadow:0 24px 70px rgba(0,0,0,.6)`, animace `slv-pop .4s`.
- Obsah: eyebrow „UHODL JSI!" (`Space Mono 12px`, `letter-spacing:.18em`, `#f4b942`); tajné slovo velké `40px 700` s gradientovou výplní (stejný gradient jako wordmark); podtitulek „Trefa na N tipů." (`#a7abb4 15px`); tlačítko „Hrát znovu" na plnou šířku (stejný modrý gradient jako „Tip").
- Zároveň s overlayem se spustí **konfety** (viz Interactions).

## Interactions & Behavior

- **Odeslání tipu**: submit formuláře → normalizace vstupu → dotaz na rank → přidání do seznamu. Prázdný vstup ignorovat. Po odeslání vyčistit input a vrátit focus do inputu.
- **Duplicita**: pokud už bylo slovo tipováno (po normalizaci), tip nepřidávat, zobrazit hlášku „Tohle slovo už jsi tipoval." a vyčistit input.
- **Normalizace** (pro porovnání/duplicity a lookup): `lowercase` + `trim` + odstranění diakritiky (`normalize('NFD').replace(/[\u0300-\u036f]/g,'')`). Zobrazuje se ale kanonické slovo (s diakritikou) vrácené z API, jinak očištěný vstup.
- **Řazení**: žebříček vždy podle ranku vzestupně (rank 1 nahoře). „Poslední tip" je vždy naposledy zadaný (podle pořadí zadání), zvlášť nahoře.
- **FLIP reorder animace**: při přidání tipu se řádky plynule přesunou na novou pozici.
  - Před updatem změř `top` každého řádku (klíč = slovo). Po updatu změř znovu; pro každý existující řádek nastav `transform: translateY(dy)` bez transitionu, pak v `requestAnimationFrame` nastav `transition: transform .45s cubic-bezier(.2,.8,.2,1)` a `transform:''`. Práh pohybu > 1px.
  - Nové řádky (bez předchozí pozice): vstupní animace `slv-rise .34s cubic-bezier(.2,.8,.2,1)`.
  - V Svelte lze řešit nativně: `animate:flip={{ duration: 450, easing: cubicOut }}` na položkách `{#each}` (klíčované slovem) + `in:fly={{ y: 14, duration: 340 }}`.
- **Konfety** (při výhře): fullscreen `<canvas>`, ~160 částic vystřelených z ~35 % výšky nahoru, gravitace, rotace, po ~1400 ms fade-out, celkem ~2600 ms, pak canvas odstranit. Barvy: `#4ea1ff, #f4b942, #ff5a4d, #3ddc97, #c77dff`. Obdélníkové konfety (`fillRect`), respektovat `devicePixelRatio` (cap 2).
- **Nová hra**: vybere jiné tajné slovo než aktuální, vymaže tipy, reset stavu, focus do inputu.
- **Responsivita**: mobile-first, sloupec do 468px, na širších obrazovkách vycentrovaný. Input `font-size:16px` (brání iOS zoomu). Header sticky.

## State Management
Stav (v prototypu lokální; v produkci část přijde z API):
- `guesses: { word: string, rank: number, order: number }[]` — historie tipů (order = pořadí zadání od 1).
- `input: string` — aktuální text v inputu.
- `message: string` — info/chybová hláška.
- `secretId` / aktuální hra — identifikátor rozehrané hry (backend zná tajné slovo, klient ne).
- Odvozené: `won = guesses.some(g => g.rank === 1)`, `guessCount = guesses.length`, seřazené `rows`, `lastRow`.
- Pro konfety si drž `prevWon`, spusť efekt jen na přechodu `false → true`.
- Pro FLIP si drž mapu předchozích `top` pozic řádků.

## Go API — návrh napojení
Prototyp počítá rank lokálně (slovník + hash). V produkci to dělá Go backend (sémantická podobnost, embeddings apod.). Navrhované endpointy:

- `POST /api/game` → založí novou hru, vrátí `{ gameId, wordLength }`. Backend si drží tajné slovo.
- `POST /api/game/{gameId}/guess` body `{ "word": "loď" }` → vrátí `{ "word": "loď", "rank": 11, "isWin": false }` (rank = pořadí blízkosti; `rank === 1` znamená výhru). Backend řeší normalizaci, neznámá slova i výpočet ranku.
- (Volitelně) `GET /api/game/{gameId}` → obnovení rozehrané hry (seznam dosavadních tipů).

Klient pak jen zobrazuje `rank` z API a počítá z něj `pct` (heat) pro teploměr — viz Design Tokens.

## Design Tokens

**Barvy**
- Pozadí stránky: `#0f1013`; radiální varianta `#191b21 → #0f1013`.
- Povrchy: řádek `#16181e`, karta posledního tipu `#181a20`, výherní karta `#16181e`.
- Borders: `#22252d`, `#2a2e37`, `#262931`, `#2c2f38`, `#2c303a`.
- Text: hlavní `#e9e9ec` / `#f2f2f4`, tlumený `#8b8e97` / `#71757f` / `#7f838d`, prázdný stav `#6b6f78`.
- Akcent modrá (tlačítka): `#4ea1ff → #3b82f6`.
- Gradient značky/výhry: `#4ea1ff → #f4b942 (55%) → #ff5a4d`.
- Info/chyba: `#f0a04b`. Výherní eyebrow: `#f4b942`.

**Teploměr (heat) — výpočet**
- `pct = max(0.02, 1 - ln(rank) / ln(TOTAL))`, kde `TOTAL = 5000` (celkový počet slov v prostoru; slaď s backendem). `pct ∈ (0,1]`, 1 = nejteplejší.
- Šířka fillu: `(4 + pct*96)%`.
- Barva (schéma **klasik**, výchozí), `hsl(H 82% 52%)`:
  - `pct < 0.5`: `H = 210 + (55-210) * (pct/0.5)` (modrá → žlutá)
  - `pct ≥ 0.5`: `H = 55 + (8-55) * ((pct-0.5)/0.5)` (žlutá → červená)
- Alternativní schémata (tweak, volitelné):
  - „oceán": `H = 250 + (165-250)*pct`, `s=70%`
  - „západ slunce": `H = 300 + (25-300)*pct`, `s=80%`

**Typografie** — Google Fonts: **Space Grotesk** (400/500/600/700) pro UI a nadpisy, **Space Mono** (400/700) pro čísla, ranky a labely.
- Wordmark 22/700, slovo v řádku 16/600, slovo v last-tip 19/600, tajné slovo (výhra) 40/700.
- Ranky: Space Mono 15 (řádek) / 17 (last-tip) / 700.
- Labely sekcí: Space Mono 11, `letter-spacing:.14em`, uppercase.

**Radius**: řádek 13px, last-tip 15px, karta výhry 22px, tlačítka/input 14px, pill 999px, badge 6–7px.

**Stíny**: tlačítko `0 6px 18px rgba(59,130,246,.35)`; last-tip `0 8px 26px rgba(0,0,0,.35)`; výhra `0 24px 70px rgba(0,0,0,.6)`.

**Animace (@keyframes)**
- `slv-pop`: `0%{scale .94; opacity 0} 60%{scale 1.02} 100%{scale 1; opacity 1}` — `.34s`/`.4s cubic-bezier(.2,.8,.2,1)`.
- `slv-rise`: `translateY(14px)→0` + opacity — `.34s`.
- `slv-fadein`: opacity `0→1` — `.2s`–`.3s`.
- FLIP reorder: `transform .45s cubic-bezier(.2,.8,.2,1)`.

## Assets
Žádné bitmapy ani ikony — vše je typografie, CSS gradienty a canvas konfety. Fonty přes Google Fonts (Space Grotesk, Space Mono).

## Files
- `Slovolov.dc.html` — kompletní hi-fi prototyp (vizuál + herní logika + FLIP + konfety). Herní logika je v `<script data-dc-script>` třídě `Component` (metody `submit`, `lookup`, `heat`, `color`, `_flip`, `_confetti`, `renderVals`). Slouží jako referenční implementace chování — mock lookup nahraďte Go API.
