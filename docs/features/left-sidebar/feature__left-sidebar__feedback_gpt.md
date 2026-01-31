# Feedback (GPT) on Left Sidebar Plan

## What I agree with

- **Mode interface / strategy** is the right long-term direction. It keeps `sidebar.go` focused on container logic and lets each mode own its own data and rendering.
- **Async loading** is worth planning for (Local Changes / History can be heavy), but should be introduced only after a synchronous baseline to avoid over‑engineering.
- **Keybinding conflicts** are real (especially `Cmd` on macOS terminals). We should ensure alternate bindings and allow config overrides.

## Risks / concerns

1. **Theme struct nesting is a breaking change**
   Moving `Sidebar*` colors into `[theme.sidebar]` will break existing user configs and tests. If we do it, we should support both flattened and nested fields during a transition (or postpone).

2. **Interface first may slow v1 delivery**
   The editor already uses simple state+functions for overlays (branch picker, refs picker). Introducing a full interface and stack at the start may be heavier than needed for the first iteration. Consider a hybrid: basic `Sidebar` container with a `Mode` enum and only two concrete modes (Menu + Branches), and promote to interface once more modes land.

3. **Existing branch picker conflicts**
   `Cmd+B` is already bound to the branch picker (popup). Switching to sidebar needs a migration decision: replace the old picker entirely or keep it as fallback. Otherwise we risk two competing UIs for the same action.

4. **Sidebar vs refs sidebar**
   There is already a left sidebar used by the refs picker (`renderRefsSidebar`). The plan should define precedence and layout merging to avoid two sidebars or broken offsets.

## Recommendations for the plan

- **Define explicit action model for sidebar events**
  Use a `SidebarAction` enum with data payloads instead of `interface{}` to keep event handling predictable.

- **Add a minimal focus/close policy upfront**
  I agree a `sidebar-close-on-select` option is useful. Default should be `false` for navigation workflows; but `true` might be better for “open once” use cases. Document it.

- **Defer theme nesting**
  Keep flattened `Sidebar*` fields in `Theme` for v1 and add a note for future cleanup. If you want nested config, support both formats rather than breaking.

- **Start with a very small v1 scope**
  Implement **Menu + Branches mode** first (using existing branch list), then migrate FileTree/History/Local Changes later. This ensures the component ships and reduces risk.

- **Handle unavailability states early**
  For modes that depend on git (Branches/Worktrees/Local Changes), set `Available=false` and show a clear status message. This prevents confusing empty panels.

## Open questions to resolve

- Do we fully replace the existing branch picker with the sidebar or keep both? If both exist, which one `Cmd+B` triggers?
- Should sidebar width be persisted (config writeback) or only applied in‑memory for the current session?
- How should sidebar interact with the refs picker overlay? (mutual exclusion vs layered rendering)

---

If you want, I can update the plan to reflect a “v1 minimal scope” and add a migration strategy for the branch picker + theme fields.
