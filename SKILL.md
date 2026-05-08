---
name: openapi-to-skill
description: "Generate an agent skill directory from an OpenAPI spec using a Nix flake one-off. Use for creating SKILL.md command references, CLI wrapper scripts, and per-schema JSON files from any OpenAPI 3.x spec."
---

# openapi-to-skill

Run one-off with:

```
nix run github:alexmickelson/openapi-to-skill -- <output-dir> <openapi-url>
```

| Argument        | Description                                            |
| --------------- | ------------------------------------------------------ |
| `<output-dir>`  | Directory to write the skill into (created if absent)  |
| `<openapi-url>` | `http`/`https` URL to an OpenAPI 3.x JSON or YAML spec |

| Flag          | Description                                                             |
| ------------- | ----------------------------------------------------------------------- |
| `--name NAME` | Override the derived project name (default: kebab-case of `info.title`) |

## Examples

```bash
# Remote spec
nix run github:alexmickelson/openapi-to-skill -- \
  ~/.agents/skills/my-api \
  https://my-api.example.com/openapi.json

# Re-run to pick up latest flake changes (bypasses Nix's commit cache)
nix run github:alexmickelson/openapi-to-skill --refresh -- \
  ~/.agents/skills/my-api \
  https://my-api.example.com/openapi.json

# Override name
nix run github:alexmickelson/openapi-to-skill -- \
  --name my-api \
  ~/.agents/skills/my-api \
  https://my-api.example.com/openapi.json
```

## Output

```
<output-dir>/
├── SKILL.md          # command reference for the agent
├── scripts/
│   └── <name>        # executable CLI wrapping each endpoint
└── schema/
    ├── User.json
    └── ...           # one file per components/schemas entry
```
