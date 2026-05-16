# Editor profile tutor fixtures

These files are local fixtures copied from the official editor runtimes so
`go test` stays deterministic and `:tutor` works from an installed binary
without network access.

- `vim-01-beginner.tutor` and `vim-01-beginner.tutor.json` are from
  `vim/vim` at `runtime/tutor/en/`:
  https://github.com/vim/vim/tree/master/runtime/tutor/en
- `helix-tutor` is from `helix-editor/helix` at `runtime/tutor`:
  https://github.com/helix-editor/helix/blob/master/runtime/tutor
