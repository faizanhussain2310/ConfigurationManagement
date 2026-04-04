/**
 * Arbiter WASM Loader
 *
 * Loads the Arbiter rule evaluation engine as a WebAssembly module.
 * After loading, the global `arbiter` object exposes:
 *
 *   arbiter.evaluate(treeJSON, contextJSON, ruleID, defaultValueJSON?) → string (JSON EvalResult)
 *   arbiter.validateRule(ruleJSON) → string (error message, empty if valid)
 *
 * Usage:
 *   const engine = await loadArbiter('/path/to/arbiter.wasm')
 *   const result = engine.evaluate(treeJSON, contextJSON, 'my-rule')
 *   const parsed = JSON.parse(result)
 */

async function loadArbiter(wasmPath) {
  if (!WebAssembly) {
    throw new Error('WebAssembly is not supported in this environment')
  }

  const go = new Go()
  const result = await WebAssembly.instantiateStreaming(
    fetch(wasmPath),
    go.importObject
  )
  go.run(result.instance)

  // Wait for arbiter to be registered on globalThis
  let attempts = 0
  while (!globalThis.arbiter && attempts < 100) {
    await new Promise(resolve => setTimeout(resolve, 10))
    attempts++
  }

  if (!globalThis.arbiter) {
    throw new Error('Arbiter WASM module failed to initialize')
  }

  return {
    /**
     * Evaluate a decision tree against a context.
     * @param {object|string} tree - The decision tree (object or JSON string)
     * @param {object} context - The evaluation context
     * @param {string} ruleID - Rule ID (used for percentage rollout hashing)
     * @param {any} [defaultValue] - Optional default value
     * @returns {object} EvalResult { value, path, default, error, elapsed }
     */
    evaluate(tree, context, ruleID, defaultValue) {
      const treeJSON = typeof tree === 'string' ? tree : JSON.stringify(tree)
      const ctxJSON = JSON.stringify(context || {})
      const defJSON = defaultValue !== undefined ? JSON.stringify(defaultValue) : ''
      const resultJSON = globalThis.arbiter.evaluate(treeJSON, ctxJSON, ruleID, defJSON)
      return JSON.parse(resultJSON)
    },

    /**
     * Validate a rule definition.
     * @param {object|string} rule - The rule to validate
     * @returns {string|null} Error message, or null if valid
     */
    validateRule(rule) {
      const ruleJSON = typeof rule === 'string' ? rule : JSON.stringify(rule)
      const err = globalThis.arbiter.validateRule(ruleJSON)
      return err || null
    },
  }
}

// Export for ES modules
if (typeof module !== 'undefined') {
  module.exports = { loadArbiter }
}
