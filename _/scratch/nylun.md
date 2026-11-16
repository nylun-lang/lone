# Nylun Standart Host Interface

Exposing the WASI interface tie nylun language and wasm runtime too closely, so instead nylun propse it's own standart host interface
mapped under the hood to wasi and custom host calls from the runtime, such as the file system, network, crypto, etc.

## How the interface is structured

The interface is structured as a set of modules, each module is a set of functions, each function is a set of arguments, each argument is a set of values.

The current standart being of:

- io:
  - stdin, stdout, stderr
  - read_line, read_text, read_bytes
  - write_line, write_text, write_bytes
  - flush

- fs:
  - open, close
  - read, write
  - seek, tell
  - stat, fstat, lstat
  - readdir, closedir
  - remove, rmdir, rename

- crypto:
  - rand_bytes
  - sha256
  - sha512
  - md5

- net:
  - tcp_listen, tcp_accept, tcp_connect
  - udp_bind, udp_send, udp_recv

- http:
  - get, post, put, delete, head, options, trace, connect, patch

- process:
  - spawn, wait, exit
  - env, set_env, unset_env
  - args, set_args, unset_args

## Example of usage

```nylun
fn print(msg : str) void require |:io:stdout| {
  stdout.write_line(msg);
}

fn main() {
  print("Hello, world!");
}
```