## 2024-05-18 - Accessible Icon Buttons with Dynamic Badges
**Learning:** When using icon-only buttons that feature dynamic visual badges (like an unread notification count badge), simply adding an `aria-label` to the button can create conflicting/duplicate announcements if the inner badge text is also read by screen readers. Furthermore, the visual icon itself does not provide context to non-visual users.
**Action:** Move the semantic meaning to the parent button's `aria-label` (e.g., `aria-label={count > 0 ? \`Notifications (${count} unread)\` : 'Notifications'}`) and explicitly hide the purely visual children (both the icon and the badge text) from screen readers by adding `aria-hidden="true"` to them.

## 2026-03-29 - Accessible Interactive Chat Input Toggles
**Learning:** In a highly interactive chat interface with collapsible input modes (e.g. `ChatInputArea`), visually hiding a keyboard and replacing it with a toggle button can break spatial and navigation context for screen readers. Simply providing "Send" buttons without `aria-busy` feedback during LLM streaming leaves non-visual users unaware of the ongoing background process.
**Action:** When a toggle button affects the visibility/state of another complex element like a textarea, explicitly link them using `React.useId()` and the `aria-controls={id}` along with `aria-expanded={boolean}` on the toggle button. For the main action button, dynamically update its content (e.g., '发送中...') and set `aria-busy={sending}` to ensure assistive technologies announce the pending state.
## 2026-03-30 - Proper ARIA Dialog Semantics & Keyboard Shortcuts
**Learning:** Custom modal dialogs often miss crucial native dialog capabilities: keyboard dismissability (the ESC key) and proper announcement by screen readers upon opening.
**Action:** Always implement `role="dialog"`, `aria-modal="true"`, and use `React.useId()` to link `aria-labelledby` with the modal title. Additionally, bind a document-level `keydown` listener for 'Escape' in a `useEffect` to safely handle keyboard dismissal, ensuring clean up on unmount or close.

## 2025-03-31 - [Add copy feedback to code blocks]
**Learning:** Adding a temporary visual state ("已复制!") and updating the `aria-label` provides a massive accessibility win for screen-reader users and visual reassurance for all users when interacting with clipboard APIs. Using `setTimeout` within a React `useCallback` requires proper cleanup using `useRef` to prevent state updates on unmounted components and overlapping timers.
**Action:** Whenever implementing a clipboard copy action, always extract the button into a stateful component that manages a `copied` boolean and provides visual/ARIA feedback. Ensure to clear timeouts in both the copy handler and the `useEffect` unmount cleanup.

## 2026-04-01 - Native ARIA Tablist Patterns for Custom Tabs
**Learning:** Custom React tab implementations that just use buttons inside a container (acting as filters) fail to communicate their relationship and state to screen readers. Users won't know they are in a tablist, how many tabs there are, or which one is selected. Also, missing keyboard focus styles makes navigation difficult.
**Action:** Always implement the standard ARIA tablist pattern: `role="tablist"` on the container, `role="tab"` and `aria-selected` on the buttons, and `aria-controls` linking to a container with `role="tabpanel"` and `aria-labelledby`. Ensure tab buttons have a clear `:focus-visible` style for keyboard accessibility.
## 2025-04-03 - [Theming and Keyboard A11y in Popovers]
**Learning:** Hardcoded hex colors (e.g. `#fff`, `#ddd`) in CSS Modules break global dark/light mode switching, notably in dropdowns and tooltips. Popovers like notification dropdowns must be naturally dismissable by both the Escape key and clicking outside to meet accessibility requirements.
**Action:** Always prefer global CSS variables (`var(--bg-card)`, `var(--text-primary)`, `var(--border)`) over hardcoded hex values in `.module.css` files. Implement `aria-controls` with generated IDs and use native `keydown` (`Escape`) and `mousedown` outside click listeners for custom dropdown components.
