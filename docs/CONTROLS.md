# Choice Controls and Top-Bar Navigation

The application draws simple dropdown choices and input suggestions itself instead of using native `select` or `datalist` elements. Complex controls such as dates and times retain system-provided pickers. Reuse Vue and the existing SVG icons; do not add runtime dependencies.

## Top Bar

The top bar uses `sticky; top: 0` and an opaque background so that it remains at the top of the viewport while scrolling. The root scroll area reserves 100 px of top positioning space, and the desktop photo rail also becomes sticky at 100 px so that navigation does not cover it. Phone viewports use the browser's default safe-area layout without enabling `viewport-fit=cover`; see the [top safe-area note](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/common-issues.md#iphone-主屏幕页面顶部发虚) for cross-application code comparisons and real-device verification boundaries.

The exact UI sections `布料库` and `成品集` always appear at the top of the page on the same row as the application icon and More menu. An ivory slider and green text identify the current module; details, edit, and recycle-bin pages preserve their module's selected state. The More menu contains only the recycle bin and material management. The top bar is the only switch entry for `成品集`.

Desktop displays the complete brand text. Below 760 px, only the brand icon remains, leaving room for the two section entries and the menu. The layout stays on one row down to 320 px, and the icon link retains its complete accessible name.

## Controls

| Usage | Behavior |
| --- | --- |
| Fabric sorting and material, location, color, and tag filters | Single-choice list with a selected marker and highlight; existing URL filter rules remain unchanged |
| Fabric status | Selecting the used-up state still requires confirmation; canceling preserves the original state and dimensions |
| Fabric-piece unit | cm / m, preserving the existing millimeter precision and numeric conversion |
| Finished-work category and sorting filters | Single-choice list using the existing server-side filtering and sorting |
| Storage-location and finished-work category entry | Editable suggestions that match existing choices while still allowing new text |
| Purchase date and completion date | Preserve the native `date` control and existing WebKit width-compatibility styling; leave unknown dates empty |

`ChoiceField` unifies single-choice and input-suggestion behavior. Arrow keys move through candidates, Home / End move to the first or last item, Enter confirms, and Escape closes the panel and returns focus to the original control. `web/src/keyboard.ts` intercepts ordinary Tab / Shift+Tab site-wide so focus remains on the current control, including within filter dialogs and choice panels; it does not intercept Ctrl/Command/Alt combinations or IME composition. Controls have no focus outline, focus shadow, or focus-only color change. Business-state indicators for candidate and selected values remain visible, following the [shared web keyboard and focus conventions](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/conventions.md#网页键盘与焦点). Fixed choices support first-character lookup. Editable suggestions do not forcibly replace custom text, and confirmation keys are ignored during Chinese IME composition. Use `combobox` / `listbox` / `option` semantics and the expanded, selected, and active-option attributes.

`DropdownPanel` uses a manual popover to enter the browser top layer, while the application fully draws its content and styling. The panel remains in the component DOM, so the filter dialog's focus boundary still applies. It opens upward or downward according to available space, constrains its width to the viewport, wraps long options, and scrolls internally. Before opening, it scrolls the control into view to accommodate WebKit's timing when focusing an off-screen input. It repositions when scrolling or when viewport and soft-keyboard dimensions change. Clicking outside, moving focus away, or opening another choice panel closes it. Escape does not propagate to the outer dialog.

## Animation and Limitations

The panel uses an approximately 160–180 ms fade with a 5 px translation; the arrow rotates and the selected check mark scales subtly. The top-bar slider moves smoothly over approximately 220 ms. As soon as an exit animation begins, mark the panel inert and hide it from the accessibility tree so stale options cannot regain focus during the animation. Remove these animations when the system setting `prefers-reduced-motion` is active.

The real-device soft keyboard, system camera, and touchscreen feel still require device verification.
