# terraform-module-scaffold

> Bootstrap a production-grade Terraform module in seconds, not an afternoon.

**Status:** 🚧 In development

## Overview

CLI generator that scaffolds a best-practice Terraform module skeleton with README, examples, tests, pre-commit and tflint config.

## Features

- Generate `main.tf`, `variables.tf`, `outputs.tf`, `versions.tf` with sane defaults
- Create `examples/basic` and a Terratest skeleton under `test/`
- Drop in `.pre-commit-config.yaml`, `.tflint.hcl` and a `Makefile`
- Render a README from a template including a variables/outputs table
- Interactive prompts plus a non-interactive `--config` mode

## Stack

Go 1.22, `cobra` CLI, `text/template` for file generation, embedded template FS.

## Usage

```bash
go install github.com/cybercapybara/terraform-module-scaffold@latest
terraform-module-scaffold new --name vpc --provider aws
```

## License

MIT
