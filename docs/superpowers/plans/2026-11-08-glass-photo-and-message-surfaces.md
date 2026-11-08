# Glass Photo Viewer and Chat Bubbles Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the photo viewer close control and chat message bubbles a restrained frosted-glass finish that fits Zently’s pink/lime visual language.

**Architecture:** Keep the existing photo viewer and message markup. Adjust CSS only: build translucency, blur, edge highlights, and readable fallback colors into the existing close button and message bubble selectors.

**Tech Stack:** React, TypeScript, CSS, Vite.

**Spec:** Current user request and attached screenshot in this conversation.

## Global Constraints

- Keep the existing chat and photo-viewer behavior intact.
- Apply glass only to the close button and message bubbles requested here.
- Preserve readable contrast, existing bubble direction/corners, and reduced-motion behavior.
- Keep fallback colors for browsers without backdrop-filter.

## Review Focus

- Close button remains visible and legible over light and dark photos.
- Message text stays readable while the chat background subtly shows through.
- Sent and received messages retain distinct pink/graphite identity.
- Photo messages keep their existing dimensions and image corners.
- Narrow mobile layouts retain a reachable close button.

---

### Task 1: Add frosted glass to the image close control

**Files:**
- Modify: `client/src/zently.css` (existing `.z-chat-image-dialog .z-close-image` rule)

- [x] Use a layered translucent surface, backdrop blur/saturation, subtle border and inset highlight; add a clear hover/focus state without changing its fixed placement.
- [x] Keep a legible opaque fallback for browsers without backdrop-filter.
- [x] Inspect the open photo viewer at desktop and mobile widths. (CSS build and interaction semantics checked; live visual check follows publish.)

### Task 2: Add a restrained glass surface to message bubbles

**Files:**
- Modify: `client/src/zently.css` (final effective `.z-bubble` own/received rules)

- [x] Add backdrop blur and subtle surface highlights to text bubbles while preserving current colors, neon edge, and asymmetric corners.
- [x] Exclude system bubbles and image-only bubbles from the new translucent treatment.
- [x] Confirm received and sent text remain readable over the chat background. (Existing contrast/colors retained; live visual check follows publish.)

### Task 3: Verify and publish

**Files:**
- No additional source files.

- [x] Run the client production build and available focused tests. (`npm run build`, `npm test` with 75 passing, and `npm run lint`.)
- [ ] Review the changed diff and verify the published site after the authorized push.
- [ ] Record results and update this plan’s checkboxes.
