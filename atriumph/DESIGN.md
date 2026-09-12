---
version: alpha
name: Atriumph Monolith
description: Near-black, architectural identity for Atriumph's company site and practical product interfaces.
colors:
  primary: "#f2f1e9"
  surface: "#111312"
  surface-raised: "#191c1a"
  text: "{colors.primary}"
  text-muted: "#b6bab3"
  line: "#3c423d"
  control-line: "#8c948b"
  ink: "#181c19"
  paper: "{colors.primary}"
  shadow: "#353e35"
  peat: "#626e60"
  moss-gray: "#969f90"
  ash: "#c5c9be"
typography:
  body:
    fontFamily: Arial, Helvetica, sans-serif
    fontSize: 1rem
    fontWeight: 400
    lineHeight: 1.5
  display:
    fontFamily: Arial, Helvetica, sans-serif
    fontSize: 2.75rem
    fontWeight: 500
    lineHeight: 1.04
    letterSpacing: -0.06em
  wordmark:
    fontFamily: Arial, Helvetica, sans-serif
    fontSize: 1.75rem
    fontWeight: 500
    lineHeight: 1
    letterSpacing: -0.06em
  tagline:
    fontFamily: Arial, Helvetica, sans-serif
    fontSize: 0.9375rem
    fontWeight: 400
    lineHeight: 1.6
  section-heading:
    fontFamily: Arial, Helvetica, sans-serif
    fontSize: 1.25rem
    fontWeight: 500
    lineHeight: 1.5
    letterSpacing: -0.035em
  project-title:
    fontFamily: Arial, Helvetica, sans-serif
    fontSize: 1.75rem
    fontWeight: 500
    lineHeight: 1.5
    letterSpacing: -0.04em
  body-small:
    fontFamily: Arial, Helvetica, sans-serif
    fontSize: 0.875rem
    fontWeight: 400
    lineHeight: 1.5
  company-body:
    fontFamily: Arial, Helvetica, sans-serif
    fontSize: 1rem
    fontWeight: 400
    lineHeight: 1.7
  metadata:
    fontFamily: "'SFMono-Regular', Consolas, 'Liberation Mono', monospace"
    fontSize: 0.75rem
    fontWeight: 400
    lineHeight: 1.6
  navigation:
    fontFamily: Arial, Helvetica, sans-serif
    fontSize: 0.8125rem
    fontWeight: 400
    lineHeight: 1.5
  control-label:
    fontFamily: Arial, Helvetica, sans-serif
    fontSize: 13px
    fontWeight: 400
    lineHeight: 1.5
  utility-display:
    fontFamily: Arial, Helvetica, sans-serif
    fontSize: 40px
    fontWeight: 500
    lineHeight: 1
    letterSpacing: -0.065em
spacing:
  space-1: 0.5rem
  space-2: 1rem
  space-3: 1.5rem
  space-4: 2rem
  space-5: 3rem
  space-6: 4.5rem
  gutter-min: 1.25rem
  gutter-max: 4rem
  shell-max: 1600px
  control-padding-block: 10px
  control-padding-inline: 12px
  control-min-height: 44px
  rule-width: 1px
  focus-width: 2px
  site-focus-offset: 5px
  utility-focus-offset: 4px
rounded:
  none: 0px
  image-frame: 24px
  image-inset: 10px
  image-plaque: 12px
components:
  button-primary:
    backgroundColor: "{colors.text}"
    textColor: "{colors.surface}"
    typography: "{typography.body}"
    rounded: "{rounded.none}"
  button-primary-hover:
    backgroundColor: "{colors.ash}"
    textColor: "{colors.surface}"
  button-secondary:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.text}"
    typography: "{typography.body}"
    rounded: "{rounded.none}"
  button-secondary-hover:
    backgroundColor: "{colors.surface-raised}"
    textColor: "{colors.text}"
  select:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.text}"
    typography: "{typography.body}"
    rounded: "{rounded.none}"
  footer:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.text-muted}"
    typography: "{typography.metadata}"
  work-record:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.text}"
    rounded: "{rounded.none}"
  work-record-hover:
    backgroundColor: "{colors.surface-raised}"
    textColor: "{colors.text}"
---

## Overview

Monolith is Atriumph's normative identity: near-black surfaces, an architectural A, restrained typography, thin rules, and technical ledger layouts. This guide governs Atriumph, not the surrounding repository. Convey a small, deliberate software company through real work and clear structure, not promotional decoration.

Atriumph was founded in **2021** and builds tools for individuals and small teams. Preserve the approved headline and supporting line exactly:

**Triumph in a new age.**

Tools for individuals and small teams to put AI to work—building applications, connecting systems, and automating the everyday.

The company homepage introduces actual products; product interfaces perform tasks. Image press is a separate working utility, not a homepage portfolio item. Products retain their names and may carry a quiet “By Atriumph” attribution. The founder's personal site remains independent.

## Colors

Tokens are normative, matching the semantic CSS in `brand.css` and the embedded interface styles in `tool.html`. `primary` aliases the existing warm paper-white text color, not a new accent. Preserve its warmth rather than substituting pure white.

Use `surface` for pages, `surface-raised` for grouped working surfaces and hover fills, `text` for primary reading, and `text-muted` for secondary descriptions and metadata. Both text colors remain legible on both dark surfaces. Use `line` only for decorative structure; its subdued contrast is insufficient for control boundaries. Interactive borders require `control-line`, with a visible focus indicator.

