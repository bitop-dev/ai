# Limitations

This list captures known gaps or constraints in the current Go implementation.

- Gateway provider only streams language models. `DoGenerate`, embeddings, and
  images return `UnsupportedFunctionalityError`.
- `StreamObject` does not parse JSON incrementally; call `Collect` to parse and
  validate the final object.
- Registry middleware is available for language and image models only.
- Provider-specific coverage varies by vendor; consult `docs/providers/*.md`
  for model and feature support.
