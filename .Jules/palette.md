## 2024-04-30 - Added ARIA status to LoadingSpinner
**Learning:** Loading spinners often lack contextual cues for screen readers. While the visual representation (a spinning circle) is clear to sighted users, screen readers need explicit text.
**Action:** Always add `role="status"` and a descriptive `aria-label` to custom loading indicators so that assistive technologies announce the loading state when it appears in the DOM.
