# keramos logout

Remove the stored credentials for a registry host.

## When to use it

- When rotating or revoking credentials, or when you no longer need access to
  a host.
- On shared or CI machines, to clear a credential you set with `keramos login`.

## What happens

1. keramos looks up `<host>` in `~/.config/keramos/credentials.json`.
2. If a credential is stored, it is deleted and the file is rewritten; keramos
   prints `Logout succeeded for <host>`.
3. If nothing is stored for that host, keramos prints `Not logged in to <host>`
   and changes nothing.

Only the saved credential is removed. Any repository registration and cached
packages stay in place.

## Usage

```
keramos logout <host> [flags]
```

## Flags

Inherits the global flags.

## Worked example

Log out of a host you previously authenticated to:

```sh
keramos logout registry.example.com
```

```
Logout succeeded for registry.example.com
```

Run it again — nothing is stored now, so keramos says so and exits 0:

```sh
keramos logout registry.example.com
```

```
Not logged in to registry.example.com
```

## See also

- [`login`](login.md) — store a credential for a host
- [`publish`](publish.md)
- [`registry`](registry.md)
