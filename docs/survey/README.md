# Where agent files are on GitHub

`tools/survey/survey.sh` counts public files in each location the spec
(`docs/spec.md`) weighs against the others: `.agents/skills` or
`.claude/skills`, `AGENTS.md` or `CLAUDE.md`, and so on. Run it with your own
`gh` login and save the result here, one file per date:

```sh
tools/survey/survey.sh > docs/survey/$(date +%F).tsv
```

It runs about 25 searches, 7 seconds apart. The counts are GitHub's
estimates, so they say which locations are more common, not how many
projects use them. Only committed files are visible: home-folder locations
(`~/.agents`, `~/.config/agents`) show up only in dotfiles repositories.
