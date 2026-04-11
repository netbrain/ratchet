# Vendored Caveman

Snippets and compression rules vendored from [caveman](https://github.com/JuliusBrussee/caveman) (MIT License, Copyright (c) 2026 Julius Brussee).

Ratchet injects these snippets into agent prompts at debate time based on the `caveman` config in `workflow.yaml`. The debate-runner reads `snippets.md` and selects the intensity-appropriate snippet for each spawned agent.

To update: pull the latest from upstream and overwrite `snippets.md` and `compress-rules.md`.
