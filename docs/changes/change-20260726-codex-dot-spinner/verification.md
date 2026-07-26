---
change: change-20260726-codex-dot-spinner
role: verification
---

<!-- lifecycle is owned by change.md -->

# Verification

```sh
cd clients/ui && npm run lint && npm run test:unit
cd clients/ui && PW_USE_SYSTEM_CHROME=1 npm run test:e2e:real
./clients/windows-shell/scripts/e2e.sh --start-run-dev
python3 <dev-docs>/scripts/docs_cli.py lint --conformance
```

## Result

- PASS: Web lint.
- PASS: focused status tests, 77 tests.
- PASS: complete Web unit suite, 118 files / 1766 tests.
- PASS: real server + real uihost Playwright status scenario.
- PASS: out-of-sandbox Windows E2E — Core 155, Platform 27, WinUI layout,
  launch smoke, and five FlaUI/UIA tests.
- PASS: the DOM contract finds exactly eight running dots and rejects the old
  ring/arc geometry.
