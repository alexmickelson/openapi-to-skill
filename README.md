
# openapi-to-skill

A Go program shipped as a Nix flake. Reads an OpenAPI spec and generates a complete agent skill directory: a `SKILL.md` command reference, a generated CLI script, and per-schema JSON files.

---

## Install skill

```bash
mkdir -p ~/.agents/skills/openapi-to-skill
curl -fsSL https://raw.githubusercontent.com/alexmickelson/openapi-to-skill/main/SKILL.md \
  -o ~/.agents/skills/openapi-to-skill/SKILL.md
```

---

## Usage

### One-off with `nix run`

```bash
nix run github:alexmickelson/openapi-to-skill -- \
  ~/.agents/skills/my-api \
  https://my-api.example.com/openapi.json
```

```bash
# Local spec file (file:// URL)
nix run github:alexmickelson/openapi-to-skill -- \
  ~/.agents/skills/petstore \
  file:///home/user/specs/petstore.yaml
```

### Install into your Nix profile

```bash
nix profile install github:alexmickelson/openapi-to-skill
openapi-to-skill ~/.agents/skills/my-api https://my-api.example.com/openapi.json
```

### NixOS / home-manager

```nix
# flake.nix inputs
inputs.openapi-to-skill.url = "github:alexmickelson/openapi-to-skill";

# home.packages or environment.systemPackages
inputs.openapi-to-skill.packages.${system}.default
```

Regenerate skills as part of a `home-manager switch` activation script:

```nix
home.activation.generateSkills = lib.hm.dag.entryAfter ["writeBoundary"] ''
  ${inputs.openapi-to-skill.packages.${system}.default}/bin/openapi-to-skill \
    $HOME/.agents/skills/my-api \
    https://my-api.example.com/openapi.json
'';
```

---

## CLI

```
openapi-to-skill [flags] <output-dir> <openapi-url>

Arguments:
  output-dir    Directory to write the skill into (created if absent)
  openapi-url   http/https/file URL or local path to an OpenAPI 3.x JSON or YAML spec

Flags:
  --name        Override the derived project name (default: kebab-case of info.title)
  --force       Overwrite an existing skill directory
```

This program will receive an output directory and an OpenAPI spec URL as arguments:

```
./generate ~/.agents/skills/project-name http://localhost:3000/openapi.json
```

It generates a complete agent skill directory:

```
~/.agents/skills/project-name/
├── SKILL.md
├── scripts/
│   └── project-name
└── schema/
    ├── User.json
    ├── CreateUserRequest.json
    └── ...
```

---

## `SKILL.md`

Describes the API's purpose and lists available CLI commands with their flags. Links to schema files for type detail. The agent loads this first; schemas are fetched on demand.

````markdown
---
name: project-name
description: 'Manage Project Name resources via CLI. Use for creating, listing, updating, and deleting resources against the project-name API.'
---

# Project Name

CLI: [./scripts/project-name](./scripts/project-name)

| Command        | Flags                                                          |
| -------------- | -------------------------------------------------------------- |
| `users list`   |                                                                |
| `users create` | `--name` `--email` · [schema](./schema/CreateUserRequest.json) |
| `users get`    | `--id`                                                         |
| `users update` | `--id` `--name` `--email`                                      |
| `users delete` | `--id`                                                         |

Set `PROJECT_NAME_TOKEN` for bearer auth.
````

---

## `scripts/project-name`

Generated executable wrapping each API endpoint as a subcommand. Flags map to path/query parameters and request body fields.

```bash
project-name users create --name "Alice" --email "alice@example.com"
```

---

## `schema/*.json`

One file per `components/schemas` entry. Linked from `SKILL.md` so the agent can load exact field shapes on demand.

```json
{
  "type": "object",
  "required": ["name", "email"],
  "properties": {
    "name":  { "type": "string" },
    "email": { "type": "string", "format": "email" }
  }
}
```

`~/.agents/skills/project-name/SKILL.md`