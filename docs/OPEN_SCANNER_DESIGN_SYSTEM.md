# OPEN SCANNER — DESIGN SYSTEM SPECIFICATION (V1.0)

**Document Version:** 1.0.0-PROD  
**Status:** COMPLETED & INTEGRATED  
**Architecture:** Open Scanner Web & Universal Client System  

---

## 1. Design Token Architecture

The Open Scanner Design System provides cross-platform semantic tokens ensuring 100% visual coherence between desktop web, tablet, mobile web, and native mobile shells.

### 1.1 Color Tokens

```css
:root {
  /* Brand Core */
  --os-primary: #2563eb;          /* Open Blue - Action, trust */
  --os-primary-hover: #1d4ed8;    /* Darker Blue on interaction */
  --os-cyan: #22c7e8;             /* Discovery Cyan - Highlights */
  --os-violet: #7c3aed;           /* Intelligence Violet - AI */
  --os-midnight: #0b1020;         /* Deep Midnight - Contrast */

  /* Neutral Light Palette */
  --os-bg: #f7f9fc;               /* Viewport background */
  --os-surface: #ffffff;          /* Card & modal containers */
  --os-surface-hover: #f1f5f9;    /* Interactive item hover */
  --os-border: #e2e8f0;           /* Structural borders */
  --os-border-subtle: #cbd5e1;    /* Input field strokes */
  --os-text: #0f172a;             /* Primary high-contrast copy */
  --os-muted: #64748b;            /* Secondary & metadata copy */

  /* Status Colors */
  --os-success: #16a34a;
  --os-success-bg: #f0fdf4;
  --os-error: #dc2626;
  --os-error-bg: #fef2f2;
  --os-warning: #d97706;
  --os-warning-bg: #fffbeb;

  /* Shadows & Elevations */
  --os-shadow-sm: 0 1px 2px 0 rgba(11, 16, 32, 0.05);
  --os-shadow-md: 0 4px 6px -1px rgba(11, 16, 32, 0.1), 0 2px 4px -2px rgba(11, 16, 32, 0.1);
  --os-shadow-lg: 0 10px 15px -3px rgba(11, 16, 32, 0.1), 0 4px 6px -4px rgba(11, 16, 32, 0.1);
  --os-shadow-modal: 0 20px 40px rgba(11, 16, 32, 0.25), 0 0 0 1px rgba(226, 232, 240, 0.8);
}
```

### 1.2 Dark Mode Adaptation (`.app.theme-dark`)

```css
.app.theme-dark {
  --os-bg: #0b1020;
  --os-surface: #131d33;
  --os-surface-hover: #1e293b;
  --os-border: #1e293b;
  --os-border-subtle: #334155;
  --os-text: #f8fafc;
  --os-muted: #94a3b8;
  --os-shadow-modal: 0 20px 40px rgba(0, 0, 0, 0.6), 0 0 0 1px rgba(51, 65, 85, 0.6);
}
```

---

## 2. Core UI Components

### 2.1 Buttons
- **Primary CTA (`.os-primary-btn`):** Background gradient `#2563eb` $\to$ `#1d4ed8`, 8px radius, white text, 14px bold font, active state lift.
- **Account Trigger (`.os-account-btn`):** Gradient `#2563eb` $\to$ `#7c3aed`, 6px radius, gold spark icon `⚡`.
- **Secondary / Link (`.os-link-btn`):** Text button with `#2563eb`, zero padding, underline on hover.

### 2.2 Input Controls & PIN Entry
- **Text Inputs (`.os-form-group input`):** 10px 12px padding, 8px radius, border `#cbd5e1`, focus ring with `3px rgba(37, 99, 235, 0.15)`.
- **PIN Input (`.pin-input`):** Monospace centered font, 18px bold, 6px letter spacing for high legibility of 6-digit codes.
- **Checkbox (`.os-form-check`):** Accessible native checkbox with `accent-color: #2563eb` and matching label typography.

### 2.3 Modal Overlay & Dialogs
- **Overlay (`.os-auth-modal-overlay`):** `fixed inset: 0`, `rgba(11, 16, 32, 0.7)`, `backdrop-filter: blur(4px)`.
- **Card (`.os-auth-modal-card`):** Max width 440px, spring pop-in animation (`cubic-bezier(0.16, 1, 0.3, 1)`), tabbed navigation (`.os-auth-tabs`).

---

## 3. Responsive Breakpoints & SERP Layout

| Breakpoint | Viewport Width | SERP Layout Model | Search Bar Width |
|---|---|---|---|
| **Small Mobile** | 320px – 374px | Single column, full width, 12px horizontal margins | 100% (296px net) |
| **Standard Mobile** | 375px – 430px | Single column, sticky header, 16px horizontal margins | 100% (343px net) |
| **Tablet** | 768px – 1023px | Single column with 652px content constraint | 584px fixed |
| **Desktop Standard** | 1024px – 1440px | Dual column: Left Rail (652px) + Right Rail (368px knowledge panel) | 692px centered/aligned |
| **Ultra-Wide / 4K** | 1920px+ | Left-aligned 180px margin, 652px result rail, bounded max container | 692px fixed |

---

## 4. Accessibility & Contrast Compliance

1. **WCAG 2.1 Level AA Compliance:**
   - Normal text contrast ratio: $\ge 4.5:1$ (`#0f172a` on `#ffffff` is $16.1:1$).
   - Large text contrast ratio: $\ge 3:1$ (`#2563eb` on `#ffffff` is $4.6:1$).
2. **Keyboard Focus & Navigation:**
   - Visual outline indicator on all interactive controls (`:focus-visible`).
   - `skip-link` pointing to `#main-content` at start of DOM.
3. **Screen Reader Landmarks:**
   - Proper `role="banner"`, `role="search"`, `role="main"`, `role="contentinfo"`, and `role="dialog"`.

