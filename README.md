# terraform-module-scaffold

[![ci](https://github.com/moveeeax/terraform-module-scaffold/actions/workflows/ci.yml/badge.svg)](https://github.com/moveeeax/terraform-module-scaffold/actions/workflows/ci.yml)
[![go](https://img.shields.io/badge/go-1.26%2B-00ADD8)](https://go.dev)
[![license](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

> Bootstrap a production-grade Terraform module in seconds, not an afternoon.

A CLI that generates a best-practice Terraform module skeleton: the four standard
`.tf` files, a runnable example, a `terraform test` suite, a Terratest harness, a
GitHub Actions workflow, `pre-commit` and `tflint` config, a Makefile, and a
README with generated inputs/outputs tables.

Everything it emits is already `terraform fmt`-clean, passes `terraform validate`
and passes its own `terraform test` suite — asserted in CI for every provider the
scaffold knows, not assumed.

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
├── Makefile                  # fmt / validate / lint / test / integration-test / docs
├── .pre-commit-config.yaml   # pre-commit-terraform hooks
├── .tflint.hcl               # terraform preset + provider ruleset
├── .gitignore                # state, lockfiles, tfvars
├── .github/workflows/ci.yml  # fmt, validate, terraform test, examples, tflint
├── examples/basic/           # runnable example wiring every required variable
├── tests/                    # terraform test suite with the provider mocked
└── test/                     # Terratest harness + its own go.mod
```

With no variables specified you get sensible defaults (`name`, `tags`) and matching
outputs, so `new --name vpc --provider aws` alone produces a module that initializes,
validates and passes `terraform test`.

### Provider versions

`versions.tf` is only useful if its floor is one the registry can actually
satisfy — `>= 5.0.0` on a provider whose newest release is 3.x fails at
`terraform init` before you have written a single resource. So the default
constraint is per provider (`aws >= 6.0`, `azurerm >= 4.0`, `kubernetes >= 2.30`,
`null >= 3.0`, …), floored at a major that exists. `provider_version` in the
config overrides it.

For a provider the scaffold does not know, both the source address and the
constraint are guesses; `new` prints a warning telling you to check `versions.tf`.

### Tests

The generated module ships two suites, and they are not interchangeable:

- `tests/*.tftest.hcl` — native `terraform test`, with the provider replaced by
  `mock_provider`. No credentials, no cloud calls, safe on every pull request;
  the generated workflow runs it. Running it needs Terraform >= 1.7 for
  `mock_provider`, which is deliberately *not* reflected in the module's own
  `required_version` — consumers should not be forced up a version for a
  test-only feature.
- `test/` — Terratest. Extend it and it provisions real infrastructure; keep it
  out of pull-request CI.

## Config format

```yaml
name: s3-bucket
provider: aws
description: Opinionated S3 bucket with versioning and tagging.
terraform_version: ">= 1.5.0"   # optional
provider_version: ">= 6.0"      # optional; defaults per provider

variables:
  - name: bucket_name
    type: string
    description: Globally unique name of the bucket.
  - name: versioning
    type: bool
    description: Whether to enable object versioning.
    default: "true"          # a default makes the variable optional

outputs:
  - name: bucket_name
    description: Name of the bucket.
    value: var.bucket_name
```

A variable with no `default` is treated as required: it is marked `yes` in the
README table and wired into `examples/basic/main.tf`.

`default` and `value` are copied into the module as raw HCL, not as strings — so
a string default is written `default: '"us-east-1"'`, and an output `value` must
reference something the module declares. A fresh scaffold has variables and
locals but no resources, so `value: aws_s3_bucket.this.arn` will not validate
until you actually declare that resource. Descriptions are free text and are
escaped for you.

## Development

```bash
make build   # go build
make test    # go test -race ./...
make lint    # gofmt -l . && go vet ./...
```

## License

MIT — see [LICENSE](LICENSE).
