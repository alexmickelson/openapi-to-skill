---
name: openapi-to-skill
description: "Generate an agent skill directory from an OpenAPI spec using a Nix flake one-off. Use for creating SKILL.md command references, CLI wrapper scripts, and per-schema JSON files from any OpenAPI 3.x spec."
---

# openapi-to-skill

Run one-off with:

```
nix run github:alexm/openapi-to-skill -- <output-dir> <openapi-url>
```

| Argument        | Description                                                                 |
| --------------- | --------------------------------------------------------------------------- |
| `<output-dir>`  | Directory to write the skill into (created if absent)                       |
| `<openapi-url>` | `http`/`https`/`file` URL or local path to an OpenAPI 3.x JSON or YAML spec |

| Flag          | Description                                                             |
| ------------- | ----------------------------------------------------------------------- |
| `--name NAME` | Override the derived project name (default: kebab-case of `info.title`) |
| `--force`     | Overwrite an existing skill directory                                   |

## Examples

```bash
# Remote spec
nix run github:alexm/openapi-to-skill -- \
  ~/.agents/skills/my-api \
  https://my-api.example.com/openapi.json

# Local spec
nix run github:alexm/openapi-to-skill -- \
  ~/.agents/skills/petstore \
  file:///home/user/specs/petstore.yaml

# Override name
nix run github:alexm/openapi-to-skill -- \
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
