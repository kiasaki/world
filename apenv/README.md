# apenv

Minimal CLI for encrypted per-project environment variables.

## Build

```sh
make build
```

## Usage

Edit a project's variables:

```sh
apenv -e project1
```

This opens a temporary decrypted file in `$EDITOR` using this format:

```env
# comments are allowed
VAR1=value1
VAR2=value2
```

Print a project's decrypted variables:

```sh
apenv -p project1
```

Print a project's variables as shell exports:

```sh
apenv -s project1
source <(apenv -s project1)
```

List projects:

```sh
apenv ls
```

Run a command with that project's variables merged into the environment:

```sh
apenv project1 ./app --port 8080
```

## Storage

- Encrypted project files: `~/.config/apenv/projects/<project>.env.enc`
- Encryption key on macOS with `security`: macOS Keychain item `apenv`
- Otherwise: plaintext base64 key in `~/.apenv`
