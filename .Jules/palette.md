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

## 2026-04-02 - Dropdown/Popover Dismissability & Linking
**Learning:** Custom dropdown and popover components (like a notification bell menu) often lack native dismissability features expected by users, such as closing when the Escape key is pressed or when clicking outside the component. They also frequently miss explicit linkage to their trigger button, leaving screen reader users without context of what element is controlled by the toggle.
**Action:** Always implement a document-level `keydown` listener for the 'Escape' key and a `mousedown` listener to detect outside clicks within a `useEffect`, ensuring the dropdown state is dismissed. Use `React.useId()` to generate an ID for the dropdown and link it to the trigger button using `aria-controls={id}` when the dropdown is visible.
## 2024-05-19 - Accessible Dismissable Dropdowns
**Learning:** Dropdowns and popovers need proper keyboard dismissability (the ESC key) and click-outside handling to be fully accessible and intuitive. Without these, users can get trapped or struggle to close the UI, and screen readers lack semantic linkage between the trigger button and the popup.
**Action:** Always implement a `keydown` listener for 'Escape' and a `mousedown` listener for outside clicks in a `useEffect`. Additionally, use `React.useId()` to dynamically link the trigger button's `aria-controls` attribute to the popup container's `id`.

## 2024-04-21 - Survey Block Native Button Accessibility Refactor
**Learning:** Replacing manually constructed accessible elements (`<span>` and `<li>` with `role="button"` and `tabIndex`) with native `<button type="button">` elements is superior. Native buttons implicitly handle keyboard events like "Enter" and "Space" reducing boilerplate (e.g. `onKeyDown`) and ensuring correct semantic interactions without extra Javascript overhead.
**Action:** When implementing new clickable options or items in lists/grids, strictly utilize `<button>` tags and apply CSS to style them as block or inline components, instead of adding ARIA button roles to non-interactive elements.
## 2026-05-03 - Accessible Loading Spinners
**Learning:** Purely visual loading indicators (like CSS spinners) do not inherently communicate their state to screen readers. If unaddressed, screen readers either ignore them or announce confusing/empty elements, leaving non-visual users completely unaware that an asynchronous operation is in progress.
**Action:** Always add `role="status"` and a descriptive `aria-label` (e.g., 'Loading...') to the container of the loading spinner. Additionally, add `aria-hidden="true"` to the actual decorative/visual child element to prevent screen readers from reading extraneous visual markup.
