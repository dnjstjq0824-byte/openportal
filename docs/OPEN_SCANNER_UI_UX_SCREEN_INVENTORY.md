# OPEN SCANNER — UI/UX SCREEN & COMPONENT INVENTORY

**Document Version:** 1.0.0-PROD  
**Status:** COMPLETE & VERIFIED  
**Target:** Global Responsive User Interface (Mobile, Tablet, Desktop)  

---

## 1. Master Screen Inventory

| Screen ID | Screen Name | URL / State | Primary User Goals | Status |
|---|---|---|---|---|
| **SCR-01** | Open Scanner Home | `/` (query empty) | Clean search entry, instant autocomplete, service shortcuts, account access | `WORKING` |
| **SCR-02** | SERP — Web Search | `/` (`q` present, `tab=web`) | Ranked organic results, query correction, sitelinks, pagination | `WORKING` |
| **SCR-03** | SERP — News Vertical | `/` (`tab=news`) | Real-time news articles, publisher badges, time-ago timestamps | `WORKING` |
| **SCR-04** | SERP — Images Vertical | `/` (`tab=images`) | Visual image search grid, aspect ratio previews, domain attribution | `WORKING` |
| **SCR-05** | SERP — Videos Vertical | `/` (`tab=videos`) | Video hit cards, duration pills, publisher channel, video modal | `WORKING` |
| **SCR-06** | SERP — Shopping Vertical | `/` (`tab=shopping`) | Merchant products, price comparisons, rating stars, purchase CTA | `WORKING` |
| **SCR-07** | AI Overview Capsule | Inline top SERP | Cited syntheses, expandable source references, safety disclosures | `WORKING` |
| **SCR-08** | Visual Search (Open Lens) | Modal overlay (`#lens-modal`) | Drag-and-drop image search, visual feature extraction | `WORKING` |
| **SCR-09** | Open Scanner Account Modal | Modal overlay (`.os-auth-modal-overlay`) | 4-tab auth: Login, Registration, 6-digit PIN Verify, Password Reset | `WORKING` |
| **SCR-10** | External IdP Gate Modal | Modal overlay (`.auth-modal-card`) | Fail-closed enterprise SSO card (unconfigured state verification) | `WORKING` |
| **SCR-11** | User Profile Flyout | Dropdown (`.profile-flyout`) | Active session details, account email, token indicator, logout button | `WORKING` |
| **SCR-12** | Search Settings Flyout | Dropdown (`.settings-flyout`) | SafeSearch level, dark/light theme, search history clear, language | `WORKING` |
| **SCR-13** | Operator Control Drawer | Drawer (`.operator-drawer`) | Real-time crawl rate, index count, cache purge, DR rehearsal drill | `WORKING` |
| **SCR-14** | Error & Outage State | Full screen / SERP error | 503 backend outage banner, zero cached hits leakage, retry CTA | `WORKING` |

---

## 2. Deep Dive: Screen Specifications & Micro-Interactions

### 2.1 SCR-01: Open Scanner Home
- **Hero Brand Header:** Centered Open Scanner brand logo with Focus Frame bracket symbol (`⛶`).
- **Omni-Search Bar:** 584px width, rounded pill geometry (24px radius), voice search icon, lens trigger icon, clear query button (`✕`).
- **Quick Links / Shortcuts:** Open News, Open Media, Open Apps, Webmaster Console, Developer APIs.
- **Top Utility Navigation:** Korean/English language toggle, Settings trigger, Google Sign-In button, Open Scanner Account button (`⚡ Open Account`).

### 2.2 SCR-02: SERP Web Results
- **Header Stickiness:** Top persistent search bar with auto-shrinking margins on scroll.
- **Query Correction Bar:** Displays "이것을 찾으셨나요: [수정된 검색어]" for detected typos (e.g. `rjatordpswls` $\to$ `검색엔진`).
- **Search Result Card (`.hit`):**
  - Source domain breadcrumb with favicon (`https://example.org > news > 2026`).
  - High-contrast clickable headline (`18px`, color `#1a0dab` or `#2563eb`).
  - Snippet description with highlighted query keywords (`14px`, `#334155`).
  - Sitelinks container for authoritative domains.
- **Google-Style Pagination:** Numbers 1 to 10 with Previous/Next controls, synchronizing `page` URL parameter.

### 2.3 SCR-09: Open Scanner Account & Email PIN Verification
- **Accessible Tabs:**
  1. `[로그인]`: Email and password inputs with password visibility toggle.
  2. `[회원가입]`: Email, display name, password (min 8 chars), password confirmation, required Terms checkbox.
  3. `[이메일 인증]`: Automatically pre-fills registered email; accepts 6-digit PIN code with auto-focus; verifies account upon submission.
  4. `[비밀번호 찾기]`: Step 1 generates reset token to registered email; Step 2 updates password with valid token.
- **Instant Status Feedback:** `.os-auth-alert.error` (red border/fill) and `.os-auth-alert.success` (green border/fill) with ARIA live notifications.

---

## 3. Responsive State Audit

- **Mobile Viewport (320px – 430px):** Single-column stack, zero horizontal overflow (`overflow-x: hidden`), full-width inputs, drawer navigation.
- **Tablet Viewport (768px):** Search bar widens to 584px, vertical tabs scroll horizontally with subtle gradient edge masks.
- **Desktop Standard (1024px – 1440px):** Strict 652px left rail for organic hits, 368px right rail for knowledge graph / weather / stock cards.
- **High-DPI / Retina Displays:** Vector SVG icons, 2x raster source srcset, crystal-clear typography rendering.

