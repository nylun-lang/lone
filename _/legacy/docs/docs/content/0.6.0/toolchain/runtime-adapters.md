---
title: "Runtime Glue & Adapters"
order: 3
---

# Runtime Glue & Adapters

When targeting Web environments (`falone compile web`), the compiled WebAssembly module expects the host environment to provide the `env.print(ptr, len)` import.

Falone's `--with-adapter` flag generates production-ready JavaScript loader glue for either Node.js or browser runtimes.

## Node.js Adapter (`--with-adapter node`)

```bash
falone compile web hello.lone -o hello_web.wat --with-adapter node
```

This generates `hello_web.wat` and `hello_web.adapter.js`.

The generated adapter:
- Provides `createEnv(() => instance.exports.memory, hooks)`
- Reads string buffers from `memory.buffer` using `TextDecoder("utf-8")`
- Streams output to `process.stdout.write`
- Provides `instantiate()` and `run()` helper functions
- Can be executed directly: `node hello_web.adapter.js hello_web.wasm`

```javascript
const { instantiate, run } = require("./hello_web.adapter.js");

async function main() {
  await run("hello_web.wasm");
}
main();
```

## Browser Adapter (`--with-adapter browser`)

```bash
falone compile web hello.lone -o hello_web.wat --with-adapter browser
```

This generates an ES Module (`hello_web.adapter.js`) for web browsers:
- Uses `WebAssembly.instantiateStreaming` with `fetch()`
- Directs string output to `console.log` by default
- Allows custom `hooks.print` overrides (e.g. redirecting output to a web terminal or DOM element)

```javascript
import { run } from "./hello_web.adapter.js";

// Run directly in the browser
run("/assets/hello_web.wasm", {
  hooks: {
    print: (text) => {
      document.getElementById("terminal").textContent += text;
    }
  }
});
```
