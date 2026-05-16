# Mode Compatibility Matrix

This document defines what qedit can honestly claim for modal editing support.
Every item marked implemented should have a full-key simulation test that enters
keys through the editor input path instead of calling editing helpers directly.

## Vim

| Area      | Implemented                                                                                                                                                                                                                                           | Not Complete Yet                                               |
|-----------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------|
| Modes     | normal, insert, replace, visual, visual-line, operator-pending, command, window prefix (`Ctrl-w`)                                                                                                                                                     | visual-block parity                                            |
| Counts    | motion counts and operator counts, including `d10j` and `2d3d`                                                                                                                                                                                        | exhaustive count combinations across every operator            |
| Motions   | `h/j/k/l`, `w/b/e`, `W/B/E`, `ge/gE`, sentence/paragraph motions (`(`/`)`, `{`/`}`), `0/^/$`, `gg/G`, `%`, `f/F/t/T`, marks (`m`, `'`, `` ` ``), jumplist (`Ctrl-o`, `Ctrl-i`), file info (`Ctrl-g`)                                                | exhaustive Vim sentence edge cases                             |
| Operators | `d`, `c`, `y`, `>`, `<`, `gu`, `gU`, `g~`, common text objects (`iw/aw`, `ip/ap`, quotes, brackets)                                                                                                                                                   | exhaustive text-object parity                                  |
| Editing   | `x/X`, `s/S`, `r/R`, `D`, `C`, `J`, `o/O`, `a/A`, `i/I`, `p/P`, `u`, `Ctrl-r`, `Ctrl-a`, `Ctrl-x`, `U`, `Y`, `~`, `.` repeat for covered changes, named registers and blackhole register for covered yank/delete/paste flows, macros (`q`, `@`, `@@`) | numbered/delete history registers, exhaustive macro edge cases |
| Search    | `/`, `?`, `n/N`, `:set ic`, `:set noic`, `:set invic`, `:set hls is`, `:nohlsearch`                                                                                                                                                                   | smartcase, search offsets, full Vim regex semantics            |
| Commands  | `:w`, visual `:'<,'>w`, `:q`, `:q!`, `:wq`, `:x`, `:s`, `:%s`, `:r`, `:r !cmd`, `:!cmd`, `:profile`, `:tutor`                                                                                                                                          | full Ex command language                                       |
| Buffers/Windows | `:ls`, `:buffers`, `:files`, `:b`, `:buffer`, `:b#`, `:bn`, `:bnext`, `:bp`, `:bprevious`, `:bd`, `:bd!`, `:bdelete`, `:bdelete!`, `Ctrl-w v/s`, `Ctrl-w w`, shared split commands `:vs`, `:hs`                                                  | buffer-local options, full Vim window command parity           |

## Helix

| Area              | Implemented                                                                                                        | Not Complete Yet                                                           |
|-------------------|--------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------|
| Modes             | normal, insert, command, selection-oriented normal behavior, window mode (`Ctrl-w` / `space w`)                    | full upstream picker command model                                         |
| Counts            | counts for motions and line extension (`2w`, `3e`, `2x`)                                                           | exhaustive count behavior across every command                             |
| Motions           | `w/b/e`, `W/B/E`, `f/F/t/T`, `gh`, `gg/ge/G`, `%`, `;`, jumplist (`Ctrl-s`, `Ctrl-o`, `Ctrl-i`)                    | exhaustive upstream jump integration                                       |
| Selection Editing | `d`, `c`, `y`, `p/P`, `R`, `x`, `>`/`<`, `~`, `` ` ``, `Alt-\``, `Ctrl-a`, `Ctrl-x`, `Ctrl-c`, regex select (`s`), regex split (`S`), line split (`Alt-s`), align (`&`), primary cycling/removal (`(`/`)`, `Alt-,`), cursor duplication (`C`, `Alt-C`), content cycling (`Alt-(`/`Alt-)`), search selection (`*`, `n/N` in select mode), syntax-node expand/shrink (`Alt-o`, `Alt-i`), match-mode pair selection/surrounds (`mi`, `ma`, `ms`, `md`, `mr`) | exhaustive multi-cursor edge cases |
| Menus             | `g`, `m`, `z`, `space`, `space b`, window menu (`Ctrl-w`)                                                          | full Helix picker parity                                                   |
| Buffers/Windows   | `gn`, `gp`, `ga`, `space b`, shared buffer commands through `:`, `Ctrl-w nv/ns`, `Ctrl-w v/s`, `Ctrl-w hjkl`, `Ctrl-w w/q/o`, `Ctrl-w HJKL`, `Ctrl-w t`, `:vs`, `:hs` | picker-open-in-split integration                                           |

## Basic

| Area    | Implemented                                                             | Not Complete Yet                      |
|---------|-------------------------------------------------------------------------|---------------------------------------|
| Editing | non-modal insert-style editing, movement keys, command line via `alt+x` | desktop-editor parity is not the goal |
| Buffers | shared buffer commands through `alt+x` command line                     | dedicated default shortcuts           |

## Claiming Support

Release notes should say "supports the declared Vim/Helix profile matrix" only
when every implemented row above has passing full-key simulation tests. Do not
claim total upstream Vim or Helix compatibility until missing rows are either
implemented or explicitly excluded from the product promise.