The image palette is ink, shadow, peat, moss-gray, ash, and paper, darkest to lightest. Green-gray belongs inside illustrations, not accent buttons or a green interface theme. Ash also supplies the existing primary-button hover fill. Full image tint uses these six colors; 70% tint interpolates toward neutral gray. Four-tone output uses ink, peat, ash, and paper. Do not reinterpret image midtones as small UI text colors.

## Typography

Use system Arial, Helvetica, sans-serif and the listed monospace fallback stack. SFMono-Regular is optional, not a required installed font. Runtime requires no packages, downloaded fonts, or external dependencies. Reserve monospace for short labels, numbers, and metadata.

The twelve tokens describe existing roles, not additional heading levels. At the default root size, body is 16px. `display` records the homepage minimum: implement `clamp(2.75rem, 5.1vw, 5.25rem)`; at widths up to 640px use `clamp(2.75rem, 12vw, 4rem)`. Utility display uses `clamp(40px, 5vw, 72px)`. Wordmark and project titles become 1.5rem on mobile.

Use body-small for project descriptions and Open links; company-body for the Company paragraph and byline. Metadata covers record numbers and footer. Homepage uppercase eyebrow/labels add .025em tracking; utility uppercase labels add .04em. Company label headings retain weight 500. Keep tight tracking on display/headings, not reading paragraphs. Do not turn essential instructions into microtype.

## Layout

Center a 1600px maximum shell with gutters `clamp(1.25rem, 4vw, 4rem)` (20–64px at the default root). The six shared spacing steps are 8, 16, 24, 32, 48, and 72px; retain component-specific values rather than forcing every measurement onto that scale.

Desktop hero and Company sections use equal columns. Hero copy has 2rem top, 3rem right, 2.5rem bottom padding; its heading has 4rem vertical padding and an 8em measure. Tagline measure is 29rem; Company text is 36rem. Work starts after 3.5rem; Company has 4rem vertical padding.

Work rows use number/title/description/action columns: `4rem minmax(0, .8fr) minmax(0, 1.2fr) auto`, 1.5rem gaps, and 2rem 1rem padding. At 900px descriptions move below titles. At 640px hero and Company stack, rows use `1.75rem 1fr auto`, .75rem gaps, and 1.5rem vertical padding. Keep meaningful reading order without horizontal scrolling. Image press uses a 280px controls column beside its preview; at 760px it stacks, retaining two control columns.

## Elevation & Depth

Build hierarchy with whitespace, 1px rules, typography, and restrained surface changes. No UI gradients, neon, heavy shadows, glass, or ornamental motion. Grain belongs to static imagery, never text or controls. Image press uses stable noise before quantization, not blur. Its default recipe is six tones, grain 24, tint 70%, contrast 110%; the village uses grain 18 and contrast 96% with the same palette engine. Preserve the engine and social outputs.

## Shapes

UI controls and containers have square corners: `rounded.none`. The other radius tokens apply **only at the 1080px reference image scale**: outer paper frame 24px, inset image/line 10px, lower-right plaque 12px. Scale them with export geometry; never apply them to UI cards.

Preserve the architectural A's silhouette and counter. Use paper on dark or ink on light, without grain or distortion. Its 64-unit vector grid requires at least 8 units of clear space; provisional minimum icon size is 24px. PNG exports are 1024px square with 192px safe inset and genuine transparency where specified.

## Components

**Header and footer.** Use a plain-text Atriumph wordmark and Work/Company anchors. Footer: Atriumph left, only Work and Company right. Company ends with body-sized “Built by kiasaki.” linking to `https://kiasaki.com`; no GitHub link. The hero marker `0x07E5 / INIT` means 2021 and must expose “Established in 2021,” not a fabricated system status.

**Landscape and mark.** Keep `assets/hero-village.png` (1200 × 1200) full-bleed, centered with `object-fit: cover`: terraces and greenery below, open sky above. Independently layer `assets/mark-light-transparent.png` over it: image box width 64% of the panel, height auto, centered at left/top 50% with translate(-50%, -50%). Its safe inset makes the visible glyph approximately 40% of panel width. Keep the paper-white glyph opaque and background transparent; no CSS opacity, blending, or effects. Use `alt=""`, `pointer-events: none`, and no JavaScript. Never burn the mark into or regenerate the village PNG.

**Work ledger.** Link real projects: [Myst](https://myst.kiasaki.com), secrets across environments; [DBDB](https://dbdb.kiasaki.com), database browsing/debugging with MCP/API access; [Kyoi](https://kyoi.kiasaki.com), persistent AI chat for internal tools, applications, API connections, and automation. Use numbered rows, descriptions, and Open actions; hover raises the surface and underlines the title.

**Controls.** Buttons/selects use 44px minimum height and 10px 12px padding. Secondary boundaries are 1px control-line; primary boundaries match their paper-white fill. Disabled utility buttons use .5 opacity and a wait cursor. Focus is 2px: currentColor with 5px offset on shared pages, text with 4px offset in the utility. Preserve native labels, keyboard access, skip links, and plain-language inline errors.

## Do's and Don'ts

- Do use one main heading and one strong primary action per task.
- Do put creation flows on dedicated pages; keep lists for existing records.
- Do move detailed instructions to Help, preserving valid state after failed imports.
- Do state statuses in text and preserve readable contrast and visible focus.
- Don't add new promotional slogans, decorative subtitles, repeated help, safety banners, or invented records.
- Don't add review/download links to the homepage footer or change social framing to match UI corners.
