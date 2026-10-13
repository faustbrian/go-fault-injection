# fault-injection documentation map

[Repository README](../README.md) · [Current Golib portal](https://github.com/faustbrian/go-library-tools/blob/main/docs/ecosystem/README.md) · [Published ecosystem snapshot](https://github.com/faustbrian/go-library-tools/blob/v1.8.4/docs/ecosystem/README.md)

This map links package-owned adoption and operational guidance. Start with
the repository README for purpose, installation, and ownership boundaries.
Published compatibility sets are frozen multi-package baselines; they do not
certify every current main revision or a deployed application.

## Module boundaries

Import only the modules needed by your application. A deprecated facade,
preview adapter, or internal harness is not interchangeable with a stable
root API; follow its own README and migration guidance before composing it.

| Module | Status | Owner detail |
| --- | --- | --- |
| `github.com/faustbrian/go-fault-injection/v2` | stable | [README](../README.md) |
| `github.com/faustbrian/go-fault-injection/benchmarks/comparison` | internal; non-releasable | [README](../benchmarks/comparison/README.md) |
| `github.com/faustbrian/go-fault-injection/integration/resilience` | internal; non-releasable | [README](../integration/resilience/README.md) |

## Choose and adopt

- [Documentation](README.md).
- [Adapter contracts](adapters.md).
- [API reference](api.md).
- [Toxiproxy and infrastructure comparison](comparison.md).
- [Extension guidance](extension.md).
- [FAQ](faq.md).

## Operate and evaluate boundaries

- [Kubernetes semantics](kubernetes.md).
- [Deterministic recipes and operations](operations.md).
- [Safety and runtime controls](safety.md).
- [Threat model](threat-model.md).

## Owned adapters, examples, and harnesses

- [Resilience campaign integration](../integration/resilience/README.md).

## Maintainer, specification, and evidence references

- [Fault-injection comparison benchmarks](../benchmarks/comparison/README.md).
- [Performance](performance.md).
- [Source documentation audit](source-documentation.md).

## Related next steps

[Contributor setup and checks](../CONTRIBUTING.md), [Compatibility policy](../COMPATIBILITY.md), [Release history](../CHANGELOG.md), [Private vulnerability reporting](../SECURITY.md), [Support](../SUPPORT.md).
