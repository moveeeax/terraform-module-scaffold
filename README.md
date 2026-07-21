# terraform-module-scaffold

[![ci](https://github.com/moveeeax/terraform-module-scaffold/actions/workflows/ci.yml/badge.svg)](https://github.com/moveeeax/terraform-module-scaffold/actions/workflows/ci.yml)
[![go](https://img.shields.io/badge/go-1.22%2B-00ADD8)](https://go.dev)
[![license](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

> Bootstrap a production-grade Terraform module in seconds, not an afternoon.

A CLI that generates a best-practice Terraform module skeleton: the four standard
`.tf` files, a runnable example, a Terratest harness, `pre-commit` and `tflint`
config, a Makefile, and a README with generated inputs/outputs tables.

Everything it emits is already `terraform fmt`-clean and passes `terraform validate` —
that is asserted in CI, not assumed.

## Install

```bash
go install github.com/moveeeax/terraform-module-scaffold@latest
```

## Usage

Interactive — prompts for anything you leave out:

```bash
terraform-module-scaffold new
```

Non-interactive, straight from flags:

```bash
terraform-module-scaffold new --name vpc --provider aws
```

Fully declarative, from a YAML spec (see [`examples/module.yaml`](examples/module.yaml)):

```bash
terraform-module-scaffold new --config examples/module.yaml -o ./modules/s3-bucket
```

### Flags

| Flag | Description |
|------|-------------|
| `--name` | Module name (lowercase, `a-z0-9_-`) |
| `--provider` | Primary provider: `aws`, `google`, `azurerm`, `kubernetes`, `null`, … |
| `--description` | Module description |
| `-o`, `--output-dir` | Output directory (default: module name) |
| `-c`, `--config` | YAML spec for non-interactive generation |
| `--force` | Overwrite a non-empty destination |
| `--no-input` | Never prompt; use flags/config and defaults only |

## What gets generated

```
vpc/
├── main.tf                   # locals wiring name/tags, TODO markers for resources
├── variables.tf              # one block per input, with type/description/default
├── outputs.tf                # one block per output
├── versions.tf               # required_version + required_providers
├── README.md                 # usage snippet + inputs/outputs tables
├── Makefile                  # fmt / validate / lint / test / docs
├── .pre-commit-config.yaml   # pre-commit-terraform hooks
├── .tflint.hcl               # terraform preset + provider ruleset
├── .gitignore                # state, lockfiles, tfvars
├── examples/basic/           # runnable example wiring every required variable
└── test/                     # Terratest harness + its own go.mod
```

With no variables specified you get sensible defaults (`name`, `tags`) and matching
outputs, so `new --name vpc --provider aws` alone produces a module that initializes
and validates.

## Config format

```yaml
name: s3-bucket
provider: aws
description: Opinionated S3 bucket with versioning and tagging.
terraform_version: ">= 1.5.0"
provider_version: ">= 5.0.0"

variables:
  - name: bucket_name
    type: string
    description: Globally unique name of the bucket.
  - name: versioning
    type: bool
    description: Whether to enable object versioning.
    default: "true"          # a default makes the variable optional

outputs:
  - name: arn
    description: ARN of the bucket.
    value: aws_s3_bucket.this.arn
```

A variable with no `default` is treated as required: it is marked `yes` in the
README table and wired into `examples/basic/main.tf`.

## Development

```bash
make build   # go build
make test    # go test -race ./...
make lint    # gofmt -l . && go vet ./...
```

## License

MIT — see [LICENSE](LICENSE).
